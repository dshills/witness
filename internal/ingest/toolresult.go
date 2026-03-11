package ingest

import (
	"encoding/json"
	"fmt"

	"github.com/dshills/witness/internal/events"
)

// ToolResult is the structured output contract for external tools.
// Tools that emit this JSON schema on stdout will have their results
// automatically converted into Witness events.
type ToolResult struct {
	Tool       string                     `json:"tool"`
	Status     string                     `json:"status"`
	Summary    string                     `json:"summary"`
	Findings   map[string]int             `json:"findings,omitempty"`
	Artifacts  *ToolArtifacts             `json:"artifacts,omitempty"`
	DurationMS int                        `json:"duration_ms,omitempty"`
	Tokens     *ToolTokens                `json:"tokens,omitempty"`
	Model      string                     `json:"model,omitempty"`
	Provider   string                     `json:"provider,omitempty"`
	Extra      map[string]json.RawMessage `json:"extra,omitempty"`
}

// ToolArtifacts lists files produced or modified by a tool.
type ToolArtifacts struct {
	Files []string `json:"files,omitempty"`
}

// ToolTokens carries LLM token usage from a tool invocation.
type ToolTokens struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
	Cached int64 `json:"cached,omitempty"`
}

// ParseToolResult attempts to parse a line as a ToolResult.
// Returns an error if the line is not valid JSON or lacks required fields.
func ParseToolResult(line []byte) (*ToolResult, error) {
	var r ToolResult
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, fmt.Errorf("unmarshal tool result: %w", err)
	}
	if r.Tool == "" {
		return nil, fmt.Errorf("tool result missing required field: tool")
	}
	if r.Status == "" {
		return nil, fmt.Errorf("tool result missing required field: status")
	}
	return &r, nil
}

// ToolResultToEvents converts a ToolResult into one or more Witness events.
// It always emits a tool.completed event, and conditionally emits
// model.request.completed and finding.recorded events.
func ToolResultToEvents(result ToolResult, runID, stageID string) []events.Event {
	var out []events.Event

	// 1. tool.completed event
	toolPayload := map[string]any{
		"tool":    result.Tool,
		"status":  result.Status,
		"summary": result.Summary,
	}
	if result.DurationMS > 0 {
		toolPayload["duration_ms"] = result.DurationMS
	}
	if result.Findings != nil {
		toolPayload["findings"] = result.Findings
	}
	if result.Artifacts != nil {
		toolPayload["artifacts"] = result.Artifacts
	}

	toolEvt := events.NewEvent(runID, events.EventToolCompleted, result.Tool, mustMarshal(toolPayload))
	toolEvt.StageID = stageID
	toolEvt.Status = result.Status
	toolEvt.Summary = result.Summary
	out = append(out, toolEvt)

	// 2. model.request.completed if token/model info present
	if result.Tokens != nil && result.Model != "" {
		modelPayload := map[string]any{
			"model":         result.Model,
			"input_tokens":  result.Tokens.Input,
			"output_tokens": result.Tokens.Output,
		}
		if result.Provider != "" {
			modelPayload["provider"] = result.Provider
		}
		if result.Tokens.Cached > 0 {
			modelPayload["cached_tokens"] = result.Tokens.Cached
		}

		modelEvt := events.NewEvent(runID, events.EventModelRequestCompleted, result.Tool, mustMarshal(modelPayload))
		modelEvt.StageID = stageID
		out = append(out, modelEvt)
	}

	// 3. finding.recorded for each category with count > 0
	for category, count := range result.Findings {
		if count <= 0 {
			continue
		}
		findingPayload := map[string]any{
			"tool":     result.Tool,
			"category": category,
			"count":    count,
		}
		findingEvt := events.NewEvent(runID, events.EventFindingRecorded, result.Tool, mustMarshal(findingPayload))
		findingEvt.StageID = stageID
		out = append(out, findingEvt)
	}

	return out
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal payload: %v", err))
	}
	return b
}
