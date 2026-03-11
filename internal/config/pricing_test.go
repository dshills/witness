package config

import "testing"

func TestEstimateCost_KnownModel(t *testing.T) {
	// claude-sonnet-4-6: $3/M input, $15/M output
	cost := EstimateCost("anthropic", "claude-sonnet-4-6", 1_000_000, 500_000, 0)
	expected := 3.0 + 7.5 // $3 input + $7.5 output
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("cost = %f, want ~%f", cost, expected)
	}
}

func TestEstimateCost_CachedTokens(t *testing.T) {
	// 1M input, 200k cached, 500k output
	cost := EstimateCost("anthropic", "claude-sonnet-4-6", 1_000_000, 500_000, 200_000)
	// Billable input: 800k → $2.40, output 500k → $7.50
	expected := 2.40 + 7.50
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("cost = %f, want ~%f", cost, expected)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	cost := EstimateCost("unknown", "model", 1_000_000, 1_000_000, 0)
	if cost != 0 {
		t.Errorf("unknown model should return 0, got %f", cost)
	}
}

func TestEstimateCostWithConfig_UserOverride(t *testing.T) {
	cfg := PricingConfig{
		Models: []ModelPricing{
			{Provider: "custom", Model: "llm-1", InputPerMToken: 1.0, OutputPerMToken: 2.0},
		},
	}
	cost := EstimateCostWithConfig(cfg, "custom", "llm-1", 1_000_000, 1_000_000, 0)
	expected := 1.0 + 2.0
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("cost = %f, want ~%f", cost, expected)
	}
}

func TestEstimateCostWithConfig_FallbackToBuiltin(t *testing.T) {
	cfg := PricingConfig{} // no user overrides
	cost := EstimateCostWithConfig(cfg, "anthropic", "claude-sonnet-4-6", 1_000_000, 0, 0)
	if cost == 0 {
		t.Error("should fall back to built-in pricing")
	}
}
