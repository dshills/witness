package files

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dshills/witness/internal/events"
	"github.com/fsnotify/fsnotify"
)

const (
	source         = "witness/files"
	debounceWindow = 100 * time.Millisecond
)

// Watcher monitors a directory tree for file changes and emits events.
type Watcher struct {
	root   string
	ignore []string
	sink   events.EventSink
	runID  string
}

// NewWatcher creates a file system Watcher.
func NewWatcher(root string, ignore []string, sink events.EventSink, runID string) *Watcher {
	return &Watcher{
		root:   root,
		ignore: ignore,
		sink:   sink,
		runID:  runID,
	}
}

// Start begins watching until the context is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()

	// Recursively add all subdirectories.
	if err := w.addRecursive(watcher, w.root); err != nil {
		return err
	}

	// Debounce state
	var mu sync.Mutex
	pending := make(map[string]fsnotify.Event)
	timer := time.NewTimer(debounceWindow)
	timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			rel := w.relPath(evt.Name)
			if w.shouldIgnore(rel) {
				continue
			}

			// Watch newly created directories.
			if evt.Op&fsnotify.Create != 0 {
				if addErr := w.addIfDir(watcher, evt.Name); addErr != nil {
					log.Printf("files watcher: add dir: %v", addErr)
				}
			}

			mu.Lock()
			pending[evt.Name] = evt
			mu.Unlock()
			timer.Reset(debounceWindow)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("files watcher: error: %v", err)

		case <-timer.C:
			mu.Lock()
			batch := pending
			pending = make(map[string]fsnotify.Event)
			mu.Unlock()

			for _, evt := range batch {
				w.emitEvent(ctx, evt)
			}
		}
	}
}

// addRecursive walks the directory tree and adds all directories to the watcher.
func (w *Watcher) addRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if !d.IsDir() {
			return nil
		}
		rel := w.relPath(path)
		if rel != "." && w.shouldIgnore(rel) {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
}

// addIfDir adds the path to the watcher if it is a directory.
func (w *Watcher) addIfDir(watcher *fsnotify.Watcher, path string) error {
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		rel := w.relPath(p)
		if rel != "." && w.shouldIgnore(rel) {
			return filepath.SkipDir
		}
		return watcher.Add(p)
	})
}

func (w *Watcher) emitEvent(ctx context.Context, fsEvt fsnotify.Event) {
	var eventType events.EventType
	switch {
	case fsEvt.Op&fsnotify.Create != 0:
		eventType = events.EventFileCreated
	case fsEvt.Op&fsnotify.Write != 0:
		eventType = events.EventFileModified
	case fsEvt.Op&fsnotify.Remove != 0 || fsEvt.Op&fsnotify.Rename != 0:
		eventType = events.EventFileDeleted
	default:
		return
	}

	rel := w.relPath(fsEvt.Name)
	payload, _ := json.Marshal(map[string]string{
		"change_id": events.NewID("fc"),
		"path":      rel,
	})
	evt := events.NewEvent(w.runID, eventType, source, payload)
	if err := w.sink.Append(ctx, evt); err != nil {
		log.Printf("files watcher: emit event: %v", err)
	}
}

func (w *Watcher) relPath(abs string) string {
	rel, err := filepath.Rel(w.root, abs)
	if err != nil {
		return abs
	}
	return rel
}

// ShouldIgnore checks if a relative path matches any ignore pattern.
// Exported for testing.
func ShouldIgnore(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		// Try direct match.
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
		// Try matching against each path component prefix.
		// e.g., pattern ".git/**" should match ".git/objects/foo".
		if strings.Contains(pattern, "**") {
			base := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(relPath, base+string(filepath.Separator)) || relPath == base {
				return true
			}
		}
		// Also match basename for simple patterns like "*.swp".
		base := filepath.Base(relPath)
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

func (w *Watcher) shouldIgnore(relPath string) bool {
	return ShouldIgnore(relPath, w.ignore)
}
