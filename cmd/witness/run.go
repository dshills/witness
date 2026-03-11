package main

import (
	"fmt"
	"os"

	"github.com/dshills/witness/internal/app"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var opts app.RunOptions

	cmd := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Run a command with live observation",
		Long: `Run a subprocess and observe it in real time.
Git changes, file changes, and stdout events are captured automatically.

Example:
  witness run -- make build
  witness run --name "deploy" -- ./deploy.sh staging
  witness run --no-git -- npm test`,
		DisableFlagParsing: false,
		Args:               cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			st, err := fsstore.New(cfg.Storage.Root)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = st.Close() }()

			exitCode, err := app.RunSubprocess(cmd.Context(), cfg, st, args, opts)
			if err != nil {
				return err
			}
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.Name, "name", "", "human-readable run name")
	cmd.Flags().BoolVar(&opts.NoGit, "no-git", false, "disable git observation")
	cmd.Flags().BoolVar(&opts.NoFiles, "no-files", false, "disable file system observation")

	return cmd
}
