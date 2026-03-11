package alerts

import (
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/models"
)

func TestStallRule_FiresAfterThreshold(t *testing.T) {
	now := time.Now()
	stallDuration := 5 * time.Minute

	rule := &StallRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{StallDuration: stallDuration}

	state := aggregate.RunState{
		Run: models.Run{
			Status:    models.RunStatusRunning,
			StartedAt: now.Add(-10 * time.Minute),
		},
		EventCount:        10,
		LastFileChangeAt:  now.Add(-6 * time.Minute),
		LastStageChangeAt: now.Add(-7 * time.Minute),
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "stall.detected" {
		t.Errorf("expected type stall.detected, got %s", alerts[0].Type)
	}
	if alerts[0].Severity != models.SeverityWarning {
		t.Errorf("expected warning severity, got %s", alerts[0].Severity)
	}
}

func TestStallRule_DoesNotFireWithRecentFileChanges(t *testing.T) {
	now := time.Now()
	stallDuration := 5 * time.Minute

	rule := &StallRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{StallDuration: stallDuration}

	state := aggregate.RunState{
		Run: models.Run{
			Status:    models.RunStatusRunning,
			StartedAt: now.Add(-10 * time.Minute),
		},
		EventCount:        10,
		LastFileChangeAt:  now.Add(-2 * time.Minute), // Recent file change.
		LastStageChangeAt: now.Add(-7 * time.Minute),
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (recent file change), got %d", len(alerts))
	}
}

func TestStallRule_DoesNotFireForCompletedRuns(t *testing.T) {
	now := time.Now()
	stallDuration := 5 * time.Minute

	rule := &StallRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{StallDuration: stallDuration}

	state := aggregate.RunState{
		Run: models.Run{
			Status:    models.RunStatusCompleted,
			StartedAt: now.Add(-10 * time.Minute),
		},
		EventCount:        10,
		LastFileChangeAt:  now.Add(-6 * time.Minute),
		LastStageChangeAt: now.Add(-7 * time.Minute),
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (completed run), got %d", len(alerts))
	}
}

func TestStallRule_DoesNotFireForBrandNewRun(t *testing.T) {
	now := time.Now()
	stallDuration := 5 * time.Minute

	rule := &StallRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{StallDuration: stallDuration}

	state := aggregate.RunState{
		Run: models.Run{
			Status:    models.RunStatusRunning,
			StartedAt: now.Add(-10 * time.Minute),
		},
		EventCount: 0, // No events yet.
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (brand new run), got %d", len(alerts))
	}
}
