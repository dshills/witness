package replay

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// testRun returns a minimal Run for testing.
func testRun() models.Run {
	return models.Run{
		RunID:  "run_test",
		Name:   "test-run",
		Status: models.RunStatusPending,
	}
}

// testEvents generates a sequence of events for testing the replay controller.
func testEvents() []events.Event {
	t0 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	mustJSON := func(v any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}

	return []events.Event{
		{
			EventID:   "evt_0",
			RunID:     "run_test",
			Type:      events.EventRunCreated,
			Timestamp: t0,
			Source:    "test",
			Summary:   "run created",
			Payload: mustJSON(map[string]any{
				"name": "test-run",
			}),
		},
		{
			EventID:   "evt_1",
			RunID:     "run_test",
			Type:      events.EventRunStarted,
			Timestamp: t0.Add(1 * time.Second),
			Source:    "test",
			Summary:   "run started",
		},
		{
			EventID:   "evt_2",
			RunID:     "run_test",
			Type:      events.EventStageCreated,
			Timestamp: t0.Add(2 * time.Second),
			Source:    "test",
			Summary:   "stage created",
			StageID:   "stage_1",
			Payload: mustJSON(map[string]any{
				"stage_id": "stage_1",
				"name":     "setup",
				"order":    0,
			}),
		},
		{
			EventID:   "evt_3",
			RunID:     "run_test",
			Type:      events.EventStageStarted,
			Timestamp: t0.Add(3 * time.Second),
			Source:    "test",
			Summary:   "stage started",
			StageID:   "stage_1",
		},
		{
			EventID:   "evt_4",
			RunID:     "run_test",
			Type:      events.EventAlertRaised,
			Timestamp: t0.Add(4 * time.Second),
			Source:    "test",
			Summary:   "alert raised",
			Payload: mustJSON(map[string]any{
				"alert_id":    "alert_1",
				"severity":    "warning",
				"type":        "budget",
				"title":       "Budget Warning",
				"description": "Approaching budget limit",
			}),
		},
		{
			EventID:   "evt_5",
			RunID:     "run_test",
			Type:      events.EventStageCompleted,
			Timestamp: t0.Add(5 * time.Second),
			Source:    "test",
			Summary:   "stage completed",
			StageID:   "stage_1",
		},
		{
			EventID:   "evt_6",
			RunID:     "run_test",
			Type:      events.EventStageCreated,
			Timestamp: t0.Add(6 * time.Second),
			Source:    "test",
			Summary:   "stage 2 created",
			StageID:   "stage_2",
			Payload: mustJSON(map[string]any{
				"stage_id": "stage_2",
				"name":     "build",
				"order":    1,
			}),
		},
		{
			EventID:   "evt_7",
			RunID:     "run_test",
			Type:      events.EventGitCommitCreated,
			Timestamp: t0.Add(7 * time.Second),
			Source:    "test",
			Summary:   "commit created",
			Payload: mustJSON(map[string]any{
				"commit_id": "commit_1",
				"sha":       "abc1234def5678",
				"message":   "initial commit",
			}),
		},
		{
			EventID:   "evt_8",
			RunID:     "run_test",
			Type:      events.EventStageStarted,
			Timestamp: t0.Add(8 * time.Second),
			Source:    "test",
			Summary:   "stage 2 started",
			StageID:   "stage_2",
		},
		{
			EventID:   "evt_9",
			RunID:     "run_test",
			Type:      events.EventRunCompleted,
			Timestamp: t0.Add(10 * time.Second),
			Source:    "test",
			Summary:   "run completed",
		},
	}
}

func TestStepForward(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Step through first few events.
	for i := 0; i < 3; i++ {
		evt, err := ctrl.StepForward()
		if err != nil {
			t.Fatalf("StepForward[%d]: %v", i, err)
		}
		if evt.EventID != evts[i].EventID {
			t.Errorf("StepForward[%d]: got event %s, want %s", i, evt.EventID, evts[i].EventID)
		}
	}

	current, total := ctrl.Progress()
	if current != 2 {
		t.Errorf("Progress current = %d, want 2", current)
	}
	if total != len(evts) {
		t.Errorf("Progress total = %d, want %d", total, len(evts))
	}
}

