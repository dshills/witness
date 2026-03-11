package events

import "context"

// EventSink is the interface for appending events to a store.
// Consumed by internal/git, internal/files, and internal/ingest.
type EventSink interface {
	Append(ctx context.Context, evt Event) error
}
