package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/SpellingDragon/wechat-robot-go/wechat/internal/crypto"
	"github.com/SpellingDragon/wechat-robot-go/wechat/internal/model"
)

// --- CDN API Types ---

// UploadURLRequest is the request body for POST /ilink/bot/getuploadurl.
type UploadURLRequest struct {
	FileKey         string          `json:"filekey,omitempty"`
	MediaType       int             `json:"media_type,omitempty"`
	ToUserID        string          `json:"to_user_id,omitempty"`
	RawSize         int             `json:"rawsize,omitempty"`
	RawFileMD5      string          `json:"rawfilemd5,omitempty"`
	FileSize        int             `json:"filesize,omitempty"`
	ThumbRawSize    int             `json:"thumb_rawsize,omitempty"`
	ThumbRawFileMD5 string          `json:"thumb_rawfilemd5,omitempty"`
	ThumbFileSize   int             `json:"thumb_filesize,omitempty"`
	NoNeedThumb     bool            `json:"no_need_thumb,omitempty"`
	AESKey          string          `json:"aeskey,omitempty"`
	BaseInfo        *model.BaseInfo `json:"base_info,omitempty"`
}

// UploadURLResponse is the response from POST /ilink/bot/getuploadurl.
type UploadURLResponse struct {
	UploadURL        string `json:"upload_url"`
	UploadParam      string `json:"upload_param"` // additional params for CDN
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
	Ret              int    `json:"ret"`
}

// UploadResult contains the CDN reference information after a successful upload.
type UploadResult struct {
	AESKey         string // hex-encoded AES key
	FileKey        string // hex-encoded file key
	EncryptedParam string // from CDN response x-encrypted-param header
	FileSize       int    // original file size
	CipherSize     int    // encrypted file size
}

// --- MediaManager ---

// MediaManager handles media file upload and download with AES encryption.
type MediaManager struct {
	apiClient      APIClient
	httpClient     *http.Client
	logger         *slog.Logger
	channelVersion string
	cdnBaseURL     string // CDN base URL for uploads (may differ from API base URL)
}

// NewMediaManager creates a new MediaManager.
func NewMediaManager(apiClient APIClient, httpClient *http.Client, logger *slog.Logger) *MediaManager {
	return &MediaManager{
		apiClient:      apiClient,
		httpClient:     httpClient,
		logger:         logger,
		channelVersion: "1.0.3",
		cdnBaseURL:     "", // must be set via SetCDNBaseURL
	}
}

// SetCDNBaseURL sets the CDN base URL (may be different from API base URL).
func (m *MediaManager) SetCDNBaseURL(url string) {
	m.cdnBaseURL = url
}

// maxRetries is the maximum number of retry attempts for CDN operations.
const maxRetries = 3

// UploadFile encrypts and uploads a file to WeChat CDN.
// Returns the CDN reference info needed for constructing message items.
// fileType should be "image", "video", or "file".
// toUserID is the recipient's user ID (required for getUploadUrl).
func (m *MediaManager) UploadFile(ctx context.Context, data []byte, toUserID, fileType string) (*UploadResult, error) {
	// 1. Generate AES key (16 bytes)
	aesKey, err := crypto.GenerateAESKey()
	if err != nil {
		return nil, fmt.Errorf("generate AES key: %w", err)
	}

	// 2. Encrypt file content using AES-128-ECB with PKCS7 padding (same as official plugin)
	encrypted, err := crypto.EncryptAESECB(data, aesKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt file: %w", err)
	}

	// 3. Get upload URL from API
	// Map fileType string to MediaType number (same as official plugin)
	mediaType := 1 // default IMAGE
	switch fileType {
	case "video":
		mediaType = 2
	case "file":
		mediaType = 3
	case "voice":
		mediaType = 4
	}

	// Calculate MD5 of original file
	md5Hash := md5.Sum(data)
	rawFileMD5 := hex.EncodeToString(md5Hash[:])

	// Generate random 16-byte filekey (hex string) - REQUIRED by official API
	fileKey := generateFileKey()

	uploadReq := &UploadURLRequest{
		FileKey:     fileKey, // REQUIRED: 16-byte random hex string
		ToUserID:    toUserID,
		MediaType:   mediaType,
		RawSize:     len(data),
		RawFileMD5:  rawFileMD5,
		FileSize:    len(encrypted), // ciphertext size (must match padded size)
		NoNeedThumb: true,
		AESKey:      hex.EncodeToString(aesKey),
		BaseInfo: &model.BaseInfo{
			ChannelVersion: m.channelVersion,
		},
	}
	var uploadResp UploadURLResponse
	if err := m.apiClient.Post(ctx, "/ilink/bot/getuploadurl", uploadReq, &uploadResp); err != nil {
		return nil, fmt.Errorf("get upload url: %w", err)
	}
	if uploadResp.Ret != 0 {
		return nil, fmt.Errorf("get upload url failed: ret=%d", uploadResp.Ret)
	}

	// 4. Upload encrypted data to CDN with retry
	cdnURL := fmt.Sprintf("%s/upload?encrypted_query_param=%s&filekey=%s",
		m.cdnBaseURL, url.QueryEscape(uploadResp.UploadParam), url.QueryEscape(fileKey))
	encryptedParam, err := m.uploadToCDN(ctx, cdnURL, encrypted)
	if err != nil {
		return nil, fmt.Errorf("upload to CDN: %w", err)
	}

	// 5. Return result
	return &UploadResult{
		AESKey:         hex.EncodeToString(aesKey),
		FileKey:        fileKey,
		EncryptedParam: encryptedParam,
		FileSize:       len(data),
		CipherSize:     len(encrypted),
	}, nil
}

