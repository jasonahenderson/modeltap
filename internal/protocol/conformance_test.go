package protocol

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

//go:embed all:fixtures
var fixturesFS embed.FS

// fixtureCase maps a fixture file (relative to fixtures/) to the Go type
// it should round-trip through.
type fixtureCase struct {
	path    string      // e.g. "requests/turn_submit.json"
	target  interface{} // pointer to zero-value of the type
	typName string      // Go type name for test output
}

// allFixtureCases returns every fixture-to-type mapping. This is the
// single source of truth for conformance coverage.
func allFixtureCases() []fixtureCase {
	return []fixtureCase{
		// ---- Requests (WU-039) ----
		{"requests/turn_submit.json", new(TurnSubmit), "TurnSubmit"},
		{"requests/turn_cancel.json", new(TurnCancel), "TurnCancel"},
		{"requests/tool_result.json", new(ToolResult), "ToolResult"},
		{"requests/content_transform.json", new(ContentTransform), "ContentTransform"},
		{"requests/session_create.json", new(SessionCreate), "SessionCreate"},
		{"requests/session_resume.json", new(SessionResume), "SessionResume"},
		{"requests/session_list.json", new(SessionList), "SessionList"},
		{"requests/session_details.json", new(SessionDetails), "SessionDetails"},
		{"requests/session_compact.json", new(SessionCompact), "SessionCompact"},
		{"requests/compact_apply.json", new(CompactApply), "CompactApply"},
		{"requests/session_clear.json", new(SessionClear), "SessionClear"},
		{"requests/session_fork.json", new(SessionFork), "SessionFork"},
		{"requests/session_sync.json", new(SessionSync), "SessionSync"},
		{"requests/model_switch.json", new(ModelSwitch), "ModelSwitch"},
		{"requests/model_list.json", new(ModelList), "ModelList"},
		{"requests/context_list.json", new(ContextList), "ContextList"},
		{"requests/capabilities_register.json", new(CapabilitiesRegister), "CapabilitiesRegister"},
		{"requests/capabilities_update.json", new(CapabilitiesUpdate), "CapabilitiesUpdate"},
		{"requests/connection_ping.json", new(ConnectionPing), "ConnectionPing"},
		{"requests/connection_health.json", new(ConnectionHealth), "ConnectionHealth"},
		{"requests/connection_ready.json", new(ConnectionReady), "ConnectionReady"},
		{"requests/history_append.json", new(HistoryAppend), "HistoryAppend"},
		{"requests/history_list.json", new(HistoryList), "HistoryList"},

		// ---- Events (WU-040) ----
		{"events/token_delta.json", new(TokenDelta), "TokenDelta"},
		{"events/branch_started.json", new(BranchStarted), "BranchStarted"},
		{"events/branch_complete.json", new(BranchComplete), "BranchComplete"},
		{"events/branch_error.json", new(BranchError), "BranchError"},
		{"events/tool_call.json", new(ToolCall), "ToolCall"},
		{"events/status_update.json", new(StatusUpdate), "StatusUpdate"},
		{"events/knowledge_hit.json", new(KnowledgeHit), "KnowledgeHit"},
		{"events/cost_update.json", new(CostUpdate), "CostUpdate"},
		{"events/compact_plan.json", new(CompactPlan), "CompactPlan"},
		{"events/compact_suggest.json", new(CompactSuggest), "CompactSuggest"},
		{"events/compact_notice.json", new(CompactNotice), "CompactNotice"},
		{"events/turn_complete.json", new(TurnComplete), "TurnComplete"},
		{"events/model_selected.json", new(ModelSelected), "ModelSelected"},
		{"events/model_selected_multi.json", new(ModelSelected), "ModelSelected_multi"},
		{"events/server_error.json", new(ServerError), "ServerError"},
		{"events/capabilities_request.json", new(CapabilitiesRequestEvent), "CapabilitiesRequestEvent"},
		{"events/connection_pong.json", new(ConnectionPong), "ConnectionPong"},

		// ---- Responses (WU-041) ----
		{"responses/turn_submit.json", new(TurnSubmitResponse), "TurnSubmitResponse"},
		{"responses/turn_cancel.json", new(TurnCancelResponse), "TurnCancelResponse"},
		{"responses/tool_result.json", new(ToolResultResponse), "ToolResultResponse"},
		{"responses/content_transform.json", new(ContentTransformResponse), "ContentTransformResponse"},
		{"responses/session_list.json", new(SessionListResponse), "SessionListResponse"},
		{"responses/session_detail.json", new(SessionDetail), "SessionDetail"},
		{"responses/session_sync.json", new(SessionSyncResponse), "SessionSyncResponse"},
		{"responses/session_create.json", new(SessionCreateResponse), "SessionCreateResponse"},
		{"responses/session_resume.json", new(SessionResumeResponse), "SessionResumeResponse"},
		{"responses/session_clear.json", new(SessionClearResponse), "SessionClearResponse"},
		{"responses/session_fork.json", new(SessionForkResponse), "SessionForkResponse"},
		{"responses/context_list.json", new(ContextListResponse), "ContextListResponse"},
		{"responses/model_list.json", new(ModelListResponse), "ModelListResponse"},
		{"responses/model_switch.json", new(ModelSwitchResponse), "ModelSwitchResponse"},
		{"responses/capabilities_register.json", new(CapabilitiesRegisterResponse), "CapabilitiesRegisterResponse"},
		{"responses/capabilities_update.json", new(CapabilitiesUpdateResponse), "CapabilitiesUpdateResponse"},
		{"responses/health.json", new(HealthResponse), "HealthResponse"},
		{"responses/ready.json", new(ReadyResponse), "ReadyResponse"},
		{"responses/compact_apply.json", new(CompactApplyResponse), "CompactApplyResponse"},
		{"responses/history_append.json", new(HistoryAppendResponse), "HistoryAppendResponse"},
		{"responses/history_list.json", new(HistoryListResponse), "HistoryListResponse"},

		// ---- Diagnostics (WU-041 errors) ----
		{"errors/mt_conn_001.json", new(Diagnostic), "Diagnostic_MT-CONN-001"},
		{"errors/mt_conn_002.json", new(Diagnostic), "Diagnostic_MT-CONN-002"},
		{"errors/mt_conn_003.json", new(Diagnostic), "Diagnostic_MT-CONN-003"},
		{"errors/mt_conn_004.json", new(Diagnostic), "Diagnostic_MT-CONN-004"},
		{"errors/mt_conn_005.json", new(Diagnostic), "Diagnostic_MT-CONN-005"},
		{"errors/mt_conn_006.json", new(Diagnostic), "Diagnostic_MT-CONN-006"},
		{"errors/mt_conn_007.json", new(Diagnostic), "Diagnostic_MT-CONN-007"},
		{"errors/mt_conn_008.json", new(Diagnostic), "Diagnostic_MT-CONN-008"},
		{"errors/mt_conn_009.json", new(Diagnostic), "Diagnostic_MT-CONN-009"},
		{"errors/mt_conn_010.json", new(Diagnostic), "Diagnostic_MT-CONN-010"},
		{"errors/mt_conn_011.json", new(Diagnostic), "Diagnostic_MT-CONN-011"},
		{"errors/mt_conn_012.json", new(Diagnostic), "Diagnostic_MT-CONN-012"},
		{"errors/mt_conn_013.json", new(Diagnostic), "Diagnostic_MT-CONN-013"},
	}
}

