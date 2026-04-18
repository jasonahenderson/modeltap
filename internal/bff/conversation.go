package bff

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// Conversation maintains the ordered canonical message history for one
// active session. It is the in-memory mirror of storage.Turn rows; both
// are kept in sync by AppendUserTurn / AppendAssistantTurn (producers)
// and RestoreFromTurns (consumer of persisted state on resume).
//
// Sequence numbers are 1-based and must be consecutive. Turn submissions
// whose sequence doesn't equal Sequence()+1 are rejected.
type Conversation struct {
	sessionID string

	mu       sync.RWMutex
	turns    []provider.Message
	sequence int
}

// NewConversation returns an empty Conversation for the given session.
func NewConversation(sessionID string) *Conversation {
	return &Conversation{sessionID: sessionID}
}

// SessionID returns the owning session's identifier.
func (c *Conversation) SessionID() string { return c.sessionID }

// TurnCount returns the number of canonical messages currently in the
// conversation.
func (c *Conversation) TurnCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.turns)
}

// Sequence returns the highest sequence number assigned so far. A fresh
// conversation returns 0; the next valid turn.submit must carry
// sequence == Sequence()+1.
func (c *Conversation) Sequence() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sequence
}

// Messages returns a defensive copy of the canonical message history.
// Callers may mutate the returned slice without affecting conversation
// state.
func (c *Conversation) Messages() []provider.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]provider.Message, len(c.turns))
	copy(out, c.turns)
	return out
}

// Reset clears the in-memory conversation and returns the number of
// turns that were cleared. Used by session.clear (design D2.6) to drop
// the active conversation while preserving persisted turns.
func (c *Conversation) Reset() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.turns)
	c.turns = nil
	c.sequence = 0
	return n
}

// TurnMetadata carries the provider-sourced fields that accompany a
// persisted turn but are not part of the canonical provider.Message.
type TurnMetadata struct {
	Model         string
	Provider      string
	InputTokens   int64
	OutputTokens  int64
	Cost          float64
	LatencyMs     int64
	FilesTouched  []string
	FilesModified []string
}

// AssistantResponse is the aggregated result of streaming a model turn.
// Consumed by AppendAssistantTurn on turn.complete (WU-053).
type AssistantResponse struct {
	Content      string
	ToolCalls    []provider.ToolCall
	Model        string
	Provider     string
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	LatencyMs    int64
}

// AppendUserTurn appends the user message from a turn.submit to the
// conversation. Validates that submit.Sequence is the next expected
// value. Returns the storage.Turn ready for persistence by the caller.
func (c *Conversation) AppendUserTurn(submit *protocol.TurnSubmit) (*storage.Turn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if submit.Sequence != c.sequence+1 {
		return nil, &TransportError{
			Code: CodeInvalidParams,
			Message: fmt.Sprintf("turn.submit sequence %d does not follow current %d",
				submit.Sequence, c.sequence),
		}
	}

	// Convert protocol attachments -> canonical provider attachments.
	atts := make([]provider.Attachment, 0, len(submit.Attachments))
	for _, a := range submit.Attachments {
		atts = append(atts, provider.Attachment{
			Path:        a.Path,
			Raw:         a.Raw,
			Content:     a.Content,
			ContentType: a.ContentType,
			Transform:   a.Transform,
		})
	}
	// Convert inline tool results.
	tr := make([]provider.ToolResult, 0, len(submit.ToolResults))
	for _, r := range submit.ToolResults {
		tr = append(tr, provider.ToolResult{
			ToolCallID: r.ToolCallID,
			Output:     r.Output,
			Status:     r.Status,
			Error:      r.Error,
			Reason:     r.Reason,
		})
	}

	msg := provider.Message{
		Role:        "user",
		Content:     submit.Content,
		Attachments: atts,
		ToolResults: tr,
		Metadata: map[string]any{
			provider.MetaKeyTurnID: submit.TurnID,
		},
	}

	c.turns = append(c.turns, msg)
	c.sequence = submit.Sequence

	turn := messageToTurn(c.sessionID, c.sequence, msg, TurnMetadata{})
	turn.ID = submit.TurnID
	if turn.ID == "" {
		turn.ID = uuid.NewString()
	}
	return turn, nil
}

