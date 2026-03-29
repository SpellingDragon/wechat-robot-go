package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SpellingDragon/wechat-robot-go/wechat/internal/text"
)

func TestSendLongText(t *testing.T) {
	var sentMessages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			var req SendMessageRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if len(req.Msg.ItemList) > 0 && req.Msg.ItemList[0].TextItem != nil {
				sentMessages = append(sentMessages, req.Msg.ItemList[0].TextItem.Text)
			}
			_ = json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), nil, "1.0.3")
	client.SetToken("test-token")

	// Create a long text that results in more than MaxChunkCount chunks with old limit
	// With new 1500 char limit, this should produce fewer chunks
	longText := strings.Repeat("This is a test sentence. ", 100) // ~2500 chars

	ctx := context.Background()
	count, err := SendLongText(ctx, client, nil, "user-123", longText, "ctx-token")
	if err != nil {
		t.Fatalf("SendLongText failed: %v", err)
	}

	// With 1500 char limit, ~2500 char text should produce at most 2 messages
	if count > text.MaxChunkCount {
		t.Errorf("SendLongText sent %d messages, expected at most %d", count, text.MaxChunkCount)
	}

	if len(sentMessages) != count {
		t.Errorf("sent %d messages, expected %d", len(sentMessages), count)
	}
}

func TestSendLongText_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), nil, "1.0.3")
	client.SetToken("test-token")

	longText := strings.Repeat("Test ", 200)

	ctx := context.Background()
	_, err := SendLongText(ctx, client, nil, "user-123", longText, "ctx-token")
	if err == nil {
		t.Error("SendLongText should return error on server error")
	}
}

// TestSplitText_PublicAPI tests the publicly exposed SplitText function via types.go
func TestSplitText_PublicAPI(t *testing.T) {
	// Test through the public alias
	chunks := SplitText("Hello World", 500)
	if len(chunks) != 1 {
		t.Errorf("SplitText() got %d chunks, want 1", len(chunks))
	}

	// Test splitting
	longText := strings.Repeat("word ", 200) // 1000 chars
	chunks = SplitText(longText, 100)
	if len(chunks) < 2 {
		t.Errorf("SplitText() got %d chunks, want at least 2", len(chunks))
	}
}
