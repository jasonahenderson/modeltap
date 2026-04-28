package bff

import (
	"errors"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestNewDiagnosticError_BuildsStructuredError(t *testing.T) {
	err := NewDiagnosticError(
		CodeProviderError,
		"upstream down",
		protocol.DiagProviderUnavailable,
		"provider",
		"connection refused",
	)
	if err.Code != CodeProviderError {
		t.Errorf("code = %d", err.Code)
	}
	diag, ok := DiagnosticOf(err)
	if !ok {
		t.Fatalf("DiagnosticOf returned !ok")
	}
	if diag.Code != protocol.DiagProviderUnavailable || diag.Category != "provider" || diag.Cause != "connection refused" {
		t.Errorf("diagnostic = %+v", diag)
	}
}

func TestWithSuggestedCommand(t *testing.T) {
	err := NewDiagnosticError(CodeSessionLocked, "locked", protocol.DiagSessionLocked, "session", "owner=other")
	if got := WithSuggestedCommand(err, "modeltap session unlock <id>"); got != err {
		t.Fatalf("WithSuggestedCommand returned %p, want %p", got, err)
	}
	diag, _ := DiagnosticOf(err)
	if diag.SuggestedCommand != "modeltap session unlock <id>" {
		t.Errorf("suggested_command = %q", diag.SuggestedCommand)
	}
}

func TestDiagnosticOf_NotAnRPCError(t *testing.T) {
	if _, ok := DiagnosticOf(errors.New("plain")); ok {
		t.Errorf("expected !ok for plain error")
	}
}

func TestDiagnosticOf_NoDiagnosticData(t *testing.T) {
	te := &TransportError{Code: CodeInternalError, Message: "boom"}
	if _, ok := DiagnosticOf(te); ok {
		t.Errorf("expected !ok for error without Data")
	}
}
