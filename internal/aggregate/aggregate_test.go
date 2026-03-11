package aggregate

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// --- helpers ---

const floatEpsilon = 1e-9

func approxEqual(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < floatEpsilon
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func makeEvent(runID string, t events.EventType, payload any, opts ...func(*events.Event)) events.Event {
	evt := events.Event{
		EventID:       events.NewID("evt"),
		SchemaVersion: events.SchemaVersion,
		Timestamp:     time.Now().UTC(),
		RunID:         runID,
		Type:          t,
		Source:        "test",
		Payload:       mustJSON(payload),
	}
	for _, o := range opts {
		o(&evt)
	}
	return evt
}

func withStageID(id string) func(*events.Event) {
	return func(e *events.Event) { e.StageID = id }
}

func withTimestamp(ts time.Time) func(*events.Event) {
	return func(e *events.Event) { e.Timestamp = ts }
}

func testRun() models.Run {
	return models.Run{
		RunID:  "run_test1",
		Name:   "test-run",
		Status: models.RunStatusPending,
	}
}

// --- Tests ---

func TestEmptyRunProducesZeroState(t *testing.T) {
	a := NewAggregator(testRun())
	snap := a.Snapshot()

	if snap.Run.RunID != "run_test1" {
		t.Errorf("RunID = %q, want %q", snap.Run.RunID, "run_test1")
	}
	if snap.EventCount != 0 {
		t.Errorf("EventCount = %d, want 0", snap.EventCount)
	}
	if len(snap.Stages) != 0 {
		t.Errorf("Stages len = %d, want 0", len(snap.Stages))
	}
	if snap.TotalCostUSD != 0 {
		t.Errorf("TotalCostUSD = %f, want 0", snap.TotalCostUSD)
	}
	if snap.FailureCount != 0 {
		t.Errorf("FailureCount = %d, want 0", snap.FailureCount)
	}
	if snap.UniqueFilesTouched() != 0 {
		t.Errorf("UniqueFilesTouched = %d, want 0", snap.UniqueFilesTouched())
	}
}

func TestRunLifecycle(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	// Created
	err := a.Apply(makeEvent(runID, events.EventRunCreated, runPayload{
		Name:     "my-run",
		RepoRoot: "/repo",
		Branch:   "main",
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap := a.Snapshot()
	if snap.Run.Name != "my-run" {
		t.Errorf("Run.Name = %q, want %q", snap.Run.Name, "my-run")
	}
	if snap.Branch != "main" {
		t.Errorf("Branch = %q, want %q", snap.Branch, "main")
	}

	// Started
	err = a.Apply(makeEvent(runID, events.EventRunStarted, nil))
	if err != nil {
		t.Fatal(err)
	}
	snap = a.Snapshot()
	if snap.Run.Status != models.RunStatusRunning {
		t.Errorf("Status = %v, want Running", snap.Run.Status)
	}

	// Completed
	err = a.Apply(makeEvent(runID, events.EventRunCompleted, nil))
	if err != nil {
		t.Fatal(err)
	}
	snap = a.Snapshot()
	if snap.Run.Status != models.RunStatusCompleted {
		t.Errorf("Status = %v, want Completed", snap.Run.Status)
	}
	if snap.Run.EndedAt == nil {
		t.Fatal("EndedAt should not be nil")
	}
}

func TestRunFailed(t *testing.T) {
	a := NewAggregator(testRun())
	_ = a.Apply(makeEvent("run1", events.EventRunStarted, nil))
	_ = a.Apply(makeEvent("run1", events.EventRunFailed, nil))
	snap := a.Snapshot()
	if snap.Run.Status != models.RunStatusFailed {
		t.Errorf("Status = %v, want Failed", snap.Run.Status)
	}
}

func TestRunCancelled(t *testing.T) {
	a := NewAggregator(testRun())
	_ = a.Apply(makeEvent("run1", events.EventRunStarted, nil))
	_ = a.Apply(makeEvent("run1", events.EventRunCancelled, nil))
	snap := a.Snapshot()
	if snap.Run.Status != models.RunStatusCancelled {
		t.Errorf("Status = %v, want Cancelled", snap.Run.Status)
	}
}

func TestRunStalled(t *testing.T) {
	a := NewAggregator(testRun())
	_ = a.Apply(makeEvent("run1", events.EventRunStarted, nil))
	_ = a.Apply(makeEvent("run1", events.EventRunStalled, nil))
	snap := a.Snapshot()
	if snap.Run.Status != models.RunStatusStalled {
		t.Errorf("Status = %v, want Stalled", snap.Run.Status)
	}
}

func TestStageLifecycle(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	// Create stage
	_ = a.Apply(makeEvent(runID, events.EventStageCreated, stagePayload{
		StageID: "stage_1",
		Name:    "build",
		Order:   1,
	}))
	snap := a.Snapshot()
	if len(snap.Stages) != 1 {
		t.Fatalf("Stages len = %d, want 1", len(snap.Stages))
	}
	if snap.Stages[0].Name != "build" {
		t.Errorf("Stage name = %q, want %q", snap.Stages[0].Name, "build")
	}
	if snap.Stages[0].Status != models.StageStatusPending {
		t.Errorf("Stage status = %v, want Pending", snap.Stages[0].Status)
	}

	// Start stage
	_ = a.Apply(makeEvent(runID, events.EventStageStarted, nil, withStageID("stage_1")))
	snap = a.Snapshot()
	if snap.Stages[0].Status != models.StageStatusRunning {
		t.Errorf("Stage status = %v, want Running", snap.Stages[0].Status)
	}
	if snap.ActiveStage == nil {
		t.Fatal("ActiveStage should not be nil")
	}
	if snap.ActiveStage.StageID != "stage_1" {
		t.Errorf("ActiveStage.StageID = %q, want %q", snap.ActiveStage.StageID, "stage_1")
	}

	// Progress
	pct := 50.0
	_ = a.Apply(makeEvent(runID, events.EventStageProgress, stagePayload{
		ProgressPercent: &pct,
		Summary:         "halfway there",
	}, withStageID("stage_1")))
	snap = a.Snapshot()
	if snap.Stages[0].ProgressPercent == nil || *snap.Stages[0].ProgressPercent != 50.0 {
		t.Errorf("ProgressPercent = %v, want 50.0", snap.Stages[0].ProgressPercent)
	}
	if snap.Stages[0].Summary != "halfway there" {
		t.Errorf("Summary = %q, want %q", snap.Stages[0].Summary, "halfway there")
	}

	// Complete stage
	_ = a.Apply(makeEvent(runID, events.EventStageCompleted, nil, withStageID("stage_1")))
	snap = a.Snapshot()
	if snap.Stages[0].Status != models.StageStatusCompleted {
		t.Errorf("Stage status = %v, want Completed", snap.Stages[0].Status)
	}
	if snap.ActiveStage != nil {
		t.Error("ActiveStage should be nil after completion")
	}
}

func TestStageFailedAndSkipped(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	_ = a.Apply(makeEvent(runID, events.EventStageCreated, stagePayload{StageID: "s1", Name: "s1"}))
	_ = a.Apply(makeEvent(runID, events.EventStageCreated, stagePayload{StageID: "s2", Name: "s2"}))

	_ = a.Apply(makeEvent(runID, events.EventStageStarted, nil, withStageID("s1")))
	_ = a.Apply(makeEvent(runID, events.EventStageFailed, nil, withStageID("s1")))

	snap := a.Snapshot()
	if snap.Stages[0].Status != models.StageStatusFailed {
		t.Errorf("s1 status = %v, want Failed", snap.Stages[0].Status)
	}

	_ = a.Apply(makeEvent(runID, events.EventStageSkipped, nil, withStageID("s2")))
	snap = a.Snapshot()
	if snap.Stages[1].Status != models.StageStatusSkipped {
		t.Errorf("s2 status = %v, want Skipped", snap.Stages[1].Status)
	}
}

func TestToolLifecycle(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	// Start tool
	_ = a.Apply(makeEvent(runID, events.EventToolStarted, toolStartedPayload{
		InvocationID: "inv_1",
		ToolName:     "golangci-lint",
		Command:      []string{"golangci-lint", "run"},
	}, withTimestamp(now)))

	snap := a.Snapshot()
	if len(snap.ToolInvocations) != 1 {
		t.Fatalf("ToolInvocations len = %d, want 1", len(snap.ToolInvocations))
	}
	if snap.ToolCounts["golangci-lint"] != 1 {
		t.Errorf("ToolCounts[golangci-lint] = %d, want 1", snap.ToolCounts["golangci-lint"])
	}
	if snap.ActiveTool == nil || snap.ActiveTool.InvocationID != "inv_1" {
		t.Error("ActiveTool should be set to inv_1")
	}

	// Tool output
	_ = a.Apply(makeEvent(runID, events.EventToolOutput, toolOutputPayload{
		InvocationID: "inv_1",
		Summary:      "found 3 issues",
	}))
	snap = a.Snapshot()
	if snap.ToolInvocations[0].Summary != "found 3 issues" {
		t.Errorf("Summary = %q, want %q", snap.ToolInvocations[0].Summary, "found 3 issues")
	}

	// Complete tool
	exitCode := 1
	completedAt := now.Add(5 * time.Second)
	_ = a.Apply(makeEvent(runID, events.EventToolCompleted, toolCompletedPayload{
		InvocationID: "inv_1",
		ExitCode:     &exitCode,
	}, withTimestamp(completedAt)))

	snap = a.Snapshot()
	if snap.ToolInvocations[0].Status != models.InvocationStatusCompleted {
		t.Errorf("Status = %v, want Completed", snap.ToolInvocations[0].Status)
	}
	if snap.ActiveTool != nil {
		t.Error("ActiveTool should be nil after completion")
	}
	if snap.ToolDurations["golangci-lint"] != 5*time.Second {
		t.Errorf("ToolDurations = %v, want 5s", snap.ToolDurations["golangci-lint"])
	}
}

func TestToolFailed(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	_ = a.Apply(makeEvent(runID, events.EventToolStarted, toolStartedPayload{
		InvocationID: "inv_f1",
		ToolName:     "broken",
	}))
	_ = a.Apply(makeEvent(runID, events.EventToolFailed, toolCompletedPayload{
		InvocationID: "inv_f1",
	}))

	snap := a.Snapshot()
	if snap.ToolInvocations[0].Status != models.InvocationStatusFailed {
		t.Errorf("Status = %v, want Failed", snap.ToolInvocations[0].Status)
	}
	if snap.ActiveTool != nil {
		t.Error("ActiveTool should be nil after failure")
	}
}

func TestModelLifecycleAndTokenAccumulation(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	// Start model request
	_ = a.Apply(makeEvent(runID, events.EventModelRequestStarted, modelRequestStartedPayload{
		RequestID: "req_1",
		Provider:  "openai",
		Model:     "gpt-4",
	}, withTimestamp(now), withStageID("stage_1")))

	snap := a.Snapshot()
	if len(snap.ModelRequests) != 1 {
		t.Fatalf("ModelRequests len = %d, want 1", len(snap.ModelRequests))
	}
	if snap.ActiveModel == nil || snap.ActiveModel.RequestID != "req_1" {
		t.Error("ActiveModel should be set to req_1")
	}
	if snap.ModelCounts["gpt-4"] != 1 {
		t.Errorf("ModelCounts[gpt-4] = %d, want 1", snap.ModelCounts["gpt-4"])
	}

	// Complete model request
	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{
		RequestID:    "req_1",
		Provider:     "openai",
		Model:        "gpt-4",
		InputTokens:  1000,
		OutputTokens: 500,
		CachedTokens: 200,
		CostUSD:      0.05,
	}, withTimestamp(now.Add(2*time.Second)), withStageID("stage_1")))

	snap = a.Snapshot()
	if snap.TotalInputTokens != 1000 {
		t.Errorf("TotalInputTokens = %d, want 1000", snap.TotalInputTokens)
	}
	if snap.TotalOutputTokens != 500 {
		t.Errorf("TotalOutputTokens = %d, want 500", snap.TotalOutputTokens)
	}
	if snap.TotalCachedTokens != 200 {
		t.Errorf("TotalCachedTokens = %d, want 200", snap.TotalCachedTokens)
	}
	if !approxEqual(snap.TotalCostUSD, 0.05) {
		t.Errorf("TotalCostUSD = %g, want 0.05", snap.TotalCostUSD)
	}
	if snap.ActiveModel != nil {
		t.Error("ActiveModel should be nil after completion")
	}

	// Check provider/model breakdowns.
	if tc := snap.TokensByProvider["openai"]; tc.Input != 1000 || tc.Output != 500 || tc.Cached != 200 {
		t.Errorf("TokensByProvider[openai] = %+v, want {1000, 500, 200}", tc)
	}
	if tc := snap.TokensByModel["gpt-4"]; tc.Input != 1000 || tc.Output != 500 || tc.Cached != 200 {
		t.Errorf("TokensByModel[gpt-4] = %+v, want {1000, 500, 200}", tc)
	}
	if tc := snap.TokensByStage["stage_1"]; tc.Input != 1000 {
		t.Errorf("TokensByStage[stage_1].Input = %d, want 1000", tc.Input)
	}
	if !approxEqual(snap.CostByProvider["openai"], 0.05) {
		t.Errorf("CostByProvider[openai] = %g, want 0.05", snap.CostByProvider["openai"])
	}
	if !approxEqual(snap.CostByModel["gpt-4"], 0.05) {
		t.Errorf("CostByModel[gpt-4] = %g, want 0.05", snap.CostByModel["gpt-4"])
	}

	// Second request from different provider.
	_ = a.Apply(makeEvent(runID, events.EventModelRequestStarted, modelRequestStartedPayload{
		RequestID: "req_2",
		Provider:  "anthropic",
		Model:     "claude-3",
	}, withTimestamp(now.Add(3*time.Second))))
	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{
		RequestID:    "req_2",
		Provider:     "anthropic",
		Model:        "claude-3",
		InputTokens:  2000,
		OutputTokens: 1000,
		CachedTokens: 0,
		CostUSD:      0.10,
	}, withTimestamp(now.Add(5*time.Second))))

	snap = a.Snapshot()
	if snap.TotalInputTokens != 3000 {
		t.Errorf("TotalInputTokens = %d, want 3000", snap.TotalInputTokens)
	}
	if !approxEqual(snap.TotalCostUSD, 0.15) {
		t.Errorf("TotalCostUSD = %g, want 0.15", snap.TotalCostUSD)
	}
	if !approxEqual(snap.CostByProvider["anthropic"], 0.10) {
		t.Errorf("CostByProvider[anthropic] = %g, want 0.10", snap.CostByProvider["anthropic"])
	}
}

