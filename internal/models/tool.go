package models

import (
	"encoding/json"
	"time"
)

// ToolInvocation represents the execution of a CLI tool or sub-tool.
type ToolInvocation struct {
	InvocationID string           `json:"invocation_id"`
	RunID        string           `json:"run_id"`
	StageID      string           `json:"stage_id,omitempty"`
	ToolName     string           `json:"tool_name"`
	Command      []string         `json:"command,omitempty"`
	StartedAt    time.Time        `json:"started_at"`
	EndedAt      *time.Time       `json:"ended_at,omitempty"`
	Status       InvocationStatus `json:"status"`
	ExitCode     *int             `json:"exit_code,omitempty"`
	DurationMS   *int             `json:"duration_ms,omitempty"`
	Summary      string           `json:"summary,omitempty"`
	Findings     json.RawMessage  `json:"findings,omitempty"`
	Metadata     json.RawMessage  `json:"metadata,omitempty"`
}
