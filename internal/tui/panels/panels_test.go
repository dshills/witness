package panels

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/tui"
)

// populatedState returns a RunState with representative data.
func populatedState() aggregate.RunState {
	now := time.Now()
	started := now.Add(-5 * time.Minute)
	stageStarted := now.Add(-2 * time.Minute)
	pct := 50.0

	return aggregate.RunState{
		Run: models.Run{
			RunID:     "run_01ABC",
			Name:      "test-run",
			Status:    models.RunStatusRunning,
			Branch:    "main",
			StartedAt: started,
			RepoRoot:  "/tmp/repo",
		},
		Stages: []models.Stage{
			{StageID: "s1", Name: "init", Order: 0, Status: models.StageStatusCompleted},
			{StageID: "s2", Name: "build", Order: 1, Status: models.StageStatusRunning, StartedAt: &stageStarted, ProgressPercent: &pct},
			{StageID: "s3", Name: "test", Order: 2, Status: models.StageStatusPending},
		},
		ActiveTool: &models.ToolInvocation{
			InvocationID: "t1",
			ToolName:     "go-test",
			StartedAt:    now.Add(-10 * time.Second),
			Summary:      "running tests",
		},
		ActiveModel: &models.ModelRequest{
			RequestID: "m1",
			Provider:  "anthropic",
			Model:     "claude-3",
			StartedAt: now.Add(-5 * time.Second),
			Purpose:   "code review",
		},
		TotalInputTokens:  150000,
		TotalOutputTokens: 50000,
		TotalCachedTokens: 10000,
		TotalCostUSD:      1.2345,
		CostByModel: map[string]float64{
			"claude-3": 0.8,
			"gpt-4":    0.4345,
		},
		ToolInvocations: []models.ToolInvocation{
			{InvocationID: "t0", ToolName: "lint", StartedAt: now.Add(-3 * time.Minute)},
		},
		ModelRequests: []models.ModelRequest{
			{RequestID: "m0", Provider: "openai", Model: "gpt-4", StartedAt: now.Add(-4 * time.Minute)},
		},
		FileChanges: []models.FileChange{
			{ChangeID: "f1", Path: "main.go", ChangeType: models.ChangeTypeModified, Timestamp: now},
			{ChangeID: "f2", Path: "go.mod", ChangeType: models.ChangeTypeModified, Timestamp: now},
			{ChangeID: "f3", Path: "new.go", ChangeType: models.ChangeTypeCreated, Timestamp: now},
		},
		Commits: []models.Commit{
			{CommitID: "c1", SHA: "abc1234567890", Message: "initial commit", Timestamp: now.Add(-3 * time.Minute)},
		},
		HotFiles: map[string]int{
			"main.go": 5,
			"go.mod":  2,
		},
		DirtyFiles: 3,
		ActiveAlerts: []models.Alert{
			{AlertID: "a1", Severity: models.SeverityWarning, Title: "High cost", Description: "exceeding budget"},
		},
		RecentEvents: []events.Event{
			{EventID: "e1", Type: events.EventRunStarted, Source: "witness", Summary: "run started", Timestamp: now.Add(-5 * time.Minute)},
			{EventID: "e2", Type: events.EventToolStarted, Source: "go-test", Summary: "test begin", Timestamp: now.Add(-10 * time.Second)},
		},
		EventCount: 42,
	}
}

func TestHeaderPanel_EmptyState(t *testing.T) {
	p := NewHeaderPanel()
	view := p.View(80, 1)
	if view == "" {
		t.Error("expected non-empty view from HeaderPanel with empty state")
	}
}

func TestHeaderPanel_PopulatedState(t *testing.T) {
	p := NewHeaderPanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})
	view := p.View(120, 1)
	if !strings.Contains(view, "test-run") {
		t.Errorf("expected run name in header, got %q", view)
	}
	if !strings.Contains(view, "running") {
		t.Errorf("expected status in header, got %q", view)
	}
}

func TestStagePanel_EmptyState(t *testing.T) {
	p := NewStagePanel()
	view := p.View(40, 5)
	if !strings.Contains(view, "no stages") {
		t.Errorf("expected 'no stages' message, got %q", view)
	}
}

func TestStagePanel_PopulatedState(t *testing.T) {
	p := NewStagePanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})
	view := p.View(60, 10)
	if !strings.Contains(view, "init") {
		t.Errorf("expected stage name 'init' in view, got %q", view)
	}
	if !strings.Contains(view, "build") {
		t.Errorf("expected stage name 'build' in view, got %q", view)
	}
}

func TestActiveWorkPanel_EmptyState(t *testing.T) {
	p := NewActiveWorkPanel()
	view := p.View(60, 5)
	if !strings.Contains(view, "idle") {
		t.Errorf("expected 'idle' in empty active work view, got %q", view)
	}
}