// AppendAssistantTurn appends the model's response to the conversation
// and returns the storage.Turn for persistence. Called when
// turn.complete fires (WU-053).
func (c *Conversation) AppendAssistantTurn(resp AssistantResponse) (*storage.Turn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := provider.Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	}
	c.turns = append(c.turns, msg)
	c.sequence++

	turn := messageToTurn(c.sessionID, c.sequence, msg, TurnMetadata{
		Model:        resp.Model,
		Provider:     resp.Provider,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		Cost:         resp.Cost,
		LatencyMs:    resp.LatencyMs,
	})
	turn.ID = uuid.NewString()
	return turn, nil
}

// RestoreFromTurns rebuilds conversation state from persisted turns.
// Turns must be in ascending sequence order (the storage layer returns
// them that way).
func (c *Conversation) RestoreFromTurns(turns []storage.Turn) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.turns = c.turns[:0]
	c.sequence = 0
	for _, t := range turns {
		msg, err := turnToMessage(&t)
		if err != nil {
			return fmt.Errorf("turn %s: %w", t.ID, err)
		}
		c.turns = append(c.turns, msg)
		if t.Sequence > c.sequence {
			c.sequence = t.Sequence
		}
	}
	return nil
}

// PendingToolCalls returns tool calls from the most recent assistant
// turn that do not yet have a result. A result is considered present
// when a subsequent user turn carries a matching ToolResult.
func (c *Conversation) PendingToolCalls() []provider.ToolCall {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Walk backwards to find the last assistant turn.
	lastAssistant := -1
	for i := len(c.turns) - 1; i >= 0; i-- {
		if c.turns[i].Role == "assistant" {
			lastAssistant = i
			break
		}
	}
	if lastAssistant < 0 || len(c.turns[lastAssistant].ToolCalls) == 0 {
		return nil
	}
	// Collect results from later user turns.
	resolved := map[string]bool{}
	for i := lastAssistant + 1; i < len(c.turns); i++ {
		for _, r := range c.turns[i].ToolResults {
			resolved[r.ToolCallID] = true
		}
	}
	var pending []provider.ToolCall
	for _, tc := range c.turns[lastAssistant].ToolCalls {
		if !resolved[tc.ID] {
			pending = append(pending, tc)
		}
	}
	return pending
}

// MatchToolResult reports whether toolCallID corresponds to an
// outstanding tool call on the latest assistant turn, returning the
// originating ToolCall when it does.
func (c *Conversation) MatchToolResult(toolCallID string) (provider.ToolCall, bool) {
	for _, tc := range c.PendingToolCalls() {
		if tc.ID == toolCallID {
			return tc, true
		}
	}
	return provider.ToolCall{}, false
}

// turnToMessage unmarshals a persisted storage.Turn's Content blob back
// into a canonical provider.Message.
func turnToMessage(t *storage.Turn) (provider.Message, error) {
	var msg provider.Message
	if len(t.Content) == 0 {
		return msg, errors.New("turn has empty Content")
	}
	if err := json.Unmarshal(t.Content, &msg); err != nil {
		return msg, fmt.Errorf("unmarshal turn content: %w", err)
	}
	return msg, nil
}

// messageToTurn marshals a canonical provider.Message into a
// storage.Turn. Per design D3.6 the canonical Message is the Turn's
// Content field exactly — no envelope wrapping.
func messageToTurn(sessionID string, sequence int, msg provider.Message, meta TurnMetadata) *storage.Turn {
	raw, err := json.Marshal(msg)
	if err != nil {
		// Marshal failure on our own canonical types is a programming error.
		raw = []byte("null")
	}

	var toolCallsRaw json.RawMessage
	if len(msg.ToolCalls) > 0 {
		if b, err := json.Marshal(msg.ToolCalls); err == nil {
			toolCallsRaw = b
		}
	}

	return &storage.Turn{
		SessionID:     sessionID,
		Sequence:      sequence,
		Role:          msg.Role,
		Content:       raw,
		Model:         meta.Model,
		Provider:      meta.Provider,
		InputTokens:   meta.InputTokens,
		OutputTokens:  meta.OutputTokens,
		Cost:          meta.Cost,
		LatencyMs:     meta.LatencyMs,
		ToolCalls:     toolCallsRaw,
		FilesTouched:  meta.FilesTouched,
		FilesModified: meta.FilesModified,
		CreatedAt:     time.Now().UTC(),
	}
}

// -----------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------

// appendMessageForTest is a direct in-memory append used by unit tests
// that need a pre-populated conversation without going through the
// validation path.
func (c *Conversation) appendMessageForTest(role, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turns = append(c.turns, provider.Message{Role: role, Content: content})
	c.sequence++
}
