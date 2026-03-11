package store

import (
	"context"
	"os"
	"path/filepath"

	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// Store is the persistence interface for runs and events.
type Store interface {
	// Run management
	CreateRun(ctx context.Context, run models.Run) error
	GetRun(ctx context.Context, runID string) (models.Run, error)
	UpdateRun(ctx context.Context, run models.Run) error
	ListRuns(ctx context.Context) ([]models.Run, error)
	DeleteRun(ctx context.Context, runID string) error

	// Event operations
	AppendEvent(ctx context.Context, runID string, evt events.Event) error
	ReadEvents(ctx context.Context, runID string) ([]events.Event, error)
	StreamEvents(ctx context.Context, runID string) (<-chan events.Event, error)

	// Snapshots
	SaveSnapshot(ctx context.Context, runID string, data []byte) error
	LoadSnapshot(ctx context.Context, runID string) ([]byte, error)

	Close() error
}

// EnsureStorageDir creates the storage root and runs directory if they don't exist.
func EnsureStorageDir(root string) error {
	return os.MkdirAll(filepath.Join(root, "runs"), 0o700)
}