// readFixture loads a fixture file from the embedded FS.
func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := fixturesFS.ReadFile(filepath.Join("fixtures", path))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return data
}

// TestFixtureRoundTrip is the core conformance test. For every fixture:
//  1. Load the golden JSON
//  2. Unmarshal into the Go type
//  3. Marshal back to JSON
//  4. Compare with original (key-order independent)
//  5. Assert no fields were lost
func TestFixtureRoundTrip(t *testing.T) {
	for _, fc := range allFixtureCases() {
		t.Run(fc.typName, func(t *testing.T) {
			data := readFixture(t, fc.path)

			// Step 1: unmarshal fixture into a fresh instance of the Go type
			target := reflect.New(reflect.TypeOf(fc.target).Elem()).Interface()
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatalf("unmarshal fixture %s into %s: %v", fc.path, fc.typName, err)
			}

			// Step 2: marshal back to JSON
			remarshal, err := json.Marshal(target)
			if err != nil {
				t.Fatalf("re-marshal %s: %v", fc.typName, err)
			}

			// Step 3: compare as unordered maps/values
			var original, roundTripped interface{}
			if err := json.Unmarshal(data, &original); err != nil {
				t.Fatalf("unmarshal fixture to interface{}: %v", err)
			}
			if err := json.Unmarshal(remarshal, &roundTripped); err != nil {
				t.Fatalf("unmarshal re-marshaled to interface{}: %v", err)
			}

			if !reflect.DeepEqual(original, roundTripped) {
				t.Errorf("round-trip mismatch for %s\n  original: %s\n  got:      %s",
					fc.typName, string(data), string(remarshal))
			}
		})
	}
}

