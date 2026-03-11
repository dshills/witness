package export

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
)

// JSONExporter writes a single JSON object containing the full run report.
type JSONExporter struct{}

// jsonReport is the top-level structure for JSON export.
type jsonReport struct {
	Run           any `json:"run"`
	Stages        any `json:"stages"`
	Tools         any `json:"tools"`
	ModelRequests any `json:"model_requests"`
	FileChanges   any `json:"file_changes"`
	Commits       any `json:"commits"`
	Alerts        any `json:"alerts"`
	Metrics       any `json:"metrics"`
	Events        any `json:"events"`
}

type jsonMetrics struct {
	TotalInputTokens  int64              `json:"total_input_tokens"`
	TotalOutputTokens int64              `json:"total_output_tokens"`
	TotalCachedTokens int64              `json:"total_cached_tokens"`
	TotalCostUSD      float64            `json:"total_cost_usd"`
	DurationSeconds   float64            `json:"duration_seconds"`
	EventCount        int64              `json:"event_count"`
	FailureCount      int                `json:"failure_count"`
	TokensByProvider  map[string]any     `json:"tokens_by_provider"`
	TokensByModel     map[string]any     `json:"tokens_by_model"`
	CostByProvider    map[string]float64 `json:"cost_by_provider"`
	CostByModel       map[string]float64 `json:"cost_by_model"`
	ToolCounts        map[string]int     `json:"tool_counts"`
}

// Export writes the run report as a single indented JSON object.
func (e *JSONExporter) Export(_ context.Context, state aggregate.RunState, evts []events.Event, w io.Writer) error {
	tokensByProvider := make(map[string]any, len(state.TokensByProvider))
	for k, v := range state.TokensByProvider {
		tokensByProvider[k] = map[string]int64{
			"input":  v.Input,
			"output": v.Output,
			"cached": v.Cached,
		}
	}
	tokensByModel := make(map[string]any, len(state.TokensByModel))
	for k, v := range state.TokensByModel {
		tokensByModel[k] = map[string]int64{
			"input":  v.Input,
			"output": v.Output,
			"cached": v.Cached,
		}
	}

	report := jsonReport{
		Run:           state.Run,
		Stages:        state.Stages,
		Tools:         state.ToolInvocations,
		ModelRequests: state.ModelRequests,
		FileChanges:   state.FileChanges,
		Commits:       state.Commits,
		Alerts:        state.Alerts,
		Metrics: jsonMetrics{
			TotalInputTokens:  state.TotalInputTokens,
			TotalOutputTokens: state.TotalOutputTokens,
			TotalCachedTokens: state.TotalCachedTokens,
			TotalCostUSD:      state.TotalCostUSD,
			DurationSeconds:   state.Duration().Seconds(),
			EventCount:        state.EventCount,
			FailureCount:      state.FailureCount,
			TokensByProvider:  tokensByProvider,
			TokensByModel:     tokensByModel,
			CostByProvider:    state.CostByProvider,
			CostByModel:       state.CostByModel,
			ToolCounts:        state.ToolCounts,
		},
		Events: evts,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
