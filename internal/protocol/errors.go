package protocol

// DiagnosticCode is a typed string representing an MT-CONN-* diagnostic
// code from FEAT-0008 §"Diagnostic Taxonomy".
type DiagnosticCode string

const (
	DiagServiceNotRunning            DiagnosticCode = "MT-CONN-001"
	DiagStaleSocket                  DiagnosticCode = "MT-CONN-002"
	DiagSocketPermission             DiagnosticCode = "MT-CONN-003"
	DiagVersionMismatch              DiagnosticCode = "MT-CONN-004"
	DiagTLSUntrusted                 DiagnosticCode = "MT-CONN-005"
	DiagAuthExpired                  DiagnosticCode = "MT-CONN-006"
	DiagStorageUnready               DiagnosticCode = "MT-CONN-007"
	DiagSessionLocked                DiagnosticCode = "MT-CONN-008"
	DiagProviderUnavailable          DiagnosticCode = "MT-CONN-009"
	DiagCapabilityRegistrationFailed DiagnosticCode = "MT-CONN-010"
	DiagModelUnavailable             DiagnosticCode = "MT-CONN-011"
	DiagHeartbeatTimeout             DiagnosticCode = "MT-CONN-012"
)

// Diagnostic carries structured error details in ErrorObject.Data or
// inline in ServerError (events.go). The code field references a
// DiagnosticCode constant; category/cause provide human-readable context.
type Diagnostic struct {
	Code                DiagnosticCode `json:"code"`
	Category            string         `json:"category"`
	Cause               string         `json:"cause"`
	AutoRepairAttempted bool           `json:"auto_repair_attempted"`
	RepairResult        string         `json:"repair_result,omitempty"`
	SuggestedCommand    string         `json:"suggested_command,omitempty"`
	PathOrEndpoint      string         `json:"path_or_endpoint,omitempty"`
}
