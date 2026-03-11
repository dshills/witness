package app

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/privacy"
)

// memStore is a minimal in-memory store for testing StoreSink.
type memStore struct {
	mu        sync.Mutex
	events    map[string][]events.Event
	runs      map[string]models.Run
	snapshots map[string][]byte
}

func newMemStore() *memStore {
	return &memStore{
		events:    make(map[string][]events.Event),
		runs:      make(map[string]models.Run),
		snapshots: make(map[string][]byte),
	}
}

func (m *memStore) CreateRun(_ context.Context, run models.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.RunID] = run
	return nil
}
func (m *memStore) GetRun(_ context.Context, runID string) (models.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[runID], nil
}
func (m *memStore) UpdateRun(_ context.Context, run models.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.RunID] = run
	return nil
}
func (m *memStore) ListRuns(_ context.Context) ([]models.Run, error) { return nil, nil }
func (m *memStore) DeleteRun(_ context.Context, _ string) error      { return nil }
func (m *memStore) AppendEvent(_ context.Context, runID string, evt events.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[runID] = append(m.events[runID], evt)
	return nil
}
func (m *memStore) ReadEvents(_ context.Context, runID string) ([]events.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events[runID], nil
}
func (m *memStore) StreamEvents(_ context.Context, _ string) (<-chan events.Event, error) {
	return nil, nil
}
func (m *memStore) SaveSnapshot(_ context.Context, runID string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots[runID] = data
	return nil
}
func (m *memStore) LoadSnapshot(_ context.Context, runID string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshots[runID], nil
}
func (m *memStore) Close() error { return nil }

func (m *memStore) SnapshotCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, v := range m.snapshots {
		if len(v) > 0 {
			count++
		}
	}
	return count
}

func (m *memStore) EventCount(runID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events[runID])
}

func TestStoreSink_Append500_TriggersSnapshot(t *testing.T) {
	st := newMemStore()
	runID := "run_test"
	run := models.Run{RunID: runID, Status: models.RunStatusRunning, StartedAt: time.Now()}
	agg := aggregate.NewAggregator(run)

	redactor, err := privacy.NewRedactor(nil)
	if err != nil {
		t.Fatal(err)
	}

	sink := NewStoreSink(st, runID, agg, redactor, nil)
	ctx := context.Background()

	// Append 500 events.
	for i := 0; i < 500; i++ {
		evt := events.NewEvent(runID, events.EventNoteRecorded, "test", json.RawMessage(`{"note":"test"}`))
		if err := sink.Append(ctx, evt); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	// Verify snapshot was saved after 500th event.
	if st.SnapshotCount() == 0 {
		t.Error("expected snapshot to be saved after 500 events")
	}

	// Verify all 500 events were stored.
	if st.EventCount(runID) != 500 {
		t.Errorf("expected 500 stored events, got %d", st.EventCount(runID))
	}

	// Verify aggregator count.
	snap := agg.Snapshot()
	if snap.EventCount != 500 {
		t.Errorf("expected aggregator event count 500, got %d", snap.EventCount)
	}
}

func TestStoreSink_Redaction(t *testing.T) {
	st := newMemStore()
	runID := "run_redact"
	run := models.Run{RunID: runID, Status: models.RunStatusRunning, StartedAt: time.Now()}
	agg := aggregate.NewAggregator(run)

	redactor, err := privacy.NewRedactor([]string{`sk-[a-zA-Z0-9]{20,}`})
	if err != nil {
		t.Fatal(err)
	}

	sink := NewStoreSink(st, runID, agg, redactor, nil)
	ctx := context.Background()

	payload, _ := json.Marshal(map[string]string{
		"api_key": "sk-abcdefghijklmnopqrstuvwxyz",
		"other":   "safe value",
	})
	evt := events.NewEvent(runID, events.EventNoteRecorded, "test", payload)
	evt.Summary = "Used key sk-abcdefghijklmnopqrstuvwxyz"

	if err := sink.Append(ctx, evt); err != nil {
		t.Fatal(err)
	}

	stored := st.events[runID][0]
	if stored.Summary != "Used key [REDACTED]" {
		t.Errorf("expected summary to be redacted, got %q", stored.Summary)
	}

	var p map[string]string
	_ = json.Unmarshal(stored.Payload, &p)
	if p["api_key"] != "[REDACTED]" {
		t.Errorf("expected api_key redacted, got %q", p["api_key"])
	}
	if p["other"] != "safe value" {
		t.Errorf("expected other field unchanged, got %q", p["other"])
	}
}
