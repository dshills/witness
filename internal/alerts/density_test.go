package alerts

import (
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

func TestFailureDensityRule_FiresForClusteredFailures(t *testing.T) {
	now := time.Now()
	rule := &FailureDensityRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{}

	// 3 failures within last 60 seconds.
	recentEvents := []events.Event{
		{Type: events.EventToolFailed, Timestamp: now.Add(-10 * time.Second)},
		{Type: events.EventTestFailed, Timestamp: now.Add(-20 * time.Second)},
		{Type: events.EventModelRequestFailed, Timestamp: now.Add(-30 * time.Second)},
	}

	state := aggregate.RunState{
		RecentEvents: recentEvents,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "failure.density.high" {
		t.Errorf("expected type failure.density.high, got %s", alerts[0].Type)
	}
	if alerts[0].Severity != models.SeverityWarning {
		t.Errorf("expected warning severity, got %s", alerts[0].Severity)
	}
}

func TestFailureDensityRule_DoesNotFireForSpreadOutFailures(t *testing.T) {
	now := time.Now()
	rule := &FailureDensityRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{}

	// 3 failures but spread over more than 60 seconds (only 1 within window).
	recentEvents := []events.Event{
		{Type: events.EventToolFailed, Timestamp: now.Add(-10 * time.Second)},
		{Type: events.EventTestFailed, Timestamp: now.Add(-90 * time.Second)},
		{Type: events.EventModelRequestFailed, Timestamp: now.Add(-120 * time.Second)},
	}

	state := aggregate.RunState{
		RecentEvents: recentEvents,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (spread out), got %d", len(alerts))
	}
}

func TestFailureDensityRule_DoesNotFireForNonFailureEvents(t *testing.T) {
	now := time.Now()
	rule := &FailureDensityRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{}

	// Lots of events but none are failures.
	recentEvents := []events.Event{
		{Type: events.EventToolCompleted, Timestamp: now.Add(-10 * time.Second)},
		{Type: events.EventModelRequestCompleted, Timestamp: now.Add(-20 * time.Second)},
		{Type: events.EventNoteRecorded, Timestamp: now.Add(-30 * time.Second)},
	}

	state := aggregate.RunState{
		RecentEvents: recentEvents,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (no failures), got %d", len(alerts))
	}
}

func TestFailureDensityRule_DoesNotFireBelowThreshold(t *testing.T) {
	now := time.Now()
	rule := &FailureDensityRule{Now: func() time.Time { return now }}
	cfg := config.AlertsConfig{}

	// Only 2 failures within window (below threshold of 3).
	recentEvents := []events.Event{
		{Type: events.EventToolFailed, Timestamp: now.Add(-10 * time.Second)},
		{Type: events.EventTestFailed, Timestamp: now.Add(-20 * time.Second)},
	}

	state := aggregate.RunState{
		RecentEvents: recentEvents,
	}

	alerts := rule.Evaluate(state, cfg)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (below threshold), got %d", len(alerts))
	}
}
