package fsstore

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/store"
	"github.com/fsnotify/fsnotify"
)

const (
	runFile      = "run.json"
	eventsFile   = "events.ndjson"
	snapshotFile = "snapshot.json"
	dedupSize    = 256
	streamBufCap = 256
)

// FSStore implements store.Store using the local filesystem.
type FSStore struct {
	root string

	mu       sync.Mutex
	dedupBuf map[string]*ringBuffer // runID -> ring buffer of recent event IDs
}

var _ store.Store = (*FSStore)(nil)

// New creates a new FSStore rooted at the given directory.
// It ensures the storage directory structure exists.
func New(root string) (*FSStore, error) {
	if err := store.EnsureStorageDir(root); err != nil {
		return nil, fmt.Errorf("ensuring storage dir: %w", err)
	}
	return &FSStore{
		root:     root,
		dedupBuf: make(map[string]*ringBuffer),
	}, nil
}

func (s *FSStore) runDir(runID string) string {
	return filepath.Join(s.root, "runs", runID)
}

// CreateRun persists a new run to disk.
func (s *FSStore) CreateRun(_ context.Context, run models.Run) error {
	dir := s.runDir(run.RunID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating run directory: %w", err)
	}
	return atomicWriteJSON(filepath.Join(dir, runFile), run)
}

// GetRun reads a run from disk.
func (s *FSStore) GetRun(_ context.Context, runID string) (models.Run, error) {
	var run models.Run
	data, err := os.ReadFile(filepath.Join(s.runDir(runID), runFile))
	if err != nil {
		return run, fmt.Errorf("reading run: %w", err)
	}
	if err := json.Unmarshal(data, &run); err != nil {
		return run, fmt.Errorf("parsing run: %w", err)
	}
	return run, nil
}

// UpdateRun overwrites the run metadata on disk.
func (s *FSStore) UpdateRun(_ context.Context, run models.Run) error {
	return atomicWriteJSON(filepath.Join(s.runDir(run.RunID), runFile), run)
}

// ListRuns scans the runs directory and returns all runs sorted by start time.
func (s *FSStore) ListRuns(_ context.Context) ([]models.Run, error) {
	runsDir := filepath.Join(s.root, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing runs: %w", err)
	}

	var runs []models.Run
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(runsDir, entry.Name(), runFile))
		if err != nil {
			log.Printf("fsstore: skipping run dir %s: %v", entry.Name(), err)
			continue
		}
		var run models.Run
		if err := json.Unmarshal(data, &run); err != nil {
			log.Printf("fsstore: skipping run dir %s: %v", entry.Name(), err)
			continue
		}
		runs = append(runs, run)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.Before(runs[j].StartedAt)
	})
	return runs, nil
}

// DeleteRun removes a run directory and all its contents.
func (s *FSStore) DeleteRun(_ context.Context, runID string) error {
	s.mu.Lock()
	delete(s.dedupBuf, runID)
	s.mu.Unlock()
	return os.RemoveAll(s.runDir(runID))
}

// AppendEvent appends an event to the run's NDJSON log with deduplication.
// Deduplication is in-memory only; after a process restart, previously seen
// event IDs are no longer tracked.
func (s *FSStore) AppendEvent(_ context.Context, runID string, evt events.Event) error {
	s.mu.Lock()
	ring, ok := s.dedupBuf[runID]
	if !ok {
		ring = newRingBuffer(dedupSize)
		s.dedupBuf[runID] = ring
	}
	if ring.Contains(evt.EventID) {
		s.mu.Unlock()
		return nil // duplicate, skip
	}
	s.mu.Unlock()

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	data = append(data, '\n')

	path := filepath.Join(s.runDir(runID), eventsFile)

	// Hold mutex during file I/O to prevent interleaved writes
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("opening events file: %w", err)
	}

	var writeErr error
	if _, writeErr = f.Write(data); writeErr == nil {
		writeErr = f.Sync()
	}
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return fmt.Errorf("writing event: %w", writeErr)
	}

	// Only record in dedup ring after successful write
	ring.Add(evt.EventID)
	return nil
}

