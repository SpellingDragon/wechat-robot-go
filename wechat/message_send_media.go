package wechat

import (
	"context"
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
