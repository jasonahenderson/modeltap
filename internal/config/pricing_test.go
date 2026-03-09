package config

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestEstimateCost_KnownModel(t *testing.T) {
	pt := NewPricingTable()

	// 1000 input tokens of claude-opus-4 at $15/MTok = $0.015
	cost := pt.EstimateCost("anthropic", "claude-opus-4", 1000, 0)
	expected := 0.015
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("EstimateCost(anthropic, claude-opus-4, 1000, 0) = %f, want %f", cost, expected)
	}

	// 1000 output tokens of claude-opus-4 at $75/MTok = $0.075
	cost = pt.EstimateCost("anthropic", "claude-opus-4", 0, 1000)
	expected = 0.075
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("EstimateCost(anthropic, claude-opus-4, 0, 1000) = %f, want %f", cost, expected)
	}

	// Both input and output: 1000 input + 500 output of claude-opus-4
	// = $0.015 + $0.0375 = $0.0525
	cost = pt.EstimateCost("anthropic", "claude-opus-4", 1000, 500)
	expected = 0.0525
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("EstimateCost(anthropic, claude-opus-4, 1000, 500) = %f, want %f", cost, expected)
	}
}

func TestEstimateCost_OpenAI(t *testing.T) {
	pt := NewPricingTable()

	// 2000 input tokens of gpt-4o at $2.50/MTok = $0.005
	cost := pt.EstimateCost("openai", "gpt-4o", 2000, 0)
	expected := 0.005
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("EstimateCost(openai, gpt-4o, 2000, 0) = %f, want %f", cost, expected)
	}

	// 1000 input + 1000 output of gpt-4o-mini
	// = $0.00015 + $0.0006 = $0.00075
	cost = pt.EstimateCost("openai", "gpt-4o-mini", 1000, 1000)
	expected = 0.00075
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("EstimateCost(openai, gpt-4o-mini, 1000, 1000) = %f, want %f", cost, expected)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	pt := NewPricingTable()

	cost := pt.EstimateCost("anthropic", "claude-unknown-model", 1000, 1000)
	if cost != 0 {
		t.Errorf("EstimateCost for unknown model = %f, want 0", cost)
	}
}

func TestEstimateCost_UnknownProvider(t *testing.T) {
	pt := NewPricingTable()

	cost := pt.EstimateCost("unknown-provider", "some-model", 1000, 1000)
	if cost != 0 {
		t.Errorf("EstimateCost for unknown provider = %f, want 0", cost)
	}
}

func TestEstimateCost_NilPricingTable(t *testing.T) {
	var pt *PricingTable
	cost := pt.EstimateCost("anthropic", "claude-opus-4", 1000, 1000)
	if cost != 0 {
		t.Errorf("EstimateCost on nil PricingTable = %f, want 0", cost)
	}
}

func TestEstimateCost_ZeroTokens(t *testing.T) {
	pt := NewPricingTable()

	cost := pt.EstimateCost("anthropic", "claude-opus-4", 0, 0)
	if cost != 0 {
		t.Errorf("EstimateCost with zero tokens = %f, want 0", cost)
	}
}

func TestDefaultPricingTable_HasCurrentModels(t *testing.T) {
	pt := NewPricingTable()

	// Check that default pricing table has entries for key models.
	models := []struct {
		provider string
		model    string
	}{
		{"anthropic", "claude-opus-4"},
		{"anthropic", "claude-sonnet-4"},
		{"anthropic", "claude-sonnet-4-20250514"},
		{"anthropic", "claude-3-5-sonnet-20241022"},
		{"openai", "gpt-4o"},
		{"openai", "gpt-4o-mini"},
		{"openai", "gpt-4"},
		{"openai", "o1"},
		{"openai", "o3-mini"},
	}

	for _, m := range models {
		t.Run(m.provider+"/"+m.model, func(t *testing.T) {
			cost := pt.EstimateCost(m.provider, m.model, 1_000_000, 0)
			if cost <= 0 {
				t.Errorf("expected positive cost for %s/%s, got %f", m.provider, m.model, cost)
			}
		})
	}
}

func TestPricingTableFromConfig_OverridesDefaults(t *testing.T) {
	cfg := PricingConfig{
		"anthropic": {
			"claude-opus-4": {InputPerMTok: 20.00, OutputPerMTok: 100.00},
		},
	}

	pt := NewPricingTableFromConfig(cfg)

	// Overridden model should use new pricing.
	cost := pt.EstimateCost("anthropic", "claude-opus-4", 1_000_000, 0)
	expected := 20.00
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("overridden cost = %f, want %f", cost, expected)
	}

	// Non-overridden model should still use defaults.
	cost = pt.EstimateCost("openai", "gpt-4o", 1_000_000, 0)
	expected = 2.50
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("default cost = %f, want %f", cost, expected)
	}
}

func TestPricingTableFromConfig_AddsNewModels(t *testing.T) {
	cfg := PricingConfig{
		"custom-provider": {
			"custom-model": {InputPerMTok: 5.00, OutputPerMTok: 25.00},
		},
	}

	pt := NewPricingTableFromConfig(cfg)

	cost := pt.EstimateCost("custom-provider", "custom-model", 1_000_000, 0)
	expected := 5.00
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("custom model cost = %f, want %f", cost, expected)
	}
}

func TestSetPricing(t *testing.T) {
	pt := NewPricingTable()

	pt.SetPricing("new-provider", "new-model", ModelPricing{
		InputPerMTok:  10.00,
		OutputPerMTok: 50.00,
	})

	cost := pt.EstimateCost("new-provider", "new-model", 1000, 1000)
	expected := 0.01 + 0.05
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("SetPricing cost = %f, want %f", cost, expected)
	}
}

func TestLoadPricingFromConfigFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	content := []byte(`port: 8080
pricing:
  anthropic:
    claude-opus-4:
      input_per_mtok: 20.00
      output_per_mtok: 100.00
  custom:
    my-model:
      input_per_mtok: 1.00
      output_per_mtok: 5.00
`)
	if err := os.WriteFile(configFile, content, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Pricing == nil {
		t.Fatal("Pricing config should not be nil")
	}

	pt := NewPricingTableFromConfig(cfg.Pricing)

	// Check overridden pricing.
	cost := pt.EstimateCost("anthropic", "claude-opus-4", 1_000_000, 0)
	expected := 20.00
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("overridden claude-opus-4 cost = %f, want %f", cost, expected)
	}

	// Check custom model from config.
	cost = pt.EstimateCost("custom", "my-model", 1_000_000, 0)
	expected = 1.00
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("custom model cost = %f, want %f", cost, expected)
	}

	// Check that non-overridden defaults are preserved.
	cost = pt.EstimateCost("openai", "gpt-4o", 1_000_000, 0)
	expected = 2.50
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("default gpt-4o cost = %f, want %f", cost, expected)
	}
}