func TestModelRequestFailed(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	_ = a.Apply(makeEvent(runID, events.EventModelRequestStarted, modelRequestStartedPayload{
		RequestID: "req_fail",
		Provider:  "openai",
		Model:     "gpt-4",
	}))
	_ = a.Apply(makeEvent(runID, events.EventModelRequestFailed, modelRequestFailedPayload{
		RequestID: "req_fail",
		Error:     "rate limited",
	}))

	snap := a.Snapshot()
	if snap.ModelRequests[0].Status != models.InvocationStatusFailed {
		t.Errorf("Status = %v, want Failed", snap.ModelRequests[0].Status)
	}
	if snap.FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", snap.FailureCount)
	}
	if snap.ActiveModel != nil {
		t.Error("ActiveModel should be nil after failure")
	}
}

func TestFileAndGitEvents(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	// File events
	_ = a.Apply(makeEvent(runID, events.EventFileCreated, fileChangePayload{
		ChangeID: "fc_1", Path: "main.go",
	}))
	_ = a.Apply(makeEvent(runID, events.EventFileModified, fileChangePayload{
		ChangeID: "fc_2", Path: "main.go",
	}))
	_ = a.Apply(makeEvent(runID, events.EventFileDeleted, fileChangePayload{
		ChangeID: "fc_3", Path: "old.go",
	}))

	snap := a.Snapshot()
	if len(snap.FileChanges) != 3 {
		t.Errorf("FileChanges len = %d, want 3", len(snap.FileChanges))
	}
	if snap.HotFiles["main.go"] != 2 {
		t.Errorf("HotFiles[main.go] = %d, want 2", snap.HotFiles["main.go"])
	}
	if snap.UniqueFilesTouched() != 2 {
		t.Errorf("UniqueFilesTouched = %d, want 2", snap.UniqueFilesTouched())
	}

	// Commit
	now := time.Now().UTC()
	_ = a.Apply(makeEvent(runID, events.EventGitCommitCreated, commitPayload{
		CommitID: "c_1", SHA: "abc123", Message: "initial",
	}, withTimestamp(now)))

	snap = a.Snapshot()
	if len(snap.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(snap.Commits))
	}
	if snap.Commits[0].SHA != "abc123" {
		t.Errorf("SHA = %q, want %q", snap.Commits[0].SHA, "abc123")
	}
	if snap.LastCommitAt.IsZero() {
		t.Error("LastCommitAt should be set")
	}

	// Branch change
	_ = a.Apply(makeEvent(runID, events.EventGitBranchChanged, branchPayload{Branch: "feature/x"}))
	snap = a.Snapshot()
	if snap.Branch != "feature/x" {
		t.Errorf("Branch = %q, want %q", snap.Branch, "feature/x")
	}

	// Repo status
	_ = a.Apply(makeEvent(runID, events.EventRepoStatusChanged, repoStatusPayload{DirtyFiles: 3}))
	snap = a.Snapshot()
	if snap.DirtyFiles != 3 {
		t.Errorf("DirtyFiles = %d, want 3", snap.DirtyFiles)
	}
}