func TestActiveWorkPanel_PopulatedState(t *testing.T) {
	p := NewActiveWorkPanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})
	view := p.View(80, 6)
	if !strings.Contains(view, "go-test") {
		t.Errorf("expected tool name in view, got %q", view)
	}
	if !strings.Contains(view, "claude-3") {
		t.Errorf("expected model name in view, got %q", view)
	}
}

func TestTokenCostPanel_EmptyState(t *testing.T) {
	p := NewTokenCostPanel(0)
	view := p.View(60, 5)
	if !strings.Contains(view, "Tokens:") {
		t.Errorf("expected 'Tokens:' in view, got %q", view)
	}
}

func TestTokenCostPanel_PopulatedState(t *testing.T) {
	p := NewTokenCostPanel(1.0)
	state := populatedState()
	p.Update(tui.StateMsg{State: state})
	view := p.View(80, 8)
	if !strings.Contains(view, "$1.2345") {
		t.Errorf("expected cost in view, got %q", view)
	}
	if !strings.Contains(view, "OVER BUDGET") {
		t.Errorf("expected budget warning in view, got %q", view)
	}
}

func TestGitFilePanel_EmptyState(t *testing.T) {
	p := NewGitFilePanel()
	view := p.View(60, 5)
	if !strings.Contains(view, "Files:") {
		t.Errorf("expected 'Files:' in view, got %q", view)
	}
}

func TestGitFilePanel_PopulatedState(t *testing.T) {
	p := NewGitFilePanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})
	view := p.View(80, 10)
	if !strings.Contains(view, "abc1234") {
		t.Errorf("expected truncated SHA in view, got %q", view)
	}
}

func TestAlertsPanel_EmptyState(t *testing.T) {
	p := NewAlertsPanel()
	view := p.View(60, 3)
	if !strings.Contains(view, "no active alerts") {
		t.Errorf("expected 'no active alerts' in view, got %q", view)
	}
}

func TestAlertsPanel_PopulatedState(t *testing.T) {
	p := NewAlertsPanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})
	view := p.View(80, 5)
	if !strings.Contains(view, "High cost") {
		t.Errorf("expected alert title in view, got %q", view)
	}
}

func TestEventStreamPanel_EmptyState(t *testing.T) {
	p := NewEventStreamPanel()
	view := p.View(60, 5)
	if !strings.Contains(view, "no events") {
		t.Errorf("expected 'no events' in view, got %q", view)
	}
}

func TestEventStreamPanel_PopulatedState(t *testing.T) {
	p := NewEventStreamPanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})
	view := p.View(80, 10)
	if !strings.Contains(view, "run.started") {
		t.Errorf("expected event type in view, got %q", view)
	}
}

func TestEventStreamPanel_ScrollAndPause(t *testing.T) {
	p := NewEventStreamPanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})

	// Press 'p' to pause.
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !p.paused {
		t.Error("expected paused after 'p'")
	}

	// Press 'r' to resume.
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if p.paused {
		t.Error("expected not paused after 'r'")
	}
}

func TestStagePanel_Scroll(t *testing.T) {
	p := NewStagePanel()
	state := populatedState()
	p.Update(tui.StateMsg{State: state})

	// Scroll down.
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if p.offset != 1 {
		t.Errorf("expected offset=1 after scroll down, got %d", p.offset)
	}

	// Scroll up.
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if p.offset != 0 {
		t.Errorf("expected offset=0 after scroll up, got %d", p.offset)
	}
}

func TestAllPanels_NoPanicOnZeroDimensions(t *testing.T) {
	allPanels := []tui.Panel{
		NewHeaderPanel(),
		NewStagePanel(),
		NewActiveWorkPanel(),
		NewTokenCostPanel(0),
		NewGitFilePanel(),
		NewAlertsPanel(),
		NewEventStreamPanel(),
	}

	for _, p := range allPanels {
		t.Run(p.Title()+"_zero", func(t *testing.T) {
			// Should not panic.
			_ = p.View(0, 0)
		})
	}
}

func TestAllPanels_Focusable(t *testing.T) {
	tests := []struct {
		panel     tui.Panel
		focusable bool
	}{
		{NewHeaderPanel(), false},
		{NewStagePanel(), true},
		{NewActiveWorkPanel(), true},
		{NewTokenCostPanel(0), true},
		{NewGitFilePanel(), true},
		{NewAlertsPanel(), true},
		{NewEventStreamPanel(), true},
	}
	for _, tt := range tests {
		t.Run(tt.panel.Title(), func(t *testing.T) {
			if got := tt.panel.Focusable(); got != tt.focusable {
				t.Errorf("Focusable() = %v, want %v", got, tt.focusable)
			}
		})
	}
}
