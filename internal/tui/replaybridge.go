package tui

import (
	"context"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/replay"

	tea "github.com/charmbracelet/bubbletea"
)

// ReplayBridge connects a replay controller to the Bubble Tea program.
// It consumes state updates from the controller's Updates channel and sends
// StateMsg and ReplayStatusMsg messages to the TUI at the configured refresh interval.
type ReplayBridge struct {
	ctrl      *replay.Controller
	refreshMS int
}

// NewReplayBridge creates a new ReplayBridge.
func NewReplayBridge(ctrl *replay.Controller, refreshMS int) *ReplayBridge {
	if refreshMS <= 0 {
		refreshMS = 100
	}
	return &ReplayBridge{
		ctrl:      ctrl,
		refreshMS: refreshMS,
	}
}

// RunReplayBridge starts the replay bridge goroutine. It reads from the
// controller's Updates channel and sends StateMsg and ReplayStatusMsg
// to the Bubble Tea program. It also periodically sends ReplayStatusMsg
// so the control bar stays current even when paused.
func RunReplayBridge(ctx context.Context, ctrl *replay.Controller, refreshMS int, p *tea.Program) {
	b := NewReplayBridge(ctrl, refreshMS)

	ticker := time.NewTicker(time.Duration(b.refreshMS) * time.Millisecond)
	defer ticker.Stop()

	var lastState *aggregate.RunState

	for {
		select {
		case <-ctx.Done():
			// Send final status.
			p.Send(b.replayStatus())
			return

		case state, ok := <-ctrl.Updates():
			if !ok {
				p.Send(b.replayStatus())
				return
			}
			lastState = &state
			p.Send(StateMsg{State: state})
			p.Send(b.replayStatus())

		case <-ticker.C:
			// Periodically send status update (for paused state, speed changes).
			p.Send(b.replayStatus())
			// If we have a last state and no recent update, resend it.
			if lastState != nil {
				// Only resend state if playing (to keep panels updating).
				if ctrl.IsPlaying() {
					p.Send(StateMsg{State: *lastState})
				}
			}
		}
	}
}

// replayStatus builds a ReplayStatusMsg from the controller's current state.
func (b *ReplayBridge) replayStatus() ReplayStatusMsg {
	current, total := b.ctrl.Progress()
	evt := b.ctrl.CurrentEvent()

	var eventTime time.Time
	if evt != nil {
		eventTime = evt.Timestamp
	}

	return ReplayStatusMsg{
		Playing:   b.ctrl.IsPlaying(),
		Speed:     b.ctrl.Speed(),
		Current:   current,
		Total:     total,
		EventTime: eventTime,
	}
}
