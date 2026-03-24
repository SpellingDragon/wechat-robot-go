package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileContextTokenStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContextTokenStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	// Save a token
	err = store.Save("user1@im.wechat", "token-abc123")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load the token
	token, err := store.Load("user1@im.wechat")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if token != "token-abc123" {
		t.Errorf("Load() = %q, want %q", token, "token-abc123")
	}
}

func TestFileContextTokenStore_LoadNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContextTokenStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	token, err := store.Load("nonexistent-user")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if token != "" {
		t.Errorf("Load() = %q, want empty string", token)
	}
}

func TestFileContextTokenStore_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContextTokenStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	// Save and then clear
	err = store.Save("user1", "token-123")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err = store.Clear("user1")
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	token, err := store.Load("user1")
	if err != nil {
		t.Fatalf("Load() after clear error = %v", err)
	}
	if token != "" {
		t.Errorf("Load() after clear = %q, want empty string", token)
	}
}

func TestFileContextTokenStore_ClearAll(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContextTokenStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	// Save multiple tokens
	for i := 0; i < 5; i++ {
		userID := "user" + string(rune('0'+i))
		err = store.Save(userID, "token-"+userID)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// Clear all
	err = store.ClearAll()
	if err != nil {
		t.Fatalf("ClearAll() error = %v", err)
	}

	// Verify all are cleared
	for i := 0; i < 5; i++ {
		userID := "user" + string(rune('0'+i))
		token, err := store.Load(userID)
		if err != nil {
			t.Fatalf("Load() after clear error = %v", err)
		}
		if token != "" {
			t.Errorf("Load() after clear for %s = %q, want empty", userID, token)
		}
	}
}

func TestFileContextTokenStore_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContextTokenStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	var wg sync.WaitGroup
	// Concurrent saves and loads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			userID := "user" + string(rune('a'+idx%26))
			token := "token-" + userID

			// Save
			err := store.Save(userID, token)
			if err != nil {
				t.Errorf("Save() error = %v", err)
			}

			// Load
			loaded, err := store.Load(userID)
			if err != nil {
				t.Errorf("Load() error = %v", err)
			}
			if loaded != token {
				t.Errorf("Load() = %q, want %q", loaded, token)
			}
		}(i)
	}
	wg.Wait()
}

func TestFileContextTokenStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens")

	// Create store and save
	store1, err := NewFileContextTokenStore(path)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	err = store1.Save("user1", "persistent-token")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create new store from same path (simulating restart)
	store2, err := NewFileContextTokenStore(path)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	token, err := store2.Load("user1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if token != "persistent-token" {
		t.Errorf("Load() = %q, want %q", token, "persistent-token")
	}
}

func TestFileContextTokenStore_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContextTokenStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	err = store.Save("user1", "token")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Check file permissions
	filePath := filepath.Join(tmpDir, "user1.json")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestMemoryContextTokenStore(t *testing.T) {
	store := NewMemoryContextTokenStore()

	// Save
	err := store.Save("user1", "token-abc")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	token, err := store.Load("user1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if token != "token-abc" {
		t.Errorf("Load() = %q, want %q", token, "token-abc")
	}

	// Clear
	err = store.Clear("user1")
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	token, err = store.Load("user1")
	if err != nil {
		t.Fatalf("Load() after clear error = %v", err)
	}
	if token != "" {
		t.Errorf("Load() after clear = %q, want empty", token)
	}

	// ClearAll
	if err := store.Save("user1", "token1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save("user2", "token2"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	err = store.ClearAll()
	if err != nil {
		t.Fatalf("ClearAll() error = %v", err)
	}

	token1, _ := store.Load("user1")
	token2, _ := store.Load("user2")
	if token1 != "" || token2 != "" {
		t.Errorf("ClearAll() failed: user1=%q, user2=%q", token1, token2)
	}
}

func TestMemoryContextTokenStore_Concurrent(t *testing.T) {
	store := NewMemoryContextTokenStore()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			userID := "user" + string(rune('0'+idx))
			token := "token-" + userID

			err := store.Save(userID, token)
			if err != nil {
				t.Errorf("Save() error = %v", err)
			}

			loaded, err := store.Load(userID)
			if err != nil {
				t.Errorf("Load() error = %v", err)
			}
			if loaded != token {
				t.Errorf("Load() = %q, want %q", loaded, token)
			}
		}(i)
	}
	wg.Wait()
}

func TestFileContextTokenStore_CleanExpired(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContextTokenStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContextTokenStore() error = %v", err)
	}

	// Save tokens
	if err := store.Save("user1", "token1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save("user2", "token2"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Count should be 2
	if c := store.Count(); c != 2 {
		t.Errorf("Count() = %d, want 2", c)
	}

	// No tokens expired yet (maxAge = 1 hour)
	removed, err := store.CleanExpired(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanExpired() error = %v", err)
	}
	if removed != 0 {
		t.Errorf("CleanExpired(1h) removed %d, want 0", removed)
	}

	// All tokens expired (maxAge = 0)
	removed, err = store.CleanExpired(0)
	if err != nil {
		t.Fatalf("CleanExpired() error = %v", err)
	}
	if removed != 2 {
		t.Errorf("CleanExpired(0) removed %d, want 2", removed)
	}

	// Count should be 0
	if c := store.Count(); c != 0 {
		t.Errorf("Count() = %d, want 0", c)
	}

	// Verify files are deleted
	entries, _ := os.ReadDir(tmpDir)
	jsonCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			jsonCount++
		}
	}
	if jsonCount != 0 {
		t.Errorf("expected 0 json files after cleanup, got %d", jsonCount)
	}
}

func TestMemoryContextTokenStore_CleanExpired(t *testing.T) {
	store := NewMemoryContextTokenStore()

	// Save tokens
	if err := store.Save("user1", "token1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save("user2", "token2"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if c := store.Count(); c != 2 {
		t.Errorf("Count() = %d, want 2", c)
	}

	// No tokens expired (maxAge = 1 hour)
	removed, err := store.CleanExpired(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanExpired() error = %v", err)
	}
	if removed != 0 {
		t.Errorf("CleanExpired(1h) removed %d, want 0", removed)
	}

	// All tokens expired (maxAge = 0)
	removed, err = store.CleanExpired(0)
	if err != nil {
		t.Fatalf("CleanExpired() error = %v", err)
	}
	if removed != 2 {
		t.Errorf("CleanExpired(0) removed %d, want 2", removed)
	}

	if c := store.Count(); c != 0 {
		t.Errorf("Count() = %d, want 0", c)
	}
}
