package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dshills/witness/internal/app"
	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/git"
	"github.com/dshills/witness/internal/store/fsstore"
	"github.com/spf13/cobra"
)

func newClaudeCmd() *cobra.Command {
	var (
		name   string
		noGit  bool
		resume bool
	)

	cmd := &cobra.Command{
		Use:   "claude [flags] [-- extra-args...]",
		Short: "Start a Claude Code session with full observation",
		Long: `Launch Claude Code under Witness observation with sensible defaults
for agentic coding sessions. Stdin is always connected, the run is
auto-named from the current repo and branch, and alert thresholds
are tuned for longer thinking times.

Examples:
  witness claude
  witness claude --name "refactor auth"
  witness claude -- --model sonnet
  witness claude --resume`,
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			// Tune config for agentic sessions.
			config.ApplyAgenticDefaults(cfg)

			st, err := fsstore.New(cfg.Storage.Root)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = st.Close() }()

			// Auto-generate run name from repo/branch if not specified.
			if name == "" {
				name = autoRunName(cmd.Context())
			}

			// Build the claude command.
			command := buildClaudeCommand(args, resume)

			opts := app.RunOptions{
				Name:        name,
				Interactive: true,
				NoGit:       noGit,
			}

			exitCode, err := app.RunSubprocess(cmd.Context(), cfg, st, command, opts)
			if err != nil {
				return err
			}
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "run name (default: auto from repo/branch)")
	cmd.Flags().BoolVar(&noGit, "no-git", false, "disable git observation")
	cmd.Flags().BoolVar(&resume, "resume", false, "resume the most recent Claude conversation")

	return cmd
}

// autoRunName generates a run name from the current directory and git branch.
func autoRunName(ctx context.Context) string {
	wd, err := os.Getwd()
	if err != nil {
		return "claude-session"
	}

	dir := filepath.Base(wd)

	// Try to get the current branch.
	if root, err := git.DetectRepoRoot(ctx, wd); err == nil {
		if branch, err := git.CurrentBranch(ctx, root); err == nil {
			return dir + "/" + branch
		}
	}

	return dir
}

// buildClaudeCommand constructs the claude CLI command with arguments.
func buildClaudeCommand(extraArgs []string, resume bool) []string {
	command := []string{"claude"}
	if resume {
		command = append(command, "--resume")
	}
	command = append(command, extraArgs...)
	return command
}
