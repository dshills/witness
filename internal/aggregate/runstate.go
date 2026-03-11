package aggregate

import (
	"encoding/json"
	"time"

	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// maxRecentEvents is the ring buffer capacity for recent events.
const maxRecentEvents = 200

// TokenCount holds token usage broken down by input, output, and cached.
type TokenCount struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
	Cached int64 `json:"cached"`
}

// RunState is the aggregated state of a single run, derived from its event stream.
type RunState struct {
	Run         models.Run             `json:"run"`
	Stages      []models.Stage         `json:"stages"`
	ActiveStage *models.Stage          `json:"active_stage,omitempty"`
	ActiveTool  *models.ToolInvocation `json:"active_tool,omitempty"`
	ActiveModel *models.ModelRequest   `json:"active_model,omitempty"`

	// Counters
	TotalInputTokens  int64                 `json:"total_input_tokens"`
	TotalOutputTokens int64                 `json:"total_output_tokens"`
	TotalCachedTokens int64                 `json:"total_cached_tokens"`
	TotalCostUSD      float64               `json:"total_cost_usd"`
	TokensByProvider  map[string]TokenCount `json:"tokens_by_provider"`
	TokensByModel     map[string]TokenCount `json:"tokens_by_model"`
	TokensByStage     map[string]TokenCount `json:"tokens_by_stage"`
	CostByProvider    map[string]float64    `json:"cost_by_provider"`
	CostByModel       map[string]float64    `json:"cost_by_model"`

	// Tool stats
	ToolInvocations []models.ToolInvocation  `json:"tool_invocations"`
	ToolCounts      map[string]int           `json:"tool_counts"`
	ToolDurations   map[string]time.Duration `json:"tool_durations"`

	// Model stats
	ModelRequests []models.ModelRequest `json:"model_requests"`
	ModelCounts   map[string]int        `json:"model_counts"`

	// Git/Files
	Branch      string              `json:"branch"`
	DirtyFiles  int                 `json:"dirty_files"`
	FileChanges []models.FileChange `json:"file_changes"`
	Commits     []models.Commit     `json:"commits"`
	HotFiles    map[string]int      `json:"hot_files"`

	// Alerts
	Alerts       []models.Alert `json:"alerts"`
	ActiveAlerts []models.Alert `json:"active_alerts"`

	// Health scores
	FailureCount int `json:"failure_count"`
	RetryCount   int `json:"retry_count"`

	// Event stream
	RecentEvents []events.Event `json:"recent_events"`
	EventCount   int64          `json:"event_count"`

	// Timing
	LastEventAt       time.Time `json:"last_event_at"`
	LastFileChangeAt  time.Time `json:"last_file_change_at"`
	LastCommitAt      time.Time `json:"last_commit_at"`
	LastStageChangeAt time.Time `json:"last_stage_change_at"`
}

// Duration returns the elapsed time of the run. If the run has ended,
// it returns the difference between EndedAt and StartedAt. Otherwise
// it returns the time since StartedAt.
func (s *RunState) Duration() time.Duration {
	if s.Run.StartedAt.IsZero() {
		return 0
	}
	if s.Run.EndedAt != nil {
		return s.Run.EndedAt.Sub(s.Run.StartedAt)
	}
	return time.Since(s.Run.StartedAt)
}

// StageDurations returns a map of stage name to duration for all stages
// that have a start time.
func (s *RunState) StageDurations() map[string]time.Duration {
	result := make(map[string]time.Duration, len(s.Stages))
	for i := range s.Stages {
		st := &s.Stages[i]
		if st.StartedAt == nil {
			continue
		}
		if st.EndedAt != nil {
			result[st.Name] = st.EndedAt.Sub(*st.StartedAt)
		} else {
			result[st.Name] = time.Since(*st.StartedAt)
		}
	}
	return result
}

// TokenBurnRate scans RecentEvents for model.request.completed events within
// the last window duration, sums their token counts, and divides by the
// elapsed time span of those events. Returns tokens per second, or 0 if none.
func (s *RunState) TokenBurnRate(window time.Duration) float64 {
	if window <= 0 {
		return 0
	}
	cutoff := time.Now().Add(-window)
	var totalTokens int64
	var earliest, latest time.Time

	for i := range s.RecentEvents {
		evt := &s.RecentEvents[i]
		if evt.Type != events.EventModelRequestCompleted {
			continue
		}
		if evt.Timestamp.Before(cutoff) {
			continue
		}
		var p modelRequestCompletedPayload
		if err := json.Unmarshal(evt.Payload, &p); err != nil {
			continue
		}
		tokens := p.InputTokens + p.OutputTokens + p.CachedTokens
		totalTokens += tokens

		if earliest.IsZero() || evt.Timestamp.Before(earliest) {
			earliest = evt.Timestamp
		}
		if latest.IsZero() || evt.Timestamp.After(latest) {
			latest = evt.Timestamp
		}
	}

	if totalTokens == 0 {
		return 0
	}

	elapsed := latest.Sub(earliest)
	if elapsed <= 0 {
		// Single event or simultaneous events: use window as denominator.
		elapsed = window
	}
	return float64(totalTokens) / elapsed.Seconds()
}

