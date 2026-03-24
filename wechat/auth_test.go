package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileTokenStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "token.json")
	store := NewFileTokenStore(path)

	creds := &Credentials{
		BotToken: "test-token-123",
		BaseURL:  "https://custom.example.com",
	}

	// Save credentials
	err := store.Save(creds)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}

	// Load credentials
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil")
	}
	if loaded.BotToken != creds.BotToken {
		t.Errorf("BotToken = %q, want %q", loaded.BotToken, creds.BotToken)
	}
	if loaded.BaseURL != creds.BaseURL {
		t.Errorf("BaseURL = %q, want %q", loaded.BaseURL, creds.BaseURL)
	}
}

func TestFileTokenStore_LoadNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")
	store := NewFileTokenStore(path)

	creds, err := store.Load()
	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}
	if creds != nil {
		t.Errorf("Load() = %v, want nil", creds)
	}
}

func TestFileTokenStore_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "token.json")
	store := NewFileTokenStore(path)

	// Save credentials first
	creds := &Credentials{BotToken: "test-token"}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Clear
	err := store.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// Verify file is removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file still exists after Clear()")
	}

	// Clear again should not error
	err = store.Clear()
	if err != nil {
		t.Errorf("Clear() on non-existent file error = %v", err)
	}
}

func TestAuth_GetQRCode(t *testing.T) {
	expectedResp := QRCodeResponse{
		QRCode:           "qr-12345",
		QRCodeImgURL:     "https://example.com/qr.png",
		QRCodeImgContent: "base64data",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/get_bot_qrcode" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("bot_type") != "3" {
			t.Errorf("bot_type = %q, want %q", r.URL.Query().Get("bot_type"), "3")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(expectedResp); err != nil {
			t.Logf("Encode error: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), nil, "1.0.0")
	auth := NewAuth(client, nil, nil)

	ctx := context.Background()
	resp, err := auth.GetQRCode(ctx)
	if err != nil {
		t.Fatalf("GetQRCode() error = %v", err)
	}
	if resp.QRCode != expectedResp.QRCode {
		t.Errorf("QRCode = %q, want %q", resp.QRCode, expectedResp.QRCode)
	}
	if resp.QRCodeImgURL != expectedResp.QRCodeImgURL {
		t.Errorf("QRCodeImgURL = %q, want %q", resp.QRCodeImgURL, expectedResp.QRCodeImgURL)
	}
}

func TestAuth_PollQRCodeStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		botToken   string
		baseURL    string
		wantStatus string
	}{
		{"wait", "wait", "", "", "wait"},
		{"scaned", "scaned", "", "", "scaned"},
		{"confirmed", "confirmed", "new-token", "https://new.example.com", "confirmed"},
		{"expired", "expired", "", "", "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/ilink/bot/get_qrcode_status" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				qrcode := r.URL.Query().Get("qrcode")
				if qrcode != "test-qr" {
					t.Errorf("qrcode = %q, want %q", qrcode, "test-qr")
				}
				resp := QRCodeStatus{
					Status:   tt.status,
					BotToken: tt.botToken,
					BaseURL:  tt.baseURL,
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					t.Logf("Encode error: %v", err)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, server.Client(), nil, "1.0.0")
			auth := NewAuth(client, nil, nil)

			ctx := context.Background()
			status, err := auth.PollQRCodeStatus(ctx, "test-qr")
			if err != nil {
				t.Fatalf("PollQRCodeStatus() error = %v", err)
			}
			if status.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", status.Status, tt.wantStatus)
			}
		})
	}
}

func TestAuth_LoginWithExistingToken(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "token.json")
	store := NewFileTokenStore(path)

	// Mock server that validates credentials
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return valid response for getconfig
		if _, err := w.Write([]byte(`{"ret": 0, "typing_ticket": "test-ticket"}`)); err != nil {
			t.Logf("Write error: %v", err)
		}
	}))
	defer server.Close()

	// Pre-save credentials with server URL
	creds := &Credentials{
		BotToken: "existing-token",
		BaseURL:  server.URL,
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create client with server URL
	client := NewClient(server.URL, server.Client(), nil, "1.0.0")
	auth := NewAuth(client, store, nil)

	ctx := context.Background()
	err := auth.Login(ctx, nil)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if client.Token() != "existing-token" {
		t.Errorf("Token() = %q, want %q", client.Token(), "existing-token")
	}
}

