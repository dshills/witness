package alerts

import (
	"testing"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/models"
)

func TestBudgetRule_FiresWhenRunCostExceeded(t *testing.T) {
	rule := &BudgetRule{}
	cfg := config.AlertsConfig{MaxRunCostUSD: 25.0}

	state := aggregate.RunState{
		TotalCostUSD: 30.0,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "budget.run.exceeded" {
		t.Errorf("expected type budget.run.exceeded, got %s", alerts[0].Type)
	}
	if alerts[0].Severity != models.SeverityError {
		t.Errorf("expected error severity, got %s", alerts[0].Severity)
	}
}

func TestBudgetRule_FiresPerStageCost(t *testing.T) {
	rule := &BudgetRule{}
	cfg := config.AlertsConfig{MaxStageCostUSD: 8.0}

	cost := 10.0
	state := aggregate.RunState{
		Stages: []models.Stage{
			{StageID: "s1", Name: "build"},
		},
		ModelRequests: []models.ModelRequest{
			{StageID: "s1", CostUSD: &cost},
		},
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for stage cost, got %d", len(alerts))
	}
	if alerts[0].Severity != models.SeverityWarning {
		t.Errorf("expected warning severity, got %s", alerts[0].Severity)
	}
}

func TestBudgetRule_FiresWhenTokensExceeded(t *testing.T) {
	rule := &BudgetRule{}
	cfg := config.AlertsConfig{MaxTokens: 1000}

	state := aggregate.RunState{
		TotalInputTokens:  600,
		TotalOutputTokens: 500,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "budget.tokens.exceeded" {
		t.Errorf("expected type budget.tokens.exceeded, got %s", alerts[0].Type)
	}
}

func TestBudgetRule_DoesNotFireBelowThresholds(t *testing.T) {
	rule := &BudgetRule{}
	cfg := config.AlertsConfig{
		MaxRunCostUSD:   25.0,
		MaxStageCostUSD: 8.0,
		MaxTokens:       10000,
	}

	state := aggregate.RunState{
		TotalCostUSD:      10.0,
		TotalInputTokens:  100,
		TotalOutputTokens: 100,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (below thresholds), got %d", len(alerts))
	}
}

func TestBudgetRule_MultipleAlerts(t *testing.T) {
	rule := &BudgetRule{}
	cfg := config.AlertsConfig{
		MaxRunCostUSD: 25.0,
		MaxTokens:     1000,
	}

	state := aggregate.RunState{
		TotalCostUSD:      30.0,
		TotalInputTokens:  600,
		TotalOutputTokens: 500,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts (cost + tokens), got %d", len(alerts))
	}
}
