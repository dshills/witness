package fsstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

func testRun(id string) models.Run {
	return models.Run{
		RunID:     id,
		Name:      "test-run",
		Status:    models.RunStatusRunning,
		StartedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func testEvent(runID string, n int) events.Event {
	return events.Event{
		EventID:       fmt.Sprintf("evt_%d", n),
		SchemaVersion: events.SchemaVersion,
		Timestamp:     time.Now().UTC(),
		RunID:         runID,
		Type:          events.EventToolStarted,
		Source:        "test",
		Payload:       json.RawMessage(`{"n":` + fmt.Sprintf("%d", n) + `}`),
	}
}

func TestCreateAndGetRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	run := testRun("run_001")

	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetRun(ctx, "run_001")
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != run.RunID {
		t.Errorf("RunID = %q, want %q", got.RunID, run.RunID)
	}
	if got.Name != run.Name {
		t.Errorf("Name = %q, want %q", got.Name, run.Name)
	}
	if got.Status != run.Status {
		t.Errorf("Status = %v, want %v", got.Status, run.Status)
	}
}

func TestUpdateRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	run := testRun("run_002")

	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}

	run.Status = models.RunStatusCompleted
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetRun(ctx, "run_002")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.RunStatusCompleted {
		t.Errorf("Status = %v, want completed", got.Status)
	}
}

func TestAppendAndReadEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	runID := "run_003"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	const count = 100
	for i := range count {
		if err := s.AppendEvent(ctx, runID, testEvent(runID, i)); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}

	evts, err := s.ReadEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evts) != count {
		t.Fatalf("got %d events, want %d", len(evts), count)
	}

	// Verify order
	for i, evt := range evts {
		want := fmt.Sprintf("evt_%d", i)
		if evt.EventID != want {
			t.Errorf("event %d: ID = %q, want %q", i, evt.EventID, want)
		}
	}
}

func TestDeduplication(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	runID := "run_004"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	evt := testEvent(runID, 1)
	if err := s.AppendEvent(ctx, runID, evt); err != nil {
		t.Fatal(err)
	}
	// Append same event again
	if err := s.AppendEvent(ctx, runID, evt); err != nil {
		t.Fatal(err)
	}

	evts, err := s.ReadEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evts) != 1 {
		t.Fatalf("got %d events, want 1 (dedup should have prevented duplicate)", len(evts))
	}
}

func TestCrashTolerance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	runID := "run_005"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	// Write 3 valid events
	for i := range 3 {
		if err := s.AppendEvent(ctx, runID, testEvent(runID, i)); err != nil {
			t.Fatal(err)
		}
	}

	// Simulate a crash: append a partial (invalid) final line
	eventsPath := filepath.Join(s.runDir(runID), eventsFile)
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"event_id":"evt_partial","schema_ver`); err != nil {
		t.Fatal(err)
	}
	f.Close() //nolint:errcheck // test helper

	evts, err := s.ReadEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evts) != 3 {
		t.Fatalf("got %d events, want 3 (partial line should be discarded)", len(evts))
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	runID := "run_006"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	data := []byte(`{"status":"running","events_count":42}`)
	if err := s.SaveSnapshot(ctx, runID, data); err != nil {
		t.Fatal(err)
	}

	got, err := s.LoadSnapshot(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("snapshot data mismatch: got %q, want %q", got, data)
	}

	// Verify atomicity: no .tmp file should remain
	tmpPath := filepath.Join(s.runDir(runID), snapshotFile+".tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after successful snapshot write")
	}
}

func TestListRuns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create runs with different start times
	for i := range 3 {
		run := models.Run{
			RunID:     fmt.Sprintf("run_%d", i),
			Name:      fmt.Sprintf("run-%d", i),
			Status:    models.RunStatusRunning,
			StartedAt: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC),
		}
		if err := s.CreateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}

	runs, err := s.ListRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("got %d runs, want 3", len(runs))
	}

	// Verify sorted by start time
	for i := 1; i < len(runs); i++ {
		if !runs[i-1].StartedAt.Before(runs[i].StartedAt) {
			t.Errorf("runs not sorted: %v >= %v", runs[i-1].StartedAt, runs[i].StartedAt)
		}
	}
}

func TestStreamEventsExistingAndNew(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runID := "run_007"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	// Write some events before streaming
	for i := range 3 {
		if err := s.AppendEvent(ctx, runID, testEvent(runID, i)); err != nil {
			t.Fatal(err)
		}
	}

	ch, err := s.StreamEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}

	// Read the 3 existing events
	for i := range 3 {
		select {
		case evt := <-ch:
			want := fmt.Sprintf("evt_%d", i)
			if evt.EventID != want {
				t.Errorf("existing event %d: ID = %q, want %q", i, evt.EventID, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for existing event %d", i)
		}
	}

	// Append a new event and verify it arrives
	if err := s.AppendEvent(ctx, runID, testEvent(runID, 99)); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-ch:
		if evt.EventID != "evt_99" {
			t.Errorf("new event: ID = %q, want evt_99", evt.EventID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for new event")
	}
}

func TestStreamEventsCancellation(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	runID := "run_008"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	ch, err := s.StreamEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}

	cancel()

	// Channel should close
	select {
	case _, ok := <-ch:
		if ok {
			// Might get an event, drain until closed
			for range ch {
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close after cancel")
	}
}

func TestSlowConsumerDoesNotBlock(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runID := "run_009"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	ch, err := s.StreamEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}

	// Fill the buffer without reading
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range streamBufCap + 50 {
			if err := s.AppendEvent(ctx, runID, testEvent(runID, i)); err != nil {
				return
			}
		}
	}()

	select {
	case <-done:
		// AppendEvent did not block — good
	case <-time.After(5 * time.Second):
		t.Fatal("AppendEvent blocked on slow consumer")
	}

	// Drain whatever we can from the channel
	_ = ch
}

func TestDeleteRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	runID := "run_010"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, runID, testEvent(runID, 1)); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteRun(ctx, runID); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(s.runDir(runID)); !os.IsNotExist(err) {
		t.Error("run directory should not exist after delete")
	}
}

func TestConcurrentAppends(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	runID := "run_011"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	const goroutines = 10
	const eventsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(gID int) {
			defer wg.Done()
			for i := range eventsPerGoroutine {
				evt := testEvent(runID, gID*1000+i)
				if err := s.AppendEvent(ctx, runID, evt); err != nil {
					t.Errorf("goroutine %d, event %d: %v", gID, i, err)
				}
			}
		}(g)
	}
	wg.Wait()

	evts, err := s.ReadEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evts) != goroutines*eventsPerGoroutine {
		t.Errorf("got %d events, want %d", len(evts), goroutines*eventsPerGoroutine)
	}
}

func TestEnsureStorageDir(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "witness-data")

	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = s

	runsDir := filepath.Join(root, "runs")
	info, err := os.Stat(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("runs should be a directory")
	}
}

func TestReadEventsEmptyFile(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	runID := "run_012"
	if err := s.CreateRun(ctx, testRun(runID)); err != nil {
		t.Fatal(err)
	}

	evts, err := s.ReadEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evts) != 0 {
		t.Errorf("expected 0 events, got %d", len(evts))
	}
}

func newTestStore(t *testing.T) *FSStore {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
