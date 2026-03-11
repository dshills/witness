package tui

import (
	"time"

	"github.com/dshills/witness/internal/aggregate"
)

// StateMsg delivers an updated RunState snapshot to the TUI.
type StateMsg struct {
	State aggregate.RunState
}

// ReplayStatusMsg delivers replay controller status to the TUI control bar.
type ReplayStatusMsg struct {
	Playing   bool
	Speed     float64
	Current   int       // current event index (0-based)
	Total     int       // total event count
	EventTime time.Time // timestamp of current event
}
