package main

import (
	"context"
	"fmt"

	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/dshills/witness/internal/tui"
	"github.com/dshills/witness/internal/tui/panels"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

func newWatchCmd() *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Open the live TUI dashboard for a run",
		Long: `Open the Witness TUI to monitor a run in real time.
By default, shows the most recent active run.
Use --run <id> to specify a run, or --run latest for the most recent regardless of status.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			s, err := fsstore.New(cfg.Storage.Root)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = s.Close() }()

			run, err := resolveRun(cmd.Context(), s, runID, false)
			if err != nil {
				return err
			}

			return launchTUI(cmd.Context(), s, cfg, run)
		},
	}

	cmd.Flags().StringVar(&runID, "run", "", "run ID to watch (or 'latest')")

	return cmd
}

func newAttachCmd() *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attach to an active run's TUI dashboard",
		Long:  `Attach to a specific active run. The run must be in a non-terminal state (running, pending, stalled, unknown).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			s, err := fsstore.New(cfg.Storage.Root)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = s.Close() }()

			if runID == "" {
				return fmt.Errorf("--run flag is required for attach")
			}

			run, err := s.GetRun(cmd.Context(), runID)
			if err != nil {
				return fmt.Errorf("loading run: %w", err)
			}

			if isTerminal(run.Status) {
				return fmt.Errorf("run %s is not active (status: %s)", runID, run.Status)
			}

			return launchTUI(cmd.Context(), s, cfg, run)
		},
	}

	cmd.Flags().StringVar(&runID, "run", "", "run ID to attach to (required)")

	return cmd
}

// resolveRun finds the appropriate run based on the runID flag.
func resolveRun(ctx context.Context, s *fsstore.FSStore, runID string, _ bool) (models.Run, error) {
	if runID == "latest" {
		return latestRun(ctx, s)
	}
	if runID != "" {
		return s.GetRun(ctx, runID)
	}

	// Default: find most recent active run.
	runs, err := s.ListRuns(ctx)
	if err != nil {
		return models.Run{}, fmt.Errorf("listing runs: %w", err)
	}

	// Search from most recent.
	for i := len(runs) - 1; i >= 0; i-- {
		if !isTerminal(runs[i].Status) {
			return runs[i], nil
		}
	}

	return models.Run{}, fmt.Errorf("no active runs found. Use 'witness watch --run <id>' to view a completed run")
}

func latestRun(ctx context.Context, s *fsstore.FSStore) (models.Run, error) {
	runs, err := s.ListRuns(ctx)
	if err != nil {
		return models.Run{}, fmt.Errorf("listing runs: %w", err)
	}
	if len(runs) == 0 {
		return models.Run{}, fmt.Errorf("no runs found")
	}
	return runs[len(runs)-1], nil
}

func isTerminal(status models.RunStatus) bool {
	switch status {
	case models.RunStatusCompleted, models.RunStatusFailed, models.RunStatusCancelled:
		return true
	default:
		return false
	}
}

func launchTUI(ctx context.Context, s *fsstore.FSStore, cfg *config.Config, run models.Run) error {
	// Set up event stream.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventCh, err := s.StreamEvents(streamCtx, run.RunID)
	if err != nil {
		return fmt.Errorf("streaming events: %w", err)
	}

	// Build panels.
	budgetLimit := cfg.Alerts.MaxRunCostUSD
	tuiPanels := []tui.Panel{
		panels.NewHeaderPanel(),
		panels.NewStagePanel(),
		panels.NewActiveWorkPanel(),
		panels.NewTokenCostPanel(budgetLimit),
		panels.NewGitFilePanel(),
		panels.NewAlertsPanel(),
		panels.NewEventStreamPanel(),
	}

	app := tui.NewApp(tuiPanels)
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Start bridge in background.
	refreshMS := cfg.UI.RefreshMS
	if refreshMS <= 0 {
		refreshMS = 500
	}
	go func() {
		tui.RunBridge(streamCtx, run, eventCh, refreshMS, p)
	}()

	// Run the TUI (blocks until quit).
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	cancel()
	return nil
}
