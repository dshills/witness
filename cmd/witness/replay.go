package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/replay"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/dshills/witness/internal/tui"
	"github.com/dshills/witness/internal/tui/panels"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

func newReplayCmd() *cobra.Command {
	var (
		speed   float64
		summary bool
		text    bool
	)

	cmd := &cobra.Command{
		Use:   "replay <run-id>",
		Short: "Replay a historical run's event timeline",
		Long: `Replay a run by stepping through its events in chronological order.
By default, opens the TUI in replay mode with playback controls.
Use --text for the old text-based timeline output.
Use --summary to print only the postmortem summary.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			s, err := fsstore.New(cfg.Storage.Root)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = s.Close() }()

			runID := args[0]
			run, err := s.GetRun(cmd.Context(), runID)
			if err != nil {
				return fmt.Errorf("loading run: %w", err)
			}

			evts, err := s.ReadEvents(cmd.Context(), runID)
			if err != nil {
				return fmt.Errorf("reading events: %w", err)
			}

			if len(evts) == 0 {
				return fmt.Errorf("no events found for run %s", runID)
			}

			// Summary-only mode: rebuild full state and print postmortem.
			if summary {
				state, err := aggregate.Rebuild(run, evts)
				if err != nil {
					return fmt.Errorf("rebuilding state: %w", err)
				}
				out := cmd.OutOrStdout()
				_, _ = fmt.Fprint(out, replay.GeneratePostmortem(*state))
				return nil
			}

			// Text-based timeline mode.
			if text {
				out := cmd.OutOrStdout()
				ctrl := replay.NewController(run, evts)
				_, _ = fmt.Fprintf(out, "Replaying run %s (%d events)\n\n", runID, len(evts))

				if speed > 0 {
					return replayAutomatic(cmd.Context(), ctrl, out, speed)
				}
				return replayImmediate(ctrl, out)
			}

			// Default: TUI replay mode.
			initialSpeed := speed
			if initialSpeed <= 0 {
				initialSpeed = 1.0
			}
			return launchReplayTUI(cmd.Context(), cfg, run, evts, initialSpeed)
		},
	}

	cmd.Flags().Float64Var(&speed, "speed", 0, "playback speed (1.0 = real time, 2.0 = double speed)")
	cmd.Flags().BoolVar(&summary, "summary", false, "print only the postmortem summary")
	cmd.Flags().BoolVar(&text, "text", false, "use text-based timeline instead of TUI")

	return cmd
}

// launchReplayTUI opens the TUI in replay mode.
func launchReplayTUI(ctx context.Context, cfg *config.Config, run models.Run, evts []events.Event, speed float64) error {
	ctrl := replay.NewController(run, evts)
	ctrl.SetSpeed(speed)

	replayCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Build panels (same as live mode).
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

	app := tui.NewReplayApp(tuiPanels, ctrl, replayCtx)
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Start replay bridge in background.
	refreshMS := cfg.UI.RefreshMS
	if refreshMS <= 0 {
		refreshMS = 100
	}
	go func() {
		tui.RunReplayBridge(replayCtx, ctrl, refreshMS, p)
	}()

	// Run the TUI (blocks until quit).
	if _, err := p.Run(); err != nil {
		cancel()
		return fmt.Errorf("TUI error: %w", err)
	}

	cancel()
	ctrl.Pause()
	return nil
}

// replayImmediate prints all events instantly without delays.
func replayImmediate(ctrl *replay.Controller, out io.Writer) error {
	for {
		evt, err := ctrl.StepForward()
		if err != nil {
			if err == replay.ErrAtEnd {
				break
			}
			return err
		}
		current, total := ctrl.Progress()
		line := formatEventLine(current, total, evt.Timestamp, string(evt.Type), evt.Source, evt.Summary)
		_, _ = fmt.Fprintln(out, line)
	}

	// Print final summary.
	state := ctrl.CurrentState()
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprint(out, replay.GeneratePostmortem(state))

	return nil
}

// replayAutomatic plays events with timing delays scaled by speed.
func replayAutomatic(ctx context.Context, ctrl *replay.Controller, out io.Writer, speed float64) error {
	ctrl.SetSpeed(speed)
	ctrl.Play(ctx)

	for {
		select {
		case <-ctx.Done():
			ctrl.Pause()
			return ctx.Err()
		case state, ok := <-ctrl.Updates():
			if !ok {
				return nil
			}
			// Print the most recent event from state.
			if len(state.RecentEvents) > 0 {
				evt := state.RecentEvents[len(state.RecentEvents)-1]
				current, total := ctrl.Progress()
				line := formatEventLine(current, total, evt.Timestamp, string(evt.Type), evt.Source, evt.Summary)
				_, _ = fmt.Fprintln(out, line)
			}

			// Check if we've reached the end.
			current, total := ctrl.Progress()
			if current >= total-1 {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprint(out, replay.GeneratePostmortem(state))
				return nil
			}
		}
	}
}

func formatEventLine(current, total int, ts time.Time, eventType, source, summary string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%d/%d]", current+1, total))
	parts = append(parts, ts.Format(time.TimeOnly))
	parts = append(parts, eventType)
	if source != "" {
		parts = append(parts, source+":")
	}
	if summary != "" {
		parts = append(parts, summary)
	}
	return strings.Join(parts, "  ")
}
