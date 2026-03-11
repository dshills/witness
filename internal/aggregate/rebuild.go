package aggregate

import (
	"fmt"

	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// Rebuild creates a RunState by replaying all events in order against a fresh aggregator.
func Rebuild(run models.Run, evts []events.Event) (*RunState, error) {
	a := NewAggregator(run)
	for i, evt := range evts {
		if err := a.Apply(evt); err != nil {
			return nil, fmt.Errorf("applying event %d (%s): %w", i, evt.Type, err)
		}
	}
	snap := a.Snapshot()
	return &snap, nil
}