func TestAlertLifecycle(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	// Raise alert
	_ = a.Apply(makeEvent(runID, events.EventAlertRaised, alertPayload{
		AlertID:     "alert_1",
		Severity:    models.SeverityWarning,
		Type:        "token_burn",
		Title:       "High burn rate",
		Description: "Tokens burning fast",
	}))

	snap := a.Snapshot()
	if len(snap.Alerts) != 1 {
		t.Fatalf("Alerts len = %d, want 1", len(snap.Alerts))
	}
	if len(snap.ActiveAlerts) != 1 {
		t.Fatalf("ActiveAlerts len = %d, want 1", len(snap.ActiveAlerts))
	}
	if snap.ActiveAlerts[0].AlertID != "alert_1" {
		t.Errorf("AlertID = %q, want %q", snap.ActiveAlerts[0].AlertID, "alert_1")
	}

	// Clear alert
	_ = a.Apply(makeEvent(runID, events.EventAlertCleared, alertClearedPayload{
		AlertID: "alert_1",
	}))

	snap = a.Snapshot()
	if len(snap.Alerts) != 1 {
		t.Errorf("Alerts len = %d, want 1 (history preserved)", len(snap.Alerts))
	}
	if len(snap.ActiveAlerts) != 0 {
		t.Errorf("ActiveAlerts len = %d, want 0", len(snap.ActiveAlerts))
	}
}