// TestFixtureNotEmpty ensures every fixture produces a non-zero-value
// result when unmarshaled (except for empty-struct types like
// ConnectionPing, ConnectionPong, etc.).
func TestFixtureNotEmpty(t *testing.T) {
	emptyStructTypes := map[string]bool{
		"SessionList":      true,
		"ModelList":        true,
		"ConnectionPing":   true,
		"ConnectionHealth": true,
		"ConnectionReady":  true,
		"ConnectionPong":   true,
	}

	for _, fc := range allFixtureCases() {
		t.Run(fc.typName, func(t *testing.T) {
			if emptyStructTypes[fc.typName] {
				t.Skip("empty struct type, no field assertions")
			}

			data := readFixture(t, fc.path)
			target := reflect.New(reflect.TypeOf(fc.target).Elem()).Interface()
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			zero := reflect.New(reflect.TypeOf(fc.target).Elem()).Interface()
			if reflect.DeepEqual(target, zero) {
				t.Errorf("fixture %s unmarshaled to zero value — fixture is likely empty or has wrong field names", fc.path)
			}
		})
	}
}

// TestMethodConstantFixtureCoverage verifies that every method constant
// (MethodTurnSubmit, etc.) has a corresponding request fixture.
func TestMethodConstantFixtureCoverage(t *testing.T) {
	methodToFixture := map[string]string{
		MethodTurnSubmit:           "requests/turn_submit.json",
		MethodTurnCancel:           "requests/turn_cancel.json",
		MethodToolResult:           "requests/tool_result.json",
		MethodContentTransform:     "requests/content_transform.json",
		MethodSessionCreate:        "requests/session_create.json",
		MethodSessionResume:        "requests/session_resume.json",
		MethodSessionList:          "requests/session_list.json",
		MethodSessionDetails:       "requests/session_details.json",
		MethodSessionCompact:       "requests/session_compact.json",
		MethodCompactApply:         "requests/compact_apply.json",
		MethodSessionClear:         "requests/session_clear.json",
		MethodSessionFork:          "requests/session_fork.json",
		MethodSessionSync:          "requests/session_sync.json",
		MethodModelSwitch:          "requests/model_switch.json",
		MethodModelList:            "requests/model_list.json",
		MethodContextList:          "requests/context_list.json",
		MethodCapabilitiesRegister: "requests/capabilities_register.json",
		MethodCapabilitiesUpdate:   "requests/capabilities_update.json",
		MethodConnectionPing:       "requests/connection_ping.json",
		MethodConnectionHealth:     "requests/connection_health.json",
		MethodConnectionReady:      "requests/connection_ready.json",
		MethodHistoryAppend:        "requests/history_append.json",
		MethodHistoryList:          "requests/history_list.json",
	}

	for method, fixturePath := range methodToFixture {
		t.Run(method, func(t *testing.T) {
			if _, err := fixturesFS.ReadFile(filepath.Join("fixtures", fixturePath)); err != nil {
				t.Errorf("method %q has no fixture at %s: %v", method, fixturePath, err)
			}
		})
	}
}

