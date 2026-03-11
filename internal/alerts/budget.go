package alerts

import (
	"fmt"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// BudgetRule fires when cost or token thresholds are exceeded.
type BudgetRule struct{}

// Name returns the rule name.
func (r *BudgetRule) Name() string { return "budget" }

// Evaluate checks run cost, per-stage cost, and token thresholds.
func (r *BudgetRule) Evaluate(state aggregate.RunState, cfg config.AlertsConfig) []models.Alert {
	var alerts []models.Alert

	// Check total run cost.
	if cfg.MaxRunCostUSD > 0 && state.TotalCostUSD > cfg.MaxRunCostUSD {
		alerts = append(alerts, models.Alert{
			AlertID:     events.NewID("alert"),
			RunID:       state.Run.RunID,
			Severity:    models.SeverityError,
			Type:        "budget.run.exceeded",
			Title:       "Run cost budget exceeded",
			Description: fmt.Sprintf("Total cost $%.2f exceeds limit $%.2f", state.TotalCostUSD, cfg.MaxRunCostUSD),
		})
	}

	// Check per-stage cost using model request data.
	if cfg.MaxStageCostUSD > 0 {
		stageCosts := computeStageCosts(state)
		for stageName, cost := range stageCosts {
			if cost > cfg.MaxStageCostUSD {
				alerts = append(alerts, models.Alert{
					AlertID:     events.NewID("alert"),
					RunID:       state.Run.RunID,
					Severity:    models.SeverityWarning,
					Type:        fmt.Sprintf("budget.stage.exceeded.%s", stageName),
					Title:       fmt.Sprintf("Stage %q cost budget exceeded", stageName),
					Description: fmt.Sprintf("Stage cost $%.2f exceeds limit $%.2f", cost, cfg.MaxStageCostUSD),
				})
			}
		}
	}

	// Check total tokens.
	if cfg.MaxTokens > 0 {
		totalTokens := state.TotalInputTokens + state.TotalOutputTokens + state.TotalCachedTokens
		if totalTokens > cfg.MaxTokens {
			alerts = append(alerts, models.Alert{
				AlertID:     events.NewID("alert"),
				RunID:       state.Run.RunID,
				Severity:    models.SeverityWarning,
				Type:        "budget.tokens.exceeded",
				Title:       "Token budget exceeded",
				Description: fmt.Sprintf("Total tokens %d exceeds limit %d", totalTokens, cfg.MaxTokens),
			})
		}
	}

	return alerts
}

// computeStageCosts sums the cost of model requests per stage.
func computeStageCosts(state aggregate.RunState) map[string]float64 {
	costs := make(map[string]float64)
	for i := range state.ModelRequests {
		req := &state.ModelRequests[i]
		if req.StageID == "" || req.CostUSD == nil {
			continue
		}
		// Resolve stage name from ID.
		name := req.StageID
		for j := range state.Stages {
			if state.Stages[j].StageID == req.StageID {
				name = state.Stages[j].Name
				break
			}
		}
		costs[name] += *req.CostUSD
	}
	return costs
}
