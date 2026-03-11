package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/alerts"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/files"
	"github.com/dshills/witness/internal/git"
	"github.com/dshills/witness/internal/ingest"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/privacy"
	"github.com/dshills/witness/internal/store"
)

const (
	runSource      = "witness/runner"
	signalDeadline = 10 * time.Second
)

// RunOptions holds flags for the run command.
type RunOptions struct {
	Name        string
	NoGit       bool
	NoFiles     bool
	Interactive bool
}

// RunSubprocess creates a run, starts the subprocess, and orchestrates
// observation goroutines. It returns the exit code of the subprocess.
func RunSubprocess(ctx context.Context, cfg *config.Config, st store.Store, command []string, opts RunOptions) (int, error) {
	if len(command) == 0 {
		return 1, fmt.Errorf("no command specified")
	}

	runID := events.NewID("run")
	now := time.Now().UTC()

	wd, _ := os.Getwd()
	host, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}

	run := models.Run{
		RunID:      runID,
		Name:       opts.Name,
		Status:     models.RunStatusPending,
		StartedAt:  now,
		Command:    command,
		WorkingDir: wd,
		Host:       host,
		User:       user,
	}

	// Create run in store.
	if err := st.CreateRun(ctx, run); err != nil {
		return 1, fmt.Errorf("creating run: %w", err)
	}

	// Set up aggregator.
	agg := aggregate.NewAggregator(run)

	// Set up redactor.
	redactor, err := privacy.NewRedactor(cfg.Privacy.RedactPatterns)
	if err != nil {
		return 1, fmt.Errorf("creating redactor: %w", err)
	}

	// Create alert engine for anomaly detection.
	alertEngine := alerts.NewEngine(cfg.Alerts)
	alertEngine.RegisterDefaultRules()

	sink := NewStoreSink(st, runID, agg, redactor, alertEngine)

	// Emit run.created.
	createdPayload, _ := json.Marshal(map[string]any{
		"name":        opts.Name,
		"command":     command,
		"working_dir": wd,
		"host":        host,
		"user":        user,
	})
	emitEvent(ctx, sink, runID, events.EventRunCreated, createdPayload)

	// Detect git root.
	var repoRoot string
	if !opts.NoGit {
		repoRoot, err = git.DetectRepoRoot(ctx, wd)
		if err != nil {
			log.Printf("witness: git not available: %v", err)
		}
	}

	// Context for observation goroutines.
	obsCtx, obsCancel := context.WithCancel(ctx)
	var wg sync.WaitGroup

	// Start git observer.
	if repoRoot != "" && !opts.NoGit {
		interval := time.Duration(cfg.Git.PollIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 5 * time.Second
		}
		gitObs := git.NewObserver(repoRoot, interval, sink, runID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = gitObs.Start(obsCtx)
		}()

		// Update run with repo info.
		if branch, err := git.CurrentBranch(ctx, repoRoot); err == nil {
			run.Branch = branch
			run.RepoRoot = repoRoot
			_ = st.UpdateRun(ctx, run)
		}
	}

	// Start file watcher.
	if !opts.NoFiles {
		watchRoot := repoRoot
		if watchRoot == "" {
			watchRoot = wd
		}
		fw := files.NewWatcher(watchRoot, cfg.Files.Ignore, sink, runID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fw.Start(obsCtx); err != nil {
				log.Printf("witness: file watcher error: %v", err)
			}
		}()
	}

	// Build subprocess command.
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = wd
	setProcGroup(cmd)

	// Interactive mode: connect stdin directly to the subprocess.
	if opts.Interactive {
		cmd.Stdin = os.Stdin
	}

	// Stdout: tee to terminal and ingest scanner.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		obsCancel()
		wg.Wait()
		return 1, fmt.Errorf("creating stdout pipe: %w", err)
	}

	// Use a pipe for feeding the scanner from the tee reader.
	pr, pw := io.Pipe()
	teeReader := io.TeeReader(stdoutPipe, pw)

	// Stderr: pipe to relay.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		obsCancel()
		wg.Wait()
		return 1, fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Start subprocess.
	if err := cmd.Start(); err != nil {
		obsCancel()
		wg.Wait()
		emitRunFailed(ctx, sink, runID, err)
		updateRunStatus(ctx, st, run, models.RunStatusFailed)
		fmt.Fprintf(os.Stderr, "witness: run %s failed (command not found)\n", runID)
		return 1, nil
	}

	// Emit run.started.
	emitEvent(ctx, sink, runID, events.EventRunStarted, json.RawMessage(`{}`))
	run.Status = models.RunStatusRunning
	_ = st.UpdateRun(ctx, run)

	// Signal handling.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine: relay stdout to terminal and feed pipe writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = pw.Close() }()
		_, _ = io.Copy(os.Stdout, teeReader)
	}()

	// Goroutine: relay stderr to terminal.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stderr, stderrPipe)
	}()

	// Goroutine: ingest scanner on stdout lines.
	sc := ingest.NewScanner(pr, sink, runID)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = sc.Scan(obsCtx)
	}()

	// Wait for subprocess or signal.
	exitCode := 0
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	cancelled := false
	select {
	case err := <-waitDone:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
	case sig := <-sigCh:
		cancelled = true
		_ = forwardSignal(cmd.Process, sig)

		// Start deadline timer.
		deadline := time.NewTimer(signalDeadline)
		select {
		case <-waitDone:
			deadline.Stop()
		case <-deadline.C:
			_ = killProcessGroup(cmd.Process)
			<-waitDone
		}
		exitCode = 130 // Convention for signal-terminated.
	}

	// Stop signal handling.
	signal.Stop(sigCh)

	// Cancel observation goroutines.
	obsCancel()

	// Emit terminal event.
	if cancelled {
		emitEvent(ctx, sink, runID, events.EventRunCancelled, json.RawMessage(`{}`))
		updateRunStatus(ctx, st, run, models.RunStatusCancelled)
	} else if exitCode == 0 {
		emitEvent(ctx, sink, runID, events.EventRunCompleted, json.RawMessage(`{}`))
		updateRunStatus(ctx, st, run, models.RunStatusCompleted)
	} else {
		payload, _ := json.Marshal(map[string]int{"exit_code": exitCode})
		emitEvent(ctx, sink, runID, events.EventRunFailed, payload)
		updateRunStatus(ctx, st, run, models.RunStatusFailed)
	}

	// Wait for all goroutines.
	wg.Wait()

	// Write final snapshot.
	sink.SaveFinalSnapshot(context.Background())

	// Print summary.
	snap := agg.Snapshot()
	fmt.Fprintf(os.Stderr, "\nwitness: run %s %s (%d events)\n", runID, run.Status, snap.EventCount)

	return exitCode, nil
}

func emitEvent(ctx context.Context, sink events.EventSink, runID string, eventType events.EventType, payload json.RawMessage) {
	evt := events.NewEvent(runID, eventType, runSource, payload)
	if err := sink.Append(ctx, evt); err != nil {
		log.Printf("witness: emit %s: %v", eventType, err)
	}
}

func emitRunFailed(ctx context.Context, sink events.EventSink, runID string, cause error) {
	payload, _ := json.Marshal(map[string]string{"error": cause.Error()})
	emitEvent(ctx, sink, runID, events.EventRunFailed, payload)
}

func updateRunStatus(ctx context.Context, st store.Store, run models.Run, status models.RunStatus) {
	run.Status = status
	now := time.Now().UTC()
	run.EndedAt = &now
	_ = st.UpdateRun(ctx, run)
}
