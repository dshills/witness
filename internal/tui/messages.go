package tui

import "github.com/dshills/witness/internal/aggregate"

// StateMsg delivers an updated RunState snapshot to the TUI.
type StateMsg struct {
	State aggregate.RunState
}
