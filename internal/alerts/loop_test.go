package alerts

import (
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/models"
)

func TestLoopRule_FiresForRepeatedTools(t *testing.T) {
	rule := &LoopRule{}
	cfg := config.AlertsConfig{LoopWindow: 8}

	// 7 out of 8 invocations use the same tool (>= 75% threshold of 6).
	invocations := make([]models.ToolInvocation, 8)
	for i := range invocations {
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     "Read",
			StageID:      "stage-1",
			StartedAt:    time.Now(),
		}
	}
	invocations[7].ToolName = "Write" // One different tool.

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "loop.detected" {
		t.Errorf("expected type loop.detected, got %s", alerts[0].Type)
	}
}

func TestLoopRule_DoesNotFireWithStageTransitions(t *testing.T) {
	rule := &LoopRule{}
	cfg := config.AlertsConfig{LoopWindow: 8}

	// All same tool but spanning two stages.
	invocations := make([]models.ToolInvocation, 8)
	for i := range invocations {
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     "Read",
			StageID:      "stage-1",
			StartedAt:    time.Now(),
		}
	}
	invocations[4].StageID = "stage-2" // Stage transition.

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (stage transition), got %d", len(alerts))
	}
}

func TestLoopRule_DoesNotFireBelowThreshold(t *testing.T) {
	rule := &LoopRule{}
	cfg := config.AlertsConfig{LoopWindow: 8}

	// 4 out of 8 (50%) is below 75% threshold.
	invocations := make([]models.ToolInvocation, 8)
	for i := range invocations {
		tool := "Read"
		if i >= 4 {
			tool = "Write"
		}
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     tool,
			StageID:      "stage-1",
			StartedAt:    time.Now(),
		}
	}

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (below threshold), got %d", len(alerts))
	}
}

func TestLoopRule_DoesNotFireWithInsufficientData(t *testing.T) {
	rule := &LoopRule{}
	cfg := config.AlertsConfig{LoopWindow: 8}

	// Only 3 invocations, less than window size.
	invocations := make([]models.ToolInvocation, 3)
	for i := range invocations {
		invocations[i] = models.ToolInvocation{
			InvocationID: "inv-" + string(rune('a'+i)),
			ToolName:     "Read",
			StageID:      "stage-1",
			StartedAt:    time.Now(),
		}
	}

	state := aggregate.RunState{
		ToolInvocations: invocations,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (insufficient data), got %d", len(alerts))
	}
}
