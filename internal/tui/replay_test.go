package tui

import (
	"context"
	"testing"

	"github.com/dshills/witness/internal/events"

	tea "github.com/charmbracelet/bubbletea"
)

// mockReplayControl is a test double for ReplayControl.
type mockReplayControl struct {
	playing        bool
	speed          float64
	current        int
	total          int
	playCount      int
	pauseCount     int
	stepFwdCount   int
	stepBwdCount   int
	setSpeedCalls  []float64
	jumpNextStage  int
	jumpPrevStage  int
	jumpNextCommit int
	jumpPrevCommit int
	jumpNextAlert  int
	jumpPrevAlert  int
}

func newMockCtrl() *mockReplayControl {
	return &mockReplayControl{speed: 1, total: 100}
}

func (m *mockReplayControl) Play(_ context.Context) {
	m.playCount++
	m.playing = true
}

func (m *mockReplayControl) Pause() {
	m.pauseCount++
	m.playing = false
}

func (m *mockReplayControl) StepForward() (*events.Event, error) {
	m.stepFwdCount++
	if m.current < m.total-1 {
		m.current++
	}
	return &events.Event{Type: events.EventRunStarted}, nil
}

func (m *mockReplayControl) StepBackward() error {
	m.stepBwdCount++
	if m.current > 0 {
		m.current--
	}
	return nil
}

func (m *mockReplayControl) SetSpeed(s float64) {
	m.setSpeedCalls = append(m.setSpeedCalls, s)
	m.speed = s
}

func (m *mockReplayControl) Speed() float64  { return m.speed }
func (m *mockReplayControl) IsPlaying() bool { return m.playing }

func (m *mockReplayControl) Progress() (int, int) { return m.current, m.total }

func (m *mockReplayControl) JumpToNextStageTransition() error {
	m.jumpNextStage++
	return nil
}

func (m *mockReplayControl) JumpToPrevStageTransition() error {
	m.jumpPrevStage++
	return nil
}

func (m *mockReplayControl) JumpToNextCommit() error {
	m.jumpNextCommit++
	return nil
}

func (m *mockReplayControl) JumpToPrevCommit() error {
	m.jumpPrevCommit++
	return nil
}

func (m *mockReplayControl) JumpToNextAlert() error {
	m.jumpNextAlert++
	return nil
}

func (m *mockReplayControl) JumpToPrevAlert() error {
	m.jumpPrevAlert++
	return nil
}

func makeReplayApp(ctrl *mockReplayControl) App {
	return NewReplayApp(testPanels(), ctrl, context.Background())
}

func TestNewReplayApp_IsReplayMode(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	if !app.IsReplayMode() {
		t.Error("expected IsReplayMode() to be true")
	}
}

func TestNewApp_IsNotReplayMode(t *testing.T) {
	app := NewApp(testPanels())
	if app.IsReplayMode() {
		t.Error("expected IsReplayMode() to be false for normal app")
	}
}

func TestReplayKey_Space_PlayPause(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	// Set window size so it's ready.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	// Press space to play.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	app = model.(App)
	if ctrl.playCount != 1 {
		t.Errorf("expected 1 play call, got %d", ctrl.playCount)
	}

	// Press space again to pause.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	_ = model.(App)
	if ctrl.pauseCount != 1 {
		t.Errorf("expected 1 pause call, got %d", ctrl.pauseCount)
	}
}

func TestReplayKey_RightArrow_StepForward(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRight})
	_ = model.(App)
	if ctrl.stepFwdCount != 1 {
		t.Errorf("expected 1 step forward, got %d", ctrl.stepFwdCount)
	}
	if ctrl.pauseCount != 1 {
		t.Errorf("expected pause on step, got %d pause calls", ctrl.pauseCount)
	}
}

func TestReplayKey_L_StepForward(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	_ = model.(App)
	if ctrl.stepFwdCount != 1 {
		t.Errorf("expected 1 step forward, got %d", ctrl.stepFwdCount)
	}
}

func TestReplayKey_LeftArrow_StepBackward(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyLeft})
	_ = model.(App)
	if ctrl.stepBwdCount != 1 {
		t.Errorf("expected 1 step backward, got %d", ctrl.stepBwdCount)
	}
}

func TestReplayKey_H_StepBackward(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	_ = model.(App)
	if ctrl.stepBwdCount != 1 {
		t.Errorf("expected 1 step backward, got %d", ctrl.stepBwdCount)
	}
}

