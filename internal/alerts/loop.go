package alerts

import (
	"fmt"
	"math"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// LoopRule fires when the same tool is invoked a disproportionate number of
// times within the configured loop window, with no stage transitions.
type LoopRule struct{}

// Name returns the rule name.
func (r *LoopRule) Name() string { return "loop" }

// Evaluate checks for loop conditions in tool invocations.
func (r *LoopRule) Evaluate(state aggregate.RunState, cfg config.AlertsConfig) []models.Alert {
	if cfg.LoopWindow <= 0 {
		return nil
	}

	threshold := int(math.Ceil(float64(cfg.LoopWindow) * 0.75))

	// Check tool invocations.
	if alert := checkToolLoop(state, cfg.LoopWindow, threshold); alert != nil {
		return []models.Alert{*alert}
	}

	return nil
}

// checkToolLoop examines the last LoopWindow tool invocations for repetition
// without stage transitions.
func checkToolLoop(state aggregate.RunState, window, threshold int) *models.Alert {
	invocations := state.ToolInvocations
	if len(invocations) < window {
		return nil
	}

	// Take the last `window` invocations.
	recent := invocations[len(invocations)-window:]

	// Check for stage transitions within the window.
	if hasStageTransitions(recent) {
		return nil
	}

	// Count tool occurrences.
	counts := make(map[string]int, window)
	for i := range recent {
		counts[recent[i].ToolName]++
	}

	// Find the most repeated tool.
	var maxTool string
	var maxCount int
	for tool, count := range counts {
		if count > maxCount {
			maxCount = count
			maxTool = tool
		}
	}

	if maxCount < threshold {
		return nil
	}

	score := float64(maxCount) / float64(window)

	return &models.Alert{
		AlertID:     events.NewID("alert"),
		RunID:       state.Run.RunID,
		Severity:    models.SeverityWarning,
		Type:        "loop.detected",
		Title:       "Repetitive tool pattern detected",
		Description: fmt.Sprintf("Tool %q invoked %d/%d times in last window (score: %.2f)", maxTool, maxCount, window, score),
	}
}

// hasStageTransitions returns true if the invocations span multiple stages.
func hasStageTransitions(invocations []models.ToolInvocation) bool {
	if len(invocations) == 0 {
		return false
	}
	firstStage := invocations[0].StageID
	for i := 1; i < len(invocations); i++ {
		if invocations[i].StageID != firstStage {
			return true
		}
	}
	return false
}
