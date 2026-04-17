package protocol

import "encoding/json"

// This file declares model-related response and payload types for
// WU-041.

// ModelInfo describes a model in the model catalog.
type ModelInfo struct {
	Name            string   `json:"name"`
	Provider        string   `json:"provider"`
	Roles           []string `json:"roles"`
	Capabilities    []string `json:"capabilities"`
	ContextWindow   int      `json:"context_window"`
	CostPer1kInput  float64  `json:"cost_per_1k_input"`
	CostPer1kOutput float64  `json:"cost_per_1k_output"`
	Description     string   `json:"description"`
	Status          string   `json:"status,omitempty"`
	Access          string   `json:"access,omitempty"`
}

// ModelListResponse is the response to model.list.
type ModelListResponse struct {
	Models          []ModelInfo    `json:"models"`
	CurrentOverride string         `json:"current_override,omitempty"`
	RoutingPolicy   RoutingPolicy  `json:"routing_policy"`
}

// RoutingPolicy maps dot-path role names to model names or arrays.
// Represented as map[string]json.RawMessage to allow string-or-array
// values. Resolution logic belongs in the handler layer (WU-059).
type RoutingPolicy map[string]json.RawMessage

// ModelSwitchResponse is the response to model.switch.
type ModelSwitchResponse struct {
	OverrideSet bool   `json:"override_set"`
	Model       string `json:"model,omitempty"`
	Reason      string `json:"reason"`
}