func TestReplayKey_IncreaseSpeed(t *testing.T) {
	ctrl := newMockCtrl()
	ctrl.speed = 1
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	// Press '>' to increase speed.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	_ = model.(App)
	if len(ctrl.setSpeedCalls) != 1 || ctrl.setSpeedCalls[0] != 2 {
		t.Errorf("expected speed set to 2, got calls: %v", ctrl.setSpeedCalls)
	}
}

func TestReplayKey_DecreaseSpeed(t *testing.T) {
	ctrl := newMockCtrl()
	ctrl.speed = 4
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	// Press '<' to decrease speed.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	_ = model.(App)
	if len(ctrl.setSpeedCalls) != 1 || ctrl.setSpeedCalls[0] != 2 {
		t.Errorf("expected speed set to 2, got calls: %v", ctrl.setSpeedCalls)
	}
}

func TestReplayKey_DotComma_Speed(t *testing.T) {
	ctrl := newMockCtrl()
	ctrl.speed = 2
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	// Press '.' to increase speed.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(".")})
	app = model.(App)
	if ctrl.speed != 4 {
		t.Errorf("expected speed 4 after '.', got %v", ctrl.speed)
	}

	// Press ',' to decrease speed.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(",")})
	_ = model.(App)
	if ctrl.speed != 2 {
		t.Errorf("expected speed 2 after ',', got %v", ctrl.speed)
	}
}

func TestReplayKey_N_JumpNextStage(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	_ = model.(App)
	if ctrl.jumpNextStage != 1 {
		t.Errorf("expected 1 jump next stage, got %d", ctrl.jumpNextStage)
	}
}

func TestReplayKey_ShiftN_JumpPrevStage(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	_ = model.(App)
	if ctrl.jumpPrevStage != 1 {
		t.Errorf("expected 1 jump prev stage, got %d", ctrl.jumpPrevStage)
	}
}

func TestReplayKey_C_JumpNextCommit(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	_ = model.(App)
	if ctrl.jumpNextCommit != 1 {
		t.Errorf("expected 1 jump next commit, got %d", ctrl.jumpNextCommit)
	}
}

func TestReplayKey_ShiftC_JumpPrevCommit(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	_ = model.(App)
	if ctrl.jumpPrevCommit != 1 {
		t.Errorf("expected 1 jump prev commit, got %d", ctrl.jumpPrevCommit)
	}
}

func TestReplayKey_ShiftA_JumpNextAlert(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	_ = model.(App)
	if ctrl.jumpNextAlert != 1 {
		t.Errorf("expected 1 jump next alert, got %d", ctrl.jumpNextAlert)
	}
}

func TestReplayKey_Tab_StillWorks(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	initialFocus := app.FocusIndex()
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = model.(App)
	if app.FocusIndex() == initialFocus {
		t.Error("expected focus to change on tab in replay mode")
	}
}

func TestReplayStatusMsg_Updates(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	status := ReplayStatusMsg{
		Playing: true,
		Speed:   4,
		Current: 50,
		Total:   200,
	}
	model, _ = app.Update(status)
	app = model.(App)

	if app.ReplayStatus().Speed != 4 {
		t.Errorf("expected replay status speed=4, got %v", app.ReplayStatus().Speed)
	}
	if app.ReplayStatus().Current != 50 {
		t.Errorf("expected replay status current=50, got %v", app.ReplayStatus().Current)
	}
}

func TestReplayApp_ViewIncludesReplayBar(t *testing.T) {
	ctrl := newMockCtrl()
	app := makeReplayApp(ctrl)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	// Set replay status so the bar has content.
	status := ReplayStatusMsg{
		Playing: false,
		Speed:   1,
		Current: 10,
		Total:   100,
	}
	model, _ = app.Update(status)
	app = model.(App)

	view := app.View()
	if !containsAny(view, "Paused", "Playing") {
		t.Error("expected replay bar in view output")
	}
}

func TestSpeedSteps(t *testing.T) {
	tests := []struct {
		current  float64
		nextWant float64
		prevWant float64
	}{
		{1, 2, 1},
		{2, 4, 1},
		{4, 8, 2},
		{8, 16, 4},
		{16, 16, 8},
		{0.5, 1, 1}, // below minimum snaps to 1
	}
	for _, tt := range tests {
		if got := nextSpeed(tt.current); got != tt.nextWant {
			t.Errorf("nextSpeed(%v) = %v, want %v", tt.current, got, tt.nextWant)
		}
		if got := prevSpeed(tt.current); got != tt.prevWant {
			t.Errorf("prevSpeed(%v) = %v, want %v", tt.current, got, tt.prevWant)
		}
	}
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
