package claudehooks

import (
	"encoding/json"
	"fmt"

	"github.com/dshills/witness/internal/events"
)

const hookSource = "claude-code/hooks"

// Convert transforms a Claude Code hook payload into zero or more Witness events.
func Convert(runID string, p HookPayload) []events.Event {
	switch p.HookEventName {
	case "PreToolUse":
		return convertPreToolUse(runID, p)
	case "PostToolUse":
		return convertPostToolUse(runID, p)
	case "SubagentStart":
		return convertSubagentStart(runID, p)
	case "SubagentStop":
		return convertSubagentStop(runID, p)
	case "UserPromptSubmit":
		return convertUserPrompt(runID, p)
	case "Stop":
		return convertStop(runID, p)
	default:
		return nil
	}
}

func convertPreToolUse(runID string, p HookPayload) []events.Event {
	invocationID := p.ToolUseID
	if invocationID == "" {
		invocationID = events.NewID("tool")
	}

	summary := toolSummary(p.ToolName, p.ToolInput)

	payload, _ := json.Marshal(map[string]any{
		"invocation_id": invocationID,
		"tool_name":     p.ToolName,
		"summary":       summary,
	})

	evt := events.NewEvent(runID, events.EventToolStarted, hookSource, payload)
	evt.Summary = fmt.Sprintf("%s: %s", p.ToolName, summary)

	if p.AgentID != "" {
		evt.StageID = p.AgentID
	}

	return []events.Event{evt}
}

func convertPostToolUse(runID string, p HookPayload) []events.Event {
	invocationID := p.ToolUseID
	if invocationID == "" {
		invocationID = events.NewID("tool")
	}

	durationMS := p.ExecutionTimeMS
	payload, _ := json.Marshal(map[string]any{
		"invocation_id": invocationID,
		"duration_ms":   durationMS,
		"summary":       truncate(toolResultSummary(p.ToolResult), 200),
	})

	evt := events.NewEvent(runID, events.EventToolCompleted, hookSource, payload)
	evt.Summary = fmt.Sprintf("%s completed (%dms)", p.ToolName, durationMS)

	if p.AgentID != "" {
		evt.StageID = p.AgentID
	}

	return []events.Event{evt}
}

func convertSubagentStart(runID string, p HookPayload) []events.Event {
	stageID := p.AgentID
	if stageID == "" {
		stageID = events.NewID("stage")
	}

	name := p.AgentType
	if name == "" {
		name = "subagent"
	}

	payload, _ := json.Marshal(map[string]any{
		"stage_id": stageID,
		"name":     name,
	})

	created := events.NewEvent(runID, events.EventStageCreated, hookSource, payload)
	created.StageID = stageID
	created.Summary = fmt.Sprintf("Agent: %s", name)

	started := events.NewEvent(runID, events.EventStageStarted, hookSource, payload)
	started.StageID = stageID

	return []events.Event{created, started}
}

func convertSubagentStop(runID string, p HookPayload) []events.Event {
	stageID := p.AgentID
	if stageID == "" {
		return nil
	}

	payload, _ := json.Marshal(map[string]any{
		"stage_id": stageID,
	})

	evt := events.NewEvent(runID, events.EventStageCompleted, hookSource, payload)
	evt.StageID = stageID

	return []events.Event{evt}
}

func convertUserPrompt(runID string, p HookPayload) []events.Event {
	prompt := truncate(p.Prompt, 500)
	payload, _ := json.Marshal(map[string]any{
		"note": prompt,
	})

	evt := events.NewEvent(runID, events.EventNoteRecorded, hookSource, payload)
	evt.Summary = fmt.Sprintf("User prompt: %s", truncate(prompt, 100))

	return []events.Event{evt}
}

func convertStop(runID string, _ HookPayload) []events.Event {
	payload, _ := json.Marshal(map[string]any{
		"note": "Claude finished responding",
	})

	evt := events.NewEvent(runID, events.EventNoteRecorded, hookSource, payload)
	evt.Summary = "Response complete"

	return []events.Event{evt}
}

// toolSummary extracts a human-readable summary from a tool's input.
func toolSummary(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return toolName
	}

	switch toolName {
	case "Bash":
		var ti ToolInputBash
		if json.Unmarshal(input, &ti) == nil && ti.Command != "" {
			if ti.Description != "" {
				return ti.Description
			}
			return truncate(ti.Command, 120)
		}
	case "Edit", "Write":
		var ti ToolInputEdit
		if json.Unmarshal(input, &ti) == nil && ti.FilePath != "" {
			return ti.FilePath
		}
	case "Read":
		var ti ToolInputRead
		if json.Unmarshal(input, &ti) == nil && ti.FilePath != "" {
			return ti.FilePath
		}
	case "Glob", "Grep":
		var ti ToolInputSearch
		if json.Unmarshal(input, &ti) == nil && ti.Pattern != "" {
			return ti.Pattern
		}
	case "Agent":
		var ti ToolInputAgent
		if json.Unmarshal(input, &ti) == nil {
			if ti.AgentName != "" {
				return ti.AgentName
			}
			return truncate(ti.Prompt, 120)
		}
	}

	return toolName
}

// toolResultSummary extracts summary text from a tool result.
func toolResultSummary(r *ToolResultPayload) string {
	if r == nil {
		return ""
	}
	return r.Text
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
