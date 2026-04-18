package provider

import (
	"errors"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestProvider_FormatMessages_Anthropic_EmptyMessages(t *testing.T) {
	p := NewAnthropicProvider()
	got, err := p.FormatMessages(FormatMessagesOpts{})
	if got != nil {
		t.Errorf("FormatMessages() returned non-nil bytes: %s", got)
	}
	if !errors.Is(err, ErrEmptyMessages) {
		t.Errorf("FormatMessages() error = %v, want ErrEmptyMessages", err)
	}
}

func TestProvider_FormatMessages_Stub_OpenAI(t *testing.T) {
	p := NewOpenAIProvider()
	got, err := p.FormatMessages(FormatMessagesOpts{})
	if got != nil {
		t.Errorf("FormatMessages() returned non-nil bytes: %s", got)
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FormatMessages() error = %v, want ErrNotImplemented", err)
	}
}

func TestProvider_FormatToolDefinitions_Anthropic_Empty(t *testing.T) {
	p := NewAnthropicProvider()
	got, err := p.FormatToolDefinitions([]protocol.ToolDefinition{})
	if got != nil {
		t.Errorf("FormatToolDefinitions() returned non-nil bytes: %s", got)
	}
	if err != nil {
		t.Errorf("FormatToolDefinitions() error = %v, want nil", err)
	}
}

func TestProvider_FormatToolDefinitions_Stub_OpenAI(t *testing.T) {
	p := NewOpenAIProvider()
	got, err := p.FormatToolDefinitions([]protocol.ToolDefinition{})
	if got != nil {
		t.Errorf("FormatToolDefinitions() returned non-nil bytes: %s", got)
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FormatToolDefinitions() error = %v, want ErrNotImplemented", err)
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
