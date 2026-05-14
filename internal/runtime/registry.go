package runtime

import (
	"sync"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// ModelSource identifies where a model entry originated. Used to
// resolve precedence during duplicate-name conflicts (design D3.5).
type ModelSource string

const (
	ModelSourceBuiltin    ModelSource = "builtin"
	ModelSourceDiscovered ModelSource = "discovered"
	ModelSourceManual     ModelSource = "manual"
)

// ModelEntry is one entry in the model registry. Info is the wire
// shape; Provider is the endpoint name that serves it; Source
// determines override precedence on refresh.
type ModelEntry struct {
	Info      protocol.ModelInfo
	Provider  string
	Source    ModelSource
	Available bool
}

// ModelRegistry aggregates the model catalog across all provider
// endpoints. It merges a built-in catalog, models auto-discovered from
// Ollama/MLX endpoints, and manual overrides declared in config.
// Safe for concurrent use.
type ModelRegistry struct {
	mu        sync.RWMutex
	models    map[string]*ModelEntry // keyed by canonical model name
	providers *ProviderRegistry

	builtins map[string][]protocol.ModelInfo // keyed by provider type
	manual   map[string]ModelOverrideConfig  // keyed by model name
}

// ModelOverrideConfig is the manual per-model override in config.
type ModelOverrideConfig struct {
	Provider      string
	ContextWindow int
	Capabilities  []string
	Description   string
}

// NewModelRegistry constructs a registry rooted at the given provider
// registry. Built-in catalogs are seeded from DefaultBuiltinModels.
func NewModelRegistry(providers *ProviderRegistry) *ModelRegistry {
	r := &ModelRegistry{
		models:    make(map[string]*ModelEntry),
		providers: providers,
		builtins:  DefaultBuiltinModels(),
		manual:    make(map[string]ModelOverrideConfig),
	}
	r.rebuild()
	return r
}

// SetManual replaces the manual override map.
func (r *ModelRegistry) SetManual(overrides map[string]ModelOverrideConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if overrides == nil {
		r.manual = make(map[string]ModelOverrideConfig)
	} else {
		r.manual = overrides
	}
	r.rebuildLocked()
}

// Refresh re-merges builtins, discovered models, and manual overrides
// against the current ProviderRegistry snapshot. Called after
// ProviderRegistry.CheckAll updates endpoint status.
func (r *ModelRegistry) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rebuildLocked()
}

// rebuild rebuilds the full catalog. Caller must NOT hold r.mu.
func (r *ModelRegistry) rebuild() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rebuildLocked()
}

// rebuildLocked rebuilds the full catalog. Caller MUST hold r.mu.
func (r *ModelRegistry) rebuildLocked() {
	r.models = make(map[string]*ModelEntry)
	if r.providers == nil {
		return
	}

	// Pass 1: built-in catalog. Only include an endpoint if one of that
	// type is configured. Use first-registered endpoint of the type.
	byType := map[string]*ProviderEndpoint{}
	for _, ep := range r.providers.All() {
		if _, ok := byType[ep.Type]; !ok {
			byType[ep.Type] = ep
		}
	}
	for pt, models := range r.builtins {
		ep := byType[pt]
		if ep == nil {
			continue // no endpoint configured for this provider type
		}
		for _, m := range models {
			entry := &ModelEntry{
				Info:      m,
				Provider:  ep.Name,
				Source:    ModelSourceBuiltin,
				Available: ep.Status() == ProviderStatusReady,
			}
			r.models[m.Name] = entry
		}
	}

	// Pass 2: discovered models. Override built-ins when names match.
	for _, ep := range r.providers.All() {
		if !ep.Discover {
			continue
		}
		for _, name := range ep.Models() {
			info := protocol.ModelInfo{
				Name:     name,
				Provider: ep.Name,
			}
			r.models[name] = &ModelEntry{
				Info:      info,
				Provider:  ep.Name,
				Source:    ModelSourceDiscovered,
				Available: ep.Status() == ProviderStatusReady,
			}
		}
	}

	// Pass 3: manual overrides. Highest precedence.
	for name, ov := range r.manual {
		ep := r.providers.Get(ov.Provider)
		avail := ep != nil && ep.Status() == ProviderStatusReady
		info := protocol.ModelInfo{
			Name:          name,
			Provider:      ov.Provider,
			ContextWindow: ov.ContextWindow,
			Capabilities:  ov.Capabilities,
			Description:   ov.Description,
		}
		r.models[name] = &ModelEntry{
			Info:      info,
			Provider:  ov.Provider,
			Source:    ModelSourceManual,
			Available: avail,
		}
	}
}

