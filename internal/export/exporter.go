package export

import (
	"context"
	"io"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
)

// Exporter writes a run's state and events to a writer in a specific format.
type Exporter interface {
	Export(ctx context.Context, state aggregate.RunState, evts []events.Event, w io.Writer) error
}
