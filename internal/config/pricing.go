package config

import (
	"log"
	"strings"
	"sync"
)

// modelPricingEntry stores cost per million tokens.
type modelPricingEntry struct {
	InputPerMToken  float64
	OutputPerMToken float64
}

// builtinPricing maps "provider/model" (lowercased) to pricing.
var builtinPricing = map[string]modelPricingEntry{
	// Anthropic
	"anthropic/claude-opus-4-6":   {InputPerMToken: 15.00, OutputPerMToken: 75.00},
	"anthropic/claude-sonnet-4-6": {InputPerMToken: 3.00, OutputPerMToken: 15.00},
	"anthropic/claude-haiku-4-5":  {InputPerMToken: 0.80, OutputPerMToken: 4.00},
	// OpenAI
	"openai/gpt-4o":      {InputPerMToken: 2.50, OutputPerMToken: 10.00},
	"openai/gpt-4o-mini": {InputPerMToken: 0.15, OutputPerMToken: 0.60},
	"openai/o1":          {InputPerMToken: 15.00, OutputPerMToken: 60.00},
	"openai/o3":          {InputPerMToken: 10.00, OutputPerMToken: 40.00},
	"openai/o3-mini":     {InputPerMToken: 1.10, OutputPerMToken: 4.40},
	"openai/o4-mini":     {InputPerMToken: 1.10, OutputPerMToken: 4.40},
}

// unknownModelWarnings tracks which unknown models have already been warned
// about to avoid spamming logs.
var (
	unknownMu     sync.Mutex
	unknownModels = make(map[string]struct{})
)

// EstimateCost returns the estimated cost in USD for a model request.
// Uses case-insensitive matching. Returns 0 if the provider/model
// combination is not found, and logs a warning once per unknown model.
func EstimateCost(provider, model string, inputTokens, outputTokens, cachedTokens int64) float64 {
	key := strings.ToLower(provider) + "/" + strings.ToLower(model)
	pricing, ok := builtinPricing[key]
	if !ok {
		warnUnknownModel(key)
		return 0
	}

	return computeCost(pricing, inputTokens, outputTokens, cachedTokens)
}

// EstimateCostWithConfig checks user-configured pricing first, then falls back to built-in.
// Uses case-insensitive matching. Returns 0 for unknown models with a warning logged once.
func EstimateCostWithConfig(cfg PricingConfig, provider, model string, inputTokens, outputTokens, cachedTokens int64) float64 {
	key := strings.ToLower(provider) + "/" + strings.ToLower(model)

	for _, mp := range cfg.Models {
		mpKey := strings.ToLower(mp.Provider) + "/" + strings.ToLower(mp.Model)
		if mpKey == key {
			return computeCost(modelPricingEntry{
				InputPerMToken:  mp.InputPerMToken,
				OutputPerMToken: mp.OutputPerMToken,
			}, inputTokens, outputTokens, cachedTokens)
		}
	}
	return EstimateCost(provider, model, inputTokens, outputTokens, cachedTokens)
}

func computeCost(pricing modelPricingEntry, inputTokens, outputTokens, cachedTokens int64) float64 {
	billableInput := inputTokens - cachedTokens
	if billableInput < 0 {
		billableInput = 0
	}

	return float64(billableInput)/1_000_000*pricing.InputPerMToken +
		float64(outputTokens)/1_000_000*pricing.OutputPerMToken
}

func warnUnknownModel(key string) {
	unknownMu.Lock()
	defer unknownMu.Unlock()
	if _, warned := unknownModels[key]; !warned {
		unknownModels[key] = struct{}{}
		log.Printf("pricing: unknown model %q, cost will be reported as 0", key)
	}
}

// ResetUnknownModelWarnings clears the warning dedup map. Intended for testing.
func ResetUnknownModelWarnings() {
	unknownMu.Lock()
	unknownModels = make(map[string]struct{})
	unknownMu.Unlock()
}
