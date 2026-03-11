package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestIsQuit(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
		want bool
	}{
		{"q quits", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}, true},
		{"ctrl+c quits", tea.KeyMsg{Type: tea.KeyCtrlC}, true},
		{"j does not quit", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, false},
		{"tab does not quit", tea.KeyMsg{Type: tea.KeyTab}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isQuit(tt.msg); got != tt.want {
				t.Errorf("isQuit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDrillDown(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want int
	}{
		{"s -> stages", "s", panelStages},
		{"t -> token cost", "t", panelTokenCost},
		{"g -> git file", "g", panelGitFile},
		{"a -> alerts", "a", panelAlerts},
		{"e -> event stream", "e", panelEventStream},
		{"m -> active work", "m", panelActiveWork},
		{"q -> not drill-down", "q", -1},
		{"x -> not drill-down", "x", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			if got := isDrillDown(msg); got != tt.want {
				t.Errorf("isDrillDown(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
