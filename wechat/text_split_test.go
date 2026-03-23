package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSplitText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected int // number of chunks
	}{
		{
			name:     "short text",
			text:     "Hello World",
			maxLen:   500,
			expected: 1,
		},
		{
			name:     "exact fit",
			text:     "Hello",
			maxLen:   5,
			expected: 1,
		},
		{
			name:     "split on newline",
			text:     "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.",
			maxLen:   30,
			expected: 3,
		},
		{
			name:     "split on sentence",
			text:     "First sentence. Second sentence. Third sentence. Fourth sentence.",
			maxLen:   25,
			expected: 4,
		},
		{
			name:     "split on space",
			text:     "This is a long text without punctuation that needs to be split on spaces",
			maxLen:   20,
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := SplitText(tt.text, tt.maxLen)
			if len(chunks) != tt.expected {
				t.Errorf("SplitText() got %d chunks, want %d", len(chunks), tt.expected)
			}

			// Verify each chunk is within limit
			for i, chunk := range chunks {
				if len(chunk) > tt.maxLen {
					t.Errorf("chunk %d length %d exceeds max %d", i, len(chunk), tt.maxLen)
				}
			}
		})
	}
}

func TestFindSplitPoint(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		wantGt int // split point should be greater than this
		wantLt int // split point should be less than or equal this
	}{
		{
			name:   "find newline",
			text:   "First line.\nSecond line.\nThird line.",
			maxLen: 15,
			wantGt: 10,
			wantLt: 15,
		},
		{
			name:   "find sentence end",
			text:   "Hello world. How are you?",
			maxLen: 15,
			wantGt: 10,
			wantLt: 15,
		},
		{
			name:   "find space",
			text:   "One two three four five six seven eight",
			maxLen: 15,
			wantGt: 8,
			wantLt: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findSplitPoint(tt.text, tt.maxLen)
			if got <= tt.wantGt || got > tt.wantLt {
				t.Errorf("findSplitPoint() = %d, want between %d and %d", got, tt.wantGt, tt.wantLt)
			}
		})
	}
}

func TestSendLongText(t *testing.T) {
	var sentMessages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			var req SendMessageRequest
			json.NewDecoder(r.Body).Decode(&req)
			if len(req.Msg.ItemList) > 0 && req.Msg.ItemList[0].TextItem != nil {
				sentMessages = append(sentMessages, req.Msg.ItemList[0].TextItem.Text)
			}
			json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), nil, "1.0.3")
	client.SetToken("test-token")

	// Create a long text
	longText := strings.Repeat("This is a test sentence. ", 50) // ~1250 chars

	ctx := context.Background()
	count, err := SendLongText(ctx, client, "user-123", longText, "ctx-token")
	if err != nil {
		t.Fatalf("SendLongText failed: %v", err)
	}

	if count < 2 {
		t.Errorf("SendLongText sent %d messages, expected at least 2", count)
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
	_, err := SendLongText(ctx, client, "user-123", longText, "ctx-token")
	if err == nil {
		t.Error("SendLongText should return error on server error")
	}
}

func BenchmarkSplitText(b *testing.B) {
	text := strings.Repeat("This is a test sentence. ", 100) // ~2500 chars

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SplitText(text, 500)
	}
}
