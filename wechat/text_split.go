package wechat

import (
	"context"
	"fmt"
)

// --- Text Splitting Utilities ---

const (
	// DefaultMaxTextLength is the default maximum text length per message.
	// WeChat has a limit around 500-1000 characters depending on client.
	// We use a conservative limit to ensure delivery.
	DefaultMaxTextLength = 500
)

// SplitText splits a long text into multiple chunks that fit within the message limit.
// It tries to split on natural boundaries (newlines, spaces, punctuation) when possible.
func SplitText(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = DefaultMaxTextLength
	}

	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > maxLen {
		// Try to find a good split point within the last 100 chars of the limit
		splitPoint := findSplitPoint(remaining, maxLen)

		if splitPoint <= 0 {
			// No good split point found, hard split at maxLen
			splitPoint = maxLen
		}

		chunks = append(chunks, remaining[:splitPoint])
		remaining = remaining[splitPoint:]

		// Skip leading whitespace in next chunk
		for len(remaining) > 0 && (remaining[0] == ' ' || remaining[0] == '\n' || remaining[0] == '\t') {
			remaining = remaining[1:]
		}
	}

	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}

	return chunks
}

// findSplitPoint finds a good position to split text, preferring natural boundaries.
func findSplitPoint(text string, maxLen int) int {
	if len(text) <= maxLen {
		return len(text)
	}

	// Look for split points in order of preference:
	// 1. Newline (best)
	// 2. Sentence end (. ! ?)
	// 3. Comma or semicolon
	// 4. Space
	// Start from maxLen and work backwards

	searchStart := maxLen
	if searchStart > len(text) {
		searchStart = len(text)
	}

	// Search window: last 100 chars before maxLen
	searchStart = maxLen - 100
	if searchStart < 0 {
		searchStart = 0
	}

	// Priority 1: Newline (prefer paragraph breaks)
	for i := maxLen; i >= searchStart; i-- {
		if i < len(text) && text[i] == '\n' {
			return i + 1
		}
	}

	// Priority 2: Sentence end (. ! ? followed by space or newline)
	for i := maxLen; i >= searchStart; i-- {
		if i < len(text) {
			c := text[i]
			if c == '.' || c == '!' || c == '?' {
				if i+1 < len(text) && (text[i+1] == ' ' || text[i+1] == '\n') {
					return i + 2
				}
				return i + 1
			}
		}
	}

	// Priority 3: Comma, semicolon, colon
	for i := maxLen; i >= searchStart; i-- {
		if i < len(text) {
			c := text[i]
			if c == ',' || c == ';' {
				return i + 1
			}
		}
	}

	// Priority 3.5: Chinese punctuation (multi-byte, check substring)
	chinesePuncts := []string{"，", "；", "："}
	for i := maxLen; i >= searchStart && i < len(text); i-- {
		for _, punct := range chinesePuncts {
			if i+len(punct) <= len(text) && text[i:i+len(punct)] == punct {
				return i + len(punct)
			}
		}
	}

	// Priority 4: Space
	for i := maxLen; i >= searchStart; i-- {
		if i < len(text) && text[i] == ' ' {
			return i + 1
		}
	}

	// No good split point found
	return maxLen
}

// SendLongText sends a long text message by automatically splitting it into multiple messages.
// It returns the number of messages sent and any error encountered.
func SendLongText(ctx context.Context, client *Client, toUserID, text, contextToken string) (int, error) {
	chunks := SplitText(text, DefaultMaxTextLength)

	for i, chunk := range chunks {
		if err := SendText(ctx, client, toUserID, chunk, contextToken); err != nil {
			return i, fmt.Errorf("send chunk %d: %w", i+1, err)
		}
	}

	return len(chunks), nil
}