func TestAuth_LoginWithInvalidToken(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "token.json")
	store := NewFileTokenStore(path)

	// Mock server that returns session expired
	var pollCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/ilink/bot/getconfig" {
			// Return session expired
			if _, err := w.Write([]byte(`{"ret": -14, "errcode": -14, "errmsg": "session expired"}`)); err != nil {
				t.Logf("Write error: %v", err)
			}
			return
		}

		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			resp := QRCodeResponse{
				QRCode:           "new-qr",
				QRCodeImgURL:     "https://example.com/qr.png",
				QRCodeImgContent: "base64-qr",
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Logf("Encode error: %v", err)
			}

		case "/ilink/bot/get_qrcode_status":
			count := pollCount.Add(1)
			var resp QRCodeStatus
			if count == 1 {
				resp = QRCodeStatus{Status: "wait"}
			} else {
				resp = QRCodeStatus{
					Status:   "confirmed",
					BotToken: "new-token",
				}
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	// Pre-save invalid credentials
	creds := &Credentials{
		BotToken: "invalid-token",
		BaseURL:  server.URL,
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	client := NewClient(server.URL, server.Client(), nil, "1.0.0")
	auth := NewAuth(client, store, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := auth.Login(ctx, nil)
	if err != nil {
		t.Fatalf("Login() should succeed after re-login, got error = %v", err)
	}

	// Should have new token
	if client.Token() != "new-token" {
		t.Errorf("Token() = %q, want %q", client.Token(), "new-token")
	}
}

func TestAuth_LoginFullFlow(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "token.json")
	store := NewFileTokenStore(path)

	var pollCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			resp := QRCodeResponse{
				QRCode:           "login-qr",
				QRCodeImgURL:     "https://example.com/qr.png",
				QRCodeImgContent: "base64-qr-image",
			}
			json.NewEncoder(w).Encode(resp)

		case "/ilink/bot/get_qrcode_status":
			count := pollCount.Add(1)
			var resp QRCodeStatus
			if count == 1 {
				// First poll: waiting
				resp = QRCodeStatus{Status: "wait"}
			} else {
				// Second poll: confirmed
				resp = QRCodeStatus{
					Status:   "confirmed",
					BotToken: "new-login-token",
					BaseURL:  "https://new-api.example.com",
				}
			}
			json.NewEncoder(w).Encode(resp)

		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), nil, "1.0.0")
	auth := NewAuth(client, store, nil)

	var qrCallbackCalled bool
	var receivedQRContent string

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := auth.Login(ctx, func(qrCodeImgContent string) {
		qrCallbackCalled = true
		receivedQRContent = qrCodeImgContent
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// Verify QR callback was called
	if !qrCallbackCalled {
		t.Error("QR code callback was not called")
	}
	if receivedQRContent != "base64-qr-image" {
		t.Errorf("QR content = %q, want %q", receivedQRContent, "base64-qr-image")
	}

	// Verify client token was set
	if client.Token() != "new-login-token" {
		t.Errorf("Token() = %q, want %q", client.Token(), "new-login-token")
	}

	// Verify credentials were saved
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.BotToken != "new-login-token" {
		t.Errorf("saved BotToken = %q, want %q", loaded.BotToken, "new-login-token")
	}
	if loaded.BaseURL != "https://new-api.example.com" {
		t.Errorf("saved BaseURL = %q, want %q", loaded.BaseURL, "https://new-api.example.com")
	}
}

func TestAuth_LoginQRCodeExpired(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "token.json")
	store := NewFileTokenStore(path)

	var qrCodeCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			count := qrCodeCount.Add(1)
			resp := QRCodeResponse{
				QRCode:           "qr-" + string(rune('0'+count)),
				QRCodeImgURL:     "https://example.com/qr.png",
				QRCodeImgContent: "base64-qr-image",
			}
			json.NewEncoder(w).Encode(resp)

		case "/ilink/bot/get_qrcode_status":
			// Always return expired
			resp := QRCodeStatus{Status: "expired"}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), nil, "1.0.0")
	auth := NewAuth(client, store, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := auth.Login(ctx, func(qrCodeImgContent string) {
		// Ignore QR code
	})

	if err != ErrQRCodeExpired {
		t.Errorf("Login() error = %v, want ErrQRCodeExpired", err)
	}

	// Should have tried 3 times
	if qrCodeCount.Load() != 3 {
		t.Errorf("QR code requests = %d, want 3", qrCodeCount.Load())
	}
}
