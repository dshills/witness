package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/ingest"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/spf13/cobra"
)

func newWrapCmd() *cobra.Command {
	var (
		runID string
		name  string
	)

	cmd := &cobra.Command{
		Use:   "wrap",
		Short: "Pipe stdin through Witness, capturing events and tool results",
		Long: `Read stdin, pass it through to stdout, and capture any Witness events
or structured tool results found in the stream. Use this to observe
tool output without Witness being the parent process.

Examples:
  prism review --json | witness wrap --run run_01J...
  golangci-lint run ./... 2>&1 | witness wrap --name "lint"
  claude --print | witness wrap --name "claude-session"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			st, err := fsstore.New(cfg.Storage.Root)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = st.Close() }()

			return runWrap(cmd.Context(), st, runID, name)
		},
	}

	cmd.Flags().StringVar(&runID, "run", "", "attach to an existing run ID")
	cmd.Flags().StringVar(&name, "name", "", "create a new run with this name")

	return cmd
}

func runWrap(ctx context.Context, st *fsstore.FSStore, runID, name string) error {
	// If no run ID, create a new run.
	if runID == "" {
		runID = events.NewID("run")
		host, _ := os.Hostname()
		user := os.Getenv("USER")
		if user == "" {
			user = os.Getenv("USERNAME")
		}
		run := models.Run{
			RunID:     runID,
			Name:      name,
			Status:    models.RunStatusRunning,
			StartedAt: time.Now().UTC(),
			Host:      host,
			User:      user,
		}
		if err := st.CreateRun(ctx, run); err != nil {
			return fmt.Errorf("creating run: %w", err)
		}
	} else {
		// Verify the run exists.
		if _, err := st.GetRun(ctx, runID); err != nil {
			return fmt.Errorf("run %s not found: %w", runID, err)
		}
	}

	// Create a minimal sink that just persists events.
	sink := &wrapSink{store: st, runID: runID}

	// Set up scanner for event/tool-result detection.
	pr, pw := io.Pipe()
	sc := ingest.NewScanner(pr, sink, runID)

	// Scanner goroutine.
	done := make(chan error, 1)
	go func() {
		done <- sc.Scan(ctx)
	}()

	// Read stdin, tee to stdout and to the scanner pipe.
	tee := io.TeeReader(os.Stdin, pw)

	// Copy stdin -> stdout (tee feeds scanner pipe).
	_, _ = io.Copy(os.Stdout, tee)
	_ = pw.Close()

	// Wait for scanner to finish.
	if err := <-done; err != nil {
		log.Printf("witness wrap: scanner error: %v", err)
	}

	if sink.count > 0 {
		fmt.Fprintf(os.Stderr, "witness: wrap captured %d events for run %s\n", sink.count, runID)
	}

	return nil
}

// wrapSink is a minimal EventSink that persists events to the store.
type wrapSink struct {
	store interface {
		AppendEvent(ctx context.Context, runID string, evt events.Event) error
	}
	runID string
	count int64
}

func (s *wrapSink) Append(ctx context.Context, evt events.Event) error {
	if err := events.Validate(evt); err != nil {
		return fmt.Errorf("wrap: invalid event: %w", err)
	}
	if err := s.store.AppendEvent(ctx, s.runID, evt); err != nil {
		return fmt.Errorf("wrap: persist: %w", err)
	}
	s.count++
	return nil
}
