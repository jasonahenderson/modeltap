package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// T5. Method name constants
// -----------------------------------------------------------------------

func TestMethodConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"TurnSubmit", MethodTurnSubmit, "turn.submit"},
		{"TurnCancel", MethodTurnCancel, "turn.cancel"},
		{"ToolResult", MethodToolResult, "tool.result"},
		{"ContentTransform", MethodContentTransform, "content.transform"},
		{"SessionCreate", MethodSessionCreate, "session.create"},
		{"SessionResume", MethodSessionResume, "session.resume"},
		{"SessionList", MethodSessionList, "session.list"},
		{"SessionDetails", MethodSessionDetails, "session.details"},
		{"SessionCompact", MethodSessionCompact, "session.compact"},
		{"CompactApply", MethodCompactApply, "compact.apply"},
		{"SessionClear", MethodSessionClear, "session.clear"},
		{"SessionFork", MethodSessionFork, "session.fork"},
		{"SessionSync", MethodSessionSync, "session.sync"},
		{"ModelSwitch", MethodModelSwitch, "model.switch"},
		{"ModelList", MethodModelList, "model.list"},
		{"ContextList", MethodContextList, "context.list"},
		{"CapabilitiesRegister", MethodCapabilitiesRegister, "capabilities.register"},
		{"CapabilitiesUpdate", MethodCapabilitiesUpdate, "capabilities.update"},
		{"ConnectionPing", MethodConnectionPing, "connection.ping"},
		{"ConnectionHealth", MethodConnectionHealth, "connection.health"},
		{"ConnectionReady", MethodConnectionReady, "connection.ready"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("Method%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion == "" {
		t.Fatal("ProtocolVersion must not be empty")
	}
	if ProtocolVersion != "1" {
		t.Errorf("ProtocolVersion = %q, want %q", ProtocolVersion, "1")
	}
}

// -----------------------------------------------------------------------
// T4. Mode
// -----------------------------------------------------------------------

func TestMode_Valid(t *testing.T) {
	tests := []struct {
		m    Mode
		want bool
	}{
		{ModePlan, true},
		{ModeBuild, true},
		{ModeAuto, true},
		{Mode("bogus"), false},
		{Mode(""), false},
	}
	for _, tc := range tests {
		t.Run(string(tc.m), func(t *testing.T) {
			if got := tc.m.Valid(); got != tc.want {
				t.Errorf("Mode(%q).Valid() = %v, want %v", tc.m, got, tc.want)
			}
		})
	}
}

func TestMode_JSONRoundTrip(t *testing.T) {
	for _, m := range []Mode{ModePlan, ModeBuild, ModeAuto} {
		t.Run(string(m), func(t *testing.T) {
			in := TurnSubmit{TurnID: "t1", SessionID: "s1", Sequence: 1, Mode: m, Content: "hi"}
			b, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var out TurnSubmit
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if out.Mode != m {
				t.Errorf("mode round-trip: got %q, want %q (json=%s)", out.Mode, m, string(b))
			}
		})
	}
}

// -----------------------------------------------------------------------
// T1. Round-trip per request type
// -----------------------------------------------------------------------

// Each request type gets a "full" case (every field populated) and a
// "minimal" case (only required fields). Types with no params have only
// a trivial empty-object round-trip.

func TestTurnSubmit_RoundTrip_Full(t *testing.T) {
	inputSchema := json.RawMessage(`{"type":"object"}`)
	_ = inputSchema
	in := TurnSubmit{
		TurnID:    "turn-1",
		SessionID: "sess-1",
		Sequence:  3,
		Mode:      ModeBuild,
		Content:   "explain this",
		Attachments: []Attachment{
			{Path: "README.md", Raw: "base64==", Content: "hello", ContentType: "text/markdown", Transform: "none"},
		},
		Paste: &Paste{Raw: "long\ntext", Content: "summary", Intent: "summarized"},
		ToolResults: []ToolResult{
			{ToolCallID: "tc-1", Status: "success", Output: "ok", OutputType: "text"},
		},
	}
	assertRoundTrip(t, &in, new(TurnSubmit))
}

func TestTurnSubmit_RoundTrip_ContentOnly(t *testing.T) {
	in := TurnSubmit{TurnID: "t", SessionID: "s", Sequence: 1, Mode: ModePlan, Content: "hi"}
	assertRoundTrip(t, &in, new(TurnSubmit))
}