// ReadEvents reads all valid events from the NDJSON log.
// Partial final lines (from crashes) are silently discarded.
func (s *FSStore) ReadEvents(_ context.Context, runID string) ([]events.Event, error) {
	path := filepath.Join(s.runDir(runID), eventsFile)
	evts, _, err := readEventsFromFile(path)
	return evts, err
}

// readEventsFromFile reads events and returns the byte offset after the last complete line.
func readEventsFromFile(path string) ([]events.Event, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("opening events file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var evts []events.Event
	var offset int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Track bytes consumed: line content + newline
		offset += int64(len(line)) + 1
		if len(line) == 0 {
			continue
		}
		var evt events.Event
		if err := json.Unmarshal(line, &evt); err != nil {
			// Discard malformed lines (crash tolerance)
			continue
		}
		evts = append(evts, evt)
	}
	if err := scanner.Err(); err != nil {
		return evts, 0, fmt.Errorf("scanning events file: %w", err)
	}

	return evts, offset, nil
}

// StreamEvents returns a channel that delivers existing events then tails for new ones.
// The channel is closed when the context is cancelled.
func (s *FSStore) StreamEvents(ctx context.Context, runID string) (<-chan events.Event, error) {
	path := filepath.Join(s.runDir(runID), eventsFile)

	// Ensure the events file exists so fsnotify can watch it
	if err := ensureFileExists(path); err != nil {
		return nil, fmt.Errorf("ensuring events file: %w", err)
	}

	// Set up watcher before reading to avoid missing events in the gap
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}
	if err := watcher.Add(path); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("watching events file: %w", err)
	}

	// Read existing events and capture offset
	existing, offset, err := readEventsFromFile(path)
	if err != nil {
		_ = watcher.Close()
		return nil, err
	}

	ch := make(chan events.Event, streamBufCap)

	go func() {
		defer close(ch)
		defer func() { _ = watcher.Close() }()

		// Send existing events
		for _, evt := range existing {
			select {
			case ch <- evt:
			case <-ctx.Done():
				return
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == 0 {
					continue
				}
				offset = s.tailNewEvents(path, offset, ch)
			case watchErr, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("fsstore: watcher error for %s: %v", path, watchErr)
			}
		}
	}()

	return ch, nil
}

// tailNewEvents reads new events from the file starting at offset.
func (s *FSStore) tailNewEvents(path string, offset int64, ch chan<- events.Event) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}

	newOffset := offset
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Track bytes consumed: line content + newline
		newOffset += int64(len(line)) + 1
		if len(line) == 0 {
			continue
		}
		var evt events.Event
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		select {
		case ch <- evt:
		default:
			log.Printf("warning: dropping event %s for slow consumer", evt.EventID)
		}
	}

	return newOffset
}

// SaveSnapshot writes snapshot data atomically with fsync.
func (s *FSStore) SaveSnapshot(_ context.Context, runID string, data []byte) error {
	dir := s.runDir(runID)
	target := filepath.Join(dir, snapshotFile)
	tmp := target + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating snapshot tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("writing snapshot tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("syncing snapshot tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("closing snapshot tmp: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming snapshot: %w", err)
	}
	return nil
}

// LoadSnapshot reads snapshot data from disk.
func (s *FSStore) LoadSnapshot(_ context.Context, runID string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.runDir(runID), snapshotFile))
	if err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}
	return data, nil
}

// Close releases resources held by the store.
func (s *FSStore) Close() error {
	return nil
}

// atomicWriteJSON writes a JSON file atomically using write-to-temp-then-rename.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writing tmp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming to target: %w", err)
	}
	return nil
}

// ensureFileExists creates the file if it doesn't exist.
func ensureFileExists(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}
