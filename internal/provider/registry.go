package provider

import (
	"net/http"
	"sync"
)

// Registry maintains a collection of registered providers and supports
// detection of the appropriate provider for a given HTTP request.
type Registry struct {
	mu        sync.RWMutex
	providers []Provider
	byName    map[string]Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]Provider),
	}
}

// Register adds a provider to the registry. If a provider with the same name
// is already registered, it is replaced.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()

	// Replace existing provider with the same name.
	if _, exists := r.byName[name]; exists {
		for i, existing := range r.providers {
			if existing.Name() == name {
				r.providers[i] = p
				break
			}
		}
	} else {
		r.providers = append(r.providers, p)
	}

	r.byName[name] = p
}

// Detect iterates through registered providers and returns the first one
// whose Detect method returns true for the given request. Returns nil if
// no provider matches.
func (r *Registry) Detect(req *http.Request) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.providers {
		if p.Detect(req) {
			return p
		}
	}
	return nil
}

// Get returns the provider with the given name, or nil if not found.
func (r *Registry) Get(name string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.byName[name]
}

// All returns a copy of all registered providers in registration order.
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Provider, len(r.providers))
	copy(result, r.providers)
	return result
}
