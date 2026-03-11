package main

import (
	"fmt"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats <run-id>",
		Short: "Display metrics and statistics for a run",
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

			// Duration by stage
			p.line("=== Duration by Stage ===")
			stageDurs := state.StageDurations()
			if len(stageDurs) > 0 {
				names := sortedKeys(stageDurs)
				for _, name := range names {
					p.linef("  %-30s %s", name, stageDurs[name].Truncate(time.Second))
				}
			} else {
				p.line("  (no stages)")
			}
			p.blank()

			// Tokens by provider
			p.line("=== Tokens by Provider ===")
			if len(state.TokensByProvider) > 0 {
				tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "  Provider\tInput\tOutput\tCached\tCost")
				for _, prov := range sortedKeys(state.TokensByProvider) {
					tc := state.TokensByProvider[prov]
					cost := state.CostByProvider[prov]
					_, _ = fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\t$%.4f\n", prov, tc.Input, tc.Output, tc.Cached, cost)
				}
				_ = tw.Flush()
			} else {
				p.line("  (no model requests)")
			}
			p.blank()

			// Tokens by model
			p.line("=== Tokens by Model ===")
			if len(state.TokensByModel) > 0 {
				tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "  Model\tInput\tOutput\tCached\tCost")
				for _, model := range sortedKeys(state.TokensByModel) {
					tc := state.TokensByModel[model]
					cost := state.CostByModel[model]
					_, _ = fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\t$%.4f\n", model, tc.Input, tc.Output, tc.Cached, cost)
				}
				_ = tw.Flush()
			} else {
				p.line("  (no model requests)")
			}
			p.blank()

			// Tools used
			p.line("=== Tools ===")
			if len(state.ToolCounts) > 0 {
				tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "  Tool\tCount\tAvg Latency")
				for _, tool := range sortedKeys(state.ToolCounts) {
					count := state.ToolCounts[tool]
					avgLat := "-"
					if dur, ok := state.ToolDurations[tool]; ok && count > 0 {
						avgLat = (dur / time.Duration(count)).Truncate(time.Millisecond).String()
					}
					_, _ = fmt.Fprintf(tw, "  %s\t%d\t%s\n", tool, count, avgLat)
				}
				_ = tw.Flush()
			} else {
				p.line("  (no tools)")
			}
			p.blank()

			// Summary stats
			p.line("=== Summary ===")
			p.linef("  Total Duration:   %s", state.Duration().Truncate(time.Second))
			p.linef("  Total Cost:       $%.4f", state.TotalCostUSD)
			p.linef("  Files Changed:    %d", state.UniqueFilesTouched())
			p.linef("  Commits:          %d", len(state.Commits))
			p.linef("  Alerts:           %d", len(state.Alerts))
			p.linef("  Events:           %d", state.EventCount)
			p.linef("  Failures:         %d", state.FailureCount)
			p.linef("  Avg Tool Latency: %s", state.AvgToolLatency().Truncate(time.Millisecond))
			p.linef("  Avg Model Latency:%s", state.AvgModelLatency().Truncate(time.Millisecond))
			mtbc := state.MeanTimeBetweenCommits()
			if mtbc > 0 {
				p.linef("  Mean Time Between Commits: %s", mtbc.Truncate(time.Second))
			}

			return nil
		},
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
