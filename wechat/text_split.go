package wechat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SpellingDragon/wechat-robot-go/wechat/internal/text"
)

// SendLongText sends a long text message by automatically splitting it into multiple messages.
// If the text would result in more than MaxChunkCount messages, it will be converted to a
// temporary text file and sent as a file attachment instead.
func SendLongText(ctx context.Context, client *Client, media *MediaManager, toUserID, textContent, contextToken string) (int, error) {
	chunks := text.SplitText(textContent, text.DefaultMaxTextLength)

	// If too many chunks, convert to file
	if len(chunks) > text.MaxChunkCount {
		return sendTextAsFile(ctx, client, media, toUserID, contextToken, textContent)
	}

	for i, chunk := range chunks {
		if err := SendText(ctx, client, toUserID, chunk, contextToken); err != nil {
			return i, fmt.Errorf("send chunk %d: %w", i+1, err)
		}
	}

	return len(chunks), nil
}

// sendTextAsFile saves the text content to a temporary file and sends it as a file attachment.
func sendTextAsFile(ctx context.Context, client *Client, media *MediaManager, toUserID, contextToken, content string) (int, error) {
	// Create temporary file
	tmpDir := os.TempDir()
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("response_%s.txt", timestamp)
	tmpPath := filepath.Join(tmpDir, filename)

	// Write content to temp file
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return 0, fmt.Errorf("write temp file: %w", err)
	}

	// Ensure temp file is cleaned up
	defer os.Remove(tmpPath)

	// Send as file
	if err := SendFileFromPath(ctx, client, media, toUserID, contextToken, tmpPath); err != nil {
		return 0, fmt.Errorf("send file: %w", err)
	}

	return 1, nil // Sent as single file message
}
