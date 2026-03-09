package config

// ModelPricing holds per-model token pricing in USD per million tokens.
type ModelPricing struct {
	InputPerMTok  float64 `yaml:"input_per_mtok" mapstructure:"input_per_mtok"`
	OutputPerMTok float64 `yaml:"output_per_mtok" mapstructure:"output_per_mtok"`
}

// PricingTable maps provider -> model -> pricing and supports cost estimation.
type PricingTable struct {
	prices map[string]map[string]ModelPricing // provider -> model -> pricing
}

// NewPricingTable creates a PricingTable with sensible defaults for current
// Anthropic and OpenAI models.
func NewPricingTable() *PricingTable {
	return &PricingTable{
		prices: map[string]map[string]ModelPricing{
			"anthropic": {
				"claude-opus-4-20250514":           {InputPerMTok: 15.00, OutputPerMTok: 75.00},
				"claude-opus-4":                    {InputPerMTok: 15.00, OutputPerMTok: 75.00},
				"claude-sonnet-4-20250514":         {InputPerMTok: 3.00, OutputPerMTok: 15.00},
				"claude-sonnet-4":                  {InputPerMTok: 3.00, OutputPerMTok: 15.00},
				"claude-3-5-sonnet-20241022":       {InputPerMTok: 3.00, OutputPerMTok: 15.00},
				"claude-3-5-haiku-20241022":        {InputPerMTok: 0.80, OutputPerMTok: 4.00},
				"claude-3-opus-20240229":           {InputPerMTok: 15.00, OutputPerMTok: 75.00},
				"claude-3-haiku-20240307":          {InputPerMTok: 0.25, OutputPerMTok: 1.25},
			},
			"openai": {
				"gpt-4o":      {InputPerMTok: 2.50, OutputPerMTok: 10.00},
				"gpt-4o-mini": {InputPerMTok: 0.15, OutputPerMTok: 0.60},
				"gpt-4-turbo": {InputPerMTok: 10.00, OutputPerMTok: 30.00},
				"gpt-4":       {InputPerMTok: 30.00, OutputPerMTok: 60.00},
				"gpt-3.5-turbo": {InputPerMTok: 0.50, OutputPerMTok: 1.50},
				"o1":          {InputPerMTok: 15.00, OutputPerMTok: 60.00},
				"o1-mini":     {InputPerMTok: 3.00, OutputPerMTok: 12.00},
				"o3-mini":     {InputPerMTok: 1.10, OutputPerMTok: 4.40},
			},
		},
	}
}

// EstimateCost returns the estimated cost in USD for the given provider, model,
// and token counts. Returns 0 for unknown providers or models (not an error).
func (p *PricingTable) EstimateCost(provider, model string, inputTokens, outputTokens int64) float64 {
	if p == nil {
		return 0
	}
	models, ok := p.prices[provider]
	if !ok {
		return 0
	}
	pricing, ok := models[model]
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) / 1_000_000 * pricing.InputPerMTok
	outputCost := float64(outputTokens) / 1_000_000 * pricing.OutputPerMTok
	return inputCost + outputCost
}

// SetPricing sets the pricing for a specific provider and model.
// This allows users to override defaults or add new models.
func (p *PricingTable) SetPricing(provider, model string, pricing ModelPricing) {
	if p.prices == nil {
		p.prices = make(map[string]map[string]ModelPricing)
	}
	if _, ok := p.prices[provider]; !ok {
		p.prices[provider] = make(map[string]ModelPricing)
	}
	p.prices[provider][model] = pricing
}

// PricingConfig represents the pricing section in the YAML config file.
// It maps provider names to their model pricing.
type PricingConfig map[string]map[string]ModelPricing

// NewPricingTableFromConfig creates a PricingTable starting with defaults,
// then overlaying any user-provided pricing from the config.
func NewPricingTableFromConfig(cfg PricingConfig) *PricingTable {
	pt := NewPricingTable()
	for provider, models := range cfg {
		for model, pricing := range models {
			pt.SetPricing(provider, model, pricing)
		}
	}
	return pt
}
