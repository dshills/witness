package tui

import (
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

// stubPanel is a minimal Panel implementation for testing.
type stubPanel struct {
	title     string
	focusable bool
	lastState *aggregate.RunState
}

func (p *stubPanel) Init() tea.Cmd { return nil }
func (p *stubPanel) Update(msg tea.Msg) (Panel, tea.Cmd) {
	if sm, ok := msg.(StateMsg); ok {
		p.lastState = &sm.State
	}
	return p, nil
}
func (p *stubPanel) View(width, height int) string {
	_ = width
	_ = height
	return p.title
}
func (p *stubPanel) Title() string   { return p.title }
func (p *stubPanel) Focusable() bool { return p.focusable }

func testPanels() []Panel {
	return []Panel{
		&stubPanel{title: "Header", focusable: false},
		&stubPanel{title: "Stages", focusable: true},
		&stubPanel{title: "ActiveWork", focusable: true},
		&stubPanel{title: "TokenCost", focusable: true},
		&stubPanel{title: "GitFile", focusable: true},
		&stubPanel{title: "Alerts", focusable: true},
		&stubPanel{title: "EventStream", focusable: true},
	}
}

func TestNewApp_InitialFocus(t *testing.T) {
	app := NewApp(testPanels())

	// Header is not focusable, so focus should be on Stages (index 1).
	if app.FocusIndex() != 1 {
		t.Errorf("expected initial focus on index 1 (Stages), got %d", app.FocusIndex())
	}
	if app.DrillDown() != -1 {
		t.Errorf("expected drillDown=-1, got %d", app.DrillDown())
	}
	if app.ShowHelp() {
		t.Error("expected showHelp=false initially")
	}
}

func TestApp_Init(t *testing.T) {
	app := NewApp(testPanels())
	cmd := app.Init()
	// All stub panels return nil from Init, so batch should also be nil.
	if cmd != nil {
		t.Error("expected nil cmd from Init with stub panels")
	}
}

func TestApp_WindowSizeMsg(t *testing.T) {
	app := NewApp(testPanels())
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}

	m, _ := app.Update(msg)
	a := m.(App)

	if a.Width() != 120 {
		t.Errorf("expected width=120, got %d", a.Width())
	}
	if a.Height() != 40 {
		t.Errorf("expected height=40, got %d", a.Height())
	}
}

func TestApp_QuitKey(t *testing.T) {
	app := NewApp(testPanels())
	// Set dimensions first.
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	m, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	_ = m
	if cmd == nil {
		t.Error("expected quit cmd from 'q' key")
	}
}

func TestApp_CtrlCQuit(t *testing.T) {
	app := NewApp(testPanels())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit cmd from ctrl+c")
	}
}

func TestApp_TabCyclesFocus(t *testing.T) {
	app := NewApp(testPanels())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	initial := app.FocusIndex()
	if initial != 1 {
		t.Fatalf("expected initial focus=1, got %d", initial)
	}

	// Tab should advance to next focusable.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = m.(App)
	if app.FocusIndex() != 2 {
		t.Errorf("expected focus=2 after tab, got %d", app.FocusIndex())
	}

	// Shift+tab should go back.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	app = m.(App)
	if app.FocusIndex() != 1 {
		t.Errorf("expected focus=1 after shift+tab, got %d", app.FocusIndex())
	}
}

func TestApp_DrillDown(t *testing.T) {
	app := NewApp(testPanels())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	// Press 's' for stages drill-down.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	app = m.(App)
	if app.DrillDown() != panelStages {
		t.Errorf("expected drillDown=%d, got %d", panelStages, app.DrillDown())
	}

	// Esc should exit drill-down.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	app = m.(App)
	if app.DrillDown() != -1 {
		t.Errorf("expected drillDown=-1 after esc, got %d", app.DrillDown())
	}
}

func TestApp_HelpToggle(t *testing.T) {
	app := NewApp(testPanels())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	// Press '?' to show help.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	app = m.(App)
	if !app.ShowHelp() {
		t.Error("expected showHelp=true after '?'")
	}

	// Any key dismisses help.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	app = m.(App)
	if app.ShowHelp() {
		t.Error("expected showHelp=false after key dismiss")
	}
}

func TestApp_StateMsgBroadcast(t *testing.T) {
	pnls := testPanels()
	app := NewApp(pnls)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	state := aggregate.RunState{
		Run: models.Run{
			RunID:     "test-run",
			Status:    models.RunStatusRunning,
			StartedAt: time.Now(),
		},
	}

	m, _ = app.Update(StateMsg{State: state})
	app = m.(App)

	// Check that all panels received the state.
	for i, p := range app.panels {
		sp := p.(*stubPanel)
		if sp.lastState == nil {
			t.Errorf("panel %d (%s) did not receive state", i, sp.title)
			continue
		}
		if sp.lastState.Run.RunID != "test-run" {
			t.Errorf("panel %d got RunID=%s, want test-run", i, sp.lastState.Run.RunID)
		}
	}
}

func TestApp_ViewNotReady(t *testing.T) {
	app := NewApp(testPanels())
	view := app.View()
	if view != "Initializing..." {
		t.Errorf("expected 'Initializing...' view before ready, got %q", view)
	}
}

func TestApp_FullLayout(t *testing.T) {
	app := NewApp(testPanels())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	view := app.View()
	if view == "" {
		t.Error("expected non-empty view for full layout")
	}
}

func TestApp_CompactLayout(t *testing.T) {
	app := NewApp(testPanels())
	// Narrow terminal triggers compact layout.
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = m.(App)

	view := app.View()
	if view == "" {
		t.Error("expected non-empty view for compact layout")
	}
}