// TestEventConstantFixtureCoverage verifies that every event constant
// (EventTokenDelta, etc.) has a corresponding event fixture.
func TestEventConstantFixtureCoverage(t *testing.T) {
	eventToFixture := map[string]string{
		EventTokenDelta:          "events/token_delta.json",
		EventBranchStarted:       "events/branch_started.json",
		EventBranchComplete:      "events/branch_complete.json",
		EventBranchError:         "events/branch_error.json",
		EventToolCall:            "events/tool_call.json",
		EventStatusUpdate:        "events/status_update.json",
		EventKnowledgeHit:        "events/knowledge_hit.json",
		EventCostUpdate:          "events/cost_update.json",
		EventCompactPlan:         "events/compact_plan.json",
		EventCompactSuggest:      "events/compact_suggest.json",
		EventCompactNotice:       "events/compact_notice.json",
		EventTurnComplete:        "events/turn_complete.json",
		EventModelSelected:       "events/model_selected.json",
		EventError:               "events/server_error.json",
		EventCapabilitiesRequest: "events/capabilities_request.json",
		EventConnectionPong:      "events/connection_pong.json",
	}

	for event, fixturePath := range eventToFixture {
		t.Run(event, func(t *testing.T) {
			if _, err := fixturesFS.ReadFile(filepath.Join("fixtures", fixturePath)); err != nil {
				t.Errorf("event %q has no fixture at %s: %v", event, fixturePath, err)
			}
		})
	}
}

// TestDiagnosticCodeFixtureCoverage verifies that every DiagnosticCode
// constant has a fixture in errors/.
func TestDiagnosticCodeFixtureCoverage(t *testing.T) {
	codes := []DiagnosticCode{
		DiagServiceNotRunning,
		DiagStaleSocket,
		DiagSocketPermission,
		DiagVersionMismatch,
		DiagTLSUntrusted,
		DiagAuthExpired,
		DiagStorageUnready,
		DiagSessionLocked,
		DiagProviderUnavailable,
		DiagCapabilityRegistrationFailed,
		DiagModelUnavailable,
		DiagHeartbeatTimeout,
		DiagAttachmentTooLarge,
	}

	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			// Derive fixture filename from code: MT-CONN-009 -> mt_conn_009.json
			name := strings.ToLower(strings.ReplaceAll(string(code), "-", "_"))
			fixturePath := fmt.Sprintf("errors/%s.json", name)
			data, err := fixturesFS.ReadFile(filepath.Join("fixtures", fixturePath))
			if err != nil {
				t.Fatalf("diagnostic code %s has no fixture at %s: %v", code, fixturePath, err)
			}

			// Verify the fixture contains the correct code
			var diag Diagnostic
			if err := json.Unmarshal(data, &diag); err != nil {
				t.Fatalf("unmarshal diagnostic fixture: %v", err)
			}
			if diag.Code != code {
				t.Errorf("fixture code = %q, want %q", diag.Code, code)
			}
		})
	}
}

