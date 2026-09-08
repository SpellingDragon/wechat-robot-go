package wechat

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Poller manages the long-polling loop for receiving messages.
type Poller struct {
	client         *Client
	handler        MessageHandler
	getUpdatesBuf  string // cursor for getupdates, initially ""
	logger         *slog.Logger
	channelVersion string
	stopCh         chan struct{}
	stopCancel     context.CancelFunc
	cursorStore    CursorStore
	mu             sync.RWMutex
	wg             sync.WaitGroup // tracks in-flight handler goroutines
}

// NewPoller creates a new Poller instance (no cursor persistence).
func NewPoller(client *Client, handler MessageHandler, logger *slog.Logger, channelVersion string) *Poller {
	return NewPollerWithCursorStore(client, handler, logger, channelVersion, nil)
}

// NewPollerWithCursorStore creates a new Poller that persists its getupdates
// cursor via the given store. If store is nil the poller behaves exactly like
// NewPoller (in-memory cursor only). On construction a persisted cursor is
// loaded; a load failure is logged and polling starts with an empty cursor.
func NewPollerWithCursorStore(client *Client, handler MessageHandler, logger *slog.Logger, channelVersion string, store CursorStore) *Poller {
	p := &Poller{
		client:         client,
		handler:        handler,
		getUpdatesBuf:  "",
		logger:         logger,
		channelVersion: channelVersion,
		stopCh:         make(chan struct{}),
		cursorStore:    store,
	}
	if store != nil {
		if cur, err := store.Get(); err != nil {
			logger.Warn("failed to load persisted cursor, starting empty", "error", err)
		} else if cur != "" {
			p.getUpdatesBuf = cur
			logger.Info("restored getupdates cursor from store", "cursor_len", len(cur))
		}
	}
	return p
}

