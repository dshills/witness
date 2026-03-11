// Package alerts implements the anomaly detection engine that evaluates
// aggregated run state and raises alerts for stalls, loops, budget overruns,
// retry storms, and failure density spikes.
package alerts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// Rule evaluates a single heuristic against the current run state.
type Rule interface {
	// Name returns a human-readable identifier for the rule.
	Name() string
	// Evaluate inspects the run state and returns zero or more alerts.
	Evaluate(state aggregate.RunState, cfg config.AlertsConfig) []models.Alert
}

// Engine runs all registered rules, deduplicates alerts, and emits new ones.
type Engine struct {
	rules []Rule
	cfg   config.AlertsConfig
	known map[string]bool // alert type keys already raised
}

// NewEngine creates an Engine with the given alert configuration.
func NewEngine(cfg config.AlertsConfig) *Engine {
	return &Engine{
		cfg:   cfg,
		known: make(map[string]bool),
	}
}

// RegisterRule adds a rule to the engine's evaluation set.
func (e *Engine) RegisterRule(r Rule) {
	e.rules = append(e.rules, r)
}

// Evaluate runs all rules against the current state, deduplicates against
// previously raised alerts, and returns only new alerts.
func (e *Engine) Evaluate(state aggregate.RunState) []models.Alert {
	var newAlerts []models.Alert
	for _, r := range e.rules {
		alerts := r.Evaluate(state, e.cfg)
		for _, a := range alerts {
			key := a.Type
			if e.known[key] {
				continue
			}
			e.known[key] = true
			newAlerts = append(newAlerts, a)
		}
	}
	return newAlerts
}

// EvaluateAndEmit runs Evaluate and emits each new alert as an alert.raised
// event via the provided sink. This is the AlertHook integration point.
func (e *Engine) EvaluateAndEmit(ctx context.Context, state aggregate.RunState, sink events.EventSink) {
	alerts := e.Evaluate(state)
	for _, a := range alerts {
		payload, err := json.Marshal(alertPayload{
			AlertID:     a.AlertID,
			Severity:    a.Severity,
			Type:        a.Type,
			Title:       a.Title,
			Description: a.Description,
			RelatedIDs:  a.RelatedIDs,
			Metadata:    a.Metadata,
		})
		if err != nil {
			continue
		}
		evt := events.NewEvent(state.Run.RunID, events.EventAlertRaised, "alerts.engine", payload)
		if appendErr := sink.Append(ctx, evt); appendErr != nil {
			// Best-effort; alert emission should not break the pipeline.
			fmt.Printf("alerts: failed to emit alert event: %v\n", appendErr)
		}
	}
}

// RegisterDefaultRules adds the built-in heuristic rules to the engine.
func (e *Engine) RegisterDefaultRules() {
	e.RegisterRule(&StallRule{})
	e.RegisterRule(&LoopRule{})
	e.RegisterRule(&BudgetRule{})
	e.RegisterRule(&RetryStormRule{})
	e.RegisterRule(&FailureDensityRule{})
}

// alertPayload mirrors the payload structure for alert.raised events.
type alertPayload struct {
	AlertID     string          `json:"alert_id"`
	Severity    models.Severity `json:"severity"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	RelatedIDs  []string        `json:"related_ids,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}
