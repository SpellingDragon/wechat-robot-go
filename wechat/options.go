package wechat

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
)

const (
	DefaultBaseURL         = "https://ilinkai.weixin.qq.com"
	DefaultCDNBaseURL      = "https://novac2c.cdn.weixin.qq.com/c2c"
	DefaultTokenFile       = ".weixin-token.json"
	DefaultChannelVersion  = "1.0.3"
	DefaultContextTokenDir = ".wechat-context-tokens"
)

// Option configures a Bot instance.
type Option func(*botConfig)

type botConfig struct {
	baseURL           string
	cdnBaseURL        string
	tokenFile         string
	tokenStore        TokenStore
	contextTokenDir   string
	contextTokenStore ContextTokenStore
	httpClient        *http.Client
	logger            *slog.Logger
	channelVersion    string
}

func defaultConfig() *botConfig {
	return &botConfig{
		baseURL:         DefaultBaseURL,
		cdnBaseURL:      DefaultCDNBaseURL,
		tokenFile:       DefaultTokenFile,
		contextTokenDir: DefaultContextTokenDir,
		httpClient:      &http.Client{},
		logger:          slog.Default(),
		channelVersion:  DefaultChannelVersion,
	}
}

// WithBaseURL sets the API base URL.
func WithBaseURL(url string) Option {
	return func(c *botConfig) { c.baseURL = url }
}

// WithCDNBaseURL sets the CDN base URL for media upload/download.
// Default: https://novac2c.cdn.weixin.qq.com/c2c
func WithCDNBaseURL(url string) Option {
	return func(c *botConfig) { c.cdnBaseURL = url }
}

// WithTokenFile sets the path for token persistence.
func WithTokenFile(path string) Option {
	return func(c *botConfig) { c.tokenFile = path }
}

// WithTokenStore sets a custom token store for login credentials.
// This allows using custom storage implementations (e.g., database, Redis).
// When provided, it takes precedence over WithTokenFile.
func WithTokenStore(store TokenStore) Option {
	return func(c *botConfig) { c.tokenStore = store }
}

// WithContextTokenDir sets the directory path for persisting context tokens.
// Context tokens are required for sending messages and must be persisted
// to support outbound messages even after gateway restarts.
func WithContextTokenDir(dir string) Option {
	return func(c *botConfig) { c.contextTokenDir = dir }
}

// WithContextTokenStore sets a custom context token store.
// This allows using custom storage implementations (e.g., database, Redis).
func WithContextTokenStore(store ContextTokenStore) Option {
	return func(c *botConfig) { c.contextTokenStore = store }
}

// WithHTTPClient sets a custom HTTP client.
// Note: Do not set http.Client.Timeout as it conflicts with long-polling.
// Use context-based timeouts instead. The poller uses per-request context
// timeouts which work correctly with long-polling.
func WithHTTPClient(client *http.Client) Option {
	return func(c *botConfig) {
		if client.Timeout > 0 {
			slog.Warn("wechat: HTTP client has Timeout set, this may interfere with long-polling; consider removing it")
		}
		c.httpClient = client
	}
}

// WithLogger sets the logger to use.
func WithLogger(logger *slog.Logger) Option {
	return func(c *botConfig) { c.logger = logger }
}

// WithLogFile creates a logger that writes to both a file and the default handler.
// The file will contain DEBUG level logs (more verbose), while the console output
// contains INFO level logs. This is useful for production debugging while keeping
// development output clean.
//
// Example usage:
//
//	bot := wechat.NewBot(
//	    wechat.WithLogFile("logs/bot.log"),
//	)
func WithLogFile(filePath string) Option {
	return func(c *botConfig) {
		// Open or create the log file
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			slog.Warn("wechat: failed to open log file, using default logger",
				"error", err, "path", filePath)
			return
		}

		// Create a multi-handler logger that writes to both file and console
		// File: DEBUG level (all logs)
		// Console: INFO level (important logs only)
		c.logger = slog.New(&multiHandler{
			handlers: []slog.Handler{
				slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}),
				slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}),
			},
		})
	}
}

// WithLogWriter creates a logger that writes to the specified io.Writer.
// The level parameter controls the minimum log level (default: Info).
//
// Example usage:
//
//	bot := wechat.NewBot(
//	    wechat.WithLogWriter(os.Stdout, wechat.LogLevelDebug),
//	)
func WithLogWriter(w io.Writer, level slog.Level) Option {
	return func(c *botConfig) {
		c.logger = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
	}
}

// multiHandler implements slog.Handler by delegating to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// WithChannelVersion sets the channel version for API requests.
func WithChannelVersion(version string) Option {
	return func(c *botConfig) { c.channelVersion = version }
}
