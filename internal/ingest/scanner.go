package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"

	"github.com/dshills/witness/internal/events"
)

// Scanner reads lines from a reader (typically subprocess stdout) and
// ingests any lines that are valid Witness JSON events. Non-event lines
// are silently ignored (they have already been relayed to the terminal).
type Scanner struct {
	reader io.Reader
	sink   events.EventSink
	runID  string
}

// NewScanner creates a new Scanner.
func NewScanner(reader io.Reader, sink events.EventSink, runID string) *Scanner {
	return &Scanner{
		reader: reader,
		sink:   sink,
		runID:  runID,
	}
}

// Scan reads lines until EOF. The reader must be drained to EOF to avoid
// deadlocking io.Pipe writers. After context cancellation, lines are still
// consumed but events are no longer ingested.
func (s *Scanner) Scan(ctx context.Context) error {
	sc := bufio.NewScanner(s.reader)
	for sc.Scan() {
		// After context cancellation, keep draining but don't ingest.
		if ctx.Err() != nil {
			continue
		}

		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		var evt events.Event
		if err := json.Unmarshal(line, &evt); err != nil {
			continue // Not JSON, pass through.
		}

		// Must have event_id and type to be treated as a Witness event.
		if evt.EventID == "" || evt.Type == "" {
			continue
		}

		// Override run_id to current run.
		evt.RunID = s.runID

		if err := s.sink.Append(ctx, evt); err != nil {
			log.Printf("ingest: append event: %v", err)
		}
	}
	return sc.Err()
}