func TestStepForwardAtEnd(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Advance to the end.
	for range evts {
		_, _ = ctrl.StepForward()
	}

	_, err := ctrl.StepForward()
	if !errors.Is(err, ErrAtEnd) {
		t.Errorf("StepForward at end: got %v, want ErrAtEnd", err)
	}
}

func TestStepForwardNoEvents(t *testing.T) {
	ctrl := NewController(testRun(), nil)
	_, err := ctrl.StepForward()
	if !errors.Is(err, ErrNoEvents) {
		t.Errorf("StepForward no events: got %v, want ErrNoEvents", err)
	}
}

func TestStepBackward(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Advance 5 events.
	for i := 0; i < 5; i++ {
		_, _ = ctrl.StepForward()
	}

	// Step backward.
	if err := ctrl.StepBackward(); err != nil {
		t.Fatalf("StepBackward: %v", err)
	}

	current, _ := ctrl.Progress()
	if current != 3 {
		t.Errorf("after StepBackward, current = %d, want 3", current)
	}

	// Verify state matches what Rebuild produces for the first 4 events.
	expected, err := aggregate.Rebuild(run, evts[:4])
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	got := ctrl.CurrentState()
	if got.EventCount != expected.EventCount {
		t.Errorf("EventCount = %d, want %d", got.EventCount, expected.EventCount)
	}
}

func TestStepBackwardToBeforeStart(t *testing.T) {
	ctrl := NewController(testRun(), testEvents())

	// Advance one event, then step back.
	_, _ = ctrl.StepForward()
	if err := ctrl.StepBackward(); err != nil {
		t.Fatalf("StepBackward to before start: %v", err)
	}

	current, _ := ctrl.Progress()
	if current != -1 {
		t.Errorf("after StepBackward to start, current = %d, want -1", current)
	}
}

func TestStepBackwardAtStart(t *testing.T) {
	ctrl := NewController(testRun(), testEvents())

	err := ctrl.StepBackward()
	if !errors.Is(err, ErrAtStart) {
		t.Errorf("StepBackward at start: got %v, want ErrAtStart", err)
	}
}

func TestJumpToIndex(t *testing.T) {
	run := testRun()
	evts := testEvents()

	tests := []struct {
		name      string
		jumpTo    int
		wantErr   error
		wantIndex int
	}{
		{"first event", 0, nil, 0},
		{"last event", len(evts) - 1, nil, len(evts) - 1},
		{"middle event", 5, nil, 5},
		{"negative index", -1, ErrIndexOutOfRange, -1},
		{"beyond end", len(evts), ErrIndexOutOfRange, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := NewController(run, evts)
			err := ctrl.JumpToIndex(tt.jumpTo)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("JumpToIndex(%d): got %v, want %v", tt.jumpTo, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("JumpToIndex(%d): %v", tt.jumpTo, err)
			}
			current, _ := ctrl.Progress()
			if current != tt.wantIndex {
				t.Errorf("after JumpToIndex(%d), current = %d, want %d", tt.jumpTo, current, tt.wantIndex)
			}
		})
	}
}

func TestJumpToIndexBackward(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Jump forward first.
	if err := ctrl.JumpToIndex(7); err != nil {
		t.Fatalf("JumpToIndex(7): %v", err)
	}

	// Jump backward.
	if err := ctrl.JumpToIndex(2); err != nil {
		t.Fatalf("JumpToIndex(2): %v", err)
	}

	current, _ := ctrl.Progress()
	if current != 2 {
		t.Errorf("after backward jump, current = %d, want 2", current)
	}

	// Verify state matches rebuild.
	expected, _ := aggregate.Rebuild(run, evts[:3])
	got := ctrl.CurrentState()
	if got.EventCount != expected.EventCount {
		t.Errorf("EventCount = %d, want %d", got.EventCount, expected.EventCount)
	}
}

