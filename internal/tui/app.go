package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

// layout thresholds.
const (
	fullLayoutMinCols = 100
	fullLayoutMinRows = 28

	// replayBarHeight is the number of lines the replay control bar uses.
	replayBarHeight = 2
)

// App is the root Bubble Tea model that owns and arranges all panels.
type App struct {
	panels    []Panel
	focusIdx  int
	width     int
	height    int
	drillDown int // -1 = dashboard, >= 0 = full-screen panel index
	showHelp  bool
	ready     bool

	// Replay mode fields (nil/zero when in live mode).
	replayCtrl   ReplayControl
	replayCtx    context.Context
	replayStatus ReplayStatusMsg
}

// NewApp creates a new App model with the given panels.
// Panels must be provided in order: Header, Stages, ActiveWork, TokenCost, GitFile, Alerts, EventStream.
func NewApp(panels []Panel) App {
	// Find first focusable panel for initial focus.
	focus := -1
	for i, p := range panels {
		if p.Focusable() {
			focus = i
			break
		}
	}
	return App{
		panels:    panels,
		focusIdx:  focus,
		drillDown: -1,
	}
}

// NewReplayApp creates a new App model configured for replay mode.
// The replay controller is used to handle replay-specific key bindings.
func NewReplayApp(panels []Panel, ctrl ReplayControl, ctx context.Context) App {
	app := NewApp(panels)
	app.replayCtrl = ctrl
	app.replayCtx = ctx
	return app
}

// IsReplayMode returns true if the app is in replay mode.
func (a App) IsReplayMode() bool {
	return a.replayCtrl != nil
}

// ReplayStatus returns the current replay status for testing.
func (a App) ReplayStatus() ReplayStatusMsg {
	return a.replayStatus
}

func (a App) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, p := range a.panels {
		if cmd := p.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.ready = true
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(m)

	case StateMsg:
		// Broadcast state to all panels.
		var cmds []tea.Cmd
		for i, p := range a.panels {
			updated, cmd := p.Update(msg)
			a.panels[i] = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return a, tea.Batch(cmds...)

	case ReplayStatusMsg:
		a.replayStatus = m
		return a, nil
	}

	// Forward other messages to focused panel.
	if a.focusIdx >= 0 && a.focusIdx < len(a.panels) {
		updated, cmd := a.panels[a.focusIdx].Update(msg)
		a.panels[a.focusIdx] = updated
		return a, cmd
	}

	return a, nil
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay intercepts all keys.
	if a.showHelp {
		a.showHelp = false
		return a, nil
	}

	// Escape exits drill-down.
	if msg.String() == keyEsc {
		if a.drillDown >= 0 {
			a.drillDown = -1
			return a, nil
		}
		return a, nil
	}

	// Quit.
	if isQuit(msg) {
		return a, tea.Quit
	}

	// Help toggle.
	if msg.String() == keyQuestion {
		a.showHelp = !a.showHelp
		return a, nil
	}

	// Replay-mode key bindings (checked before drill-down shortcuts to avoid conflicts).
	if a.replayCtrl != nil {
		if handled, model, cmd := a.handleReplayKey(msg); handled {
			return model, cmd
		}
	}

	// Tab / shift-tab for focus cycling.
	if msg.String() == keyTab {
		a.advanceFocus(1)
		return a, nil
	}
	if msg.String() == keyShiftTab {
		a.advanceFocus(-1)
		return a, nil
	}

	// Drill-down shortcuts.
	if idx := isDrillDown(msg); idx >= 0 && idx < len(a.panels) {
		a.drillDown = idx
		a.focusIdx = idx
		return a, nil
	}

	// Forward to focused panel.
	if a.focusIdx >= 0 && a.focusIdx < len(a.panels) {
		updated, cmd := a.panels[a.focusIdx].Update(msg)
		a.panels[a.focusIdx] = updated
		return a, cmd
	}

	return a, nil
}

