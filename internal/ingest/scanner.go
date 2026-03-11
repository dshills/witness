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
// ingests any lines that are valid Witness JSON events. Lines that are
// not Witness events are checked against the tool adapter registry and
// converted to events if they match. Non-event, non-tool lines are
// silently ignored (they have already been relayed to the terminal).
type Scanner struct {
	reader   io.Reader
	sink     events.EventSink
	runID    string
	stageID  string
	adapters *AdapterRegistry
}

// NewScanner creates a new Scanner.
func NewScanner(reader io.Reader, sink events.EventSink, runID string) *Scanner {
	return &Scanner{
		reader:   reader,
		sink:     sink,
		runID:    runID,
		adapters: NewAdapterRegistry(),
	}
}

// SetStageID sets the stage ID used when converting tool results to events.
func (s *Scanner) SetStageID(stageID string) {
	s.stageID = stageID
}

// SetAdapterRegistry replaces the default adapter registry.
func (s *Scanner) SetAdapterRegistry(r *AdapterRegistry) {
	s.adapters = r
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

		// Try to parse as a Witness event first.
		if s.tryWitnessEvent(ctx, line) {
			continue
		}

		// Try tool adapter parsing for non-Witness JSON.
		s.tryToolResult(ctx, line)
	}
	return sc.Err()
}

// tryWitnessEvent attempts to parse and ingest a line as a native Witness event.
// Returns true if the line was a valid Witness event (regardless of ingest success).
func (s *Scanner) tryWitnessEvent(ctx context.Context, line []byte) bool {
	var evt events.Event
	if err := json.Unmarshal(line, &evt); err != nil {
		return false
	}

	// Must have event_id and type to be treated as a Witness event.
	if evt.EventID == "" || evt.Type == "" {
		return false
	}

	// Override run_id to current run.
	evt.RunID = s.runID

	if err := s.sink.Append(ctx, evt); err != nil {
		log.Printf("ingest: append event: %v", err)
	}
	return true
}

// tryToolResult attempts to parse a line using the adapter registry and
// converts any matched tool result into Witness events.
func (s *Scanner) tryToolResult(ctx context.Context, line []byte) {
	result, err := s.adapters.TryParse(line)
	if err != nil {
		log.Printf("ingest: tool adapter parse error: %v", err)
		return
	}
	if result == nil {
		return // No adapter matched.
	}

	evts := ToolResultToEvents(*result, s.runID, s.stageID)
	for _, evt := range evts {
		if err := s.sink.Append(ctx, evt); err != nil {
			log.Printf("ingest: append tool event: %v", err)
		}
	}
}
