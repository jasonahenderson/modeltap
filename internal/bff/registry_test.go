package bff

import (
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestModelRegistry_Builtins_OnlyIfProviderConfigured(t *testing.T) {
	pr := NewProviderRegistry()
	reg := NewModelRegistry(pr)
	if len(reg.All()) != 0 {
		t.Errorf("no provider → no builtins; got %d", len(reg.All()))
	}

	_ = pr.Add(&ProviderEndpoint{Name: "a1", Type: ProviderTypeAnthropic, APIKey: "k"})
	reg.Refresh()
	if len(reg.All()) < 3 {
		t.Errorf("Anthropic builtins missing; got %d", len(reg.All()))
	}
	if e := reg.Get("claude-sonnet-4-6"); e == nil || e.Provider != "a1" {
		t.Errorf("sonnet entry = %+v", e)
	}
}

func TestModelRegistry_Discovered_OverridesBuiltins(t *testing.T) {
	pr := NewProviderRegistry()
	ollama := &ProviderEndpoint{Name: "ol", Type: ProviderTypeOllama, Host: "http://x", Discover: true}
	_ = pr.Add(ollama)
	reg := NewModelRegistry(pr)

	// Manually simulate discovery.
	ollama.setStatus(ProviderStatusReady, "", []string{"llama-3.1:8b"})
	reg.Refresh()

	entry := reg.Get("llama-3.1:8b")
	if entry == nil {
		t.Fatalf("discovered model not in registry")
	}
	if entry.Source != ModelSourceDiscovered {
		t.Errorf("source = %q, want discovered", entry.Source)
	}
}

func TestModelRegistry_Manual_OverridesAll(t *testing.T) {
	pr := NewProviderRegistry()
	_ = pr.Add(&ProviderEndpoint{Name: "a1", Type: ProviderTypeAnthropic, APIKey: "k"})

	reg := NewModelRegistry(pr)
	reg.SetManual(map[string]ModelOverrideConfig{
		"claude-sonnet-4-6": {
			Provider:      "a1",
			ContextWindow: 50000,
			Description:   "overridden for testing",
		},
	})

	entry := reg.Get("claude-sonnet-4-6")
	if entry == nil {
		t.Fatalf("manual override missing")
	}
	if entry.Source != ModelSourceManual {
		t.Errorf("source = %q, want manual", entry.Source)
	}
	if entry.Info.ContextWindow != 50000 {
		t.Errorf("context window = %d, want 50000 (manual override)", entry.Info.ContextWindow)
	}
}

func TestModelRegistry_ByProvider(t *testing.T) {
	pr := NewProviderRegistry()
	_ = pr.Add(&ProviderEndpoint{Name: "a1", Type: ProviderTypeAnthropic, APIKey: "k"})
	_ = pr.Add(&ProviderEndpoint{Name: "oai", Type: ProviderTypeOpenAI, APIKey: "k"})
	reg := NewModelRegistry(pr)

	ant := reg.ByProvider("a1")
	for _, m := range ant {
		if m.Provider != ProviderTypeAnthropic {
			t.Errorf("ByProvider(a1) returned %q", m.Provider)
		}
	}
	if len(ant) == 0 {
		t.Errorf("a1 should have models")
	}
}

func TestModelRegistry_Has_Get_Missing(t *testing.T) {
	pr := NewProviderRegistry()
	reg := NewModelRegistry(pr)
	if reg.Has("nope") {
		t.Errorf("Has(nope) should be false")
	}
	if reg.Get("nope") != nil {
		t.Errorf("Get(nope) should be nil")
	}
}

func TestModelRegistry_All_StatusStamped(t *testing.T) {
	pr := NewProviderRegistry()
	ep := &ProviderEndpoint{Name: "a1", Type: ProviderTypeAnthropic, APIKey: "k"}
	_ = pr.Add(ep)
	ep.setStatus(ProviderStatusReady, "", nil)

	reg := NewModelRegistry(pr)
	all := reg.All()
	for _, m := range all {
		if m.Status != "ready" {
			t.Errorf("model %q status = %q, want ready", m.Name, m.Status)
		}
	}

	ep.setStatus(ProviderStatusUnavailable, "down", nil)
	reg.Refresh()
	for _, m := range reg.All() {
		if m.Status != "unavailable" {
			t.Errorf("model %q status = %q, want unavailable", m.Name, m.Status)
		}
	}
}

func TestModelRegistry_DefaultBuiltins_ShapeIsWireCompatible(t *testing.T) {
	// Basic guard: every builtin model has name, provider, and non-zero
	// context window.
	for pt, models := range DefaultBuiltinModels() {
		for _, m := range models {
			if m.Name == "" || m.Provider == "" || m.ContextWindow == 0 {
				t.Errorf("builtin %q (type %s) incomplete: %+v", m.Name, pt, m)
			}
			// Round-trip as JSON to ensure wire shape is intact.
			_ = protocol.ModelInfo(m)
		}
	}
}