// handleReplayKey processes replay-specific key bindings.
// Returns (handled, model, cmd). If handled is false, the key should be
// processed by normal key handling.
func (a App) handleReplayKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	ctrl := a.replayCtrl
	k := msg.String()

	switch k {
	case keySpace:
		if ctrl.IsPlaying() {
			ctrl.Pause()
		} else {
			ctrl.Play(a.replayCtx)
		}
		return true, a, nil

	case keyRight, keyL:
		ctrl.Pause()
		_, _ = ctrl.StepForward()
		return true, a, nil

	case keyLeft, keyH:
		ctrl.Pause()
		_ = ctrl.StepBackward()
		return true, a, nil

	case keyGt, keyDot:
		newSpeed := nextSpeed(ctrl.Speed())
		ctrl.SetSpeed(newSpeed)
		return true, a, nil

	case keyLt, keyComma:
		newSpeed := prevSpeed(ctrl.Speed())
		ctrl.SetSpeed(newSpeed)
		return true, a, nil

	case keyN:
		_ = ctrl.JumpToNextStageTransition()
		return true, a, nil

	case keyShiftN:
		_ = ctrl.JumpToPrevStageTransition()
		return true, a, nil

	case keyC:
		_ = ctrl.JumpToNextCommit()
		return true, a, nil

	case keyShiftC:
		_ = ctrl.JumpToPrevCommit()
		return true, a, nil

	case keyShiftA:
		_ = ctrl.JumpToNextAlert()
		return true, a, nil
	}

	return false, a, nil
}

func (a *App) advanceFocus(dir int) {
	if len(a.panels) == 0 {
		return
	}
	start := a.focusIdx
	if start < 0 {
		start = 0
	}
	idx := start
	for range len(a.panels) {
		idx = (idx + dir + len(a.panels)) % len(a.panels)
		if a.panels[idx].Focusable() {
			a.focusIdx = idx
			return
		}
	}
}

func (a App) View() string {
	if !a.ready {
		return "Initializing..."
	}

	var view string
	if a.drillDown >= 0 && a.drillDown < len(a.panels) {
		view = a.drillDownView()
	} else if a.width >= fullLayoutMinCols && a.height >= fullLayoutMinRows {
		view = a.fullLayout()
	} else {
		view = a.compactLayout()
	}

	if a.showHelp {
		view = a.overlayHelp(view)
	}

	return view
}

// replayBarLines returns the number of lines reserved for the replay bar.
func (a App) replayBarLines() int {
	if a.replayCtrl != nil {
		return replayBarHeight
	}
	return 0
}

func (a App) drillDownView() string {
	p := a.panels[a.drillDown]
	title := panelTitleBar(p.Title(), a.width, a.drillDown == a.focusIdx)
	contentHeight := a.height - 2 - a.replayBarLines()
	content := p.View(a.width, contentHeight)
	footer := padToWidth(" ESC to return", a.width)

	if a.replayCtrl != nil {
		return title + "\n" + content + "\n" + RenderReplayBar(a.replayStatus, a.width)
	}
	return title + "\n" + content + "\n" + footer
}

