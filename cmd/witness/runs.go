package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/spf13/cobra"
)

func newRunsCmd() *cobra.Command {
	var statusFilter string
	var limit int

	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List all recorded runs",
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

			runs, err := s.ListRuns(cmd.Context())
			if err != nil {
				return fmt.Errorf("listing runs: %w", err)
			}

			// Sort by started_at descending (most recent first)
			sort.Slice(runs, func(i, j int) bool {
				return runs[i].StartedAt.After(runs[j].StartedAt)
			})

			// Filter by status
			if statusFilter != "" {
				filtered := make([]models.Run, 0, len(runs))
				for _, r := range runs {
					if strings.EqualFold(r.Status.String(), statusFilter) {
						filtered = append(filtered, r)
					}
				}
				runs = filtered
			}

			// Apply limit
			if limit > 0 && len(runs) > limit {
				runs = runs[:limit]
			}

			if len(runs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No runs found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tName\tStatus\tStarted\tDuration\tCost")
			_, _ = fmt.Fprintln(w, "--\t----\t------\t-------\t--------\t----")

			for _, r := range runs {
				name := r.Name
				if name == "" {
					name = "-"
				}
				started := r.StartedAt.Format(time.DateTime)
				dur := "-"
				if !r.StartedAt.IsZero() {
					if r.EndedAt != nil {
						dur = r.EndedAt.Sub(r.StartedAt).Truncate(time.Second).String()
					} else {
						dur = time.Since(r.StartedAt).Truncate(time.Second).String()
					}
				}
				// Cost is not stored on the Run model; show "-" unless we rebuild state.
				// For the list view we keep it lightweight.
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t-\n",
					r.RunID, name, r.Status, started, dur)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (running, completed, failed, etc.)")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of runs to display")

	return cmd
}
