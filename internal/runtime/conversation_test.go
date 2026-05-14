package runtime

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

func TestConversation_AppendUserTurn_Success(t *testing.T) {
	c := NewConversation("sess-1")
	submit := &protocol.TurnSubmit{
		TurnID:    "turn-1",
		SessionID: "sess-1",
		Sequence:  1,
		Mode:      protocol.ModeBuild,
		Content:   "hello",
	}
	turn, err := c.AppendUserTurn(submit)
	if err != nil {
		t.Fatalf("AppendUserTurn: %v", err)
	}
	if turn.ID != "turn-1" {
		t.Errorf("turn.ID = %q", turn.ID)
	}
	if turn.Sequence != 1 {
		t.Errorf("turn.Sequence = %d", turn.Sequence)
	}
	if c.Sequence() != 1 || c.TurnCount() != 1 {
		t.Errorf("state = (seq=%d, n=%d)", c.Sequence(), c.TurnCount())
	}
	msgs := c.Messages()
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("msg = %+v", msgs[0])
	}
}

func TestConversation_AppendUserTurn_SequenceMismatch(t *testing.T) {
	c := NewConversation("sess-1")
	// Expected sequence is 1; send 2 → error.
	_, err := c.AppendUserTurn(&protocol.TurnSubmit{
		TurnID:    "turn-1",
		SessionID: "sess-1",
		Sequence:  2,
		Mode:      protocol.ModeBuild,
		Content:   "hi",
	})
	if err == nil {
		t.Fatalf("expected sequence error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams, got %T %v", err, err)
	}
}

func TestConversation_AppendAssistantTurn(t *testing.T) {
	c := NewConversation("sess-1")
	// First user turn.
	_, err := c.AppendUserTurn(&protocol.TurnSubmit{TurnID: "t1", SessionID: "sess-1", Sequence: 1, Mode: protocol.ModeBuild, Content: "hi"})
	if err != nil {
		t.Fatalf("user append: %v", err)
	}
	// Assistant turn.
	at, err := c.AppendAssistantTurn(AssistantResponse{
		Content:      "hello back",
		Model:        "test-model",
		Provider:     "test",
		InputTokens:  10,
		OutputTokens: 20,
		Cost:         0.05,
	})
	if err != nil {
		t.Fatalf("assistant append: %v", err)
	}
	if at.Role != "assistant" || at.Model != "test-model" || at.Cost != 0.05 {
		t.Errorf("turn = %+v", at)
	}
	// User-turn sequence (the wire contract) advances only on user
	// turns. Storage sequence advances per-message so the assistant
	// row has its own ordering slot.
	if c.Sequence() != 1 {
		t.Errorf("user-turn sequence = %d, want 1", c.Sequence())
	}
	if c.StorageSequence() != 2 {
		t.Errorf("storage sequence = %d, want 2", c.StorageSequence())
	}
	if at.Sequence != 2 {
		t.Errorf("assistant turn.Sequence = %d, want 2", at.Sequence)
	}
}

// PATCH-0031: the second user turn after one accepted round-trip must
// be accepted with submit.Sequence = 2, not 3. The harness's
// runtimeState.NextSequence counts user submits only, so the Runtime must
// validate against the same counter — not a per-message counter that
// the assistant turn would have bumped.
func TestConversation_TwoUserTurns_AcceptsSequence2(t *testing.T) {
	c := NewConversation("sess-1")

	if _, err := c.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "t1", SessionID: "sess-1", Sequence: 1,
		Mode: protocol.ModeBuild, Content: "first",
	}); err != nil {
		t.Fatalf("first user turn: %v", err)
	}
	if _, err := c.AppendAssistantTurn(AssistantResponse{Content: "reply"}); err != nil {
		t.Fatalf("assistant turn: %v", err)
	}

	// The harness will send sequence=2 for the second user turn.
	turn, err := c.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "t2", SessionID: "sess-1", Sequence: 2,
		Mode: protocol.ModeBuild, Content: "second",
	})
	if err != nil {
		t.Fatalf("second user turn should accept Sequence=2: %v", err)
	}
	if turn.Sequence != 3 {
		t.Errorf("second user turn storage Sequence = %d, want 3", turn.Sequence)
	}
	if c.Sequence() != 2 {
		t.Errorf("user-turn sequence after second user = %d, want 2", c.Sequence())
	}
	if c.StorageSequence() != 3 {
		t.Errorf("storage sequence after second user = %d, want 3", c.StorageSequence())
	}

	// Sending Sequence=3 (one ahead) must still error, to catch a
	// regression in the other direction.
	_, err = c.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "t3", SessionID: "sess-1", Sequence: 4,
		Mode: protocol.ModeBuild, Content: "skips one",
	})
	if err == nil {
		t.Fatalf("expected error on Sequence=4 after userSequence=2")
	}
}

