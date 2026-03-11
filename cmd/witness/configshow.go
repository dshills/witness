package main

import (
	"fmt"

	"github.com/dshills/witness/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Display the effective configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}

	configCmd.AddCommand(showCmd)
	return configCmd
}