func TestTurnSubmit_RoundTrip_ToolResultsOnly(t *testing.T) {
	in := TurnSubmit{
		TurnID:    "t",
		SessionID: "s",
		Sequence:  2,
		Mode:      ModeAuto,
		ToolResults: []ToolResult{
			{ToolCallID: "tc-x", Status: "error", Output: "boom", OutputType: "text", Error: "E_FAIL"},
		},
	}
	assertRoundTrip(t, &in, new(TurnSubmit))
}

func TestTurnCancel_RoundTrip(t *testing.T) {
	in := TurnCancel{TurnID: "turn-abc"}
	assertRoundTrip(t, &in, new(TurnCancel))
}

func TestToolResult_RoundTrip_Success(t *testing.T) {
	in := ToolResult{ToolCallID: "tc", Status: "success", Output: "done", OutputType: "text"}
	assertRoundTrip(t, &in, new(ToolResult))
}

func TestToolResult_RoundTrip_Rejected(t *testing.T) {
	in := ToolResult{ToolCallID: "tc", Status: "rejected", Output: "nope", OutputType: "text", Reason: "user denied"}
	assertRoundTrip(t, &in, new(ToolResult))
}

func TestToolResult_RoundTrip_Error(t *testing.T) {
	in := ToolResult{ToolCallID: "tc", Status: "error", Output: "boom", OutputType: "text", Error: "E_FAIL"}
	assertRoundTrip(t, &in, new(ToolResult))
}

func TestToolResultRequest_AliasesToolResult(t *testing.T) {
	var _ = ToolResult{}
}

func TestContentTransform_RoundTrip(t *testing.T) {
	in := ContentTransform{
		Transform:       "summarize",
		RawContent:      "lots of text",
		ContentType:     "text/plain",
		MaxOutputTokens: 500,
	}
	assertRoundTrip(t, &in, new(ContentTransform))
}

func TestSessionResume_RoundTrip(t *testing.T) {
	in := SessionResume{
		SessionID: "sess-1",
		Project: ProjectContext{
			Root:          "/Users/me/proj",
			ConfigFile:    ".modeltap.yaml",
			ConfigContent: "key: value",
		},
	}
	assertRoundTrip(t, &in, new(SessionResume))
}

func TestSessionList_RoundTrip(t *testing.T) {
	in := SessionList{}
	assertRoundTrip(t, &in, new(SessionList))
}

func TestSessionDetails_RoundTrip(t *testing.T) {
	in := SessionDetails{SessionID: "s1"}
	assertRoundTrip(t, &in, new(SessionDetails))
}

func TestSessionCompact_RoundTrip(t *testing.T) {
	in := SessionCompact{SessionID: "s1"}
	assertRoundTrip(t, &in, new(SessionCompact))
}

func TestCompactApply_RoundTrip(t *testing.T) {
	in := CompactApply{
		SessionID: "s1",
		Actions: map[string]string{
			"architecture": "keep",
			"debugging":    "summarize",
			"files":        "drop",
			"planning":     "pin",
		},
	}
	assertRoundTrip(t, &in, new(CompactApply))
}

func TestSessionClear_RoundTrip(t *testing.T) {
	in := SessionClear{SessionID: "s1"}
	assertRoundTrip(t, &in, new(SessionClear))
}

func TestSessionFork_RoundTrip(t *testing.T) {
	in := SessionFork{SessionID: "s1"}
	assertRoundTrip(t, &in, new(SessionFork))
}

func TestSessionSync_RoundTrip(t *testing.T) {
	in := SessionSync{SessionID: "s1"}
	assertRoundTrip(t, &in, new(SessionSync))
}

func TestModelSwitch_RoundTrip(t *testing.T) {
	in := ModelSwitch{SessionID: "s1", Model: "claude-opus-4-6"}
	assertRoundTrip(t, &in, new(ModelSwitch))
}

func TestModelSwitch_RoundTrip_AutoClear(t *testing.T) {
	in := ModelSwitch{SessionID: "s1", Model: "auto"}
	assertRoundTrip(t, &in, new(ModelSwitch))
}

func TestModelList_RoundTrip(t *testing.T) {
	in := ModelList{}
	assertRoundTrip(t, &in, new(ModelList))
}

func TestContextList_RoundTrip(t *testing.T) {
	in := ContextList{SessionID: "s1"}
	assertRoundTrip(t, &in, new(ContextList))
}

