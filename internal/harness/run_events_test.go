package harness

import (
	"encoding/json"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestConnectionManager_PrefersRunIDOnLegacyTurnEvents(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{SocketPath: "/none"}, sender)

	deltaRaw, _ := json.Marshal(protocol.TokenDelta{TurnID: "turn-1", RunID: "run-1", Text: "hi"})
	cm.HandleEvent(protocol.EventTokenDelta, deltaRaw)

	completeRaw, _ := json.Marshal(protocol.TurnComplete{TurnID: "turn-1", RunID: "run-1"})
	cm.HandleEvent(protocol.EventTurnComplete, completeRaw)

	msgs := sender.snapshot()
	if len(msgs) != 2 {
		t.Fatalf("msgs = %d, want 2", len(msgs))
	}
	delta, ok := msgs[0].(StreamTokenMsg)
	if !ok {
		t.Fatalf("first msg = %T, want StreamTokenMsg", msgs[0])
	}
	if delta.TurnID != "run-1" {
		t.Fatalf("delta TurnID = %q, want run-1", delta.TurnID)
	}
	complete, ok := msgs[1].(StreamCompleteMsg)
	if !ok {
		t.Fatalf("second msg = %T, want StreamCompleteMsg", msgs[1])
	}
	if complete.TurnID != "run-1" {
		t.Fatalf("complete TurnID = %q, want run-1", complete.TurnID)
	}
}
