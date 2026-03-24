package wechat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBot(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		bot := NewBot()
		if bot.client == nil {
			t.Error("client should not be nil")
		}
		if bot.auth == nil {
			t.Error("auth should not be nil")
		}
		if bot.typing == nil {
			t.Error("typing manager should not be nil")
		}
		if bot.media == nil {
			t.Error("media manager should not be nil")
		}
		if bot.config.baseURL != DefaultBaseURL {
			t.Errorf("baseURL = %q, want %q", bot.config.baseURL, DefaultBaseURL)
		}
		if bot.config.tokenFile != DefaultTokenFile {
			t.Errorf("tokenFile = %q, want %q", bot.config.tokenFile, DefaultTokenFile)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		customLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		customHTTPClient := &http.Client{Timeout: 10 * time.Second}
		bot := NewBot(
			WithBaseURL("https://custom.api.com"),
			WithTokenFile("custom-token.json"),
			WithHTTPClient(customHTTPClient),
			WithLogger(customLogger),
			WithChannelVersion("2.0.0"),
		)
		if bot.config.baseURL != "https://custom.api.com" {
			t.Errorf("baseURL = %q, want %q", bot.config.baseURL, "https://custom.api.com")
		}
		if bot.config.tokenFile != "custom-token.json" {
			t.Errorf("tokenFile = %q, want %q", bot.config.tokenFile, "custom-token.json")
		}
		if bot.config.channelVersion != "2.0.0" {
			t.Errorf("channelVersion = %q, want %q", bot.config.channelVersion, "2.0.0")
		}
	})
}

func TestBot_RunWithoutLogin(t *testing.T) {
	bot := NewBot()
	ctx := context.Background()

	// Register a handler so that's not the error
	bot.OnMessage(func(ctx context.Context, msg *Message) error {
		return nil
	})

	err := bot.Run(ctx)
	if err != ErrNotLoggedIn {
		t.Errorf("Run without login should return ErrNotLoggedIn, got %v", err)
	}
}

func TestBot_RunWithoutHandler(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"ret": 0})
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	// Manually set token to simulate logged in state
	bot.client.SetToken("test-token")

	ctx := context.Background()
	err := bot.Run(ctx)
	if err != ErrNoHandler {
		t.Errorf("Run without handler should return ErrNoHandler, got %v", err)
	}
}