func (a App) fullLayout() string {
	var b strings.Builder

	// Header (full width, 1 line).
	header := a.panels[panelHeader].View(a.width, 1)
	b.WriteString(header)
	b.WriteByte('\n')

	// Calculate available height after header and replay bar.
	bodyHeight := a.height - 2 - a.replayBarLines() // header + bottom status/bar

	// Split columns.
	leftWidth := a.width * 40 / 100
	rightWidth := a.width - leftWidth - 1 // 1 for separator

	// Top row: Stages (left) | ActiveWork (right)
	topHeight := bodyHeight * 35 / 100
	if topHeight < 4 {
		topHeight = 4
	}

	// Middle row: TokenCost (left) | GitFile (right)
	midHeight := bodyHeight * 25 / 100
	if midHeight < 3 {
		midHeight = 3
	}

	// Alerts row.
	alertHeight := bodyHeight * 15 / 100
	if alertHeight < 2 {
		alertHeight = 2
	}

	// Event stream gets the rest.
	streamHeight := bodyHeight - topHeight - midHeight - alertHeight - 4 // 4 for title bars
	if streamHeight < 3 {
		streamHeight = 3
	}

	// Top row.
	b.WriteString(a.twoColumn(
		a.panels[panelStages], topHeight,
		a.panels[panelActiveWork], topHeight,
		leftWidth, rightWidth))
	b.WriteByte('\n')

	// Middle row.
	b.WriteString(a.twoColumn(
		a.panels[panelTokenCost], midHeight,
		a.panels[panelGitFile], midHeight,
		leftWidth, rightWidth))
	b.WriteByte('\n')

	// Alerts (full width).
	b.WriteString(panelTitleBar(a.panels[panelAlerts].Title(), a.width, a.focusIdx == panelAlerts))
	b.WriteByte('\n')
	b.WriteString(a.panels[panelAlerts].View(a.width, alertHeight))
	b.WriteByte('\n')

	// Event stream (full width).
	b.WriteString(panelTitleBar(a.panels[panelEventStream].Title(), a.width, a.focusIdx == panelEventStream))
	b.WriteByte('\n')
	b.WriteString(a.panels[panelEventStream].View(a.width, streamHeight))

	// Replay control bar at the bottom.
	if a.replayCtrl != nil {
		b.WriteByte('\n')
		b.WriteString(RenderReplayBar(a.replayStatus, a.width))
	}

	return b.String()
}

func (a App) twoColumn(left Panel, leftHeight int, right Panel, rightHeight int, leftWidth, rightWidth int) string {
	leftTitle := panelTitleBar(left.Title(), leftWidth, a.panelIndex(left) == a.focusIdx)
	rightTitle := panelTitleBar(right.Title(), rightWidth, a.panelIndex(right) == a.focusIdx)

	leftContent := left.View(leftWidth, leftHeight)
	rightContent := right.View(rightWidth, rightHeight)

	// Join title bars side by side.
	titleRow := leftTitle + " " + rightTitle

	// Join content lines side by side.
	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(rightContent, "\n")

	maxLines := leftHeight
	if rightHeight > maxLines {
		maxLines = rightHeight
	}

	var rows []string
	rows = append(rows, titleRow)
	for i := range maxLines {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		l = padToWidth(l, leftWidth)
		r = padToWidth(r, rightWidth)
		rows = append(rows, l+"|"+r)
	}

	return strings.Join(rows, "\n")
}

func (a App) panelIndex(p Panel) int {
	for i, panel := range a.panels {
		if panel == p {
			return i
		}
	}
	return -1
}

func (a App) compactLayout() string {
	var b strings.Builder

	// Header.
	b.WriteString(a.panels[panelHeader].View(a.width, 1))
	b.WriteByte('\n')

	available := a.height - 2 - a.replayBarLines() // header + bottom

	// ActiveWork (compact).
	awHeight := 4
	if awHeight > available {
		awHeight = available
	}
	b.WriteString(panelTitleBar(a.panels[panelActiveWork].Title(), a.width, a.focusIdx == panelActiveWork))
	b.WriteByte('\n')
	b.WriteString(a.panels[panelActiveWork].View(a.width, awHeight))
	b.WriteByte('\n')
	available -= awHeight + 1

	// Alerts.
	alertHeight := 3
	if alertHeight > available {
		alertHeight = available
	}
	if alertHeight > 0 {
		b.WriteString(panelTitleBar(a.panels[panelAlerts].Title(), a.width, a.focusIdx == panelAlerts))
		b.WriteByte('\n')
		b.WriteString(a.panels[panelAlerts].View(a.width, alertHeight))
		b.WriteByte('\n')
		available -= alertHeight + 1
	}

	// Event stream gets the rest.
	if available > 1 {
		b.WriteString(panelTitleBar(a.panels[panelEventStream].Title(), a.width, a.focusIdx == panelEventStream))
		b.WriteByte('\n')
		b.WriteString(a.panels[panelEventStream].View(a.width, available-1))
	}

	// Replay control bar at the bottom.
	if a.replayCtrl != nil {
		b.WriteByte('\n')
		b.WriteString(RenderReplayBar(a.replayStatus, a.width))
	}

	return b.String()
}