// generateFileKey generates a random 16-byte hex string for CDN upload.
func generateFileKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// rand.Read should never fail with crypto/rand, but handle gracefully
	}
	return hex.EncodeToString(b)
}

// uploadToCDN uploads encrypted data to the CDN URL with retry logic.
func (m *MediaManager) uploadToCDN(ctx context.Context, url string, data []byte) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		encryptedParam, err := m.doUploadToCDN(ctx, url, data)
		if err == nil {
			return encryptedParam, nil
		}

		lastErr = err

		// Check if it's a 4xx error - don't retry
		if httpErr, ok := err.(*httpError); ok && httpErr.statusCode >= 400 && httpErr.statusCode < 500 {
			return "", err
		}

		// Log retry attempt
		if m.logger != nil {
			m.logger.Warn("CDN upload failed, retrying",
				"attempt", attempt,
				"max_retries", maxRetries,
				"error", err,
			)
		}
	}

	return "", fmt.Errorf("upload failed after %d attempts: %w", maxRetries, lastErr)
}

// httpError represents an HTTP error with status code.
type httpError struct {
	statusCode int
	message    string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("http %d: %s", e.statusCode, e.message)
}

// doUploadToCDN performs a single upload attempt.
// Note: Official API uses POST method, not PUT.
func (m *MediaManager) doUploadToCDN(ctx context.Context, url string, data []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", &httpError{
			statusCode: resp.StatusCode,
			message:    string(body),
		}
	}

	// Get encrypted param from response header
	encryptedParam := resp.Header.Get("x-encrypted-param")
	if encryptedParam == "" {
		return "", fmt.Errorf("CDN response missing x-encrypted-param header")
	}
	return encryptedParam, nil
}

// DownloadFile downloads and decrypts a media file from WeChat CDN.
// aesKeyHex is the hex-encoded AES key.
func (m *MediaManager) DownloadFile(ctx context.Context, url string, aesKeyHex string) ([]byte, error) {
	return m.DownloadFileWithKey(ctx, url, aesKeyHex)
}

