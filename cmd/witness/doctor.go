package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dshills/witness/internal/config"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check system health and configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}

			out := cmd.OutOrStdout()
			allOK := true

			// Check 1: Storage directory exists and is writable
			storageOK := true
			info, err := os.Stat(cfg.Storage.Root)
			if err != nil {
				storageOK = false
			} else if !info.IsDir() {
				storageOK = false
			} else {
				// Test writability by creating a temp file
				tmp := filepath.Join(cfg.Storage.Root, ".witness-doctor-check")
				if writeErr := os.WriteFile(tmp, []byte("ok"), 0o600); writeErr != nil {
					storageOK = false
				} else {
					_ = os.Remove(tmp)
				}
			}
			if storageOK {
				printCheck(out, "PASS", "Storage directory: %s", cfg.Storage.Root)
			} else {
				printCheck(out, "FAIL", "Storage directory: %s (not found or not writable)", cfg.Storage.Root)
				allOK = false
			}

			// Check 2: Git binary available
			gitPath, err := exec.LookPath("git")
			if err != nil {
				printCheck(out, "FAIL", "Git binary: not found in PATH")
				allOK = false
			} else {
				printCheck(out, "PASS", "Git binary: %s", gitPath)
			}

			// Check 3: Terminal color support
			noColor := os.Getenv("NO_COLOR")
			if noColor != "" {
				printCheck(out, "WARN", "Terminal colors: disabled (NO_COLOR is set)")
			} else {
				printCheck(out, "PASS", "Terminal colors: enabled")
			}

			// Check 4: Config file parseable
			// If we got this far, config was already loaded successfully
			printCheck(out, "PASS", "Config: loaded successfully")

			// Check 5: Count runs and disk usage
			runsDir := filepath.Join(cfg.Storage.Root, "runs")
			entries, err := os.ReadDir(runsDir)
			if err != nil {
				if os.IsNotExist(err) {
					printCheck(out, "PASS", "Runs: 0 runs (directory not yet created)")
				} else {
					printCheck(out, "FAIL", "Runs: cannot read runs directory: %v", err)
					allOK = false
				}
			} else {
				runCount := 0
				for _, e := range entries {
					if e.IsDir() {
						runCount++
					}
				}
				diskUsage := dirSize(runsDir)
				printCheck(out, "PASS", "Runs: %d runs, %s disk usage", runCount, formatBytes(diskUsage))
			}

			if !allOK {
				return fmt.Errorf("some checks failed")
			}
			return nil
		},
	}
}

// printCheck writes a formatted check line to the writer.
func printCheck(w io.Writer, level string, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "[%s] %s\n", level, fmt.Sprintf(format, args...))
}

// dirSize walks a directory tree and sums file sizes.
func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip errors
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// formatBytes returns a human-readable byte count.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	units := "KMGTPE"
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), units[exp])
}