func TestTestFailedIncrementsFailureCount(t *testing.T) {
	a := NewAggregator(testRun())
	_ = a.Apply(makeEvent("run1", events.EventTestFailed, nil))
	_ = a.Apply(makeEvent("run1", events.EventTestFailed, nil))
	snap := a.Snapshot()
	if snap.FailureCount != 2 {
		t.Errorf("FailureCount = %d, want 2", snap.FailureCount)
	}
}

func TestUnknownEventTypeDontError(t *testing.T) {
	a := NewAggregator(testRun())
	err := a.Apply(makeEvent("run1", events.EventType("custom.unknown"), map[string]string{"foo": "bar"}))
	if err != nil {
		t.Errorf("unexpected error for unknown event: %v", err)
	}
	snap := a.Snapshot()
	if snap.EventCount != 1 {
		t.Errorf("EventCount = %d, want 1", snap.EventCount)
	}
}

func TestRecentEventsRingBuffer(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	// Apply 250 events.
	for i := range 250 {
		_ = a.Apply(makeEvent(runID, events.EventType("custom.test"),
			map[string]int{"index": i}))
	}

	snap := a.Snapshot()
	if len(snap.RecentEvents) != maxRecentEvents {
		t.Errorf("RecentEvents len = %d, want %d", len(snap.RecentEvents), maxRecentEvents)
	}
	if snap.EventCount != 250 {
		t.Errorf("EventCount = %d, want 250", snap.EventCount)
	}
}

