package protocol

import "encoding/json"

// This file declares tool-related types for WU-041. ToolDefinition is
// moved here from messages.go (same package, bare-name reference).

// ToolDefinition is a harness-registered tool catalog entry. Full schema
// details live in FEAT-0008 "Tool Catalog Schema"; extension types (e.g.,
// server-side tool routing) land in WU-041. InputSchema is a
// json.RawMessage so JSON Schema payloads pass through without interpretation.
type ToolDefinition struct {
	Name                 string          `json:"name"`
	Namespace            string          `json:"namespace"`
	Description          string          `json:"description"`
	InputSchema          json.RawMessage `json:"input_schema"`
	OutputEnvelope       string          `json:"output_envelope"`
	RiskLevel            string          `json:"risk_level"`
	CapabilitiesRequired []string        `json:"capabilities_required,omitempty"`
}

// ToolCatalog is a convenience wrapper for a full tool catalog snapshot.
type ToolCatalog struct {
	Tools []ToolDefinition `json:"tools"`
}

// CapabilitiesRegisterResponse is the response to capabilities.register.
type CapabilitiesRegisterResponse struct {
	Registered         []ToolDefinition   `json:"registered"`
	ServerCapabilities ServerCapabilities `json:"server_capabilities"`
	Rejected           []RejectedTool     `json:"rejected,omitempty"`
}

// RejectedTool is nested in CapabilitiesRegisterResponse.Rejected.
type RejectedTool struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
