package tui

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/replay"

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

// stateCounter is a tea.Model that counts incoming StateMsg / ReplayStatusMsg
// messages. It lets the bridge tests verify that the goroutine actually
// delivered state to the program without caring about rendering.
type stateCounter struct {
	state  *atomic.Int64
	status *atomic.Int64
}

func (m stateCounter) Init() tea.Cmd { return nil }
func (m stateCounter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case StateMsg:
		m.state.Add(1)
	case ReplayStatusMsg:
		m.status.Add(1)
	case tea.QuitMsg:
		return m, tea.Quit
	}
	return m, nil
}
func (m stateCounter) View() string { return "" }

// quietProgram builds a tea.Program that produces no terminal I/O, suitable
// for running in tests.
func quietProgram(m tea.Model) *tea.Program {
	return tea.NewProgram(m,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutSignals(),
		tea.WithoutRenderer(),
	)
}

// TESTREC-7531EB78: race-safety test for RunBridge. Feeds events from multiple
// goroutines while the bridge drains them and forwards state to a live
// tea.Program. Run with `go test -race`.
func TestRunBridge(t *testing.T) {
	t.Parallel()

	var stateCount atomic.Int64
	model := stateCounter{state: &stateCount, status: new(atomic.Int64)}

	p := quietProgram(model)
	progDone := make(chan struct{})
	go func() {
		_, _ = p.Run()
		close(progDone)
	}()
	t.Cleanup(func() {
		p.Quit()
		<-progDone
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := make(chan events.Event, 64)
	bridgeDone := make(chan struct{})
	go func() {
		RunBridge(ctx, models.Run{RunID: "run_bridge_test"}, eventCh, 10, p)
		close(bridgeDone)
	}()

	const producers = 8
	const perProducer = 25
	var wg sync.WaitGroup
	wg.Add(producers)
	for g := 0; g < producers; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				eventCh <- events.NewEvent("run_bridge_test", events.EventRunStarted, "test", json.RawMessage(`{}`))
			}
		}(g)
	}
	wg.Wait()

	// Give the ticker at least a couple of windows to flush snapshots.
	time.Sleep(50 * time.Millisecond)

	close(eventCh)
	select {
	case <-bridgeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("RunBridge did not return after eventCh close")
	}

	if got := stateCount.Load(); got == 0 {
		t.Error("expected at least one StateMsg, got 0")
	}
}

// TESTREC-F0FE62F2: race-safety test for RunReplayBridge. Drives the replay
// Controller with StepForward calls from a goroutine while the bridge forwards
// state to a live tea.Program. Run with `go test -race`.
func TestRunReplayBridge(t *testing.T) {
	t.Parallel()

	run := models.Run{RunID: "run_replay_test", Status: models.RunStatusPending}
	evts := []events.Event{
		{EventID: "e0", RunID: run.RunID, Type: events.EventRunCreated, Timestamp: time.Now(), Source: "test", Payload: json.RawMessage(`{}`)},
		{EventID: "e1", RunID: run.RunID, Type: events.EventRunStarted, Timestamp: time.Now(), Source: "test", Payload: json.RawMessage(`{}`)},
		{EventID: "e2", RunID: run.RunID, Type: events.EventRunCompleted, Timestamp: time.Now(), Source: "test", Payload: json.RawMessage(`{}`)},
	}
	ctrl := replay.NewController(run, evts)

	var stateCount, statusCount atomic.Int64
	model := stateCounter{state: &stateCount, status: &statusCount}

	p := quietProgram(model)
	progDone := make(chan struct{})
	go func() {
		_, _ = p.Run()
		close(progDone)
	}()
	t.Cleanup(func() {
		p.Quit()
		<-progDone
	})

	ctx, cancel := context.WithCancel(context.Background())
	bridgeDone := make(chan struct{})
	go func() {
		RunReplayBridge(ctx, ctrl, 10, p)
		close(bridgeDone)
	}()

	// Drive updates concurrently: StepForward on the controller pushes into
	// the Updates channel, which the bridge forwards to the program.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < len(evts)-1; i++ {
			_, _ = ctrl.StepForward()
		}
	}()
	wg.Wait()

	// Let the ticker fire at least once.
	time.Sleep(30 * time.Millisecond)

	cancel()
	select {
	case <-bridgeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("RunReplayBridge did not return after ctx cancel")
	}

	if got := statusCount.Load(); got == 0 {
		t.Error("expected at least one ReplayStatusMsg, got 0")
	}
}
