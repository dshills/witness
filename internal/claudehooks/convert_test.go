package claudehooks

import (
	"encoding/json"
	"testing"

	"github.com/dshills/witness/internal/events"
)

func TestConvertPreToolUse(t *testing.T) {
	p := HookPayload{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolUseID:     "tooluse_123",
		ToolInput:     json.RawMessage(`{"command":"go test ./...","description":"Run tests"}`),
	}

	evts := Convert("run_1", p)
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}

	evt := evts[0]
	if evt.Type != events.EventToolStarted {
		t.Errorf("expected type %s, got %s", events.EventToolStarted, evt.Type)
	}
	if evt.RunID != "run_1" {
		t.Errorf("expected runID run_1, got %s", evt.RunID)
	}

	var payload map[string]any
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["invocation_id"] != "tooluse_123" {
		t.Errorf("expected invocation_id tooluse_123, got %v", payload["invocation_id"])
	}
	if payload["tool_name"] != "Bash" {
		t.Errorf("expected tool_name Bash, got %v", payload["tool_name"])
	}
	// Should use description from ToolInputBash.
	if payload["summary"] != "Run tests" {
		t.Errorf("expected summary 'Run tests', got %v", payload["summary"])
	}
}

func TestConvertPostToolUse(t *testing.T) {
	p := HookPayload{
		HookEventName:   "PostToolUse",
		ToolName:        "Edit",
		ToolUseID:       "tooluse_456",
		ExecutionTimeMS: 150,
		ToolResult: &ToolResultPayload{
			Type: "text",
			Text: "File updated successfully",
		},
	}

	evts := Convert("run_2", p)
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}

	evt := evts[0]
	if evt.Type != events.EventToolCompleted {
		t.Errorf("expected type %s, got %s", events.EventToolCompleted, evt.Type)
	}
	if evt.Summary != "Edit completed (150ms)" {
		t.Errorf("unexpected summary: %s", evt.Summary)
	}
}

func TestConvertSubagentStartStop(t *testing.T) {
	start := HookPayload{
		HookEventName: "SubagentStart",
		AgentID:       "agent_abc",
		AgentType:     "Explore",
	}

	evts := Convert("run_3", start)
	if len(evts) != 2 {
		t.Fatalf("expected 2 events (created+started), got %d", len(evts))
	}
	if evts[0].Type != events.EventStageCreated {
		t.Errorf("expected stage.created, got %s", evts[0].Type)
	}
	if evts[1].Type != events.EventStageStarted {
		t.Errorf("expected stage.started, got %s", evts[1].Type)
	}
	if evts[0].StageID != "agent_abc" {
		t.Errorf("expected stageID agent_abc, got %s", evts[0].StageID)
	}

	stop := HookPayload{
		HookEventName: "SubagentStop",
		AgentID:       "agent_abc",
		AgentType:     "Explore",
	}

	stopEvts := Convert("run_3", stop)
	if len(stopEvts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(stopEvts))
	}
	if stopEvts[0].Type != events.EventStageCompleted {
		t.Errorf("expected stage.completed, got %s", stopEvts[0].Type)
	}
}

func TestConvertUserPromptSubmit(t *testing.T) {
	p := HookPayload{
		HookEventName: "UserPromptSubmit",
		Prompt:        "fix the bug in auth middleware",
	}

	evts := Convert("run_4", p)
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}
	if evts[0].Type != events.EventNoteRecorded {
		t.Errorf("expected note.recorded, got %s", evts[0].Type)
	}
}

func TestConvertStop(t *testing.T) {
	p := HookPayload{
		HookEventName: "Stop",
	}

	evts := Convert("run_5", p)
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}
	if evts[0].Type != events.EventNoteRecorded {
		t.Errorf("expected note.recorded, got %s", evts[0].Type)
	}
}

func TestConvertUnknownHook(t *testing.T) {
	p := HookPayload{
		HookEventName: "SomeFutureHook",
	}

	evts := Convert("run_6", p)
	if len(evts) != 0 {
		t.Errorf("expected 0 events for unknown hook, got %d", len(evts))
	}
}

func TestConvertPreToolUseWithAgentID(t *testing.T) {
	p := HookPayload{
		HookEventName: "PreToolUse",
		ToolName:      "Read",
		ToolUseID:     "tooluse_789",
		ToolInput:     json.RawMessage(`{"file_path":"/tmp/test.go"}`),
		AgentID:       "agent_xyz",
	}

	evts := Convert("run_7", p)
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}
	if evts[0].StageID != "agent_xyz" {
		t.Errorf("expected stageID agent_xyz, got %s", evts[0].StageID)
	}
}

func TestToolSummaryVariousTools(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		want     string
	}{
		{"bash with description", "Bash", `{"command":"ls -la","description":"List files"}`, "List files"},
		{"bash without description", "Bash", `{"command":"ls -la"}`, "ls -la"},
		{"edit", "Edit", `{"file_path":"/tmp/foo.go"}`, "/tmp/foo.go"},
		{"write", "Write", `{"file_path":"/tmp/bar.go"}`, "/tmp/bar.go"},
		{"read", "Read", `{"file_path":"/tmp/baz.go"}`, "/tmp/baz.go"},
		{"glob", "Glob", `{"pattern":"**/*.go"}`, "**/*.go"},
		{"grep", "Grep", `{"pattern":"func main"}`, "func main"},
		{"agent", "Agent", `{"agent_name":"reviewer"}`, "reviewer"},
		{"unknown tool", "CustomTool", `{}`, "CustomTool"},
		{"empty input", "Bash", ``, "Bash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolSummary(tt.toolName, json.RawMessage(tt.input))
			if got != tt.want {
				t.Errorf("toolSummary(%s, %s) = %q, want %q", tt.toolName, tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("this is a long string", 10); got != "this is a ..." {
		t.Errorf("truncate long = %q", got)
	}
}
