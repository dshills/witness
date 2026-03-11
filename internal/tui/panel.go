// Package tui implements the Bubble Tea terminal UI for Witness.
package tui

import tea "github.com/charmbracelet/bubbletea"

// Panel is the interface that all TUI panels implement.
// Each panel renders a self-contained section of the dashboard.
type Panel interface {
	// Init returns an initial command for the panel.
	Init() tea.Cmd

	// Update processes a message and returns the updated panel and any command.
	Update(msg tea.Msg) (Panel, tea.Cmd)

	// View renders the panel content within the given dimensions.
	View(width, height int) string

	// Title returns the panel's display title.
	Title() string

	// Focusable reports whether the panel can receive keyboard focus.
	Focusable() bool
}