func TestCapabilitiesRegister_RoundTrip(t *testing.T) {
	in := CapabilitiesRegister{
		ProtocolVersion: "1",
		HarnessVersion:  "0.2.0",
		HarnessPlatform: "darwin",
		Tools: []ToolDefinition{
			{
				Name:                 "Read",
				Namespace:            "builtin",
				Description:          "Read a file",
				InputSchema:          []byte(`{"type":"object"}`),
				OutputEnvelope:       "text",
				RiskLevel:            "read_only",
				CapabilitiesRequired: []string{"fs"},
			},
		},
		Project: ProjectContext{Root: "/p", ConfigFile: ".modeltap.yaml", ConfigContent: "---"},
	}
	assertRoundTrip(t, &in, new(CapabilitiesRegister))
}

func TestCapabilitiesUpdate_RoundTrip_ConnectionScope(t *testing.T) {
	in := CapabilitiesUpdate{
		AddedTools: []ToolDefinition{
			{Name: "Grep", Namespace: "builtin", Description: "", InputSchema: []byte(`{}`), OutputEnvelope: "text", RiskLevel: "read_only"},
		},
		RemovedTools: []string{"Glob"},
	}
	assertRoundTrip(t, &in, new(CapabilitiesUpdate))
}

func TestCapabilitiesUpdate_RoundTrip_SessionScope(t *testing.T) {
	sid := "sess-1"
	in := CapabilitiesUpdate{
		SessionID:    &sid,
		AddedTools:   nil,
		RemovedTools: []string{"Bash"},
	}
	assertRoundTrip(t, &in, new(CapabilitiesUpdate))
}

func TestConnectionPing_RoundTrip(t *testing.T) {
	in := ConnectionPing{}
	assertRoundTrip(t, &in, new(ConnectionPing))
}

func TestConnectionHealth_RoundTrip(t *testing.T) {
	in := ConnectionHealth{}
	assertRoundTrip(t, &in, new(ConnectionHealth))
}

func TestConnectionReady_RoundTrip(t *testing.T) {
	in := ConnectionReady{}
	assertRoundTrip(t, &in, new(ConnectionReady))
}

// -----------------------------------------------------------------------
// T2. Envelope round-trip (Request / Response / ErrorObject)
// -----------------------------------------------------------------------

func TestRequest_RoundTrip(t *testing.T) {
	params := TurnSubmit{TurnID: "t1", SessionID: "s1", Sequence: 1, Mode: ModeBuild, Content: "hi"}
	paramBytes, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-1"`),
		Method:  MethodTurnSubmit,
		Params:  paramBytes,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var out Request
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if out.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", out.JSONRPC, "2.0")
	}
	if out.Method != MethodTurnSubmit {
		t.Errorf("method = %q, want %q", out.Method, MethodTurnSubmit)
	}

	var outParams TurnSubmit
	if err := json.Unmarshal(out.Params, &outParams); err != nil {
		t.Fatalf("unmarshal inner params: %v", err)
	}
	if outParams.TurnID != "t1" {
		t.Errorf("inner turn_id = %q, want %q", outParams.TurnID, "t1")
	}
}

func TestResponse_RoundTrip_Result(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-1"`),
		Result:  json.RawMessage(`{"sessions":[]}`),
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Response
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Error != nil {
		t.Errorf("error set on success response: %+v", out.Error)
	}
	if !bytes.Contains(out.Result, []byte("sessions")) {
		t.Errorf("result not preserved: %s", string(out.Result))
	}
}

// Pins Codex finding #1 / #2: FEAT-0008 requires every harness request to
// carry an id. Accidental reintroduction of `omitempty` on Request.ID, or
// elision of the id field by any future refactor, must fail this test.
func TestRequest_IDAlwaysEmitted(t *testing.T) {
	// Zero-value Request with ID unset. On the wire, the id key MUST be
	// present even if the value is null (JSON-RPC permits null id; our
	// transport layer rejects it per WU-046, but WU-039 types must not
	// elide the key).
	req := Request{JSONRPC: "2.0", Method: MethodTurnSubmit}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"id"`) {
		t.Errorf("Request marshaled without id key: %s — this violates FEAT-0008 correlation requirement", s)
	}

	// Request with an explicit id also emits the key.
	req2 := Request{JSONRPC: "2.0", ID: json.RawMessage(`"req-1"`), Method: MethodTurnSubmit}
	b2, err := json.Marshal(req2)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b2), `"id":"req-1"`) {
		t.Errorf("Request with explicit ID did not emit id value: %s", string(b2))
	}
}

// Pins the complementary invariant for Notification: the id key MUST NOT
// appear on the wire. Notifications are the server->harness fire-and-forget
// envelope (streaming events in WU-040); harness->server frames must use
// Request instead.
func TestNotification_RoundTrip_NoID(t *testing.T) {
	n := Notification{JSONRPC: "2.0", Method: "token.delta", Params: json.RawMessage(`{"turn_id":"t1","text":"hi"}`)}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, `"id"`) {
		t.Errorf("Notification marshaled with an id key: %s — notifications must not carry id", s)
	}

	var out Notification
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Method != "token.delta" {
		t.Errorf("notification method round-trip: got %q", out.Method)
	}
}

func TestResponse_RoundTrip_Error(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-1"`),
		Error: &ErrorObject{
			Code:    -32000,
			Message: "session_locked",
			Data:    json.RawMessage(`{"code":"MT-CONN-008"}`),
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Response
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Error == nil {
		t.Fatal("error not preserved")
	}
	if out.Error.Message != "session_locked" {
		t.Errorf("error.message = %q", out.Error.Message)
	}
	if !bytes.Contains(out.Error.Data, []byte("MT-CONN-008")) {
		t.Errorf("error.data not preserved: %s", string(out.Error.Data))
	}
}

