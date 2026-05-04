# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
