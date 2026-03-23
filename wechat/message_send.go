package wechat

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// SendImageFromPath reads a local image file, uploads it to CDN, and sends it as an image message.
func SendImageFromPath(ctx context.Context, client *Client, media *MediaManager, toUserID, contextToken, imagePath string) error {
	// Read file
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("read image file: %w", err)
	}

	// Upload to CDN with "image" media type (following official plugin format)
	result, err := media.UploadFile(ctx, data, toUserID, "image")
	if err != nil {
		return fmt.Errorf("upload image to CDN: %w", err)
	}

	// Build image item (following official plugin format)
	imageItem := media.BuildImageItem(result, 0, 0)

	// Send the image
	return SendImageWithItem(ctx, client, toUserID, contextToken, imageItem.ImageItem)
}

// SendImageWithItem sends an image message using an ImageItem.
func SendImageWithItem(ctx context.Context, client *Client, toUserID, contextToken string, imageItem *ImageItem) error {
	msg := &Message{
		FromUserID:   "", // Must be empty string (not omitted) per API spec
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:      ItemTypeImage,
				ImageItem: imageItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// generateClientID generates a unique client ID for message tracking.
// Format: openclaw-weixin-{timestamp36}-{random}
func generateClientID() string {
	// Generate random part (8 characters)
	b := make([]byte, 4)
	rand.Read(b)
	randomPart := fmt.Sprintf("%x", b)

	// For timestamp, use a simple counter or random
	timestampPart := fmt.Sprintf("%x", len(b))

	return fmt.Sprintf("openclaw-weixin-%s-%s", timestampPart, randomPart)
}

// SendText sends a text message to a user.
// contextToken must be provided from a received message for proper conversation linking.
func SendText(ctx context.Context, client *Client, toUserID, text, contextToken string) error {
	clientID := generateClientID()

	msg := &Message{
		FromUserID:   "", // Must be empty string (not omitted) per API spec
		ToUserID:     toUserID,
		ClientID:     clientID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type: ItemTypeText,
				TextItem: &TextItem{
					Text: text,
				},
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// Reply sends a text reply to an incoming message, automatically using its context_token.
func Reply(ctx context.Context, client *Client, msg *Message, text string) error {
	return SendText(ctx, client, msg.FromUserID, text, msg.ContextToken)
}

// SendImage sends an image message to a user.
// The imageItem should contain the CDN-uploaded image information.
func SendImage(ctx context.Context, client *Client, toUserID, contextToken string, imageItem *ImageItem) error {
	msg := &Message{
		FromUserID:   "", // Must be empty string (not omitted) per API spec
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:      ItemTypeImage,
				ImageItem: imageItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// SendFile sends a file message to a user.
// The fileItem should contain the CDN-uploaded file information.
func SendFile(ctx context.Context, client *Client, toUserID, contextToken string, fileItem *FileItem) error {
	msg := &Message{
		FromUserID:   "", // Must be empty string (not omitted) per API spec
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:     ItemTypeFile,
				FileItem: fileItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// SendMessage sends a custom message with multiple items.
func SendMessage(ctx context.Context, client *Client, toUserID, contextToken string, items []MessageItem) error {
	msg := &Message{
		FromUserID:   "", // Must be empty string (not omitted) per API spec
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList:     items,
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// SendVoice sends a voice message to a user.
// The voiceItem should contain the CDN-uploaded voice information.
func SendVoice(ctx context.Context, client *Client, toUserID, contextToken string, voiceItem *VoiceItem) error {
	msg := &Message{
		FromUserID:   "", // Must be empty string (not omitted) per API spec
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:      ItemTypeVoice,
				VoiceItem: voiceItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// SendVideo sends a video message to a user.
// The videoItem should contain the CDN-uploaded video information.
func SendVideo(ctx context.Context, client *Client, toUserID, contextToken string, videoItem *VideoItem) error {
	msg := &Message{
		FromUserID:   "", // Must be empty string (not omitted) per API spec
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:      ItemTypeVideo,
				VideoItem: videoItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// ReplyWithMedia sends a rich media reply with both text and media items.
func ReplyWithMedia(ctx context.Context, client *Client, msg *Message, text string, mediaItems []MessageItem) error {
	items := []MessageItem{
		{
			Type: ItemTypeText,
			TextItem: &TextItem{
				Text: text,
			},
		},
	}
	items = append(items, mediaItems...)

	return SendMessage(ctx, client, msg.FromUserID, msg.ContextToken, items)
}

// SendVoiceFromPath reads a local voice file, uploads it to CDN, and sends it as a voice message.
func SendVoiceFromPath(ctx context.Context, client *Client, media *MediaManager, toUserID, contextToken, voicePath string, duration int) error {
	// Read file
	data, err := os.ReadFile(voicePath)
	if err != nil {
		return fmt.Errorf("read voice file: %w", err)
	}

	// Upload to CDN with "voice" media type (following official plugin format)
	result, err := media.UploadFile(ctx, data, toUserID, "voice")
	if err != nil {
		return fmt.Errorf("upload voice to CDN: %w", err)
	}

	// Build voice item (following official plugin format)
	voiceItem := media.BuildVoiceItemPtr(result, duration)

	// Send the voice
	return SendVoiceWithItem(ctx, client, toUserID, contextToken, voiceItem)
}

// SendVoiceWithItem sends a voice message using a VoiceItem.
func SendVoiceWithItem(ctx context.Context, client *Client, toUserID, contextToken string, voiceItem *VoiceItem) error {
	msg := &Message{
		FromUserID:   "",
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:      ItemTypeVoice,
				VoiceItem: voiceItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// SendFileFromPath reads a local file, uploads it to CDN, and sends it as a file message.
func SendFileFromPath(ctx context.Context, client *Client, media *MediaManager, toUserID, contextToken, filePath string) error {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Get filename
	fileName := filepath.Base(filePath)

	// Upload to CDN with "file" media type (following official plugin format)
	result, err := media.UploadFile(ctx, data, toUserID, "file")
	if err != nil {
		return fmt.Errorf("upload file to CDN: %w", err)
	}

	// Build file item
	fileItem := media.BuildFileItemPtr(result, fileName)

	// Send the file
	return SendFileWithItem(ctx, client, toUserID, contextToken, fileItem)
}

// SendFileWithItem sends a file message using a FileItem.
func SendFileWithItem(ctx context.Context, client *Client, toUserID, contextToken string, fileItem *FileItem) error {
	msg := &Message{
		FromUserID:   "",
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:     ItemTypeFile,
				FileItem: fileItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

// SendVideoFromPath reads a local video file, uploads it to CDN, and sends it as a video message.
func SendVideoFromPath(ctx context.Context, client *Client, media *MediaManager, toUserID, contextToken, videoPath string) error {
	// Read file
	data, err := os.ReadFile(videoPath)
	if err != nil {
		return fmt.Errorf("read video file: %w", err)
	}

	// Upload to CDN with "video" media type (following official plugin format)
	result, err := media.UploadFile(ctx, data, toUserID, "video")
	if err != nil {
		return fmt.Errorf("upload video to CDN: %w", err)
	}

	// Build video item (following official plugin format)
	videoItem := media.BuildVideoItemPtr(result, 0, 0, 0)

	// Send the video
	return SendVideoWithItem(ctx, client, toUserID, contextToken, videoItem)
}

// SendVideoWithItem sends a video message using a VideoItem.
func SendVideoWithItem(ctx context.Context, client *Client, toUserID, contextToken string, videoItem *VideoItem) error {
	msg := &Message{
		FromUserID:   "",
		ToUserID:     toUserID,
		ClientID:     generateClientID(),
		MessageType:  MessageTypeBot,
		MessageState: MessageStateFinish,
		ContextToken: contextToken,
		ItemList: []MessageItem{
			{
				Type:      ItemTypeVideo,
				VideoItem: videoItem,
			},
		},
	}

	req := &SendMessageRequest{
		Msg: msg,
		BaseInfo: &BaseInfo{
			ChannelVersion: client.channelVersion,
		},
	}

	var resp SendMessageResponse
	if err := client.Post(ctx, "/ilink/bot/sendmessage", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.ErrCode, Message: resp.ErrMsg}
	}

	return nil
}

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
