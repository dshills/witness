package config

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Storage: StorageConfig{
			Root: filepath.Join(home, ".witness"),
		},
		UI: UIConfig{
			RefreshMS: 500,
			Theme:     "auto",
		},
		Alerts: AlertsConfig{
			StallDuration:   10 * time.Minute,
			LoopWindow:      8,
			MaxRunCostUSD:   25.00,
			MaxStageCostUSD: 8.00,
		},
		Files: FilesConfig{
			Ignore: []string{
				".git/**",
				"node_modules/**",
				"vendor/**",
				"dist/**",
				"build/**",
				".next/**",
				"*.swp",
				"*.swo",
				"*~",
				".DS_Store",
			},
		},
		Privacy: PrivacyConfig{
			RedactPatterns: []string{
				`(?i)(sk-[a-zA-Z0-9]{20,}|AKIA[A-Z0-9]{16})`,
				`(?i)bearer\s+[A-Za-z0-9\-._~+/=]{20,}`,
				`(?i)(password|secret|apikey)\s*[=:]\s*\S{8,}`,
			},
		},
		Git: GitConfig{
			PollIntervalSeconds: 5,
		},
		Capture: CaptureConfig{
			MaxOutputLines: 200,
		},
	}
}
