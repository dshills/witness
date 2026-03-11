package alerts

import (
	"fmt"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

const (
	failureDensityWindow    = 60 * time.Second
	failureDensityThreshold = 3
)

// failureEventTypes is the set of event types considered as failures.
var failureEventTypes = map[events.EventType]bool{
	events.EventTestFailed:         true,
	events.EventToolFailed:         true,
	events.EventModelRequestFailed: true,
}

// FailureDensityRule fires when multiple failure events cluster within
// a short time window.
type FailureDensityRule struct {
	// Now is an optional override for the current time (for testing).
	Now func() time.Time
}

// Name returns the rule name.
func (r *FailureDensityRule) Name() string { return "failure_density" }

// Evaluate checks for failure clustering within a 60-second window.
func (r *FailureDensityRule) Evaluate(state aggregate.RunState, _ config.AlertsConfig) []models.Alert {
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}

	cutoff := now.Add(-failureDensityWindow)
	var failureCount int

	for i := range state.RecentEvents {
		evt := &state.RecentEvents[i]
		if !failureEventTypes[evt.Type] {
			continue
		}
		if evt.Timestamp.Before(cutoff) {
			continue
		}
		failureCount++
	}

	if failureCount < failureDensityThreshold {
		return nil
	}

	return []models.Alert{
		{
			AlertID:     events.NewID("alert"),
			RunID:       state.Run.RunID,
			Timestamp:   now,
			Severity:    models.SeverityWarning,
			Type:        "failure.density.high",
			Title:       "High failure density detected",
			Description: fmt.Sprintf("%d failure events within last %s", failureCount, failureDensityWindow),
		},
	}
}