func TestConversation_RestoreFromTurns(t *testing.T) {
	c := NewConversation("sess-1")
	userMsg := provider.Message{Role: "user", Content: "hi"}
	userRaw, _ := json.Marshal(userMsg)
	asstMsg := provider.Message{Role: "assistant", Content: "hello"}
	asstRaw, _ := json.Marshal(asstMsg)

	turns := []storage.Turn{
		{ID: "t1", SessionID: "sess-1", Sequence: 1, Role: "user", Content: userRaw},
		{ID: "t2", SessionID: "sess-1", Sequence: 2, Role: "assistant", Content: asstRaw},
	}
	if err := c.RestoreFromTurns(turns); err != nil {
		t.Fatalf("RestoreFromTurns: %v", err)
	}
	// One user message restored → wire sequence is 1; storage rows
	// occupy slots 1 and 2.
	if c.TurnCount() != 2 || c.Sequence() != 1 {
		t.Errorf("state = (n=%d, seq=%d)", c.TurnCount(), c.Sequence())
	}
	if c.StorageSequence() != 2 {
		t.Errorf("storage sequence = %d, want 2", c.StorageSequence())
	}
	msgs := c.Messages()
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("roles = [%s %s]", msgs[0].Role, msgs[1].Role)
	}
}

func TestConversation_Reset(t *testing.T) {
	c := NewConversation("sess-1")
	c.appendMessageForTest("user", "a")
	c.appendMessageForTest("assistant", "b")
	if c.Reset() != 2 {
		t.Errorf("Reset count mismatch")
	}
	if c.Sequence() != 0 || c.TurnCount() != 0 {
		t.Errorf("state after reset = (seq=%d, n=%d)", c.Sequence(), c.TurnCount())
	}
}

func TestConversation_PendingToolCalls_AndMatch(t *testing.T) {
	c := NewConversation("sess-1")
	// user, then assistant with 2 tool calls.
	_, err := c.AppendUserTurn(&protocol.TurnSubmit{TurnID: "u1", SessionID: "sess-1", Sequence: 1, Mode: protocol.ModeBuild, Content: "go"})
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	_, err = c.AppendAssistantTurn(AssistantResponse{
		Content: "calling tools",
		ToolCalls: []provider.ToolCall{
			{ID: "call-a", Name: "read", Input: json.RawMessage(`{}`)},
			{ID: "call-b", Name: "write", Input: json.RawMessage(`{}`)},
		},
		Model: "m",
	})
	if err != nil {
		t.Fatalf("assistant: %v", err)
	}

	pending := c.PendingToolCalls()
	if len(pending) != 2 {
		t.Fatalf("pending = %d, want 2", len(pending))
	}

	// MatchToolResult finds call-a.
	if _, ok := c.MatchToolResult("call-a"); !ok {
		t.Errorf("call-a should match")
	}
	if _, ok := c.MatchToolResult("call-unknown"); ok {
		t.Errorf("unknown id should not match")
	}

	// User turn with a result for call-a resolves it. The wire-contract
	// sequence counts user submissions only, so the second user turn
	// is Sequence=2 (not 3 — the assistant turn does not advance it).
	_, err = c.AppendUserTurn(&protocol.TurnSubmit{
		TurnID:    "u2",
		SessionID: "sess-1",
		Sequence:  2,
		Mode:      protocol.ModeBuild,
		Content:   "",
		ToolResults: []protocol.ToolResult{
			{ToolCallID: "call-a", Status: "success", Output: "ok", OutputType: "text"},
		},
	})
	if err != nil {
		t.Fatalf("user+result: %v", err)
	}
	pending = c.PendingToolCalls()
	if len(pending) != 1 || pending[0].ID != "call-b" {
		t.Errorf("pending after result = %+v", pending)
	}
}

func TestMessageToTurn_AndBack(t *testing.T) {
	msg := provider.Message{
		Role:    "user",
		Content: "hello",
		Attachments: []provider.Attachment{
			{Path: "a.txt", Raw: "", Content: "a", ContentType: "text/plain", Transform: "none"},
		},
	}
	turn := messageToTurn("sess-1", 1, msg, TurnMetadata{Model: "m"})
	if turn.Role != "user" || turn.Sequence != 1 || turn.Model != "m" {
		t.Errorf("turn = %+v", turn)
	}
	round, err := turnToMessage(turn)
	if err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if round.Role != "user" || round.Content != "hello" || len(round.Attachments) != 1 {
		t.Errorf("round-tripped = %+v", round)
	}
}

func TestConversation_Messages_ReturnsCopy(t *testing.T) {
	c := NewConversation("sess-1")
	c.appendMessageForTest("user", "a")
	snap := c.Messages()
	if len(snap) != 1 {
		t.Fatalf("snap = %d", len(snap))
	}
	snap[0].Content = "MUTATED"
	again := c.Messages()
	if again[0].Content == "MUTATED" {
		t.Errorf("Messages() aliased — internal state mutated")
	}
}
