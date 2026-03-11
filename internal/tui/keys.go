package tui

import tea "github.com/charmbracelet/bubbletea"

// Key constants for TUI navigation.
const (
	keyQ        = "q"
	keyTab      = "tab"
	keyShiftTab = "shift+tab"
	keyJ        = "j"
	keyK        = "k"
	keyDown     = "down"
	keyUp       = "up"
	keyP        = "p"
	keyR        = "r"
	keySlash    = "/"
	keyEsc      = "esc"
	keyQuestion = "?"
	keyS        = "s"
	keyT        = "t"
	keyG        = "g"
	keyA        = "a"
	keyE        = "e"
	keyM        = "m"
	keyCtrlC    = "ctrl+c"
)

// isQuit returns true if the key message represents a quit action.
func isQuit(msg tea.KeyMsg) bool {
	return msg.String() == keyQ || msg.String() == keyCtrlC
}

// isDrillDown returns the panel index for a drill-down key, or -1 if not a drill-down.
func isDrillDown(msg tea.KeyMsg) int {
	switch msg.String() {
	case keyS:
		return panelStages
	case keyT:
		return panelTokenCost
	case keyG:
		return panelGitFile
	case keyA:
		return panelAlerts
	case keyE:
		return panelEventStream
	case keyM:
		return panelActiveWork
	default:
		return -1
	}
}

// Panel index constants for drill-down targeting.
const (
	panelHeader      = 0
	panelStages      = 1
	panelActiveWork  = 2
	panelTokenCost   = 3
	panelGitFile     = 4
	panelAlerts      = 5
	panelEventStream = 6
)
