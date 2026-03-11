package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/privacy"
	"github.com/dshills/witness/internal/store"
)

// snapshotInterval is the number of events between automatic snapshots.
const snapshotInterval = 500

// StoreSink bridges observers and the store. It validates, redacts, persists,
// and aggregates each event. Every 500 events it saves a snapshot.
type StoreSink struct {
	store      store.Store
	runID      string
	aggregator *aggregate.Aggregator
	redactor   *privacy.Redactor
	mu         sync.Mutex
	count      int64
}

// Compile-time interface check.
var _ events.EventSink = (*StoreSink)(nil)

// NewStoreSink creates a StoreSink for the given run.
func NewStoreSink(s store.Store, runID string, agg *aggregate.Aggregator, redactor *privacy.Redactor) *StoreSink {
	return &StoreSink{
		store:      s,
		runID:      runID,
		aggregator: agg,
		redactor:   redactor,
	}
}

// Append validates, redacts, persists, and aggregates a single event.
func (s *StoreSink) Append(ctx context.Context, evt events.Event) error {
	// 1. Validate
	if err := events.Validate(evt); err != nil {
		return fmt.Errorf("storesink: invalid event: %w", err)
	}

	// 2. Redact sensitive fields
	if s.redactor != nil {
		evt.Summary = s.redactor.Redact(evt.Summary)
		evt.Payload = redactPayloadStrings(s.redactor, evt.Payload)
	}

	// 3. Persist to store
	if err := s.store.AppendEvent(ctx, s.runID, evt); err != nil {
		return fmt.Errorf("storesink: persisting event: %w", err)
	}

	// 4. Apply to aggregator
	if err := s.aggregator.Apply(evt); err != nil {
		log.Printf("storesink: aggregator error: %v", err)
	}

	// 5. Periodic snapshot
	s.mu.Lock()
	s.count++
	count := s.count
	s.mu.Unlock()

	if count%snapshotInterval == 0 {
		s.saveSnapshot(ctx)
	}

	return nil
}

// Count returns the total number of events appended.
func (s *StoreSink) Count() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// saveSnapshot marshals the current aggregator state and persists it.
func (s *StoreSink) saveSnapshot(ctx context.Context) {
	snap := s.aggregator.Snapshot()
	data, err := json.Marshal(snap)
	if err != nil {
		log.Printf("storesink: snapshot marshal error: %v", err)
		return
	}
	if err := s.store.SaveSnapshot(ctx, s.runID, data); err != nil {
		log.Printf("storesink: snapshot save error: %v", err)
	}
}

// SaveFinalSnapshot saves the final snapshot. Called at run end.
func (s *StoreSink) SaveFinalSnapshot(ctx context.Context) {
	s.saveSnapshot(ctx)
}

// redactPayloadStrings redacts string values in a JSON payload.
func redactPayloadStrings(r *privacy.Redactor, payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return payload
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		// Not an object; return as-is.
		return payload
	}
	changed := false
	for k, v := range m {
		if s, ok := v.(string); ok {
			redacted := r.Redact(s)
			if redacted != s {
				m[k] = redacted
				changed = true
			}
		}
	}
	if !changed {
		return payload
	}
	data, err := json.Marshal(m)
	if err != nil {
		return payload
	}
	return data
}
