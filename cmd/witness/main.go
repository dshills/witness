package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dshills/witness/internal/config"
	"github.com/dshills/witness/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "witness",
		Short: "Terminal-first observability for AI development workflows",
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.witness/config.yaml)")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		// Skip config loading for version command
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("witness " + version.String())
		},
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newRunsCmd())
	rootCmd.AddCommand(newInspectCmd())
	rootCmd.AddCommand(newStatsCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newReplayCmd())
	rootCmd.AddCommand(newWatchCmd())
	rootCmd.AddCommand(newAttachCmd())
	rootCmd.AddCommand(newWrapCmd())
	rootCmd.AddCommand(newClaudeCmd())

	// Pre-run: load config (available to all subcommands via closure)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if cfgFile == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			cfgFile = filepath.Join(home, ".witness", "config.yaml")
		}
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		config.ApplyEnv(cfg)
		// Store config on the executing command so subcommands see it
		cmd.SetContext(config.WithConfig(cmd.Context(), cfg))
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