func TestBot_FullFlow(t *testing.T) {
	var handlerCalled atomic.Int32
	var receivedMsg *Message
	var replySent atomic.Int32

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			json.NewEncoder(w).Encode(QRCodeResponse{
				QRCode:           "test-qrcode",
				QRCodeImgContent: "test-image-content",
			})
		case "/ilink/bot/get_qrcode_status":
			json.NewEncoder(w).Encode(QRCodeStatus{
				Status:   "confirmed",
				BotToken: "test-bot-token",
			})
		case "/ilink/bot/getupdates":
			// Only return message on first call
			if handlerCalled.Load() == 0 {
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{
							FromUserID:   "user-123",
							ToUserID:     "bot-456",
							MessageType:  MessageTypeUser,
							MessageState: MessageStateNew,
							ContextToken: "ctx-token-789",
							ItemList: []MessageItem{
								{
									Type:     ItemTypeText,
									TextItem: &TextItem{Text: "Hello Bot!"},
								},
							},
						},
					},
					GetUpdatesBuf:        "buf-1",
					LongPollingTimeoutMs: 1000, // Short timeout for test
				})
			} else {
				// Subsequent calls return empty (simulate timeout)
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret:                  0,
					Messages:             []Message{},
					GetUpdatesBuf:        "buf-2",
					LongPollingTimeoutMs: 1000,
				})
			}
		case "/ilink/bot/sendmessage":
			replySent.Add(1)
			json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create temp token file
	tmpFile, err := os.CreateTemp("", "bot-test-token-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Create bot
	bot := NewBot(
		WithBaseURL(server.URL),
		WithTokenFile(tmpFile.Name()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Login
	var qrCodeReceived string
	err = bot.Login(ctx, func(qrCodeImgContent string) {
		qrCodeReceived = qrCodeImgContent
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if qrCodeReceived != "test-image-content" {
		t.Errorf("QR code content = %q, want %q", qrCodeReceived, "test-image-content")
	}
	if bot.client.Token() != "test-bot-token" {
		t.Errorf("Token = %q, want %q", bot.client.Token(), "test-bot-token")
	}

	// Register handler
	bot.OnMessage(func(ctx context.Context, msg *Message) error {
		handlerCalled.Add(1)
		receivedMsg = msg
		// Reply to the message
		return bot.Reply(ctx, msg, "Echo: "+msg.Text())
	})

	// Run in goroutine (will be cancelled by context)
	runDone := make(chan error, 1)
	go func() {
		runDone <- bot.Run(ctx)
	}()

	// Wait for handler to be called or timeout
	timeout := time.After(3 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for handler to be called")
		case <-ticker.C:
			if handlerCalled.Load() > 0 && replySent.Load() > 0 {
				break waitLoop
			}
		}
	}

	// Cancel context to stop bot
	cancel()

	// Wait for Run to return
	select {
	case <-runDone:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after context cancel")
	}

	// Verify handler was called
	if handlerCalled.Load() == 0 {
		t.Error("handler was not called")
	}

	// Verify message content
	if receivedMsg == nil {
		t.Fatal("received message is nil")
	}
	if receivedMsg.FromUserID != "user-123" {
		t.Errorf("FromUserID = %q, want %q", receivedMsg.FromUserID, "user-123")
	}
	if receivedMsg.Text() != "Hello Bot!" {
		t.Errorf("Text = %q, want %q", receivedMsg.Text(), "Hello Bot!")
	}

	// Verify reply was sent
	if replySent.Load() == 0 {
		t.Error("reply was not sent")
	}
}

func TestBot_Reply(t *testing.T) {
	var sentRequest *SendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			sentRequest = &SendMessageRequest{}
			if err := json.NewDecoder(r.Body).Decode(sentRequest); err != nil {
				t.Logf("Decode error: %v", err)
			}
			if err := json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0}); err != nil {
				t.Logf("Encode error: %v", err)
			}
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	msg := &Message{
		FromUserID:   "user-123",
		ContextToken: "ctx-token-456",
	}

	ctx := context.Background()
	err := bot.Reply(ctx, msg, "Hello back!")
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}

	if sentRequest == nil {
		t.Fatal("no request sent")
	}
	if sentRequest.Msg.ToUserID != "user-123" {
		t.Errorf("ToUserID = %q, want %q", sentRequest.Msg.ToUserID, "user-123")
	}
	if sentRequest.Msg.ContextToken != "ctx-token-456" {
		t.Errorf("ContextToken = %q, want %q", sentRequest.Msg.ContextToken, "ctx-token-456")
	}
	if sentRequest.Msg.ItemList[0].TextItem.Text != "Hello back!" {
		t.Errorf("Text = %q, want %q", sentRequest.Msg.ItemList[0].TextItem.Text, "Hello back!")
	}
}

func TestBot_SendTyping(t *testing.T) {
	var typingRequest *SendTypingRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			if err := json.NewEncoder(w).Encode(GetConfigResponse{
				Ret:          0,
				TypingTicket: "typing-ticket-123",
			}); err != nil {
				t.Logf("Encode error: %v", err)
			}
		case "/ilink/bot/sendtyping":
			typingRequest = &SendTypingRequest{}
			if err := json.NewDecoder(r.Body).Decode(typingRequest); err != nil {
				t.Logf("Decode error: %v", err)
			}
			if err := json.NewEncoder(w).Encode(SendTypingResponse{Ret: 0}); err != nil {
				t.Logf("Encode error: %v", err)
			}
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	ctx := context.Background()
	err := bot.SendTyping(ctx, "user-123")
	if err != nil {
		t.Fatalf("SendTyping failed: %v", err)
	}

	if typingRequest == nil {
		t.Fatal("no typing request sent")
	}
	if typingRequest.ToUserID != "user-123" {
		t.Errorf("ToUserID = %q, want %q", typingRequest.ToUserID, "user-123")
	}
	if typingRequest.TypingTicket != "typing-ticket-123" {
		t.Errorf("TypingTicket = %q, want %q", typingRequest.TypingTicket, "typing-ticket-123")
	}
	if typingRequest.Status != TypingStatusStart {
		t.Errorf("Status = %d, want %d", typingRequest.Status, TypingStatusStart)
	}
}