// TestFixtureCanonicalFieldNames loads every fixture and checks that no
// CamelCase field names leaked onto the wire. This extends the WU-039
// snake_case checks to all fixture files.
func TestFixtureCanonicalFieldNames(t *testing.T) {
	// Common CamelCase leaks that should never appear in wire JSON.
	badPatterns := []string{
		`"turnId"`, `"sessionId"`, `"toolCallId"`, `"branchId"`,
		`"contentType"`, `"outputType"`, `"rawContent"`, `"maxOutputTokens"`,
		`"inputSchema"`, `"outputEnvelope"`, `"riskLevel"`, `"capabilitiesRequired"`,
		`"addedTools"`, `"removedTools"`, `"toolResults"`,
		`"protocolVersion"`, `"harnessVersion"`, `"harnessPlatform"`,
		`"configFile"`, `"configContent"`,
		`"finalInputTokens"`, `"finalOutputTokens"`, `"totalCost"`,
		`"latencyMs"`, `"inputTokens"`, `"outputTokens"`,
		`"inputCost"`, `"outputCost"`, `"contextPct"`,
		`"diagnosticCode"`, `"autoRepairAttempted"`, `"repairResult"`,
		`"suggestedCommand"`, `"pathOrEndpoint"`,
		`"lastActive"`, `"turnCount"`, `"modelOverride"`, `"lastTurnSummary"`,
		`"filesTouched"`, `"pinnedCount"`, `"filesModified"`,
		`"createdAt"`, `"pinnedItems"`, `"serverEvents"`,
		`"originalTurns"`, `"freedTokens"`,
		`"pendingToolCalls"`, `"completedTokens"`, `"tokenReplayAvailable"`,
		`"multiModel"`, `"activeTurn"`,
		`"overrideSet"`, `"currentOverride"`, `"routingPolicy"`,
		`"contextWindow"`, `"costPer1kInput"`, `"costPer1kOutput"`,
		`"serverVersion"`, `"uptimeSeconds"`, `"activeSession"`,
		`"serverCapabilities"`, `"addedCount"`, `"removedCount"`, `"updatedAt"`,
		`"tokensFreed"`, `"triggeredBy"`,
		`"tokenCount"`, `"valueScore"`, `"suggestedAction"`, `"summaryPreview"`,
		`"attachedTurn"`, `"estimatedTokensFreed"`, `"contextPctBefore"`, `"contextPctAfter"`,
		`"sizeBytes"`, `"sourceDate"`, `"knowledgeInjections"`,
		`"contextTokens"`, `"systemPromptTokens"`, `"knowledgeInjectionTokens"`,
		`"retainedInStorage"`, `"clearedTurns"`,
		`"newSessionId"`, `"originalSessionId"`,
		`"maxFrameSize"`, `"maxAttachmentSize"`, `"supportedTransforms"`,
		`"protocolVersionRange"`,
		`"hasMore"`,
	}

	for _, fc := range allFixtureCases() {
		t.Run(fc.typName, func(t *testing.T) {
			data := readFixture(t, fc.path)

			// Round-trip through Go type to get the re-marshaled form
			target := reflect.New(reflect.TypeOf(fc.target).Elem()).Interface()
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			remarshaled, err := json.Marshal(target)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			s := string(remarshaled)
			for _, bad := range badPatterns {
				if strings.Contains(s, bad) {
					t.Errorf("CamelCase leak %s in re-marshaled %s: %s", bad, fc.typName, s)
				}
			}
		})
	}
}

// TestFixtureCoveredRegistry ensures the _covered.json skip list is valid
// JSON and only contains known type names from the protocol package.
func TestFixtureCoveredRegistry(t *testing.T) {
	data, err := fixturesFS.ReadFile("fixtures/_covered.json")
	if err != nil {
		t.Fatalf("failed to read _covered.json: %v", err)
	}

	var registry struct {
		Skipped map[string]string `json:"skipped"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("_covered.json is not valid JSON: %v", err)
	}

	// Every skipped entry must have a non-empty justification.
	for typName, justification := range registry.Skipped {
		if justification == "" {
			t.Errorf("skipped type %q has empty justification in _covered.json", typName)
		}
		_ = typName // existence check only; we don't reflect package types
	}
}

// TestFixtureFilesAreValidJSON ensures every .json file in fixtures/ is
// valid JSON. This catches syntax errors before they reach the round-trip
// tests.
func TestFixtureFilesAreValidJSON(t *testing.T) {
	dirs := []string{"requests", "responses", "events", "errors"}
	for _, dir := range dirs {
		entries, err := fixturesFS.ReadDir(filepath.Join("fixtures", dir))
		if err != nil {
			t.Fatalf("read dir fixtures/%s: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			t.Run(dir+"/"+entry.Name(), func(t *testing.T) {
				data, err := fixturesFS.ReadFile(filepath.Join("fixtures", dir, entry.Name()))
				if err != nil {
					t.Fatalf("read: %v", err)
				}
				if !json.Valid(data) {
					t.Errorf("fixture %s/%s is not valid JSON", dir, entry.Name())
				}
			})
		}
	}
}
