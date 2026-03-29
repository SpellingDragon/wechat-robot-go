package text_test

import (
	"strings"
	"testing"

	"github.com/SpellingDragon/wechat-robot-go/wechat/internal/text"
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
		{
			name:     "empty text",
			text:     "",
			maxLen:   100,
			expected: 1,
		},
		{
			name:     "zero maxLen uses default",
			text:     "Short text",
			maxLen:   0,
			expected: 1,
		},
		{
			name:     "negative maxLen uses default",
			text:     "Short text",
			maxLen:   -10,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := text.SplitText(tt.text, tt.maxLen)
			if len(chunks) != tt.expected {
				t.Errorf("SplitText() got %d chunks, want %d", len(chunks), tt.expected)
			}

			// Verify each chunk is within limit (use default if maxLen <= 0)
			effectiveMax := tt.maxLen
			if effectiveMax <= 0 {
				effectiveMax = text.DefaultMaxTextLength
			}
			for i, chunk := range chunks {
				if len(chunk) > effectiveMax {
					t.Errorf("chunk %d length %d exceeds max %d", i, len(chunk), effectiveMax)
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
		{
			name:   "text shorter than maxLen",
			text:   "Short",
			maxLen: 100,
			wantGt: 4, // Should return len(text) = 5
			wantLt: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := text.FindSplitPoint(tt.text, tt.maxLen)
			if got <= tt.wantGt || got > tt.wantLt {
				t.Errorf("FindSplitPoint() = %d, want between %d and %d", got, tt.wantGt, tt.wantLt)
			}
		})
	}
}

func TestSplitTextChinesePunctuation(t *testing.T) {
	// Test Chinese punctuation splitting
	chineseText := "这是第一句话，这是第二句话；这是第三句话：这是第四句话。"

	chunks := text.SplitText(chineseText, 30)
	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for Chinese text, got %d", len(chunks))
	}

	// Verify each chunk is within limit
	for i, chunk := range chunks {
		if len(chunk) > 30 {
			t.Errorf("chunk %d length %d exceeds max 30", i, len(chunk))
		}
	}
}

func TestSplitTextPreservesContent(t *testing.T) {
	// Verify that joining all chunks gives back original text (minus trimmed whitespace)
	original := "Hello world. This is a test.\nNew paragraph here. More text follows."

	chunks := text.SplitText(original, 20)

	// Reconstruct and verify no content is lost (though whitespace may be trimmed)
	reconstructed := strings.Join(chunks, "")

	// The original should contain all words from reconstructed (order preserved)
	originalWords := strings.Fields(original)
	reconstructedWords := strings.Fields(reconstructed)

	if len(originalWords) != len(reconstructedWords) {
		t.Errorf("Word count mismatch: original=%d, reconstructed=%d",
			len(originalWords), len(reconstructedWords))
	}
}

func TestDefaultMaxTextLength(t *testing.T) {
	if text.DefaultMaxTextLength != 1500 {
		t.Errorf("DefaultMaxTextLength = %d, want 1500", text.DefaultMaxTextLength)
	}
}

func TestMaxChunkCount(t *testing.T) {
	if text.MaxChunkCount != 5 {
		t.Errorf("MaxChunkCount = %d, want 5", text.MaxChunkCount)
	}
}

func BenchmarkSplitText(b *testing.B) {
	longText := strings.Repeat("This is a test sentence. ", 100) // ~2500 chars

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = text.SplitText(longText, 500)
	}
}

func BenchmarkFindSplitPoint(b *testing.B) {
	longText := strings.Repeat("This is a test sentence. ", 50) // ~1250 chars

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = text.FindSplitPoint(longText, 500)
	}
}
