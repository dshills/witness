package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level Witness configuration.
type Config struct {
	Storage StorageConfig `yaml:"storage"`
	UI      UIConfig      `yaml:"ui"`
	Alerts  AlertsConfig  `yaml:"alerts"`
	Files   FilesConfig   `yaml:"files"`
	Privacy PrivacyConfig `yaml:"privacy"`
	Git     GitConfig     `yaml:"git"`
	Pricing PricingConfig `yaml:"pricing"`
	Capture CaptureConfig `yaml:"capture"`
}

// StorageConfig controls where Witness persists data.
type StorageConfig struct {
	Root string `yaml:"root"`
}

// UIConfig controls terminal UI behavior.
type UIConfig struct {
	RefreshMS int    `yaml:"refresh_ms"`
	Theme     string `yaml:"theme"`
}

// AlertsConfig controls alert thresholds.
type AlertsConfig struct {
	StallDuration   time.Duration `yaml:"stall_duration"`
	LoopWindow      int           `yaml:"loop_window"`
	MaxRunCostUSD   float64       `yaml:"max_run_cost_usd"`
	MaxStageCostUSD float64       `yaml:"max_stage_cost_usd"`
	MaxTokens       int64         `yaml:"max_tokens,omitempty"`
}

// FilesConfig controls file system observation.
type FilesConfig struct {
	Ignore  []string `yaml:"ignore"`
	Include []string `yaml:"include,omitempty"`
}

// PrivacyConfig controls redaction behavior.
type PrivacyConfig struct {
	RedactPatterns []string `yaml:"redact_patterns"`
}

// GitConfig controls Git integration.
type GitConfig struct {
	PollIntervalSeconds int `yaml:"poll_interval_seconds"`
}

// PricingConfig allows user-override of model pricing.
type PricingConfig struct {
	Models []ModelPricing `yaml:"models,omitempty"`
}

// ModelPricing defines cost per million tokens for a provider/model pair.
type ModelPricing struct {
	Provider        string  `yaml:"provider"`
	Model           string  `yaml:"model"`
	InputPerMToken  float64 `yaml:"input_per_m_token"`
	OutputPerMToken float64 `yaml:"output_per_m_token"`
}

// CaptureConfig controls subprocess capture behavior.
type CaptureConfig struct {
	MaxOutputLines int `yaml:"max_output_lines"`
}

// Load reads a YAML config file and merges it over defaults.
// If the file does not exist, defaults are returned.
func Load(path string) (*Config, error) {
	cfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ApplyEnv overrides config values from WITNESS_* environment variables.
func ApplyEnv(cfg *Config) {
	if v := os.Getenv("WITNESS_STORAGE_ROOT"); v != "" {
		cfg.Storage.Root = v
	}
	if v := os.Getenv("WITNESS_UI_REFRESH_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.UI.RefreshMS = n
		} else {
			fmt.Fprintf(os.Stderr, "warning: WITNESS_UI_REFRESH_MS=%q is not a valid integer, ignored\n", v)
		}
	}
	if v := os.Getenv("WITNESS_STALL_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Alerts.StallDuration = d
		} else {
			fmt.Fprintf(os.Stderr, "warning: WITNESS_STALL_DURATION=%q is not a valid duration, ignored\n", v)
		}
	}
	if v := os.Getenv("WITNESS_MAX_RUN_COST_USD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Alerts.MaxRunCostUSD = f
		} else {
			fmt.Fprintf(os.Stderr, "warning: WITNESS_MAX_RUN_COST_USD=%q is not a valid number, ignored\n", v)
		}
	}
}

// ApplyAgenticDefaults adjusts config thresholds for agentic coding sessions
// (e.g., Claude Code) where longer thinking times and higher costs are normal.
func ApplyAgenticDefaults(cfg *Config) {
	// Agents think longer — stall at 30min not 10min.
	if cfg.Alerts.StallDuration <= 10*time.Minute {
		cfg.Alerts.StallDuration = 30 * time.Minute
	}
	// Agentic sessions use more tokens and cost more.
	if cfg.Alerts.MaxRunCostUSD <= 25.00 {
		cfg.Alerts.MaxRunCostUSD = 100.00
	}
	if cfg.Alerts.MaxStageCostUSD <= 8.00 {
		cfg.Alerts.MaxStageCostUSD = 25.00
	}
	// Wider loop window — agents legitimately retry more.
	if cfg.Alerts.LoopWindow <= 8 {
		cfg.Alerts.LoopWindow = 15
	}
}

// Validate checks the config for obviously bad values.
func (c *Config) Validate() error {
	if c.Alerts.StallDuration > 0 && c.Alerts.StallDuration < time.Second {
		return fmt.Errorf("config: stall_duration %v is suspiciously small (< 1s)", c.Alerts.StallDuration)
	}
	return nil
}
