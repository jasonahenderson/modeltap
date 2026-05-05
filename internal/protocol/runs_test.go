package protocol

import (
	"encoding/json"
	"testing"
)

func TestRunProtocolFixturesRoundTrip(t *testing.T) {
	createRaw := []byte(`{"session_id":"sess-1","idempotency_key":"idem-1","workflow_type":"implementation","title":"Build it"}`)
	var create RunCreate
	if err := json.Unmarshal(createRaw, &create); err != nil {
		t.Fatalf("RunCreate unmarshal: %v", err)
	}
	if create.WorkflowType != "implementation" || create.IdempotencyKey == "" {
		t.Fatalf("RunCreate = %+v", create)
	}

	eventRaw := []byte(`{"run_id":"run-1","seq":2,"session_id":"sess-1","stage":"model_call","status":"running","created_at":"2026-05-05T00:00:00Z"}`)
	var ev RunEventPayload
	if err := json.Unmarshal(eventRaw, &ev); err != nil {
		t.Fatalf("RunEventPayload unmarshal: %v", err)
	}
	if !ev.Validate() {
		t.Fatalf("RunEventPayload Validate=false: %+v", ev)
	}

	var resp TurnSubmitResponse
	if err := json.Unmarshal([]byte(`{"turn_id":"turn-1","session_id":"sess-1","status":"accepted","run_id":"run-1"}`), &resp); err != nil {
		t.Fatalf("TurnSubmitResponse unmarshal: %v", err)
	}
	if resp.RunID != "run-1" {
		t.Fatalf("RunID = %q", resp.RunID)
	}
	withoutRun, err := json.Marshal(TurnSubmitResponse{TurnID: "turn-1", Status: "accepted"})
	if err != nil {
		t.Fatalf("TurnSubmitResponse marshal: %v", err)
	}
	if string(withoutRun) != `{"turn_id":"turn-1","status":"accepted"}` {
		t.Fatalf("omitempty response = %s", withoutRun)
	}
}