func TestRebuildMatchesIncremental(t *testing.T) {
	runID := "run_test1"
	run := testRun()
	now := time.Now().UTC()

	evts := []events.Event{
		makeEvent(runID, events.EventRunCreated, runPayload{Name: "rebuild-test"}, withTimestamp(now)),
		makeEvent(runID, events.EventRunStarted, nil, withTimestamp(now.Add(1*time.Second))),
		makeEvent(runID, events.EventStageCreated, stagePayload{StageID: "s1", Name: "build"}, withTimestamp(now.Add(2*time.Second))),
		makeEvent(runID, events.EventStageStarted, nil, withStageID("s1"), withTimestamp(now.Add(3*time.Second))),
		makeEvent(runID, events.EventToolStarted, toolStartedPayload{InvocationID: "t1", ToolName: "go"}, withTimestamp(now.Add(4*time.Second))),
		makeEvent(runID, events.EventToolCompleted, toolCompletedPayload{InvocationID: "t1"}, withTimestamp(now.Add(6*time.Second))),
		makeEvent(runID, events.EventModelRequestStarted, modelRequestStartedPayload{RequestID: "m1", Provider: "openai", Model: "gpt-4"}, withTimestamp(now.Add(7*time.Second))),
		makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{RequestID: "m1", Provider: "openai", Model: "gpt-4", InputTokens: 100, OutputTokens: 50, CostUSD: 0.01}, withTimestamp(now.Add(8*time.Second))),
		makeEvent(runID, events.EventFileCreated, fileChangePayload{Path: "main.go"}, withTimestamp(now.Add(9*time.Second))),
		makeEvent(runID, events.EventGitCommitCreated, commitPayload{SHA: "abc"}, withTimestamp(now.Add(10*time.Second))),
		makeEvent(runID, events.EventStageCompleted, nil, withStageID("s1"), withTimestamp(now.Add(11*time.Second))),
		makeEvent(runID, events.EventRunCompleted, nil, withTimestamp(now.Add(12*time.Second))),
	}

	// Incremental
	a := NewAggregator(run)
	for _, evt := range evts {
		_ = a.Apply(evt)
	}
	incremental := a.Snapshot()

	// Rebuild
	rebuilt, err := Rebuild(run, evts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare key fields.
	if rebuilt.Run.Status != incremental.Run.Status {
		t.Errorf("Status mismatch: rebuild=%v, incremental=%v", rebuilt.Run.Status, incremental.Run.Status)
	}
	if rebuilt.TotalInputTokens != incremental.TotalInputTokens {
		t.Errorf("TotalInputTokens mismatch: rebuild=%d, incremental=%d", rebuilt.TotalInputTokens, incremental.TotalInputTokens)
	}
	if rebuilt.TotalCostUSD != incremental.TotalCostUSD {
		t.Errorf("TotalCostUSD mismatch: rebuild=%f, incremental=%f", rebuilt.TotalCostUSD, incremental.TotalCostUSD)
	}
	if rebuilt.EventCount != incremental.EventCount {
		t.Errorf("EventCount mismatch: rebuild=%d, incremental=%d", rebuilt.EventCount, incremental.EventCount)
	}
	if len(rebuilt.Stages) != len(incremental.Stages) {
		t.Errorf("Stages len mismatch: rebuild=%d, incremental=%d", len(rebuilt.Stages), len(incremental.Stages))
	}
	if len(rebuilt.ToolInvocations) != len(incremental.ToolInvocations) {
		t.Errorf("ToolInvocations len mismatch: rebuild=%d, incremental=%d", len(rebuilt.ToolInvocations), len(incremental.ToolInvocations))
	}
	if len(rebuilt.Commits) != len(incremental.Commits) {
		t.Errorf("Commits len mismatch: rebuild=%d, incremental=%d", len(rebuilt.Commits), len(incremental.Commits))
	}
}

