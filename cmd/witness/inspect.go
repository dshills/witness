package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <run-id>",
		Short: "Display detailed information about a run",
		Args:  cobra.ExactArgs(1),
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

			state, err := aggregate.Rebuild(run, evts)
			if err != nil {
				return fmt.Errorf("rebuilding state: %w", err)
			}

			out := cmd.OutOrStdout()
			p := printer{w: out}

			// Run metadata
			p.line("=== Run ===")
			p.linef("  ID:       %s", state.Run.RunID)
			p.linef("  Name:     %s", state.Run.Name)
			p.linef("  Status:   %s", state.Run.Status)
			p.linef("  Duration: %s", state.Duration().Truncate(time.Second))
			if state.Run.RepoRoot != "" {
				p.linef("  Repo:     %s", state.Run.RepoRoot)
			}
			if state.Run.Branch != "" {
				p.linef("  Branch:   %s", state.Run.Branch)
			}
			if len(state.Run.Command) > 0 {
				p.linef("  Command:  %s", strings.Join(state.Run.Command, " "))
			}
			p.blank()

			// Stages
			if len(state.Stages) > 0 {
				p.line("=== Stages ===")
				for i := range state.Stages {
					st := &state.Stages[i]
					icon := stageIcon(st.Status)
					dur := "-"
					if st.StartedAt != nil {
						if st.EndedAt != nil {
							dur = st.EndedAt.Sub(*st.StartedAt).Truncate(time.Second).String()
						} else {
							dur = "running"
						}
					}
					p.linef("  %s %s (%s) [%s]", icon, st.Name, st.Status, dur)
				}
				p.blank()
			}

			// Token/cost summary
			if state.TotalInputTokens > 0 || state.TotalOutputTokens > 0 {
				p.line("=== Token Usage ===")
				p.linef("  Input:  %d", state.TotalInputTokens)
				p.linef("  Output: %d", state.TotalOutputTokens)
				p.linef("  Cached: %d", state.TotalCachedTokens)
				p.linef("  Cost:   $%.4f", state.TotalCostUSD)
				p.blank()

				if len(state.TokensByModel) > 0 {
					tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
					_, _ = fmt.Fprintln(tw, "  Model\tInput\tOutput\tCost")
					for model, tc := range state.TokensByModel {
						cost := state.CostByModel[model]
						_, _ = fmt.Fprintf(tw, "  %s\t%d\t%d\t$%.4f\n", model, tc.Input, tc.Output, cost)
					}
					_ = tw.Flush()
					p.blank()
				}
			}

			// Git summary
			if len(state.Commits) > 0 || len(state.FileChanges) > 0 {
				p.line("=== Git ===")
				p.linef("  Commits:       %d", len(state.Commits))
				p.linef("  Files changed: %d", state.UniqueFilesTouched())
				p.blank()
			}

			// Active alerts
			if len(state.ActiveAlerts) > 0 {
				p.line("=== Active Alerts ===")
				for i := range state.ActiveAlerts {
					a := &state.ActiveAlerts[i]
					p.linef("  [%s] %s: %s", a.Severity, a.Title, a.Description)
				}
				p.blank()
			}

			// Last 10 events
			recent := state.RecentEvents
			if len(recent) > 10 {
				recent = recent[len(recent)-10:]
			}
			if len(recent) > 0 {
				p.line("=== Recent Events ===")
				for i := range recent {
					e := &recent[i]
					p.linef("  %s  %s  %s",
						e.Timestamp.Format(time.TimeOnly), e.Type, e.Summary)
				}
			}

			return nil
		},
	}
}

// printer wraps an io.Writer and discards write errors (appropriate for CLI output).
type printer struct {
	w io.Writer
}

func (p *printer) line(s string) {
	_, _ = fmt.Fprintln(p.w, s)
}

func (p *printer) linef(format string, args ...any) {
	_, _ = fmt.Fprintf(p.w, format+"\n", args...)
}

func (p *printer) blank() {
	_, _ = fmt.Fprintln(p.w)
}

func stageIcon(status models.StageStatus) string {
	switch status {
	case models.StageStatusCompleted:
		return "[OK]"
	case models.StageStatusFailed:
		return "[FAIL]"
	case models.StageStatusRunning:
		return "[..]"
	case models.StageStatusSkipped:
		return "[SKIP]"
	default:
		return "[  ]"
	}
}