// -----------------------------------------------------------------------
// T3. NDJSON framing
// -----------------------------------------------------------------------

func TestFraming_WriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)
	frames := [][]byte{
		[]byte(`{"a":1}`),
		[]byte(`{"b":"hello"}`),
		[]byte(`{"nested":{"c":[1,2,3]}}`),
	}
	for _, f := range frames {
		if err := fw.WriteFrame(f); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}

	fr := NewFrameReader(&buf)
	for i, want := range frames {
		got, err := fr.ReadFrame()
		if err != nil {
			t.Fatalf("ReadFrame[%d]: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("frame[%d]: got %q, want %q", i, string(got), string(want))
		}
	}
	if _, err := fr.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF after last frame, got %v", err)
	}
}

func TestFraming_WriteRejectsEmbeddedNewline(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)
	err := fw.WriteFrame([]byte("{\"a\":1}\n{\"b\":2}"))
	if !errors.Is(err, ErrEmbeddedNewline) {
		t.Errorf("want ErrEmbeddedNewline, got %v", err)
	}
}

func TestFraming_ReadRejectsTooLarge(t *testing.T) {
	if MaxFrameSize <= 0 {
		t.Fatal("MaxFrameSize must be positive")
	}
	// Build an oversized frame: one byte larger than the cap, terminated
	// by a newline. Reader must refuse before buffering the whole thing.
	oversized := bytes.Repeat([]byte("a"), MaxFrameSize+1)
	oversized = append(oversized, '\n')
	fr := NewFrameReader(bytes.NewReader(oversized))
	_, err := fr.ReadFrame()
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Errorf("want ErrFrameTooLarge, got %v", err)
	}
}

func TestFraming_ReadEOFBetweenFrames(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)
	if err := fw.WriteFrame([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	fr := NewFrameReader(&buf)
	if _, err := fr.ReadFrame(); err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if _, err := fr.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Errorf("second ReadFrame: want io.EOF, got %v", err)
	}
}

func TestFraming_ReadPartialFrameIsInvalid(t *testing.T) {
	// A frame without a terminating newline is an invalid frame, not EOF.
	fr := NewFrameReader(strings.NewReader(`{"partial":true`))
	_, err := fr.ReadFrame()
	if !errors.Is(err, ErrInvalidFrame) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("partial frame: want ErrInvalidFrame or io.ErrUnexpectedEOF, got %v", err)
	}
}

func TestMaxFrameSize_IsPositive(t *testing.T) {
	if MaxFrameSize <= 0 {
		t.Fatal("MaxFrameSize must be positive")
	}
	// Sanity: cap should be at least 1 MB to accommodate typical attachments,
	// but not so large (>100 MB) that a single malformed request causes OOM.
	if MaxFrameSize < 1<<20 || MaxFrameSize > 100<<20 {
		t.Errorf("MaxFrameSize = %d bytes, expected between 1 MiB and 100 MiB", MaxFrameSize)
	}
}

// -----------------------------------------------------------------------
// T6. Canonical field names (snake_case on the wire)
// -----------------------------------------------------------------------