func TestTokenBurnRate(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	// Two model completions within the last minute.
	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{
		RequestID:    "r1",
		Provider:     "openai",
		Model:        "gpt-4",
		InputTokens:  500,
		OutputTokens: 200,
		CachedTokens: 100,
		CostUSD:      0.03,
	}, withTimestamp(now.Add(-30*time.Second))))

	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{
		RequestID:    "r2",
		Provider:     "openai",
		Model:        "gpt-4",
		InputTokens:  300,
		OutputTokens: 100,
		CachedTokens: 0,
		CostUSD:      0.02,
	}, withTimestamp(now)))

	snap := a.Snapshot()

	// Window of 1 minute should include both events.
	rate := snap.TokenBurnRate(1 * time.Minute)
	if rate <= 0 {
		t.Errorf("TokenBurnRate = %f, want > 0", rate)
	}

	// Total tokens = 500+200+100 + 300+100+0 = 1200
	// Elapsed = 30s
	// Rate = 1200/30 = 40 tokens/sec
	expectedRate := 1200.0 / 30.0
	if rate < expectedRate-1 || rate > expectedRate+1 {
		t.Errorf("TokenBurnRate = %f, want ~%f", rate, expectedRate)
	}

	// Window of 0 should return 0 (no events).
	rate = snap.TokenBurnRate(0)
	if rate != 0 {
		t.Errorf("TokenBurnRate(0) = %f, want 0", rate)
	}
}

