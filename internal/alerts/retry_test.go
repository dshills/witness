package alerts

import (
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/models"
)

func TestRetryStormRule_FiresForRepeatedFailures(t *testing.T) {
	rule := &RetryStormRule{}
	cfg := config.AlertsConfig{}

	invocations := make([]models.ToolInvocation, 10)
	for i := range invocations {
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     "go-test",
			StartedAt:    time.Now(),
			Status:       models.InvocationStatusFailed,
		}
	}
	// Make 5 of them completed (not failed).
	for i := 0; i < 5; i++ {
		invocations[i].Status = models.InvocationStatusCompleted
	}

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "retry.storm" {
		t.Errorf("expected type retry.storm, got %s", alerts[0].Type)
	}
}

func TestRetryStormRule_DoesNotFireForDifferentTools(t *testing.T) {
	rule := &RetryStormRule{}
	cfg := config.AlertsConfig{}

	invocations := make([]models.ToolInvocation, 10)
	for i := range invocations {
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     "tool-" + string(rune('a'+i)), // All different tools.
			StartedAt:    time.Now(),
			Status:       models.InvocationStatusFailed,
		}
	}

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (different tools), got %d", len(alerts))
	}
}

func TestRetryStormRule_DoesNotFireBelowWindowSize(t *testing.T) {
	rule := &RetryStormRule{}
	cfg := config.AlertsConfig{}

	// Only 5 invocations, below the window of 10.
	invocations := make([]models.ToolInvocation, 5)
	for i := range invocations {
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     "go-test",
			StartedAt:    time.Now(),
			Status:       models.InvocationStatusFailed,
		}
	}

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (below window), got %d", len(alerts))
	}
}

func TestRetryStormRule_DoesNotFireForMixedSuccess(t *testing.T) {
	rule := &RetryStormRule{}
	cfg := config.AlertsConfig{}

	invocations := make([]models.ToolInvocation, 10)
	for i := range invocations {
		status := models.InvocationStatusCompleted
		if i%3 == 0 {
			status = models.InvocationStatusFailed
		}
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     "go-test",
			StartedAt:    time.Now(),
			Status:       status,
		}
	}

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (mixed success/failure), got %d", len(alerts))
	}
}
