package wechat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/SpellingDragon/wechat-robot-go/wechat/internal/media"
)

// ErrNoHandler is returned when Run is called without a registered message handler.
var ErrNoHandler = errors.New("wechat: no message handler registered")

// ErrNoContextToken is returned when trying to send a message without a context token.
var ErrNoContextToken = errors.New("wechat: no context token for user")

// Bot is the top-level abstraction for a WeChat bot.
// It integrates authentication, message polling, typing status, and media handling
// into a simple, easy-to-use API.
type Bot struct {
	config        *botConfig
	client        *Client
	auth          *Auth
	poller        *Poller
	typing        *TypingManager
	media         *MediaManager
	handler       MessageHandler
	middlewares   []Middleware
	contextTokens ContextTokenStore
	mu            sync.Mutex
}

// NewBot creates a new Bot instance with the given options.
func NewBot(opts ...Option) *Bot {
	// 1. Start with default config and apply all options
	cfg := defaultConfig()

	// 2. Load config from file if exists (before applying options, so options can override)
	if fileCfg, err := LoadConfig(); err == nil && fileCfg != nil {
		if fileCfg.BaseURL != "" {
			cfg.baseURL = fileCfg.BaseURL
		}
		if fileCfg.CDNBaseURL != "" {
			cfg.cdnBaseURL = fileCfg.CDNBaseURL
		}
	}

	// 3. Apply all options (can override config file settings)
	for _, opt := range opts {
		opt(cfg)
	}

	// 2. Create Client
	client := NewClient(cfg.baseURL, cfg.httpClient, cfg.logger, cfg.channelVersion)

	// 3. Create Auth with FileTokenStore
	tokenStore := NewFileTokenStore(cfg.tokenFile)
	auth := NewAuth(client, tokenStore, cfg.logger)

	// 4. Create ContextTokenStore for persisting context tokens
	var contextTokens ContextTokenStore
	if cfg.contextTokenStore != nil {
		contextTokens = cfg.contextTokenStore
	} else {
		// Create file-based store for persistence
		dir := cfg.contextTokenDir
		if dir == "" {
			dir = DefaultContextTokenDir
		}
		var err error
		contextTokens, err = NewFileContextTokenStore(dir)
		if err != nil {
			cfg.logger.Warn("failed to create context token store, using memory store",
				"error", err)
			contextTokens = NewMemoryContextTokenStore()
		}
	}

	// 5. Create TypingManager
	typing := NewTypingManager(client, cfg.logger)

	// 6. Create MediaManager and set CDN base URL
	mediaManager := media.NewMediaManager(client, client.HTTPClient(), cfg.logger)
	mediaManager.SetCDNBaseURL(cfg.cdnBaseURL)

	return &Bot{
		config:        cfg,
		client:        client,
		auth:          auth,
		typing:        typing,
		media:         mediaManager,
		contextTokens: contextTokens,
	}
}

// Login performs QR code login.
// If valid credentials exist, they are reused automatically.
// The onQRCode callback is called with the QR code image content for display.
// If onQRCode is nil, the QR code URL is logged.
func (b *Bot) Login(ctx context.Context, onQRCode func(qrCodeImgContent string)) error {
	return b.auth.Login(ctx, onQRCode)
}

// OnMessage registers a handler for incoming user messages.
// Only one handler can be registered; subsequent calls replace the previous handler.
func (b *Bot) OnMessage(handler MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handler = handler
}

// Use registers one or more middlewares.
// Middlewares are applied in the order registered (first registered = outermost).
// Must be called before Run.
func (b *Bot) Use(middlewares ...Middleware) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.middlewares = append(b.middlewares, middlewares...)
}

// Run starts the bot's message polling loop.
// It blocks until ctx is cancelled or an unrecoverable error occurs.
// Must call Login before Run.
// Must call OnMessage before Run (otherwise no messages will be processed).
func (b *Bot) Run(ctx context.Context) error {
	// 1. Check if logged in
	if b.client.Token() == "" {
		return ErrNotLoggedIn
	}

	// 2. Check if handler is registered
	b.mu.Lock()
	handler := b.handler
	b.mu.Unlock()
	if handler == nil {
		return ErrNoHandler
	}

	// 3. Create Poller with wrapped handler
	b.poller = NewPoller(b.client, b.wrapHandler(), b.config.logger, b.config.channelVersion)

	// 4. Run poller
	return b.poller.Run(ctx)
}

