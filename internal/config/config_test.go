package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Storage.Root == "" {
		t.Error("Storage.Root should not be empty")
	}
	if cfg.UI.RefreshMS != 500 {
		t.Errorf("UI.RefreshMS = %d, want 500", cfg.UI.RefreshMS)
	}
	if cfg.Alerts.StallDuration != 10*time.Minute {
		t.Errorf("Alerts.StallDuration = %v, want 10m", cfg.Alerts.StallDuration)
	}
	if cfg.Alerts.LoopWindow != 8 {
		t.Errorf("Alerts.LoopWindow = %d, want 8", cfg.Alerts.LoopWindow)
	}
	if len(cfg.Files.Ignore) == 0 {
		t.Error("Files.Ignore should have default patterns")
	}
	if len(cfg.Privacy.RedactPatterns) == 0 {
		t.Error("Privacy.RedactPatterns should have default patterns")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Storage.Root == "" {
		t.Error("should return defaults when file is missing")
	}
}

func TestLoad_YAMLOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
storage:
  root: /custom/path
ui:
  refresh_ms: 250
alerts:
  stall_duration: 5m
  loop_window: 12
  max_run_cost_usd: 50.0
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Storage.Root != "/custom/path" {
		t.Errorf("Storage.Root = %q, want /custom/path", cfg.Storage.Root)
	}
	if cfg.UI.RefreshMS != 250 {
		t.Errorf("UI.RefreshMS = %d, want 250", cfg.UI.RefreshMS)
	}
	if cfg.Alerts.StallDuration != 5*time.Minute {
		t.Errorf("Alerts.StallDuration = %v, want 5m", cfg.Alerts.StallDuration)
	}
	if cfg.Alerts.LoopWindow != 12 {
		t.Errorf("Alerts.LoopWindow = %d, want 12", cfg.Alerts.LoopWindow)
	}
	if cfg.Alerts.MaxRunCostUSD != 50.0 {
		t.Errorf("Alerts.MaxRunCostUSD = %f, want 50.0", cfg.Alerts.MaxRunCostUSD)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`{{{invalid`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_SuspiciouslySmallDuration(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
alerts:
  stall_duration: 500ms
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for suspiciously small stall_duration")
	}
}

func TestApplyEnv(t *testing.T) {
	cfg := DefaultConfig()

	t.Setenv("WITNESS_STORAGE_ROOT", "/env/path")
	t.Setenv("WITNESS_UI_REFRESH_MS", "100")
	t.Setenv("WITNESS_STALL_DURATION", "20m")
	t.Setenv("WITNESS_MAX_RUN_COST_USD", "99.50")

	ApplyEnv(&cfg)

	if cfg.Storage.Root != "/env/path" {
		t.Errorf("Storage.Root = %q, want /env/path", cfg.Storage.Root)
	}
	if cfg.UI.RefreshMS != 100 {
		t.Errorf("UI.RefreshMS = %d, want 100", cfg.UI.RefreshMS)
	}
	if cfg.Alerts.StallDuration != 20*time.Minute {
		t.Errorf("Alerts.StallDuration = %v, want 20m", cfg.Alerts.StallDuration)
	}
	if cfg.Alerts.MaxRunCostUSD != 99.50 {
		t.Errorf("Alerts.MaxRunCostUSD = %f, want 99.50", cfg.Alerts.MaxRunCostUSD)
	}
}
