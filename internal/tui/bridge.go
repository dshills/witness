package tui

import (
	"context"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

// Bridge connects an event stream from the store to the Bubble Tea program.
// It runs a background goroutine that reads events, applies them to an
// aggregator, and sends StateMsg snapshots at the configured refresh interval.
type Bridge struct {
	aggregator *aggregate.Aggregator
	refreshMS  int
}

// NewBridge creates a new Bridge with the given refresh interval in milliseconds.
func NewBridge(run models.Run, refreshMS int) *Bridge {
	if refreshMS <= 0 {
		refreshMS = 500
	}
	return &Bridge{
		aggregator: aggregate.NewAggregator(run),
		refreshMS:  refreshMS,
	}
}

// RunBridge starts the bridge as a long-running goroutine that sends
// repeated StateMsg messages to the Bubble Tea program.
// It consumes events from eventCh, applies them to an aggregator,
// and sends snapshots at the configured refresh interval.
func RunBridge(ctx context.Context, run models.Run, eventCh <-chan events.Event, refreshMS int, p *tea.Program) {
	b := NewBridge(run, refreshMS)

	ticker := time.NewTicker(time.Duration(b.refreshMS) * time.Millisecond)
	defer ticker.Stop()

	dirty := true

	for {
		select {
		case <-ctx.Done():
			p.Send(StateMsg{State: b.aggregator.Snapshot()})
			return

		case evt, ok := <-eventCh:
			if !ok {
				p.Send(StateMsg{State: b.aggregator.Snapshot()})
				return
			}
			_ = b.aggregator.Apply(evt)
			dirty = true

		case <-ticker.C:
			if dirty {
				dirty = false
				p.Send(StateMsg{State: b.aggregator.Snapshot()})
			}
		}
	}
}