func TestCostBurnRate(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{
		RequestID: "r1", Provider: "openai", Model: "gpt-4",
		InputTokens: 100, OutputTokens: 50, CostUSD: 0.10,
	}, withTimestamp(now.Add(-20*time.Second))))

	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{
		RequestID: "r2", Provider: "openai", Model: "gpt-4",
		InputTokens: 100, OutputTokens: 50, CostUSD: 0.20,
	}, withTimestamp(now)))

	snap := a.Snapshot()
	rate := snap.CostBurnRate(1 * time.Minute)

	// Cost = 0.30, elapsed = 20s, rate = 0.015/s
	expectedRate := 0.30 / 20.0
	if rate < expectedRate-0.001 || rate > expectedRate+0.001 {
		t.Errorf("CostBurnRate = %f, want ~%f", rate, expectedRate)
	}
}

func TestAvgToolLatency(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	_ = a.Apply(makeEvent(runID, events.EventToolStarted, toolStartedPayload{InvocationID: "t1", ToolName: "go"}, withTimestamp(now)))
	_ = a.Apply(makeEvent(runID, events.EventToolCompleted, toolCompletedPayload{InvocationID: "t1"}, withTimestamp(now.Add(2*time.Second))))

	_ = a.Apply(makeEvent(runID, events.EventToolStarted, toolStartedPayload{InvocationID: "t2", ToolName: "lint"}, withTimestamp(now.Add(3*time.Second))))
	_ = a.Apply(makeEvent(runID, events.EventToolCompleted, toolCompletedPayload{InvocationID: "t2"}, withTimestamp(now.Add(7*time.Second))))

	snap := a.Snapshot()
	avg := snap.AvgToolLatency()
	// (2s + 4s) / 2 = 3s
	if avg != 3*time.Second {
		t.Errorf("AvgToolLatency = %v, want 3s", avg)
	}
}

func TestAvgModelLatency(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	_ = a.Apply(makeEvent(runID, events.EventModelRequestStarted, modelRequestStartedPayload{RequestID: "m1", Provider: "x", Model: "y"}, withTimestamp(now)))
	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{RequestID: "m1", Provider: "x", Model: "y", InputTokens: 10, OutputTokens: 5, CostUSD: 0.01}, withTimestamp(now.Add(1*time.Second))))

	_ = a.Apply(makeEvent(runID, events.EventModelRequestStarted, modelRequestStartedPayload{RequestID: "m2", Provider: "x", Model: "y"}, withTimestamp(now.Add(2*time.Second))))
	_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{RequestID: "m2", Provider: "x", Model: "y", InputTokens: 10, OutputTokens: 5, CostUSD: 0.01}, withTimestamp(now.Add(5*time.Second))))

	snap := a.Snapshot()
	avg := snap.AvgModelLatency()
	// (1s + 3s) / 2 = 2s
	if avg != 2*time.Second {
		t.Errorf("AvgModelLatency = %v, want 2s", avg)
	}
}

func TestMeanTimeBetweenCommits(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	_ = a.Apply(makeEvent(runID, events.EventGitCommitCreated, commitPayload{SHA: "a"}, withTimestamp(now)))
	_ = a.Apply(makeEvent(runID, events.EventGitCommitCreated, commitPayload{SHA: "b"}, withTimestamp(now.Add(10*time.Second))))
	_ = a.Apply(makeEvent(runID, events.EventGitCommitCreated, commitPayload{SHA: "c"}, withTimestamp(now.Add(30*time.Second))))

	snap := a.Snapshot()
	mtbc := snap.MeanTimeBetweenCommits()
	// (10s + 20s) / 2 = 15s
	if mtbc != 15*time.Second {
		t.Errorf("MeanTimeBetweenCommits = %v, want 15s", mtbc)
	}

	// Single commit: 0
	a2 := NewAggregator(testRun())
	_ = a2.Apply(makeEvent(runID, events.EventGitCommitCreated, commitPayload{SHA: "x"}, withTimestamp(now)))
	snap2 := a2.Snapshot()
	if snap2.MeanTimeBetweenCommits() != 0 {
		t.Errorf("MeanTimeBetweenCommits with 1 commit = %v, want 0", snap2.MeanTimeBetweenCommits())
	}
}

