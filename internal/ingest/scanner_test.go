package ingest

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dshills/witness/internal/events"
)

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

func TestScanner_WitnessJSONLine(t *testing.T) {
	evt := events.Event{
		EventID:       "evt_test123",
		SchemaVersion: events.SchemaVersion,
		Timestamp:     time.Now().UTC(),
		RunID:         "run_other",
		Type:          events.EventNoteRecorded,
		Source:        "test",
		Payload:       json.RawMessage(`{"message":"hello"}`),
	}
	line, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}

	input := string(line) + "\n"
	sink := &mockSink{}
	sc := NewScanner(strings.NewReader(input), sink, "run_current")

	if err := sc.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink.events))
	}

	got := sink.events[0]
	if got.RunID != "run_current" {
		t.Errorf("expected run_id overridden to run_current, got %s", got.RunID)
	}
	if got.EventID != "evt_test123" {
		t.Errorf("expected event_id preserved, got %s", got.EventID)
	}
}

func TestScanner_RawLinePassedThrough(t *testing.T) {
	// Non-JSON lines should be silently ignored.
	input := "some regular output\nnot json at all\n{}\n"
	sink := &mockSink{}
	sc := NewScanner(strings.NewReader(input), sink, "run_test")

	if err := sc.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()

	// {} parses as JSON but has no event_id or type, so should be skipped.
	if len(sink.events) != 0 {
		t.Errorf("expected 0 events for non-witness lines, got %d", len(sink.events))
	}
}

func TestScanner_MixedInput(t *testing.T) {
	evt := events.Event{
		EventID:       "evt_abc",
		SchemaVersion: events.SchemaVersion,
		Timestamp:     time.Now().UTC(),
		RunID:         "run_x",
		Type:          events.EventToolStarted,
		Source:        "tool",
		Payload:       json.RawMessage(`{"tool_name":"make"}`),
	}
	evtLine, _ := json.Marshal(evt)

	input := "Building project...\n" + string(evtLine) + "\nDone.\n"
	sink := &mockSink{}
	sc := NewScanner(strings.NewReader(input), sink, "run_mine")

	if err := sc.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.events) != 1 {
		t.Fatalf("expected 1 witness event among mixed output, got %d", len(sink.events))
	}
	if sink.events[0].RunID != "run_mine" {
		t.Errorf("expected run_id=run_mine, got %s", sink.events[0].RunID)
	}
}
