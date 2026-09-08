package wechat

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFileCursorStore_SaveAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cursor.txt")
	store := NewFileCursorStore(path)

	// Get on missing file returns empty, nil
	cur, err := store.Get()
	if err != nil {
		t.Fatalf("Get() on missing file error = %v", err)
	}
	if cur != "" {
		t.Errorf("Get() on missing file = %q, want empty", cur)
	}

	// Save then read back
	if err := store.Save("cursor_abc_123"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	cur, err = store.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if cur != "cursor_abc_123" {
		t.Errorf("Get() = %q, want %q", cur, "cursor_abc_123")
	}

	// Overwrite
	if err := store.Save("cursor_def_456"); err != nil {
		t.Fatalf("Save() overwrite error = %v", err)
	}
	cur, _ = store.Get()
	if cur != "cursor_def_456" {
		t.Errorf("Get() after overwrite = %q, want %q", cur, "cursor_def_456")
	}
}

func TestFileCursorStore_FilePerm0600(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "cursor.txt")
	store := NewFileCursorStore(path)

	if err := store.Save("secret_cursor"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("file perm = %v, want %v", got, os.FileMode(0o600))
	}
}

func TestFileCursorStore_SpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileCursorStore(filepath.Join(tmpDir, "cursor.txt"))

	cases := []string{
		"curs or with space",
		"cursor/with/slashes",
		"中文游标+emoji🙂",
		strings.Repeat("x", 4096),
	}
	for _, c := range cases {
		if err := store.Save(c); err != nil {
			t.Fatalf("Save(%q...) error = %v", c[:8], err)
		}
		got, err := store.Get()
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got != c {
			t.Errorf("roundtrip mismatch: got %d bytes, want %d bytes", len(got), len(c))
		}
	}
}

func TestFileCursorStore_ConcurrentSave(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileCursorStore(filepath.Join(tmpDir, "cursor.txt"))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = store.Save("cursor_from_goroutine")
			_, _ = store.Get()
		}(i)
	}
	wg.Wait()

	// No assertion on final value (any of the concurrent writes may win);
	// -race will flag data races if the mutex is broken.
	if _, err := store.Get(); err != nil {
		t.Fatalf("Get() after concurrent writes error = %v", err)
	}
}

func TestFileCursorStore_EmptyCursor(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileCursorStore(filepath.Join(tmpDir, "cursor.txt"))

	if err := store.Save("nonempty"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save(""); err != nil {
		t.Fatalf("Save(empty) error = %v", err)
	}
	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "" {
		t.Errorf("Get() = %q, want empty (explicit clear)", got)
	}
}
