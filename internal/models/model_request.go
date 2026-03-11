package models

import (
	"encoding/json"
	"time"
)

// ModelRequest represents an LLM/API request.
type ModelRequest struct {
	RequestID       string           `json:"request_id"`
	RunID           string           `json:"run_id"`
	StageID         string           `json:"stage_id,omitempty"`
	Provider        string           `json:"provider"`
	Model           string           `json:"model"`
	StartedAt       time.Time        `json:"started_at"`
	EndedAt         *time.Time       `json:"ended_at,omitempty"`
	Status          InvocationStatus `json:"status"`
	InputTokens     *int64           `json:"input_tokens,omitempty"`
	OutputTokens    *int64           `json:"output_tokens,omitempty"`
	CachedTokens    *int64           `json:"cached_tokens,omitempty"`
	ReasoningTokens *int64           `json:"reasoning_tokens,omitempty"`
	CostUSD         *float64         `json:"cost_usd,omitempty"`
	LatencyMS       *int             `json:"latency_ms,omitempty"`
	Purpose         string           `json:"purpose,omitempty"`
	ToolName        string           `json:"tool_name,omitempty"`
	Metadata        json.RawMessage  `json:"metadata,omitempty"`
}