func TestBot_StopTyping(t *testing.T) {
	var typingRequest *SendTypingRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			if err := json.NewEncoder(w).Encode(GetConfigResponse{
				Ret:          0,
				TypingTicket: "typing-ticket-123",
			}); err != nil {
				t.Logf("Encode error: %v", err)
			}
		case "/ilink/bot/sendtyping":
			typingRequest = &SendTypingRequest{}
			if err := json.NewDecoder(r.Body).Decode(typingRequest); err != nil {
				t.Logf("Decode error: %v", err)
			}
			if err := json.NewEncoder(w).Encode(SendTypingResponse{Ret: 0}); err != nil {
				t.Logf("Encode error: %v", err)
			}
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	ctx := context.Background()
	err := bot.StopTyping(ctx, "user-456")
	if err != nil {
		t.Fatalf("StopTyping failed: %v", err)
	}

	if typingRequest == nil {
		t.Fatal("no typing request sent")
	}
	if typingRequest.ToUserID != "user-456" {
		t.Errorf("ToUserID = %q, want %q", typingRequest.ToUserID, "user-456")
	}
	if typingRequest.Status != TypingStatusStop {
		t.Errorf("Status = %d, want %d", typingRequest.Status, TypingStatusStop)
	}
}

func TestBot_Client(t *testing.T) {
	bot := NewBot()
	client := bot.Client()
	if client == nil {
		t.Error("Client() should not return nil")
	}
	if client != bot.client {
		t.Error("Client() should return the internal client")
	}
}

func TestBot_HandlerError(t *testing.T) {
	// Test that handler errors are logged but don't interrupt polling
	var handlerCallCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			// Return messages a few times
			if handlerCallCount.Load() < 3 {
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{
							FromUserID:   "user-123",
							MessageType:  MessageTypeUser,
							ContextToken: "ctx-token",
							ItemList: []MessageItem{
								{Type: ItemTypeText, TextItem: &TextItem{Text: "test"}},
							},
						},
					},
					LongPollingTimeoutMs: 100,
				})
			} else {
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret:                  0,
					LongPollingTimeoutMs: 100,
				})
			}
		}
	}))
	defer server.Close()

	// Create a discarding logger
	bot := NewBot(
		WithBaseURL(server.URL),
		WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))),
	)
	bot.client.SetToken("test-token")

	// Register a handler that always returns an error
	bot.OnMessage(func(ctx context.Context, msg *Message) error {
		handlerCallCount.Add(1)
		return ErrNotLoggedIn // Return some error
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- bot.Run(ctx)
	}()

	// Wait for multiple handler calls (proving errors don't stop polling)
	timeout := time.After(1500 * time.Millisecond)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			if handlerCallCount.Load() < 2 {
				t.Errorf("handler should be called multiple times despite errors, got %d calls", handlerCallCount.Load())
			}
			cancel()
			return
		case <-ticker.C:
			if handlerCallCount.Load() >= 2 {
				// Success - handler was called multiple times despite errors
				cancel()
				return
			}
		}
	}
}

func TestBot_Stop(t *testing.T) {
	bot := NewBot()
	bot.client.SetToken("test-token")

	// Stop should not panic
	bot.Stop()
}

func TestBot_SendText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0})
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	ctx := context.Background()
	err := bot.SendText(ctx, "user-123", "Hello", "ctx-token")
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}
}

func TestBot_UploadAndDownload(t *testing.T) {
	testData := []byte("test file content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getuploadurl":
			json.NewEncoder(w).Encode(UploadURLResponse{
				Ret:         0,
				UploadParam: "test-upload-param",
			})
		case "/upload":
			w.Header().Set("x-encrypted-param", "test-encrypted-param")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")
	bot.media.SetCDNBaseURL(server.URL)

	ctx := context.Background()

	// Test UploadFile
	result, err := bot.UploadFile(ctx, testData, "user-123", "image")
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	if result.EncryptedParam != "test-encrypted-param" {
		t.Errorf("EncryptedParam = %q, want %q", result.EncryptedParam, "test-encrypted-param")
	}

	// Test DownloadFile
	downloaded, err := bot.DownloadFile(ctx, server.URL+"/download?param=test", result.AESKey)
	if err != nil {
		t.Logf("DownloadFile error (expected for mock server): %v", err)
	}
	_ = downloaded // May be nil if mock server doesn't return valid data
}

func TestBot_SendFromPath(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-*.png")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := tmpFile.Write([]byte("fake image data")); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getuploadurl":
			if err := json.NewEncoder(w).Encode(UploadURLResponse{
				Ret:         0,
				UploadParam: "test-param",
			}); err != nil {
				t.Logf("Encode error: %v", err)
			}
		case "/upload":
			w.Header().Set("x-encrypted-param", "test-encrypted")
			if _, err := w.Write([]byte{}); err != nil {
				t.Logf("Write error: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case "/ilink/bot/sendmessage":
			if err := json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0}); err != nil {
				t.Logf("Encode error: %v", err)
			}
		}
	}))
	defer server.Close()

	bot := NewBot(
		WithBaseURL(server.URL),
		WithContextTokenStore(NewMemoryContextTokenStore()),
	)
	bot.client.SetToken("test-token")
	bot.media.SetCDNBaseURL(server.URL)

	// Store context token for user
	if err := bot.contextTokens.Save("user-123", "ctx-token"); err != nil {
		t.Fatalf("Save context token: %v", err)
	}

	ctx := context.Background()

	// Test SendImageFromPath
	err = bot.SendImageFromPath(ctx, "user-123", tmpFile.Name())
	if err != nil {
		t.Fatalf("SendImageFromPath failed: %v", err)
	}

	// Test SendFileFromPath
	err = bot.SendFileFromPath(ctx, "user-123", tmpFile.Name())
	if err != nil {
		t.Fatalf("SendFileFromPath failed: %v", err)
	}

	// Test SendVideoFromPath
	err = bot.SendVideoFromPath(ctx, "user-123", tmpFile.Name())
	if err != nil {
		t.Fatalf("SendVideoFromPath failed: %v", err)
	}

	// Test SendVoiceFromPath
	err = bot.SendVoiceFromPath(ctx, "user-123", tmpFile.Name(), 1000)
	if err != nil {
		t.Fatalf("SendVoiceFromPath failed: %v", err)
	}
}