// Get returns the entry for a canonical model name, or nil when absent.
func (r *ModelRegistry) Get(name string) *ModelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.models[name]
}

// Has reports whether a model name is registered (regardless of
// availability).
func (r *ModelRegistry) Has(name string) bool {
	return r.Get(name) != nil
}

// All returns every model's wire-shape Info, sorted by name. Status
// is stamped from the entry's Available flag.
func (r *ModelRegistry) All() []protocol.ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]protocol.ModelInfo, 0, len(r.models))
	for _, e := range r.models {
		info := e.Info
		if e.Available {
			info.Status = "ready"
		} else {
			info.Status = "unavailable"
		}
		out = append(out, info)
	}
	sortModelInfoByName(out)
	return out
}

// ByProvider returns models routed to a specific endpoint name.
func (r *ModelRegistry) ByProvider(endpointName string) []protocol.ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]protocol.ModelInfo, 0)
	for _, e := range r.models {
		if e.Provider == endpointName {
			out = append(out, e.Info)
		}
	}
	sortModelInfoByName(out)
	return out
}

// sortModelInfoByName orders models deterministically. Avoids pulling
// in sort.Slice at call sites.
func sortModelInfoByName(models []protocol.ModelInfo) {
	for i := 1; i < len(models); i++ {
		for j := i; j > 0 && models[j-1].Name > models[j].Name; j-- {
			models[j-1], models[j] = models[j], models[j-1]
		}
	}
}

// DefaultBuiltinModels returns the v1 starter catalog. Keeping the list
// intentionally small — the cloud providers publish full catalogs that
// change rapidly and each entry requires manual pricing data.
func DefaultBuiltinModels() map[string][]protocol.ModelInfo {
	return map[string][]protocol.ModelInfo{
		ProviderTypeAnthropic: {
			{
				Name:            "claude-opus-4-6",
				Provider:        ProviderTypeAnthropic,
				ContextWindow:   1_000_000,
				CostPer1kInput:  0.015,
				CostPer1kOutput: 0.075,
				Capabilities:    []string{"tool_use", "vision"},
			},
			{
				Name:            "claude-sonnet-4-6",
				Provider:        ProviderTypeAnthropic,
				ContextWindow:   200_000,
				CostPer1kInput:  0.003,
				CostPer1kOutput: 0.015,
				Capabilities:    []string{"tool_use", "vision"},
			},
			{
				Name:            "claude-haiku-4-5",
				Provider:        ProviderTypeAnthropic,
				ContextWindow:   200_000,
				CostPer1kInput:  0.0008,
				CostPer1kOutput: 0.004,
				Capabilities:    []string{"tool_use"},
			},
		},
		ProviderTypeOpenAI: {
			{
				Name:            "gpt-5",
				Provider:        ProviderTypeOpenAI,
				ContextWindow:   256_000,
				CostPer1kInput:  0.005,
				CostPer1kOutput: 0.015,
				Capabilities:    []string{"tool_use", "vision"},
			},
			{
				Name:            "o4-mini",
				Provider:        ProviderTypeOpenAI,
				ContextWindow:   200_000,
				CostPer1kInput:  0.0011,
				CostPer1kOutput: 0.0044,
				Capabilities:    []string{"tool_use"},
			},
		},
	}
}
