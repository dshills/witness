package config

// modelPricingEntry stores cost per million tokens.
type modelPricingEntry struct {
	InputPerMToken  float64
	OutputPerMToken float64
}

// builtinPricing maps "provider/model" to pricing.
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

// EstimateCost returns the estimated cost in USD for a model request.
// Returns 0 if the provider/model combination is not found.
func EstimateCost(provider, model string, inputTokens, outputTokens, cachedTokens int64) float64 {
	key := provider + "/" + model
	pricing, ok := builtinPricing[key]
	if !ok {
		return 0
	}

	billableInput := inputTokens - cachedTokens
	if billableInput < 0 {
		billableInput = 0
	}

	cost := float64(billableInput)/1_000_000*pricing.InputPerMToken +
		float64(outputTokens)/1_000_000*pricing.OutputPerMToken

	return cost
}

// EstimateCostWithConfig checks user-configured pricing first, then falls back to built-in.
func EstimateCostWithConfig(cfg PricingConfig, provider, model string, inputTokens, outputTokens, cachedTokens int64) float64 {
	for _, mp := range cfg.Models {
		if mp.Provider == provider && mp.Model == model {
			billableInput := inputTokens - cachedTokens
			if billableInput < 0 {
				billableInput = 0
			}
			return float64(billableInput)/1_000_000*mp.InputPerMToken +
				float64(outputTokens)/1_000_000*mp.OutputPerMToken
		}
	}
	return EstimateCost(provider, model, inputTokens, outputTokens, cachedTokens)
}