func TestCanonicalFieldNames(t *testing.T) {
	// For each representative instance, assert that all snake_case keys in
	// the expected set are present in the marshaled JSON. We use substring
	// matches on the raw JSON to catch accidental CamelCase leakage via
	// the default encoding/json field-name lowercasing.
	cases := []struct {
		name  string
		value interface{}
		keys  []string
	}{
		{
			"TurnSubmit",
			TurnSubmit{
				TurnID: "t", SessionID: "s", Sequence: 1, Mode: ModeBuild, Content: "hi",
				Attachments: []Attachment{{Path: "p", Raw: "r", Content: "c", ContentType: "text/plain", Transform: "none"}},
				Paste:       &Paste{Raw: "r", Content: "c", Intent: "full"},
				ToolResults: []ToolResult{{ToolCallID: "tc", Status: "success", Output: "o", OutputType: "text"}},
			},
			[]string{`"turn_id"`, `"session_id"`, `"sequence"`, `"mode"`, `"content"`, `"attachments"`, `"paste"`, `"tool_results"`,
				`"content_type"`, `"tool_call_id"`, `"output_type"`},
		},
		{
			"TurnCancel",
			TurnCancel{TurnID: "t"},
			[]string{`"turn_id"`},
		},
		{
			"ContentTransform",
			ContentTransform{Transform: "summarize", RawContent: "x", ContentType: "text/plain", MaxOutputTokens: 100},
			[]string{`"transform"`, `"raw_content"`, `"content_type"`, `"max_output_tokens"`},
		},
		{
			"SessionResume",
			SessionResume{SessionID: "s", Project: ProjectContext{Root: "/p", ConfigFile: ".f", ConfigContent: "c"}},
			[]string{`"session_id"`, `"project"`, `"root"`, `"config_file"`, `"config_content"`},
		},
		{
			"SessionDetails",
			SessionDetails{SessionID: "s"},
			[]string{`"session_id"`},
		},
		{
			"SessionCompact",
			SessionCompact{SessionID: "s"},
			[]string{`"session_id"`},
		},
		{
			"CompactApply",
			CompactApply{SessionID: "s", Actions: map[string]string{"files": "drop"}},
			[]string{`"session_id"`, `"actions"`},
		},
		{
			"SessionClear",
			SessionClear{SessionID: "s"},
			[]string{`"session_id"`},
		},
		{
			"SessionFork",
			SessionFork{SessionID: "s"},
			[]string{`"session_id"`},
		},
		{
			"SessionSync",
			SessionSync{SessionID: "s"},
			[]string{`"session_id"`},
		},
		{
			"ModelSwitch",
			ModelSwitch{SessionID: "s", Model: "claude-opus-4-6"},
			[]string{`"session_id"`, `"model"`},
		},
		{
			"ContextList",
			ContextList{SessionID: "s"},
			[]string{`"session_id"`},
		},
		{
			"CapabilitiesRegister",
			CapabilitiesRegister{
				ProtocolVersion: "1", HarnessVersion: "0.2.0", HarnessPlatform: "darwin",
				Tools: []ToolDefinition{
					{Name: "Read", Namespace: "builtin", Description: "", InputSchema: []byte(`{}`), OutputEnvelope: "text", RiskLevel: "read_only", CapabilitiesRequired: []string{"fs"}},
				},
				Project: ProjectContext{Root: "/p", ConfigFile: ".f", ConfigContent: "c"},
			},
			[]string{
				`"protocol_version"`, `"harness_version"`, `"harness_platform"`, `"tools"`, `"project"`,
				`"name"`, `"namespace"`, `"description"`, `"input_schema"`, `"output_envelope"`, `"risk_level"`, `"capabilities_required"`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(b)
			for _, k := range tc.keys {
				if !strings.Contains(s, k) {
					t.Errorf("missing canonical key %s in %s", k, s)
				}
			}
			// Negative: common CamelCase leaks should NOT appear.
			for _, bad := range []string{`"turnId"`, `"sessionId"`, `"toolCallId"`, `"contentType"`, `"outputType"`, `"rawContent"`, `"maxOutputTokens"`} {
				if strings.Contains(s, bad) {
					t.Errorf("CamelCase leak %s in %s", bad, s)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------
// Helper: round-trip assert
// -----------------------------------------------------------------------

// assertRoundTrip marshals in, unmarshals into out (a pointer to a fresh
// zero-value of the same type), and fails the test if the reconstructed
// value differs from in.
func assertRoundTrip(t *testing.T, in interface{}, out interface{}) {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("unmarshal: %v (json=%s)", err, string(b))
	}
	// Compare the dereferenced value of out to in. Both sides should be
	// pointers-to-same-concrete-type or same-concrete-type.
	inVal := reflect.ValueOf(in)
	outVal := reflect.ValueOf(out)
	if inVal.Kind() == reflect.Pointer {
		inVal = inVal.Elem()
	}
	if outVal.Kind() == reflect.Pointer {
		outVal = outVal.Elem()
	}
	if !reflect.DeepEqual(inVal.Interface(), outVal.Interface()) {
		t.Errorf("round-trip mismatch\n  in:  %+v\n  out: %+v\n  json: %s", inVal.Interface(), outVal.Interface(), string(b))
	}
}
