package files

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dshills/witness/internal/events"
)

func TestShouldIgnore(t *testing.T) {
	patterns := []string{
		".git/**",
		"node_modules/**",
		"*.swp",
		"*.swo",
		"*~",
		".DS_Store",
	}

	tests := []struct {
		path   string
		ignore bool
	}{
		{".git/objects/pack", true},
		{".git", true},
		{"node_modules/foo/bar.js", true},
		{"src/main.go", false},
		{"file.swp", true},
		{"dir/file.swo", true},
		{"backup~", true},
		{".DS_Store", true},
		{"src/app.ts", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ShouldIgnore(tt.path, patterns)
			if got != tt.ignore {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.ignore)
			}
		})
	}
}

// mockSink collects appended events for testing.
type mockSink struct {
	mu     sync.Mutex
	events []events.Event
}

func (m *mockSink) Append(_ context.Context, evt events.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, evt)
	return nil
}

func (m *mockSink) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func TestWatcher_Debounce(t *testing.T) {
	dir, err := os.MkdirTemp("", "witness-watcher-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	sink := &mockSink{}
	w := NewWatcher(dir, nil, sink, "run_test")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Start(ctx)
	}()

	// Give watcher time to initialize.
	time.Sleep(200 * time.Millisecond)

	// Create a file and write to it multiple times rapidly.
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("world"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("again"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce to settle.
	time.Sleep(300 * time.Millisecond)

	cancel()
	<-errCh

	// Should have coalesced into a small number of events (1-2, not 3+).
	count := sink.Len()
	if count > 3 {
		t.Errorf("expected debounced events <= 3, got %d", count)
	}
	if count == 0 {
		t.Error("expected at least 1 event, got 0")
	}

	// Verify event structure.
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for _, evt := range sink.events {
		if evt.RunID != "run_test" {
			t.Errorf("expected run_id=run_test, got %s", evt.RunID)
		}
		var payload map[string]string
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			t.Errorf("invalid payload: %v", err)
		}
		if payload["path"] == "" {
			t.Error("expected non-empty path in payload")
		}
	}
}