// Stop stops the bot's message polling loop.
func (b *Bot) Stop() {
	if b.poller != nil {
		b.poller.Stop()
	}
}

// Reply sends a text reply to an incoming message.
// It automatically uses the message's context_token for conversation linking.
func (b *Bot) Reply(ctx context.Context, msg *Message, text string) error {
	return Reply(ctx, b.client, msg, text)
}

// SendText sends a text message to a user.
// contextToken is required for proper conversation linking.
func (b *Bot) SendText(ctx context.Context, toUserID, text, contextToken string) error {
	return SendText(ctx, b.client, toUserID, text, contextToken)
}

// SendTyping sends a "typing" indicator to the user.
func (b *Bot) SendTyping(ctx context.Context, toUserID string) error {
	return b.typing.SendTyping(ctx, toUserID)
}

// StopTyping cancels the "typing" indicator for the user.
func (b *Bot) StopTyping(ctx context.Context, toUserID string) error {
	return b.typing.StopTyping(ctx, toUserID)
}

// UploadFile encrypts and uploads a file to WeChat CDN.
// toUserID is the recipient's user ID (required for getUploadUrl).
func (b *Bot) UploadFile(ctx context.Context, data []byte, toUserID, fileType string) (*UploadResult, error) {
	return b.media.UploadFile(ctx, data, toUserID, fileType)
}

// DownloadFile downloads and decrypts a media file from WeChat CDN.
func (b *Bot) DownloadFile(ctx context.Context, url string, aesKeyHex string) ([]byte, error) {
	return b.media.DownloadFile(ctx, url, aesKeyHex)
}

// DownloadImage downloads and decrypts an image from a message.
// It constructs the CDN URL from the message's image item and the provided CDN base URL.
func (b *Bot) DownloadImage(ctx context.Context, msg *Message, cdnBaseURL string) ([]byte, error) {
	img := msg.GetImageItem()
	if img == nil {
		return nil, errors.New("message does not contain an image")
	}
	return b.media.DownloadImage(ctx, cdnBaseURL, img)
}

// DownloadImageFromItem downloads and decrypts an image from an ImageItem.
func (b *Bot) DownloadImageFromItem(ctx context.Context, cdnBaseURL string, img *ImageItem) ([]byte, error) {
	return b.media.DownloadImage(ctx, cdnBaseURL, img)
}

// DownloadVoice downloads and decrypts a voice message from a VoiceItem.
func (b *Bot) DownloadVoice(ctx context.Context, voice *VoiceItem, cdnBaseURL string) ([]byte, error) {
	return b.media.DownloadVoice(ctx, cdnBaseURL, voice)
}

// DownloadFileFromItem downloads and decrypts a file from a FileItem.
func (b *Bot) DownloadFileFromItem(ctx context.Context, file *FileItem, cdnBaseURL string) ([]byte, error) {
	return b.media.DownloadFileItem(ctx, cdnBaseURL, file)
}

// DownloadVideoFromItem downloads and decrypts a video from a VideoItem.
func (b *Bot) DownloadVideoFromItem(ctx context.Context, video *VideoItem, cdnBaseURL string) ([]byte, error) {
	return b.media.DownloadVideoItem(ctx, cdnBaseURL, video)
}

// Media returns the MediaManager for this bot.
// The MediaManager is used for uploading and downloading media files.
func (b *Bot) Media() *MediaManager {
	return b.media
}