func TestJumpToNextStageTransition(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// From start, first stage transition should be EventStageCreated (index 2).
	if err := ctrl.JumpToNextStageTransition(); err != nil {
		t.Fatalf("JumpToNextStageTransition: %v", err)
	}

	evt := ctrl.CurrentEvent()
	if evt == nil {
		t.Fatal("CurrentEvent is nil")
	}
	if evt.Type != events.EventStageCreated {
		t.Errorf("event type = %s, want %s", evt.Type, events.EventStageCreated)
	}

	// Next stage transition: EventStageStarted (index 3).
	if err := ctrl.JumpToNextStageTransition(); err != nil {
		t.Fatalf("JumpToNextStageTransition: %v", err)
	}
	evt = ctrl.CurrentEvent()
	if evt.Type != events.EventStageStarted {
		t.Errorf("event type = %s, want %s", evt.Type, events.EventStageStarted)
	}
}

func TestJumpToNextAlert(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	if err := ctrl.JumpToNextAlert(); err != nil {
		t.Fatalf("JumpToNextAlert: %v", err)
	}

	evt := ctrl.CurrentEvent()
	if evt == nil {
		t.Fatal("CurrentEvent is nil")
	}
	if evt.Type != events.EventAlertRaised {
		t.Errorf("event type = %s, want %s", evt.Type, events.EventAlertRaised)
	}

	// No more alerts.
	err := ctrl.JumpToNextAlert()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("JumpToNextAlert after last: got %v, want ErrNotFound", err)
	}
}

func TestJumpToNextCommit(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	if err := ctrl.JumpToNextCommit(); err != nil {
		t.Fatalf("JumpToNextCommit: %v", err)
	}

	evt := ctrl.CurrentEvent()
	if evt == nil {
		t.Fatal("CurrentEvent is nil")
	}
	if evt.Type != events.EventGitCommitCreated {
		t.Errorf("event type = %s, want %s", evt.Type, events.EventGitCommitCreated)
	}
}

func TestJumpToPrevStageTransition(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Jump to index 8 (stage_2 started).
	if err := ctrl.JumpToIndex(8); err != nil {
		t.Fatalf("JumpToIndex(8): %v", err)
	}

	// Previous stage transition should be stage_2 created (index 6).
	if err := ctrl.JumpToPrevStageTransition(); err != nil {
		t.Fatalf("JumpToPrevStageTransition: %v", err)
	}

	current, _ := ctrl.Progress()
	evt := ctrl.CurrentEvent()
	if current != 6 {
		t.Errorf("current = %d, want 6", current)
	}
	if evt.Type != events.EventStageCreated {
		t.Errorf("event type = %s, want %s", evt.Type, events.EventStageCreated)
	}
}

func TestCurrentState(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Before any events.
	state := ctrl.CurrentState()
	if state.Run.RunID != "run_test" {
		t.Errorf("RunID = %s, want run_test", state.Run.RunID)
	}

	// After all events.
	for range evts {
		_, _ = ctrl.StepForward()
	}

	state = ctrl.CurrentState()
	if state.Run.Status != models.RunStatusCompleted {
		t.Errorf("Status = %s, want completed", state.Run.Status)
	}
	if state.EventCount != int64(len(evts)) {
		t.Errorf("EventCount = %d, want %d", state.EventCount, len(evts))
	}

	// Verify state matches full rebuild.
	expected, _ := aggregate.Rebuild(run, evts)
	if state.EventCount != expected.EventCount {
		t.Errorf("EventCount mismatch: got %d, want %d", state.EventCount, expected.EventCount)
	}
	if len(state.Stages) != len(expected.Stages) {
		t.Errorf("Stages count: got %d, want %d", len(state.Stages), len(expected.Stages))
	}
}

func TestPlayWithSpeedZero(t *testing.T) {
	ctrl := NewController(testRun(), testEvents())
	ctrl.SetSpeed(0)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ctrl.Play(ctx)

	// Speed 0 should not advance.
	time.Sleep(50 * time.Millisecond)
	current, _ := ctrl.Progress()
	if current != -1 {
		t.Errorf("with speed=0, current = %d, want -1", current)
	}
	if ctrl.IsPlaying() {
		t.Error("with speed=0, IsPlaying should be false")
	}
}

