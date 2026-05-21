package runtime

import (
	"encoding/json"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// NewDiagnosticError builds a TransportError with a structured
// MT-CONN-* diagnostic encoded into Data per FEAT-0008's diagnostic
// taxonomy. Use this from handlers when the wire-visible failure has a
// known diagnostic code so the harness can render the right banner /
// suggested-command UI.
//
// code is a JSON-RPC application code (Code* constants).
// diagCode is one of the MT-CONN-NNN constants in protocol/errors.go.
// category is a short label like "session", "provider", "model".
// cause is a human-readable explanation embedded in the diagnostic.
func NewDiagnosticError(code int, message string, diagCode protocol.DiagnosticCode, category, cause string) *TransportError {
	diag := protocol.Diagnostic{
		Code:     diagCode,
		Category: category,
		Cause:    cause,
	}
	raw, _ := json.Marshal(diag)
	return &TransportError{
		Code:    code,
		Message: message,
		Data:    json.RawMessage(raw),
	}
}

// WithSuggestedCommand stamps a suggested_command on an existing
// diagnostic-bearing TransportError. The harness surfaces this in the
// status bar / banner for the user to click. Returns the same error
// for convenient chaining.
func WithSuggestedCommand(err *TransportError, cmd string) *TransportError {
	if err == nil || err.Data == nil {
		return err
	}
	raw, ok := err.Data.(json.RawMessage)
	if !ok {
		return err
	}
	var diag protocol.Diagnostic
	if jerr := json.Unmarshal(raw, &diag); jerr != nil {
		return err
	}
	diag.SuggestedCommand = cmd
	updated, jerr := json.Marshal(diag)
	if jerr != nil {
		return err
	}
	err.Data = json.RawMessage(updated)
	return err
}

// DiagnosticOf inspects e and returns the embedded protocol.Diagnostic,
// if any. Returns (Diagnostic{}, false) when e is not a *TransportError
// or when its Data does not contain a diagnostic.
func DiagnosticOf(e error) (protocol.Diagnostic, bool) {
	te, ok := e.(*TransportError)
	if !ok || te == nil || te.Data == nil {
		return protocol.Diagnostic{}, false
	}
	raw, ok := te.Data.(json.RawMessage)
	if !ok {
		return protocol.Diagnostic{}, false
	}
	var diag protocol.Diagnostic
	if jerr := json.Unmarshal(raw, &diag); jerr != nil {
		return protocol.Diagnostic{}, false
	}
	return diag, diag.Code != ""
}
