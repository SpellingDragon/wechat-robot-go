package wechat

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CursorStore is the interface for persisting the getupdates cursor so that
// the poller can resume from where it left off after a restart.
type CursorStore interface {
	// Get returns the persisted cursor. Returns ("", nil) if no cursor exists.
	Get() (string, error)
	// Save persists the cursor.
	Save(cursor string) error
}

// FileCursorStore implements CursorStore by persisting the cursor to a plain
// text file (single line, no trailing newline guarantees).
type FileCursorStore struct {
	path string
	mu   sync.Mutex
}

// NewFileCursorStore creates a new FileCursorStore with the given file path.
func NewFileCursorStore(path string) *FileCursorStore {
	return &FileCursorStore{path: path}
}

// Get reads the cursor from the file.
// Returns ("", nil) if the file does not exist.
func (f *FileCursorStore) Get() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read cursor file: %w", err)
	}

	cursor := string(data)
	// Trim a single trailing newline if present (defensive: plain text store).
	cursor = trimTrailingNewline(cursor)
	return cursor, nil
}

// Save writes the cursor to the file atomically (tmp+rename) with 0600 perms.
// An empty cursor is persisted as an empty file so that a previously saved
// cursor can be explicitly cleared.
func (f *FileCursorStore) Save(cursor string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("create cursor dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".cursor-*")
	if err != nil {
		return fmt.Errorf("create cursor tmp file: %w", err)
	}
	tmpName := tmp.Name()

	if err := os.Chmod(tmpName, 0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod cursor tmp file: %w", err)
	}

	if _, err := tmp.WriteString(cursor); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write cursor: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close cursor tmp file: %w", err)
	}
	if err := os.Rename(tmpName, f.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename cursor file: %w", err)
	}
	return nil
}

func trimTrailingNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