// CostBurnRate scans RecentEvents for model.request.completed events within
// the last window duration, sums their costs, and divides by the elapsed time
// span. Returns cost per second, or 0 if none.
func (s *RunState) CostBurnRate(window time.Duration) float64 {
	if window <= 0 {
		return 0
	}
	cutoff := time.Now().Add(-window)
	var totalCost float64
	var earliest, latest time.Time

	for i := range s.RecentEvents {
		evt := &s.RecentEvents[i]
		if evt.Type != events.EventModelRequestCompleted {
			continue
		}
		if evt.Timestamp.Before(cutoff) {
			continue
		}
		var p modelRequestCompletedPayload
		if err := json.Unmarshal(evt.Payload, &p); err != nil {
			continue
		}
		totalCost += p.CostUSD

		if earliest.IsZero() || evt.Timestamp.Before(earliest) {
			earliest = evt.Timestamp
		}
		if latest.IsZero() || evt.Timestamp.After(latest) {
			latest = evt.Timestamp
		}
	}

	if totalCost == 0 {
		return 0
	}

	elapsed := latest.Sub(earliest)
	if elapsed <= 0 {
		elapsed = window
	}
	return totalCost / elapsed.Seconds()
}

// AvgToolLatency returns the average duration of completed tool invocations.
func (s *RunState) AvgToolLatency() time.Duration {
	var total time.Duration
	var count int
	for i := range s.ToolInvocations {
		inv := &s.ToolInvocations[i]
		if inv.EndedAt == nil {
			continue
		}
		total += inv.EndedAt.Sub(inv.StartedAt)
		count++
	}
	if count == 0 {
		return 0
	}
	return total / time.Duration(count)
}

// AvgModelLatency returns the average duration of completed model requests.
func (s *RunState) AvgModelLatency() time.Duration {
	var total time.Duration
	var count int
	for i := range s.ModelRequests {
		req := &s.ModelRequests[i]
		if req.EndedAt == nil {
			continue
		}
		total += req.EndedAt.Sub(req.StartedAt)
		count++
	}
	if count == 0 {
		return 0
	}
	return total / time.Duration(count)
}

// MeanTimeBetweenCommits returns the average time between consecutive commits.
// Returns 0 if fewer than 2 commits exist.
func (s *RunState) MeanTimeBetweenCommits() time.Duration {
	if len(s.Commits) < 2 {
		return 0
	}
	var total time.Duration
	for i := 1; i < len(s.Commits); i++ {
		total += s.Commits[i].Timestamp.Sub(s.Commits[i-1].Timestamp)
	}
	return total / time.Duration(len(s.Commits)-1)
}

// UniqueFilesTouched returns the number of unique file paths that have been changed.
func (s *RunState) UniqueFilesTouched() int {
	return len(s.HotFiles)
}

// UnmarshalRunState deserializes a RunState from JSON bytes.
func UnmarshalRunState(data []byte) (*RunState, error) {
	var rs RunState
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

// initMaps ensures all map fields are non-nil.
func (s *RunState) initMaps() {
	if s.TokensByProvider == nil {
		s.TokensByProvider = make(map[string]TokenCount)
	}
	if s.TokensByModel == nil {
		s.TokensByModel = make(map[string]TokenCount)
	}
	if s.TokensByStage == nil {
		s.TokensByStage = make(map[string]TokenCount)
	}
	if s.CostByProvider == nil {
		s.CostByProvider = make(map[string]float64)
	}
	if s.CostByModel == nil {
		s.CostByModel = make(map[string]float64)
	}
	if s.ToolCounts == nil {
		s.ToolCounts = make(map[string]int)
	}
	if s.ToolDurations == nil {
		s.ToolDurations = make(map[string]time.Duration)
	}
	if s.ModelCounts == nil {
		s.ModelCounts = make(map[string]int)
	}
	if s.HotFiles == nil {
		s.HotFiles = make(map[string]int)
	}
}
