package main

import (
	"fmt"
	"io"
	"os"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/export"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var format string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "export <run-id>",
		Short: "Export a run in JSON, NDJSON, or Markdown format",
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

			var exporter export.Exporter
			switch format {
			case "json":
				exporter = &export.JSONExporter{}
			case "ndjson":
				exporter = &export.NDJSONExporter{}
			case "markdown", "md":
				exporter = &export.MarkdownExporter{}
			default:
				return fmt.Errorf("unknown format %q (supported: json, ndjson, markdown)", format)
			}

			w := io.Writer(cmd.OutOrStdout())
			if outputFile != "" {
				f, err := os.Create(outputFile)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer func() { _ = f.Close() }()
				w = f
			}

			return exporter.Export(cmd.Context(), *state, evts, w)
		},
	}

	cmd.Flags().StringVar(&format, "format", "json", "output format: json, ndjson, markdown")
	cmd.Flags().StringVar(&outputFile, "output", "", "write to file instead of stdout")

	return cmd
}
