package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// mockCursorStore records interactions for assertions.
type mockCursorStore struct {
	mu       sync.Mutex
	persist  string
	loadErr  error
	saveErr  bool
	saves    int
	loaded   int
	getCalls int
}

func (m *mockCursorStore) Get() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	if m.loadErr != nil {
		return "", m.loadErr
	}
	return m.persist, nil
}

func (m *mockCursorStore) Save(cursor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saves++
	if m.saveErr {
		return errors.New("disk full (simulated)")
	}
	m.persist = cursor
	return nil
}

func newCursorTestServer(t *testing.T, mu *sync.Mutex, cursors *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			http.NotFound(w, r)
			return
		}
		var req GetUpdatesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		*cursors = append(*cursors, req.GetUpdatesBuf)
		mu.Unlock()

		resp := GetUpdatesResponse{
			Ret:                  0,
			Messages:             []Message{},
			GetUpdatesBuf:        "cur_" + req.GetUpdatesBuf,
			LongPollingTimeoutMs: 50,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestPollerWithCursorStore_LoadsPersistedCursor(t *testing.T) {
	var mu sync.Mutex
	var cursors []string
	server := newCursorTestServer(t, &mu, &cursors)
	defer server.Close()

	store := &mockCursorStore{persist: "saved_before_restart"}
	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPollerWithCursorStore(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3", store)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = poller.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(cursors) < 2 {
		t.Fatalf("expected at least 2 poll requests, got %d", len(cursors))
	}
	if cursors[0] != "saved_before_restart" {
		t.Errorf("first request cursor = %q, want persisted %q", cursors[0], "saved_before_restart")
	}
	// next cursor derives from the restored one
	if cursors[1] != "cur_saved_before_restart" {
		t.Errorf("second request cursor = %q, want %q", cursors[1], "cur_saved_before_restart")
	}
}

func TestPollerWithCursorStore_SavesOnCursorUpdate(t *testing.T) {
	var mu sync.Mutex
	var cursors []string
	server := newCursorTestServer(t, &mu, &cursors)
	defer server.Close()

	store := &mockCursorStore{}
	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPollerWithCursorStore(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3", store)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = poller.Run(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.saves == 0 {
		t.Errorf("expected Save to be called after cursor updates, got 0")
	}
}

func TestPollerWithCursorStore_SaveFailureDoesNotBreakPolling(t *testing.T) {
	var mu sync.Mutex
	var cursors []string
	server := newCursorTestServer(t, &mu, &cursors)
	defer server.Close()

	store := &mockCursorStore{saveErr: true}
	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPollerWithCursorStore(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3", store)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := poller.Run(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() should keep polling despite save failures, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(cursors) < 2 {
		t.Errorf("polling stalled after save failure: only %d requests", len(cursors))
	}
}

func TestPollerWithCursorStore_LoadFailureStartsEmpty(t *testing.T) {
	var mu sync.Mutex
	var cursors []string
	server := newCursorTestServer(t, &mu, &cursors)
	defer server.Close()

	store := &mockCursorStore{loadErr: errors.New("corrupted store")}
	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPollerWithCursorStore(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3", store)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = poller.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(cursors) == 0 || cursors[0] != "" {
		t.Errorf("after load failure first cursor should be empty, got %v", cursors)
	}
}

func TestPollerWithCursorStore_NilStoreLegacyBehavior(t *testing.T) {
	var mu sync.Mutex
	var cursors []string
	server := newCursorTestServer(t, &mu, &cursors)
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3")

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = poller.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(cursors) < 2 {
		t.Fatalf("legacy poller should still work, got %d requests", len(cursors))
	}
	if cursors[0] != "" {
		t.Errorf("legacy first cursor = %q, want empty", cursors[0])
	}
}