// Run starts the long-polling loop. Blocks until ctx is cancelled or an unrecoverable error occurs.
func (p *Poller) Run(ctx context.Context) error {
	const (
		defaultTimeoutMs     = 35000 // server holds connection up to 35 seconds
		httpTimeoutPaddingMs = 10000 // add 10s padding to HTTP timeout
		minHTTPTimeoutMs     = 20000 // minimum HTTP timeout
		maxConsecutiveFails  = 3
		backoffDelay         = 30 * time.Second
	)

	// Create an internal context that can be cancelled by Stop()
	internalCtx, internalCancel := context.WithCancel(ctx)
	p.mu.Lock()
	p.stopCancel = internalCancel
	p.mu.Unlock()
	defer internalCancel()

	consecutiveFails := 0
	httpTimeout := time.Duration(defaultTimeoutMs+httpTimeoutPaddingMs) * time.Millisecond

	for {
		select {
		case <-internalCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return ErrPollerStopped
		case <-p.stopCh:
			return ErrPollerStopped
		default:
		}

		p.logger.Debug("polling for messages", "cursor_len", len(p.getUpdatesBuf))

		// Create a context with timeout for this poll
		pollCtx, cancel := context.WithTimeout(internalCtx, httpTimeout)

		resp, err := p.poll(pollCtx)
		cancel()

		if err != nil {
			// Check if context was cancelled (normal shutdown)
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Check for session expired
			if IsSessionExpired(err) {
				p.logger.Error("session expired", "error", err)
				return ErrSessionExpired
			}

			// Distinguish timeout (normal for long-polling) from real errors
			var netErr net.Error
			isTimeout := errors.Is(err, context.DeadlineExceeded) ||
				(errors.As(err, &netErr) && netErr.Timeout())

			if isTimeout {
				p.logger.Debug("poll timeout, reconnecting")
				consecutiveFails = 0 // Reset! This is normal behavior
				continue
			}

			// Real error - check if it was stopped via Stop()
			if errors.Is(err, context.Canceled) {
				return ErrPollerStopped
			}

			consecutiveFails++
			p.logger.Warn("poll error",
				"error", err,
				"consecutive_fails", consecutiveFails,
			)

			// After 3 consecutive failures, backoff for 30 seconds
			if consecutiveFails >= maxConsecutiveFails {
				p.logger.Info("backing off after consecutive failures", "delay", backoffDelay)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-p.stopCh:
					return ErrPollerStopped
				case <-time.After(backoffDelay):
				}
				consecutiveFails = 0
			}
			continue
		}

		// Reset consecutive fails on success
		consecutiveFails = 0

		p.logger.Debug("poll response",
			"message_count", len(resp.Messages),
			"timeout_ms", resp.LongPollingTimeoutMs,
		)

		// Update HTTP timeout based on server's longpolling_timeout_ms (with minimum protection)
		if resp.LongPollingTimeoutMs > 0 {
			newTimeout := time.Duration(resp.LongPollingTimeoutMs+httpTimeoutPaddingMs) * time.Millisecond
			if newTimeout < time.Duration(minHTTPTimeoutMs)*time.Millisecond {
				newTimeout = time.Duration(minHTTPTimeoutMs) * time.Millisecond
			}
			httpTimeout = newTimeout
		}

		// Process messages - only handle user messages (message_type == 1)
		// Use internalCtx so that message processing isn't interrupted by external ctx cancellation
		// Each message is processed in a goroutine to allow concurrent message processing.
		// This is important for scenarios where the handler blocks (e.g., waiting for agent response)
		// so subsequent messages can still be processed.
		processedCount := 0
		failedCount := 0
		var msgMu sync.Mutex // protects processedCount and failedCount
		for i := range resp.Messages {
			msg := &resp.Messages[i]
			if msg.MessageType != MessageTypeUser {
				continue
			}

			p.wg.Add(1)
			go func(msg *Message) {
				defer p.wg.Done()
				if err := p.handler(internalCtx, msg); err != nil {
					p.logger.Error("handler error",
						"error", err,
						"from_user_id", msg.FromUserID,
					)
					msgMu.Lock()
					failedCount++
					msgMu.Unlock()
				} else {
					msgMu.Lock()
					processedCount++
					msgMu.Unlock()
				}
			}(msg)
		}

		// Wait for all messages in this batch to complete before updating cursor
		p.wg.Wait()

		p.logger.Debug("messages processed",
			"processed", processedCount,
			"failed", failedCount,
			"total", len(resp.Messages),
		)

		// Update cursor AFTER processing all messages
		// Note: Even if some messages failed, we update the cursor to avoid
		// getting the same messages again. Failed messages should be handled
		// by the user's handler (e.g., store in DB for retry).
		if resp.GetUpdatesBuf != "" {
			p.getUpdatesBuf = resp.GetUpdatesBuf
			if p.cursorStore != nil {
				if err := p.cursorStore.Save(p.getUpdatesBuf); err != nil {
					// Persist failure must not break polling; worst case is a
					// replayed window after restart (same as pre-store behavior).
					p.logger.Warn("failed to persist getupdates cursor", "error", err)
				}
			}
		}
	}
}

// poll performs a single long-poll request.
func (p *Poller) poll(ctx context.Context) (*GetUpdatesResponse, error) {
	req := &GetUpdatesRequest{
		GetUpdatesBuf: p.getUpdatesBuf,
		BaseInfo: &BaseInfo{
			ChannelVersion: p.channelVersion,
		},
	}

	var resp GetUpdatesResponse
	if err := p.client.Post(ctx, "/ilink/bot/getupdates", req, &resp); err != nil {
		return nil, err
	}

	// Check for API errors in response
	// ret=0 is success (including empty messages which is normal timeout)
	if resp.Ret != 0 {
		return nil, &APIError{Code: resp.Ret, Message: "getupdates failed"}
	}

	return &resp, nil
}

// Stop signals the poller to stop and waits for in-flight handlers to complete.
func (p *Poller) Stop() {
	select {
	case <-p.stopCh:
		// Already stopped
	default:
		close(p.stopCh)
		// Cancel any ongoing HTTP request
		p.mu.RLock()
		cancel := p.stopCancel
		p.mu.RUnlock()
		if cancel != nil {
			cancel()
		}
	}
	// Wait for all in-flight handlers to finish
	p.wg.Wait()
}

// PollerWithTimeout creates a Client with custom HTTP timeout for long-polling.
// This is a helper to create a properly configured HTTP client.
func PollerWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}