// SendImageFromPath sends an image file to a user.
// It uploads the file and sends the image message in one step.
func (b *Bot) SendImageFromPath(ctx context.Context, toUserID string, imagePath string) error {
	token, err := b.contextTokens.Load(toUserID)
	if err != nil {
		return fmt.Errorf("load context token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("%w: no context token found for user %s", ErrNoContextToken, toUserID)
	}
	return SendImageFromPath(ctx, b.client, b.media, toUserID, token, imagePath)
}

// SendVoiceFromPath sends a voice file to a user.
func (b *Bot) SendVoiceFromPath(ctx context.Context, toUserID string, voicePath string, duration int) error {
	token, err := b.contextTokens.Load(toUserID)
	if err != nil {
		return fmt.Errorf("load context token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("%w: no context token found for user %s", ErrNoContextToken, toUserID)
	}
	return SendVoiceFromPath(ctx, b.client, b.media, toUserID, token, voicePath, duration)
}

// SendFileFromPath sends a file to a user.
func (b *Bot) SendFileFromPath(ctx context.Context, toUserID string, filePath string) error {
	token, err := b.contextTokens.Load(toUserID)
	if err != nil {
		return fmt.Errorf("load context token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("%w: no context token found for user %s", ErrNoContextToken, toUserID)
	}
	return SendFileFromPath(ctx, b.client, b.media, toUserID, token, filePath)
}

// SendVideoFromPath sends a video file to a user.
func (b *Bot) SendVideoFromPath(ctx context.Context, toUserID string, videoPath string) error {
	token, err := b.contextTokens.Load(toUserID)
	if err != nil {
		return fmt.Errorf("load context token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("%w: no context token found for user %s", ErrNoContextToken, toUserID)
	}
	return SendVideoFromPath(ctx, b.client, b.media, toUserID, token, videoPath)
}

// CDNBaseURL returns the configured CDN base URL.
func (b *Bot) CDNBaseURL() string {
	return b.config.cdnBaseURL
}

// Client returns the underlying API client for advanced usage.
func (b *Bot) Client() *Client {
	return b.client
}

// wrapHandler wraps the user's handler with logging, panic recovery, and context token persistence.
// It also applies any registered middlewares.
func (b *Bot) wrapHandler() MessageHandler {
	return func(ctx context.Context, msg *Message) error {
		// Persist context token for this user conversation
		if msg.ContextToken != "" && msg.FromUserID != "" {
			if err := b.contextTokens.Save(msg.FromUserID, msg.ContextToken); err != nil {
				b.config.logger.Warn("failed to persist context token",
					"user_id", msg.FromUserID,
					"error", err)
			}
		}

		b.mu.Lock()
		handler := b.handler
		middlewares := b.middlewares
		b.mu.Unlock()

		if handler == nil {
			return nil
		}

		// Apply middlewares: Chain(m1, m2, m3)(handler)
		wrapped := handler
		if len(middlewares) > 0 {
			wrapped = Chain(middlewares...)(handler)
		}

		// Call the wrapped handler with panic recovery (built-in safety net)
		func() {
			defer func() {
				if r := recover(); r != nil {
					b.config.logger.Error("handler panic recovered",
						slog.String("from_user_id", msg.FromUserID),
						slog.Any("panic", r),
					)
				}
			}()

			err := wrapped(ctx, msg)
			if err != nil {
				// Log the error and continue (don't interrupt polling)
				b.config.logger.Error("message handler error",
					slog.String("from_user_id", msg.FromUserID),
					slog.String("error", err.Error()),
				)
			}
		}()

		return nil // Always return nil to not interrupt polling
	}
}

// SendTextToUser sends a text message to a user using the persisted context token.
// This method can be used for proactive messaging (not in response to a received message).
// It retrieves the context token from the persistent store.
func (b *Bot) SendTextToUser(ctx context.Context, toUserID, text string) error {
	token, err := b.contextTokens.Load(toUserID)
	if err != nil {
		return fmt.Errorf("load context token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("%w: no context token found for user %s (send a message first or wait for user to message)", ErrNoContextToken, toUserID)
	}
	return SendText(ctx, b.client, toUserID, text, token)
}

// SendImageToUser sends an image message to a user using the persisted context token.
func (b *Bot) SendImageToUser(ctx context.Context, toUserID string, imageItem *ImageItem) error {
	token, err := b.contextTokens.Load(toUserID)
	if err != nil {
		return fmt.Errorf("load context token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("%w: no context token found for user %s", ErrNoContextToken, toUserID)
	}
	return SendImage(ctx, b.client, toUserID, token, imageItem)
}

// SendFileToUser sends a file message to a user using the persisted context token.
func (b *Bot) SendFileToUser(ctx context.Context, toUserID string, fileItem *FileItem) error {
	token, err := b.contextTokens.Load(toUserID)
	if err != nil {
		return fmt.Errorf("load context token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("%w: no context token found for user %s", ErrNoContextToken, toUserID)
	}
	return SendFile(ctx, b.client, toUserID, token, fileItem)
}

// GetContextToken retrieves the persisted context token for a user.
func (b *Bot) GetContextToken(userID string) (string, error) {
	return b.contextTokens.Load(userID)
}

// ClearContextToken removes the persisted context token for a user.
func (b *Bot) ClearContextToken(userID string) error {
	return b.contextTokens.Clear(userID)
}

// ClearAllContextTokens removes all persisted context tokens.
func (b *Bot) ClearAllContextTokens() error {
	return b.contextTokens.ClearAll()
}
