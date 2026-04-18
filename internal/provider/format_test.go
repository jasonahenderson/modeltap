package provider

import (
	"errors"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestProvider_FormatMessages_Stub_Anthropic(t *testing.T) {
	p := NewAnthropicProvider()
	got, err := p.FormatMessages(FormatMessagesOpts{})
	if got != nil {
		t.Errorf("FormatMessages() returned non-nil bytes: %s", got)
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FormatMessages() error = %v, want ErrNotImplemented", err)
	}
}

func TestProvider_FormatMessages_OpenAI_EmptyOpts(t *testing.T) {
	p := NewOpenAIProvider()
	_, err := p.FormatMessages(FormatMessagesOpts{})
	// With full implementation, empty messages returns ErrEmptyMessages.
	if !errors.Is(err, ErrEmptyMessages) {
		t.Errorf("FormatMessages() error = %v, want ErrEmptyMessages", err)
	}
}

func TestProvider_FormatToolDefinitions_Stub_Anthropic(t *testing.T) {
	p := NewAnthropicProvider()
	got, err := p.FormatToolDefinitions([]protocol.ToolDefinition{})
	if got != nil {
		t.Errorf("FormatToolDefinitions() returned non-nil bytes: %s", got)
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FormatToolDefinitions() error = %v, want ErrNotImplemented", err)
	}
}

func TestProvider_FormatToolDefinitions_OpenAI_Empty(t *testing.T) {
	p := NewOpenAIProvider()
	got, err := p.FormatToolDefinitions([]protocol.ToolDefinition{})
	if err != nil {
		t.Errorf("FormatToolDefinitions() error = %v, want nil", err)
	}
	// Empty tools should produce a valid JSON array "[]" or "null".
	if got == nil {
		t.Error("FormatToolDefinitions() returned nil bytes for empty tools")
	}
}

func TestErrorSentinels_Distinct(t *testing.T) {
	sentinels := []error{
		ErrNotImplemented,
		ErrWindowTooSmall,
		ErrEmptyMessages,
		ErrTruncationEmpty,
		ErrInvalidToolInput,
		ErrUnsupportedOutputType,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) should not match sentinel %d (%v)", i, a, j, b)
			}
		}
	}
}
