package export_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/export"
	"github.com/dshills/witness/internal/models"
)

func testState() aggregate.RunState {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	ended := now.Add(5 * time.Minute)
	stageStart := now.Add(1 * time.Minute)
	stageEnd := now.Add(4 * time.Minute)

	return aggregate.RunState{
		Run: models.Run{
			RunID:     "run_001",
			Name:      "test-run",
			RepoRoot:  "/tmp/repo",
			Branch:    "main",
			Status:    models.RunStatusCompleted,
			StartedAt: now,
			EndedAt:   &ended,
			Command:   []string{"claude", "code"},
		},
		Stages: []models.Stage{
			{
				StageID:   "stage_001",
				RunID:     "run_001",
				Name:      "planning",
				Order:     1,
				Status:    models.StageStatusCompleted,
				StartedAt: &stageStart,
				EndedAt:   &stageEnd,
			},
		},
		TotalInputTokens:  1000,
		TotalOutputTokens: 500,
		TotalCachedTokens: 200,
		TotalCostUSD:      0.05,
		TokensByProvider: map[string]aggregate.TokenCount{
			"anthropic": {Input: 1000, Output: 500, Cached: 200},
		},
		TokensByModel: map[string]aggregate.TokenCount{
			"claude-4": {Input: 1000, Output: 500, Cached: 200},
		},
		CostByProvider: map[string]float64{"anthropic": 0.05},
		CostByModel:    map[string]float64{"claude-4": 0.05},
		ToolCounts:     map[string]int{"bash": 3, "read": 5},
		ToolDurations:  map[string]time.Duration{"bash": 10 * time.Second},
		ModelCounts:    map[string]int{"claude-4": 2},
		HotFiles:       map[string]int{"main.go": 3},
		Commits: []models.Commit{
			{CommitID: "c1", RunID: "run_001", SHA: "abc1234567890", Message: "initial commit"},
		},
		Alerts: []models.Alert{
			{AlertID: "a1", RunID: "run_001", Severity: models.SeverityWarning, Title: "High cost", Description: "Cost exceeded threshold"},
		},
		EventCount: 25,
	}
}

func testEvents() []events.Event {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	return []events.Event{
		{
			EventID:       "evt_001",
			SchemaVersion: "1.0",
			Timestamp:     now,
			RunID:         "run_001",
			Type:          events.EventRunStarted,
			Source:        "test",
			Payload:       json.RawMessage(`{}`),
		},
		{
			EventID:       "evt_002",
			SchemaVersion: "1.0",
			Timestamp:     now.Add(5 * time.Minute),
			RunID:         "run_001",
			Type:          events.EventRunCompleted,
			Source:        "test",
			Payload:       json.RawMessage(`{}`),
		},
	}
}

func TestJSONExporter(t *testing.T) {
	state := testState()
	evts := testEvents()

	var buf bytes.Buffer
	exp := &export.JSONExporter{}
	if err := exp.Export(context.Background(), state, evts, &buf); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify valid JSON
	var result map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check expected top-level keys
	expectedKeys := []string{"run", "stages", "tools", "model_requests", "file_changes", "commits", "alerts", "metrics", "events"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("missing expected key %q in JSON output", key)
		}
	}

	// Check metrics contain expected fields
	var metrics map[string]json.RawMessage
	if err := json.Unmarshal(result["metrics"], &metrics); err != nil {
		t.Fatalf("metrics is not valid JSON: %v", err)
	}
	metricKeys := []string{"total_input_tokens", "total_output_tokens", "total_cost_usd", "tool_counts"}
	for _, key := range metricKeys {
		if _, ok := metrics[key]; !ok {
			t.Errorf("missing expected metric key %q", key)
		}
	}

	// Verify run_id is present in run
	var run map[string]any
	if err := json.Unmarshal(result["run"], &run); err != nil {
		t.Fatalf("run is not valid JSON: %v", err)
	}
	if run["run_id"] != "run_001" {
		t.Errorf("run_id = %v, want run_001", run["run_id"])
	}
}
