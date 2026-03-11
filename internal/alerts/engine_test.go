package alerts

import (
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// fakeRule returns a fixed set of alerts when Evaluate is called.
type fakeRule struct {
	name   string
	alerts []models.Alert
}

func (r *fakeRule) Name() string { return r.name }
func (r *fakeRule) Evaluate(_ aggregate.RunState, _ config.AlertsConfig) []models.Alert {
	return r.alerts
}

func TestEngine_Deduplication(t *testing.T) {
	cfg := config.AlertsConfig{}
	engine := NewEngine(cfg)

	alert := models.Alert{
		AlertID:  events.NewID("alert"),
		Type:     "test.alert",
		Severity: models.SeverityWarning,
		Title:    "Test alert",
	}

	engine.RegisterRule(&fakeRule{name: "rule1", alerts: []models.Alert{alert}})

	state := aggregate.RunState{}

	// First evaluation should return the alert.
	got := engine.Evaluate(state)
	if len(got) != 1 {
		t.Fatalf("first Evaluate: expected 1 alert, got %d", len(got))
	}

	// Second evaluation should return nothing (dedup).
	got = engine.Evaluate(state)
	if len(got) != 0 {
		t.Fatalf("second Evaluate: expected 0 alerts (dedup), got %d", len(got))
	}
}

func TestEngine_MultipleRulesFireIndependently(t *testing.T) {
	cfg := config.AlertsConfig{}
	engine := NewEngine(cfg)

	alert1 := models.Alert{AlertID: events.NewID("alert"), Type: "type.a", Severity: models.SeverityWarning}
	alert2 := models.Alert{AlertID: events.NewID("alert"), Type: "type.b", Severity: models.SeverityError}

	engine.RegisterRule(&fakeRule{name: "ruleA", alerts: []models.Alert{alert1}})
	engine.RegisterRule(&fakeRule{name: "ruleB", alerts: []models.Alert{alert2}})

	state := aggregate.RunState{}

	got := engine.Evaluate(state)
	if len(got) != 2 {
		t.Fatalf("expected 2 alerts from independent rules, got %d", len(got))
	}

	typesSeen := map[string]bool{}
	for _, a := range got {
		typesSeen[a.Type] = true
	}
	if !typesSeen["type.a"] || !typesSeen["type.b"] {
		t.Errorf("expected both type.a and type.b, got %v", typesSeen)
	}
}

func TestEngine_RegisterDefaultRules(t *testing.T) {
	cfg := config.AlertsConfig{
		StallDuration:   10 * time.Minute,
		LoopWindow:      8,
		MaxRunCostUSD:   25.0,
		MaxStageCostUSD: 8.0,
	}
	engine := NewEngine(cfg)
	engine.RegisterDefaultRules()

	// Should have 5 default rules.
	if len(engine.rules) != 5 {
		t.Errorf("expected 5 default rules, got %d", len(engine.rules))
	}
}
