package events

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// SchemaVersion is the current event schema version.
const SchemaVersion = "1.0"

// Event is the base event envelope for all Witness events.
type Event struct {
	EventID       string          `json:"event_id"`
	SchemaVersion string          `json:"schema_version"`
	Timestamp     time.Time       `json:"timestamp"`
	RunID         string          `json:"run_id"`
	Type          EventType       `json:"type"`
	Source        string          `json:"source"`
	Payload       json.RawMessage `json:"payload"`

	// Optional fields
	StageID       string            `json:"stage_id,omitempty"`
	Status        string            `json:"status,omitempty"`
	Summary       string            `json:"summary,omitempty"`
	TraceID       string            `json:"trace_id,omitempty"`
	SpanID        string            `json:"span_id,omitempty"`
	ParentEventID string            `json:"parent_event_id,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
}

var (
	idMu      sync.Mutex
	idEntropy = ulid.Monotonic(rand.Reader, 0)
)

// NewID generates a new ULID-based ID with the given prefix.
// Example: NewID("run") → "run_01JXXXXXX..."
func NewID(prefix string) string {
	idMu.Lock()
	id := ulid.MustNew(ulid.Timestamp(time.Now()), idEntropy)
	idMu.Unlock()
	return fmt.Sprintf("%s_%s", prefix, id.String())
}

// NewEvent creates a new event with generated ID and current timestamp.
func NewEvent(runID string, eventType EventType, source string, payload json.RawMessage) Event {
	return Event{
		EventID:       NewID("evt"),
		SchemaVersion: SchemaVersion,
		Timestamp:     time.Now().UTC(),
		RunID:         runID,
		Type:          eventType,
		Source:        source,
		Payload:       payload,
	}
}
