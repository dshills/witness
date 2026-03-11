package events

import (
	"strings"
	"testing"
)

func TestNewID_Prefix(t *testing.T) {
	tests := []string{"run", "evt", "stage"}
	for _, prefix := range tests {
		id := NewID(prefix)
		if !strings.HasPrefix(id, prefix+"_") {
			t.Errorf("NewID(%q) = %q, expected prefix %q_", prefix, id, prefix)
		}
	}
}

func TestNewID_Unique(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		id := NewID("test")
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNewEvent(t *testing.T) {
	evt := NewEvent("run_123", EventRunStarted, "test", []byte(`{"key":"value"}`))
	if evt.RunID != "run_123" {
		t.Errorf("RunID = %q, want %q", evt.RunID, "run_123")
	}
	if evt.Type != EventRunStarted {
		t.Errorf("Type = %q, want %q", evt.Type, EventRunStarted)
	}
	if evt.Source != "test" {
		t.Errorf("Source = %q, want %q", evt.Source, "test")
	}
	if evt.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", evt.SchemaVersion, SchemaVersion)
	}
	if evt.EventID == "" {
		t.Error("EventID should not be empty")
	}
	if evt.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}