func TestDuration(t *testing.T) {
	a := NewAggregator(testRun())
	snap := a.Snapshot()
	if snap.Duration() != 0 {
		t.Errorf("Duration before start = %v, want 0", snap.Duration())
	}

	now := time.Now().UTC()
	_ = a.Apply(makeEvent("run1", events.EventRunStarted, nil, withTimestamp(now)))
	endTime := now.Add(10 * time.Second)
	_ = a.Apply(makeEvent("run1", events.EventRunCompleted, nil, withTimestamp(endTime)))

	snap = a.Snapshot()
	if snap.Duration() != 10*time.Second {
		t.Errorf("Duration = %v, want 10s", snap.Duration())
	}
}

func TestStageDurations(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"
	now := time.Now().UTC()

	_ = a.Apply(makeEvent(runID, events.EventStageCreated, stagePayload{StageID: "s1", Name: "build"}, withTimestamp(now)))
	_ = a.Apply(makeEvent(runID, events.EventStageStarted, nil, withStageID("s1"), withTimestamp(now.Add(1*time.Second))))
	_ = a.Apply(makeEvent(runID, events.EventStageCompleted, nil, withStageID("s1"), withTimestamp(now.Add(6*time.Second))))

	snap := a.Snapshot()
	durations := snap.StageDurations()
	if d, ok := durations["build"]; !ok || d != 5*time.Second {
		t.Errorf("StageDurations[build] = %v, want 5s", d)
	}
}

func TestJSONSerializable(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	_ = a.Apply(makeEvent(runID, events.EventRunCreated, runPayload{Name: "json-test"}))
	_ = a.Apply(makeEvent(runID, events.EventRunStarted, nil))

	snap := a.Snapshot()

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	restored, err := UnmarshalRunState(data)
	if err != nil {
		t.Fatalf("UnmarshalRunState error: %v", err)
	}

	if restored.Run.Name != "json-test" {
		t.Errorf("Restored Run.Name = %q, want %q", restored.Run.Name, "json-test")
	}
	if restored.EventCount != snap.EventCount {
		t.Errorf("Restored EventCount = %d, want %d", restored.EventCount, snap.EventCount)
	}
}

func TestConcurrentApply(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	var wg sync.WaitGroup
	const goroutines = 20
	const eventsPerGoroutine = 50

	wg.Add(goroutines)
	for g := range goroutines {
		go func(gid int) {
			defer wg.Done()
			for i := range eventsPerGoroutine {
				evt := makeEvent(runID, events.EventType("custom.concurrent"),
					map[string]int{"goroutine": gid, "index": i})
				_ = a.Apply(evt)
			}
		}(g)
	}
	wg.Wait()

	snap := a.Snapshot()
	expected := int64(goroutines * eventsPerGoroutine)
	if snap.EventCount != expected {
		t.Errorf("EventCount = %d, want %d", snap.EventCount, expected)
	}
}

func TestConcurrentApplyWithMixedEvents(t *testing.T) {
	a := NewAggregator(testRun())
	runID := "run_test1"

	var wg sync.WaitGroup
	const goroutines = 10

	wg.Add(goroutines)
	for g := range goroutines {
		go func(gid int) {
			defer wg.Done()
			toolID := fmt.Sprintf("tool_%d", gid)
			_ = a.Apply(makeEvent(runID, events.EventToolStarted, toolStartedPayload{
				InvocationID: toolID,
				ToolName:     fmt.Sprintf("tool-%d", gid),
			}))
			_ = a.Apply(makeEvent(runID, events.EventToolCompleted, toolCompletedPayload{
				InvocationID: toolID,
			}))
			_ = a.Apply(makeEvent(runID, events.EventModelRequestStarted, modelRequestStartedPayload{
				RequestID: fmt.Sprintf("req_%d", gid),
				Provider:  "test",
				Model:     "test-model",
			}))
			_ = a.Apply(makeEvent(runID, events.EventModelRequestCompleted, modelRequestCompletedPayload{
				RequestID:    fmt.Sprintf("req_%d", gid),
				Provider:     "test",
				Model:        "test-model",
				InputTokens:  100,
				OutputTokens: 50,
				CostUSD:      0.01,
			}))
		}(g)
	}
	wg.Wait()

	snap := a.Snapshot()
	// 4 events per goroutine * 10 goroutines = 40
	if snap.EventCount != int64(goroutines*4) {
		t.Errorf("EventCount = %d, want %d", snap.EventCount, goroutines*4)
	}
	if snap.TotalInputTokens != int64(goroutines*100) {
		t.Errorf("TotalInputTokens = %d, want %d", snap.TotalInputTokens, goroutines*100)
	}
}