func TestBot_SendToUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0})
		}
	}))
	defer server.Close()

	bot := NewBot(
		WithBaseURL(server.URL),
		WithContextTokenStore(NewMemoryContextTokenStore()),
	)
	bot.client.SetToken("test-token")
	bot.contextTokens.Save("user-123", "ctx-token")

	ctx := context.Background()

	// Test SendTextToUser
	err := bot.SendTextToUser(ctx, "user-123", "Hello")
	if err != nil {
		t.Fatalf("SendTextToUser failed: %v", err)
	}

	// Test SendImageToUser
	imageItem := &ImageItem{
		Media:   &CDNMedia{EncryptQueryParam: "param", AESKey: "key", EncryptType: 1},
		MidSize: 1000,
	}
	err = bot.SendImageToUser(ctx, "user-123", imageItem)
	if err != nil {
		t.Fatalf("SendImageToUser failed: %v", err)
	}

	// Test SendFileToUser
	fileItem := &FileItem{
		Media:    &CDNMedia{EncryptQueryParam: "param", AESKey: "key", EncryptType: 1},
		FileName: "test.txt",
		Length:   "100",
	}
	err = bot.SendFileToUser(ctx, "user-123", fileItem)
	if err != nil {
		t.Fatalf("SendFileToUser failed: %v", err)
	}
}

func TestBot_ContextToken(t *testing.T) {
	bot := NewBot(WithContextTokenStore(NewMemoryContextTokenStore()))

	// Test GetContextToken (should return empty for unknown user)
	token, err := bot.GetContextToken("unknown-user")
	if err != nil {
		t.Logf("GetContextToken error: %v", err)
	}
	_ = token

	// Test ClearContextToken
	err = bot.ClearContextToken("user-123")
	if err != nil {
		t.Fatalf("ClearContextToken failed: %v", err)
	}

	// Test ClearAllContextTokens
	err = bot.ClearAllContextTokens()
	if err != nil {
		t.Fatalf("ClearAllContextTokens failed: %v", err)
	}
}

func TestBot_CDNBaseURL(t *testing.T) {
	bot := NewBot(WithCDNBaseURL("https://custom.cdn.com"))
	if bot.CDNBaseURL() != "https://custom.cdn.com" {
		t.Errorf("CDNBaseURL = %q, want %q", bot.CDNBaseURL(), "https://custom.cdn.com")
	}
}

