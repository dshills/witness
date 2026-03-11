package alerts

import (
	"fmt"
	"math"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// StallRule fires when a run is actively running but has had no file or stage
// changes for longer than the configured stall duration.
type StallRule struct {
	// Now is an optional override for the current time (for testing).
	Now func() time.Time
}

// Name returns the rule name.
func (r *StallRule) Name() string { return "stall" }

// Evaluate checks for stall conditions.
func (r *StallRule) Evaluate(state aggregate.RunState, cfg config.AlertsConfig) []models.Alert {
	if cfg.StallDuration <= 0 {
		return nil
	}

	// Only fire for running runs.
	if state.Run.Status != models.RunStatusRunning {
		return nil
	}

	// Must have received at least one event (not brand new).
	if state.EventCount == 0 {
		return nil
	}

	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}

	// Check both file and stage change staleness.
	fileStaleDur := staleDuration(now, state.LastFileChangeAt, state.Run.StartedAt)
	stageStaleDur := staleDuration(now, state.LastStageChangeAt, state.Run.StartedAt)

	if fileStaleDur <= cfg.StallDuration || stageStaleDur <= cfg.StallDuration {
		return nil
	}

	// Use the shorter of the two as the stalled duration for scoring.
	stalledDur := fileStaleDur
	if stageStaleDur < stalledDur {
		stalledDur = stageStaleDur
	}

	score := math.Min(1.0, float64(stalledDur)/float64(2*cfg.StallDuration))

	return []models.Alert{
		{
			AlertID:     events.NewID("alert"),
			RunID:       state.Run.RunID,
			Timestamp:   now,
			Severity:    models.SeverityWarning,
			Type:        "stall.detected",
			Title:       "Run appears stalled",
			Description: fmt.Sprintf("No file or stage changes for %s (score: %.2f)", stalledDur.Round(time.Second), score),
		},
	}
}

// staleDuration returns the time since the last activity. If lastActivity is
// zero (never happened), it falls back to the run start time.
func staleDuration(now, lastActivity, runStart time.Time) time.Duration {
	ref := lastActivity
	if ref.IsZero() {
		ref = runStart
	}
	if ref.IsZero() {
		return 0
	}
	return now.Sub(ref)
}
