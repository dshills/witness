package export

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
)

// NDJSONExporter writes one JSON line per event.
type NDJSONExporter struct{}

// Export writes each event as a single JSON line.
func (e *NDJSONExporter) Export(_ context.Context, _ aggregate.RunState, evts []events.Event, w io.Writer) error {
	enc := json.NewEncoder(w)
	for i := range evts {
		if err := enc.Encode(evts[i]); err != nil {
			return err
		}
	}
	return nil
}