// DownloadFileWithKey downloads and decrypts a media file using the provided AES key.
// The aesKeyStr is expected to be base64-encoded and can be:
//   - base64(16 raw bytes) → direct AES key
//   - base64(32 hex chars) → hex string that needs to be parsed as hex
func (m *MediaManager) DownloadFileWithKey(ctx context.Context, url string, aesKeyStr string) ([]byte, error) {
	// Download encrypted file
	encrypted, err := m.downloadFromCDN(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("download from CDN: %w", err)
	}

	// Decode AES key from base64
	decoded, err := base64.StdEncoding.DecodeString(aesKeyStr)
	if err != nil {
		return nil, fmt.Errorf("decode base64 AES key: %w", err)
	}

	var aesKeyBytes []byte

	if len(decoded) == 16 {
		// Direct 16-byte AES key
		aesKeyBytes = decoded
	} else if len(decoded) == 32 && isHexString(string(decoded)) {
		// Hex string: parse as hex to get 16 raw bytes
		aesKeyBytes, err = hex.DecodeString(string(decoded))
		if err != nil {
			return nil, fmt.Errorf("decode hex AES key: %w", err)
		}
	} else {
		return nil, fmt.Errorf("AES key must decode to 16 bytes or 32-char hex string, got %d bytes", len(decoded))
	}

	if len(aesKeyBytes) != 16 {
		return nil, fmt.Errorf("AES key must be 16 bytes, got %d", len(aesKeyBytes))
	}

	// Decrypt file content
	plaintext, err := crypto.DecryptAESECB(encrypted, aesKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("decrypt file: %w", err)
	}

	return plaintext, nil
}

// isHexString checks if a string contains only hex characters.
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) == 32
}

// DownloadImage downloads and decrypts an image from WeChat CDN.
// It handles both hex-encoded and base64-encoded AES keys from ImageItem.
func (m *MediaManager) DownloadImage(ctx context.Context, cdnBaseURL string, imageItem *model.ImageItem) ([]byte, error) {
	// Build download URL
	downloadURL := fmt.Sprintf("%s/download?encrypted_query_param=%s",
		cdnBaseURL, url.QueryEscape(imageItem.Media.EncryptQueryParam))

	// Determine AES key source and decode
	var aesKeyStr string
	if imageItem.AESKey != "" {
		aesKeyBytes, _ := hex.DecodeString(imageItem.AESKey)
		aesKeyStr = base64.StdEncoding.EncodeToString(aesKeyBytes)
	} else if imageItem.Media != nil && imageItem.Media.AESKey != "" {
		aesKeyStr = imageItem.Media.AESKey
	} else {
		return nil, fmt.Errorf("no AES key found in image item")
	}

	return m.DownloadFileWithKey(ctx, downloadURL, aesKeyStr)
}

// downloadFromCDN downloads raw bytes from CDN.
func (m *MediaManager) downloadFromCDN(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed: http %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// DownloadVoice downloads and decrypts a voice message from a VoiceItem.
func (m *MediaManager) DownloadVoice(ctx context.Context, cdnBaseURL string, voiceItem *model.VoiceItem) ([]byte, error) {
	if voiceItem.Media == nil || voiceItem.Media.EncryptQueryParam == "" {
		return nil, fmt.Errorf("voice item has no media data")
	}

	// Build download URL
	downloadURL := fmt.Sprintf("%s/download?encrypted_query_param=%s",
		cdnBaseURL, url.QueryEscape(voiceItem.Media.EncryptQueryParam))

	// Download and decrypt
	return m.DownloadFileWithKey(ctx, downloadURL, voiceItem.Media.AESKey)
}

// DownloadFileItem downloads and decrypts a file from a FileItem.
func (m *MediaManager) DownloadFileItem(ctx context.Context, cdnBaseURL string, fileItem *model.FileItem) ([]byte, error) {
	if fileItem.Media == nil || fileItem.Media.EncryptQueryParam == "" {
		return nil, fmt.Errorf("file item has no media data")
	}

	// Build download URL
	downloadURL := fmt.Sprintf("%s/download?encrypted_query_param=%s",
		cdnBaseURL, url.QueryEscape(fileItem.Media.EncryptQueryParam))

	// Download and decrypt
	return m.DownloadFileWithKey(ctx, downloadURL, fileItem.Media.AESKey)
}

// DownloadVideoItem downloads and decrypts a video from a VideoItem.
func (m *MediaManager) DownloadVideoItem(ctx context.Context, cdnBaseURL string, videoItem *model.VideoItem) ([]byte, error) {
	if videoItem.Media == nil || videoItem.Media.EncryptQueryParam == "" {
		return nil, fmt.Errorf("video item has no media data")
	}

	// Build download URL
	downloadURL := fmt.Sprintf("%s/download?encrypted_query_param=%s",
		cdnBaseURL, url.QueryEscape(videoItem.Media.EncryptQueryParam))

	// Download and decrypt
	return m.DownloadFileWithKey(ctx, downloadURL, videoItem.Media.AESKey)
}
