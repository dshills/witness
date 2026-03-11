package ingest

import (
	"encoding/json"
	"testing"

	"github.com/dshills/witness/internal/events"
)

func TestParseToolResult_Valid(t *testing.T) {
	input := `{"tool":"prism","status":"pass","summary":"No issues found","findings":{"error":0,"warning":2},"duration_ms":1500}`
	result, err := ParseToolResult([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tool != "prism" {
		t.Errorf("tool = %q, want %q", result.Tool, "prism")
	}
	if result.Status != "pass" {
		t.Errorf("status = %q, want %q", result.Status, "pass")
	}
	if result.Summary != "No issues found" {
		t.Errorf("summary = %q, want %q", result.Summary, "No issues found")
	}
	if result.DurationMS != 1500 {
		t.Errorf("duration_ms = %d, want 1500", result.DurationMS)
	}
	if result.Findings["warning"] != 2 {
		t.Errorf("findings[warning] = %d, want 2", result.Findings["warning"])
	}
}

func TestParseToolResult_MissingTool(t *testing.T) {
	input := `{"status":"pass","summary":"ok"}`
	_, err := ParseToolResult([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing tool field")
	}
}

func TestParseToolResult_MissingStatus(t *testing.T) {
	input := `{"tool":"lint","summary":"ok"}`
	_, err := ParseToolResult([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing status field")
	}
}

func TestParseToolResult_InvalidJSON(t *testing.T) {
	_, err := ParseToolResult([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseToolResult_MinimalFields(t *testing.T) {
	input := `{"tool":"check","status":"pass"}`
	result, err := ParseToolResult([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tool != "check" {
		t.Errorf("tool = %q, want %q", result.Tool, "check")
	}
	if result.Findings != nil {
		t.Errorf("findings should be nil for minimal input, got %v", result.Findings)
	}
	if result.Tokens != nil {
		t.Errorf("tokens should be nil for minimal input")
	}
	if result.Artifacts != nil {
		t.Errorf("artifacts should be nil for minimal input")
	}
}

func TestParseToolResult_WithTokens(t *testing.T) {
	input := `{"tool":"reviewer","status":"pass","summary":"ok","model":"claude-sonnet-4-6","provider":"anthropic","tokens":{"input":1000,"output":500,"cached":200}}`
	result, err := ParseToolResult([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want %q", result.Model, "claude-sonnet-4-6")
	}
	if result.Provider != "anthropic" {
		t.Errorf("provider = %q, want %q", result.Provider, "anthropic")
	}
	if result.Tokens.Input != 1000 {
		t.Errorf("tokens.input = %d, want 1000", result.Tokens.Input)
	}
	if result.Tokens.Output != 500 {
		t.Errorf("tokens.output = %d, want 500", result.Tokens.Output)
	}
	if result.Tokens.Cached != 200 {
		t.Errorf("tokens.cached = %d, want 200", result.Tokens.Cached)
	}
}

func TestToolResultToEvents_ToolCompleted(t *testing.T) {
	result := ToolResult{
		Tool:       "golangci-lint",
		Status:     "pass",
		Summary:    "No issues",
		DurationMS: 2500,
	}

	evts := ToolResultToEvents(result, "run_1", "stage_1")
	if len(evts) == 0 {
		t.Fatal("expected at least one event")
	}

	toolEvt := evts[0]
	if toolEvt.Type != events.EventToolCompleted {
		t.Errorf("type = %q, want %q", toolEvt.Type, events.EventToolCompleted)
	}
	if toolEvt.RunID != "run_1" {
		t.Errorf("run_id = %q, want %q", toolEvt.RunID, "run_1")
	}
	if toolEvt.StageID != "stage_1" {
		t.Errorf("stage_id = %q, want %q", toolEvt.StageID, "stage_1")
	}
	if toolEvt.Status != "pass" {
		t.Errorf("status = %q, want %q", toolEvt.Status, "pass")
	}
	if toolEvt.Summary != "No issues" {
		t.Errorf("summary = %q, want %q", toolEvt.Summary, "No issues")
	}

	var payload map[string]any
	if err := json.Unmarshal(toolEvt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["duration_ms"] != float64(2500) {
		t.Errorf("payload duration_ms = %v, want 2500", payload["duration_ms"])
	}
}

func TestToolResultToEvents_WithModelInfo(t *testing.T) {
	result := ToolResult{
		Tool:     "prism",
		Status:   "pass",
		Summary:  "Review complete",
		Model:    "claude-sonnet-4-6",
		Provider: "anthropic",
		Tokens:   &ToolTokens{Input: 5000, Output: 1000, Cached: 500},
	}

	evts := ToolResultToEvents(result, "run_2", "stage_2")

	// Should have tool.completed + model.request.completed
	var modelEvt *events.Event
	for i := range evts {
		if evts[i].Type == events.EventModelRequestCompleted {
			modelEvt = &evts[i]
			break
		}
	}
	if modelEvt == nil {
		t.Fatal("expected model.request.completed event")
	}

	var payload map[string]any
	if err := json.Unmarshal(modelEvt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["model"] != "claude-sonnet-4-6" {
		t.Errorf("model = %v, want claude-sonnet-4-6", payload["model"])
	}
	if payload["provider"] != "anthropic" {
		t.Errorf("provider = %v, want anthropic", payload["provider"])
	}
	if payload["input_tokens"] != float64(5000) {
		t.Errorf("input_tokens = %v, want 5000", payload["input_tokens"])
	}
}

func TestToolResultToEvents_NoModelWithoutTokens(t *testing.T) {
	result := ToolResult{
		Tool:   "lint",
		Status: "pass",
		Model:  "gpt-4o",
		// No Tokens field
	}

	evts := ToolResultToEvents(result, "run_3", "")
	for _, evt := range evts {
		if evt.Type == events.EventModelRequestCompleted {
			t.Error("should not emit model.request.completed without tokens")
		}
	}
}

func TestToolResultToEvents_FindingEvents(t *testing.T) {
	result := ToolResult{
		Tool:   "speccritic",
		Status: "fail",
		Findings: map[string]int{
			"error":   3,
			"warning": 1,
			"info":    0, // should not generate an event
		},
	}

	evts := ToolResultToEvents(result, "run_4", "stage_4")

	var findingEvts []events.Event
	for _, evt := range evts {
		if evt.Type == events.EventFindingRecorded {
			findingEvts = append(findingEvts, evt)
		}
	}

	// error (3) and warning (1), but not info (0)
	if len(findingEvts) != 2 {
		t.Fatalf("expected 2 finding events, got %d", len(findingEvts))
	}

	categories := make(map[string]bool)
	for _, fe := range findingEvts {
		var payload map[string]any
		if err := json.Unmarshal(fe.Payload, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		cat, _ := payload["category"].(string)
		categories[cat] = true
	}
	if !categories["error"] {
		t.Error("expected finding event for category 'error'")
	}
	if !categories["warning"] {
		t.Error("expected finding event for category 'warning'")
	}
}

func TestToolResultToEvents_EmptyStageID(t *testing.T) {
	result := ToolResult{
		Tool:   "test",
		Status: "pass",
	}

	evts := ToolResultToEvents(result, "run_5", "")
	if len(evts) == 0 {
		t.Fatal("expected at least one event")
	}
	if evts[0].StageID != "" {
		t.Errorf("stage_id should be empty, got %q", evts[0].StageID)
	}
}