func TestPlayAndPause(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)
	ctrl.SetSpeed(1000) // Very fast.

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ctrl.Play(ctx)

	// Wait for some events to be processed.
	time.Sleep(200 * time.Millisecond)

	ctrl.Pause()

	if ctrl.IsPlaying() {
		t.Error("after Pause, IsPlaying should be false")
	}

	current, _ := ctrl.Progress()
	if current < 0 {
		t.Error("expected some events to have been processed")
	}
}

func TestIsPlaying(t *testing.T) {
	ctrl := NewController(testRun(), testEvents())
	if ctrl.IsPlaying() {
		t.Error("new controller should not be playing")
	}
}

func TestProgressBeforeStart(t *testing.T) {
	evts := testEvents()
	ctrl := NewController(testRun(), evts)

	current, total := ctrl.Progress()
	if current != -1 {
		t.Errorf("initial current = %d, want -1", current)
	}
	if total != len(evts) {
		t.Errorf("total = %d, want %d", total, len(evts))
	}
}

func TestCurrentEventBeforeStart(t *testing.T) {
	ctrl := NewController(testRun(), testEvents())
	if evt := ctrl.CurrentEvent(); evt != nil {
		t.Errorf("CurrentEvent before start should be nil, got %v", evt)
	}
}

func TestUpdatesChannel(t *testing.T) {
	ctrl := NewController(testRun(), testEvents())

	ch := ctrl.Updates()
	if ch == nil {
		t.Fatal("Updates channel is nil")
	}

	// StepForward should send an update.
	_, _ = ctrl.StepForward()

	select {
	case state := <-ch:
		if state.EventCount != 1 {
			t.Errorf("update EventCount = %d, want 1", state.EventCount)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected update on channel after StepForward")
	}
}

func TestSpeed(t *testing.T) {
	ctrl := NewController(testRun(), testEvents())

	// Default speed is 1.0.
	if got := ctrl.Speed(); got != 1.0 {
		t.Errorf("initial Speed() = %v, want 1.0", got)
	}

	ctrl.SetSpeed(4.0)
	if got := ctrl.Speed(); got != 4.0 {
		t.Errorf("Speed() after SetSpeed(4) = %v, want 4.0", got)
	}
}

func TestJumpToPrevCommit(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Advance to the end.
	for range evts {
		_, _ = ctrl.StepForward()
	}

	// Jump back to the commit event (index 7).
	err := ctrl.JumpToPrevCommit()
	if err != nil {
		t.Fatalf("JumpToPrevCommit: %v", err)
	}

	current, _ := ctrl.Progress()
	if current != 7 {
		t.Errorf("current = %d, want 7", current)
	}
	evt := ctrl.CurrentEvent()
	if evt.Type != events.EventGitCommitCreated {
		t.Errorf("event type = %s, want %s", evt.Type, events.EventGitCommitCreated)
	}
}

func TestJumpToPrevCommit_NoCommitBefore(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Advance to index 2 (before any commit).
	for range 3 {
		_, _ = ctrl.StepForward()
	}

	err := ctrl.JumpToPrevCommit()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestJumpToPrevAlert(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Advance to the end.
	for range evts {
		_, _ = ctrl.StepForward()
	}

	// Jump back to the alert event (index 4).
	err := ctrl.JumpToPrevAlert()
	if err != nil {
		t.Fatalf("JumpToPrevAlert: %v", err)
	}

	current, _ := ctrl.Progress()
	if current != 4 {
		t.Errorf("current = %d, want 4", current)
	}
	evt := ctrl.CurrentEvent()
	if evt.Type != events.EventAlertRaised {
		t.Errorf("event type = %s, want %s", evt.Type, events.EventAlertRaised)
	}
}

func TestJumpToPrevAlert_NoAlertBefore(t *testing.T) {
	run := testRun()
	evts := testEvents()
	ctrl := NewController(run, evts)

	// Advance to index 2 (before the alert at index 4).
	for range 3 {
		_, _ = ctrl.StepForward()
	}

	err := ctrl.JumpToPrevAlert()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
