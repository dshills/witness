package alerts

import (
	"fmt"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

const (
	retryWindowSize    = 10
	retryFailThreshold = 5
)

// RetryStormRule fires when the same tool has failed repeatedly within
// the last 10 tool invocations.
type RetryStormRule struct{}

// Name returns the rule name.
func (r *RetryStormRule) Name() string { return "retry_storm" }

// Evaluate checks for retry storm conditions.
func (r *RetryStormRule) Evaluate(state aggregate.RunState, _ config.AlertsConfig) []models.Alert {
	invocations := state.ToolInvocations
	if len(invocations) < retryWindowSize {
		return nil
	}

	recent := invocations[len(invocations)-retryWindowSize:]

	// Count failures per tool.
	failCounts := make(map[string]int)
	for i := range recent {
		if recent[i].Status == models.InvocationStatusFailed {
			failCounts[recent[i].ToolName]++
		}
	}

	for tool, count := range failCounts {
		if count >= retryFailThreshold {
			return []models.Alert{
				{
					AlertID:     events.NewID("alert"),
					RunID:       state.Run.RunID,
					Severity:    models.SeverityWarning,
					Type:        "retry.storm",
					Title:       "Retry storm detected",
					Description: fmt.Sprintf("Tool %q failed %d/%d times in last %d invocations", tool, count, retryWindowSize, retryWindowSize),
				},
			}
		}
	}

	return nil
}