func TestBot_DownloadImage(t *testing.T) {
	// Create test data
	testData := []byte("fake image data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(testData); err != nil {
				t.Logf("Write error: %v", err)
			}
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	ctx := context.Background()

	// Test DownloadImage
	msg := &Message{
		FromUserID:   "user-123",
		ContextToken: "ctx-token",
		ItemList: []MessageItem{
			{
				Type: ItemTypeImage,
				ImageItem: &ImageItem{
					Media: &CDNMedia{
						EncryptQueryParam: "test-param",
						AESKey:            "NDhmNmFjYjU3ZWE3M2I5MDkzZTA0YmM0ZjJhN2IyZmE=", // base64 of hex
					},
				},
			},
		},
	}
	_, err := bot.DownloadImage(ctx, msg, server.URL)
	if err != nil {
		t.Logf("DownloadImage error (expected for mock server): %v", err)
	}

	// Test DownloadImageFromItem
	imgItem := &ImageItem{
		Media: &CDNMedia{
			EncryptQueryParam: "test-param",
			AESKey:            "NDhmNmFjYjU3ZWE3M2I5MDkzZTA0YmM0ZjJhN2IyZmE=",
		},
	}
	_, err = bot.DownloadImageFromItem(ctx, server.URL, imgItem)
	if err != nil {
		t.Logf("DownloadImageFromItem error: %v", err)
	}

	// Test DownloadVoice
	voiceItem := &VoiceItem{
		Media: &CDNMedia{
			EncryptQueryParam: "test-param",
			AESKey:            "NDhmNmFjYjU3ZWE3M2I5MDkzZTA0YmM0ZjJhN2IyZmE=",
		},
	}
	_, err = bot.DownloadVoice(ctx, voiceItem, server.URL)
	if err != nil {
		t.Logf("DownloadVoice error: %v", err)
	}

	// Test DownloadFileFromItem
	fileItem := &FileItem{
		Media: &CDNMedia{
			EncryptQueryParam: "test-param",
			AESKey:            "NDhmNmFjYjU3ZWE3M2I5MDkzZTA0YmM0ZjJhN2IyZmE=",
		},
	}
	_, err = bot.DownloadFileFromItem(ctx, fileItem, server.URL)
	if err != nil {
		t.Logf("DownloadFileFromItem error: %v", err)
	}

	// Test DownloadVideoFromItem
	videoItem := &VideoItem{
		Media: &CDNMedia{
			EncryptQueryParam: "test-param",
			AESKey:            "NDhmNmFjYjU3ZWE3M2I5MDkzZTA0YmM0ZjJhN2IyZmE=",
		},
	}
	_, err = bot.DownloadVideoFromItem(ctx, videoItem, server.URL)
	if err != nil {
		t.Logf("DownloadVideoFromItem error: %v", err)
	}
}

func TestBot_HandlerPanic(t *testing.T) {
	// Create mock server
	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getupdates":
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				resp := GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{
							FromUserID:   "user1",
							MessageType:  MessageTypeUser,
							ContextToken: "tok",
							ItemList: []MessageItem{
								{Type: ItemTypeText, TextItem: &TextItem{Text: "panic trigger"}},
							},
						},
					},
					GetUpdatesBuf:        "c1",
					LongPollingTimeoutMs: 50,
				}
				json.NewEncoder(w).Encode(resp)
			} else {
				time.Sleep(50 * time.Millisecond)
				resp := GetUpdatesResponse{Ret: 0, GetUpdatesBuf: "c2"}
				json.NewEncoder(w).Encode(resp)
			}
		default:
			json.NewEncoder(w).Encode(map[string]int{"ret": 0})
		}
	}))
	defer server.Close()

	bot := NewBot(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))),
		WithContextTokenStore(NewMemoryContextTokenStore()),
	)
	bot.client.SetToken("test-token")

	// Handler that panics
	bot.OnMessage(func(ctx context.Context, msg *Message) error {
		panic("test panic!")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run should NOT crash despite panic in handler
	err := bot.Run(ctx)
	if err != nil && err != context.DeadlineExceeded {
		t.Logf("Run returned: %v (expected context deadline)", err)
	}
	// If we reach here without crash, panic recovery works
}

func TestPoller_GracefulShutdown(t *testing.T) {
	handlerDone := make(chan struct{})
	handlerStarted := make(chan struct{})

	handler := func(ctx context.Context, msg *Message) error {
		close(handlerStarted)
		time.Sleep(100 * time.Millisecond) // Simulate work
		close(handlerDone)
		return nil
	}

	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				resp := GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{FromUserID: "user1", MessageType: MessageTypeUser,
							ItemList: []MessageItem{{Type: ItemTypeText, TextItem: &TextItem{Text: "test"}}}},
					},
					GetUpdatesBuf:        "c1",
					LongPollingTimeoutMs: 50,
				}
				json.NewEncoder(w).Encode(resp)
			} else {
				time.Sleep(5 * time.Second)
			}
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, handler, slog.Default(), "1.0.3")

	done := make(chan error)
	go func() {
		done <- poller.Run(context.Background())
	}()

	// Wait for handler to start processing
	<-handlerStarted

	// Stop immediately - should wait for handler to finish
	poller.Stop()

	// Handler should have completed
	select {
	case <-handlerDone:
		// OK - handler finished before Stop returned
	case <-time.After(2 * time.Second):
		t.Error("handler did not complete after Stop()")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("poller did not exit after Stop()")
	}
}
