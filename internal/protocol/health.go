package protocol

// This file declares health and capability response types for WU-041.

// HealthResponse is the response to connection.health.
type HealthResponse struct {
	ServerVersion   string                    `json:"server_version"`
	ProtocolVersion string                    `json:"protocol_version"`
	UptimeSeconds   int                       `json:"uptime_seconds"`
	Auth            DependencyStatus          `json:"auth"`
	Storage         DependencyStatus          `json:"storage"`
	Capabilities    DependencyStatus          `json:"capabilities"`
	Providers       map[string]ProviderStatus `json:"providers"`
	Routing         DependencyStatus          `json:"routing"`
	ActiveSession   *ActiveSessionInfo        `json:"active_session,omitempty"`
}

// ReadyResponse is the response to connection.ready.
type ReadyResponse struct {
	Ready bool `json:"ready"`
}

// DependencyStatus describes the status of a server dependency
// (auth, storage, capabilities, routing).
type DependencyStatus struct {
	Status string `json:"status"`
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ProviderStatus describes the status of an AI provider.
type ProviderStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Models int    `json:"models,omitempty"`
}

// ActiveSessionInfo is nested in HealthResponse.
type ActiveSessionInfo struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
}

// ServerCapabilities describes the server's protocol capabilities,
// returned in CapabilitiesRegisterResponse.
type ServerCapabilities struct {
	ProtocolVersion      string   `json:"protocol_version"`
	ProtocolVersionRange string   `json:"protocol_version_range,omitempty"`
	SupportedTransforms  []string `json:"supported_transforms,omitempty"`
	MaxFrameSize         int      `json:"max_frame_size"`
	MaxAttachmentSize    int      `json:"max_attachment_size"`
}