func (a App) overlayHelp(base string) string {
	help := a.helpTextForMode()
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(60)

	rendered := helpStyle.Render(help)

	// Center the overlay.
	x := (a.width - lipgloss.Width(rendered)) / 2
	y := (a.height - lipgloss.Height(rendered)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return placeOverlay(base, rendered, x, y, a.width, a.height)
}

// helpTextForMode returns help text appropriate for the current mode.
func (a App) helpTextForMode() string {
	if a.replayCtrl != nil {
		return replayHelpText()
	}
	return helpText()
}

// Width returns the current terminal width.
func (a App) Width() int { return a.width }

// Height returns the current terminal height.
func (a App) Height() int { return a.height }

// FocusIndex returns the currently focused panel index.
func (a App) FocusIndex() int { return a.focusIdx }

// DrillDown returns the current drill-down panel index, or -1.
func (a App) DrillDown() int { return a.drillDown }

// ShowHelp returns whether the help overlay is visible.
func (a App) ShowHelp() bool { return a.showHelp }

func panelTitleBar(title string, width int, focused bool) string {
	style := lipgloss.NewStyle().Bold(true)
	if focused {
		style = style.Foreground(lipgloss.Color("14")) // cyan
	}
	t := style.Render(" " + title + " ")
	remaining := width - lipgloss.Width(t)
	if remaining > 0 {
		t += strings.Repeat("\u2500", remaining)
	}
	return t
}

func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		// Truncate to width by taking runes.
		runes := []rune(s)
		if len(runes) > width {
			return string(runes[:width])
		}
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func helpText() string {
	return `Keyboard Shortcuts
-------------------
q / ctrl+c   Quit
tab          Focus next panel
shift+tab    Focus previous panel
j / down     Scroll down
k / up       Scroll up
p            Pause event stream
r            Resume live tail
/            Filter events
esc          Close filter / exit drill-down
?            Toggle this help

Drill-down (full-screen):
s            Stages
t            Tokens / Cost
g            Git / Files
a            Alerts
e            Event stream
m            Model / Active work`
}

func replayHelpText() string {
	return `Replay Keyboard Shortcuts
-------------------------
q / ctrl+c   Quit
space        Play / pause
right / l    Step forward
left / h     Step backward
> / .        Increase speed
< / ,        Decrease speed
n            Next stage transition
N            Previous stage transition
c            Next commit
C            Previous commit
A            Next alert
tab          Focus next panel
shift+tab    Focus previous panel
j / down     Scroll down
k / up       Scroll up
esc          Exit drill-down
?            Toggle this help

Drill-down (full-screen):
s            Stages
t            Tokens / Cost
g            Git / Files
a            Alerts
e            Event stream
m            Model / Active work`
}

// placeOverlay places an overlay string on top of a base string.
func placeOverlay(base, overlay string, x, y, totalWidth, totalHeight int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure we have enough base lines.
	for len(baseLines) < totalHeight {
		baseLines = append(baseLines, strings.Repeat(" ", totalWidth))
	}

	for i, ol := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		bl := baseLines[row]
		runes := []rune(bl)

		// Ensure the base line is wide enough.
		for len(runes) < x+len([]rune(ol)) {
			runes = append(runes, ' ')
		}

		olRunes := []rune(ol)
		for j, r := range olRunes {
			col := x + j
			if col < len(runes) {
				runes[col] = r
			}
		}
		baseLines[row] = string(runes)
	}

	return strings.Join(baseLines, "\n")
}
