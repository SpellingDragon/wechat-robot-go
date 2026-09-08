# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Getupdates cursor persistence (`CursorStore` interface + `FileCursorStore`,
  atomic 0600 writes):
  - `NewPollerWithCursorStore` — poller construction with cursor restore; a
    load failure logs a warning and starts with an empty cursor.
  - `WithCursorStore` bot option — wire persistence through `Bot` options;
    `NewPoller` / legacy configs keep the in-memory-only behavior unchanged.
  - The poller saves the cursor after each batch update; a save failure only
    logs (worst case after restart: same replay window as before this change).

## [v1.5.0] - 2026-07-30

### Added

- Streaming media download with constant memory usage:
  - `Bot.DownloadImageFromItemTo` / `Bot.DownloadVoiceTo` / `Bot.DownloadFileFromItemTo` / `Bot.DownloadVideoFromItemTo` - streaming variants of the existing `Download*FromItem` methods that decrypt on the fly and write plaintext to an `io.Writer`, returning the number of plaintext bytes written.
  - `DownloadOptions{MaxSize int64}` - enforces a ciphertext size limit (`0` = unlimited), rejecting up front via `Content-Length` when available and mid-stream via a byte counter otherwise (covers chunked and misreported lengths).
  - `ErrMaxSizeExceeded` sentinel error, detectable via `errors.Is`.
  - `MediaManager.DownloadToWriter` and per-item `Download*To` streaming methods.
  - `crypto.NewECBDecryptReader` - streaming AES-128-ECB decrypter (internal).
  - `DeclaredSize() int64` accessors on `FileItem`, `VoiceItem`, `VideoItem` and `ImageItem` normalizing the declared plaintext size (`0` = unknown).

### Changed

- `MediaManager.DownloadFileWithKey` (and all `[]byte` download APIs built on it) now decrypts while downloading instead of buffering the full ciphertext and allocating a separate plaintext copy, reducing peak memory from ~2x file size to ~1x. Signatures and behavior are unchanged.

Contract note: when a streaming download returns an error, the writer may already hold a partial, incomplete output; cleanup is the caller's responsibility.

## [v1.3.0] - 2026-05-04

### Added

- `WithTokenStore(store TokenStore)` - Allow injecting a custom TokenStore so login credential persistence can use external backends instead of file-only storage.

## [v1.2.1] - 2026-04-13

### Changed

- Optimized intelligent text splitting to reduce message fragmentation.
- Updated example code to match new `SendLongText` signature.
- Added `Bot.Media()` convenience method.

## [v1.2.0] - 2026-03-29

### Added

- `WithLogFile(path string)` - Creates a logger that writes to both file (DEBUG) and console (INFO)
- `WithLogWriter(w io.Writer, level slog.Level)` - Creates a logger with custom writer and level
- `multiHandler` implementation for `slog.Handler` interface for unified logging output

### Changed

- Improved application-level logging flexibility for production debugging

## [v1.1.0] - 2026-03-28

### Added

- Context token persistence for proactive messaging
- Support for proactive outbound messages via `SendTextToUser`, `SendImageToUser`, `SendFileToUser`

### Fixed

- Raised gocyclo threshold to 30 for complex text splitting logic
- Resolved remaining lint issues

## [v1.0.0] - 2026-03-27

### Added

- Initial release
- QR code login with persistent credentials
- Long polling for real-time message receiving
- Typing status indicator
- Rich media support (image, voice, video, file)
- Long text splitting with intelligent boundary detection
- Middleware system (logging, rate limiting, authentication)
- Panic recovery and graceful shutdown
- Zero external dependencies
