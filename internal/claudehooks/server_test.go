package claudehooks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dshills/witness/internal/events"
)

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

func (m *mockSink) Events() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	dst := make([]events.Event, len(m.events))
	copy(dst, m.events)
	return dst
}

func TestServerReceivesHook(t *testing.T) {
	sink := &mockSink{}
	srv, err := NewServer(sink, "run_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// Send a PreToolUse hook.
	payload := HookPayload{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolUseID:     "tooluse_test",
		ToolInput:     json.RawMessage(`{"command":"echo hello"}`),
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(srv.URL()+"/hook", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	evts := sink.Events()
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}
	if evts[0].Type != events.EventToolStarted {
		t.Errorf("expected tool.started, got %s", evts[0].Type)
	}
}

func TestServerHealth(t *testing.T) {
	sink := &mockSink{}
	srv, err := NewServer(sink, "run_health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	resp, err := http.Get(srv.URL() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestServerRejectsGet(t *testing.T) {
	sink := &mockSink{}
	srv, err := NewServer(sink, "run_reject")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	resp, err := http.Get(srv.URL() + "/hook")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestHooksSettingsForAddr(t *testing.T) {
	settings := HooksSettingsForAddr("127.0.0.1:9999")
	if settings == "" {
		t.Fatal("expected non-empty settings")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("settings is not valid JSON: %v", err)
	}

	hooks, ok := parsed["hooks"].(map[string]any)
	if !ok {
		t.Fatal("expected hooks key")
	}

	expectedHooks := []string{"PreToolUse", "PostToolUse", "SubagentStart", "SubagentStop", "UserPromptSubmit", "Stop"}
	for _, name := range expectedHooks {
		if _, ok := hooks[name]; !ok {
			t.Errorf("missing hook: %s", name)
		}
	}
}
