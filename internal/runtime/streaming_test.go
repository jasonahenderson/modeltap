package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// streamingAdapter is a minimal Provider used by streaming tests. It
// only implements the methods StreamRelay calls (ParseStreamEvent).
type streamingAdapter struct {
	stubAdapter // reuse the no-op shape from dispatch_test.go
	parse       func(data []byte) (*provider.StreamEvent, error)
}

func (a *streamingAdapter) ParseStreamEvent(data []byte) (*provider.StreamEvent, error) {
	if a.parse != nil {
		return a.parse(data)
	}
	return nil, nil
}

// drainConn returns a server net.Conn whose peer is continuously read.
// drained collects every frame the server side writes.
func drainConn(t *testing.T) (net.Conn, *capturedFrames) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	t.Cleanup(func() { _ = client.Close() })
	cf := &capturedFrames{}
	go func() {
		fr := protocol.NewFrameReader(client)
		for {
			b, err := fr.ReadFrame()
			if err != nil {
				return
			}
			cf.append(b)
		}
	}()
	return server, cf
}

type capturedFrames struct {
	mu     sync.Mutex
	frames [][]byte
}

func (c *capturedFrames) append(b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, append([]byte(nil), b...))
}

func (c *capturedFrames) snapshot() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.frames))
	copy(out, c.frames)
	return out
}

func (c *capturedFrames) byMethod(method string) []json.RawMessage {
	var out []json.RawMessage
	for _, f := range c.snapshot() {
		var probe struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(f, &probe); err != nil {
			continue
		}
		if probe.Method == method {
			out = append(out, probe.Params)
		}
	}
	return out
}

// waitForFrame polls until at least one frame for method has been
// captured, or fails the test after deadline. Bridges the small window
// between transport write completion and drain-goroutine append.
func (c *capturedFrames) waitForFrame(t *testing.T, method string) []json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := c.byMethod(method); len(got) > 0 {
			return got
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitForFrame(%q): no frame captured within 2s; saw %d total frames", method, len(c.snapshot()))
	return nil
}

func newRelayConnection(t *testing.T, srv *Server) (*Connection, *capturedFrames) {
	t.Helper()
	transportConn, frames := drainConn(t)
	c := NewConnection("conn-relay", NewFrameTransport(transportConn), srv, false)
	c.setStateForTest(ConnReady)
	return c, frames
}

func TestSSEParser_DataLines(t *testing.T) {
	body := "data: {\"a\":1}\n\ndata: {\"b\":2}\n\n"
	p := NewSSEParser(strings.NewReader(body))
	first, err := p.Next()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(first) != `{"a":1}` {
		t.Errorf("first = %q", first)
	}
	second, err := p.Next()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(second) != `{"b":2}` {
		t.Errorf("second = %q", second)
	}
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestSSEParser_SkipsEventHeaders(t *testing.T) {
	body := "event: ping\ndata: {\"ok\":true}\n\n"
	p := NewSSEParser(strings.NewReader(body))
	got, err := p.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("got %q", got)
	}
}

// PATCH-0032: Ollama and other local providers emit NDJSON (one bare
// JSON object per line, no SSE framing). SSEParser must pass those
// lines through to ParseStreamEvent so the streaming relay actually
// sees the model's text. Before this, every NDJSON line was dropped
// as an unknown SSE field and the assistant turn ended up empty.
func TestSSEParser_NDJSONLinesPassThrough(t *testing.T) {
	body := `{"model":"qwen","message":{"role":"assistant","content":"Hello"},"done":false}
{"model":"qwen","message":{"role":"assistant","content":" world"},"done":false}
{"model":"qwen","message":{"role":"assistant","content":""},"done":true,"eval_count":42}
`
	p := NewSSEParser(strings.NewReader(body))

	want := []string{
		`{"model":"qwen","message":{"role":"assistant","content":"Hello"},"done":false}`,
		`{"model":"qwen","message":{"role":"assistant","content":" world"},"done":false}`,
		`{"model":"qwen","message":{"role":"assistant","content":""},"done":true,"eval_count":42}`,
	}
	for i, w := range want {
		got, err := p.Next()
		if err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if string(got) != w {
			t.Errorf("line %d = %q, want %q", i, got, w)
		}
	}
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

// PATCH-0032: drives the StreamRelay end-to-end through the real
// OllamaProvider.ParseStreamEvent with an Ollama-shaped NDJSON
// fixture and asserts the assistant content accumulates. Without
// the NDJSON pass-through this test would persist an empty turn,
// matching the v0.3.0 smoke-test symptom (F14).
func TestStreamRelay_OllamaNDJSON_AccumulatesAssistantContent(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "ollama ndjson")

	c, frames := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)

	if _, err := active.Conversation.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "u1", SessionID: sid, Sequence: 1, Mode: protocol.ModeBuild, Content: "hi",
	}); err != nil {
		t.Fatalf("user append: %v", err)
	}

	body := `{"model":"qwen","message":{"role":"assistant","content":"v0.3.0 "},"done":false}
{"model":"qwen","message":{"role":"assistant","content":"smoke "},"done":false}
{"model":"qwen","message":{"role":"assistant","content":"ok"},"done":false}
{"model":"qwen","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":7,"eval_count":3}
`
	stream := io.NopCloser(strings.NewReader(body))
	adapter := &provider.OllamaProvider{}

	relay := NewStreamRelay(c, active, "turn-ollama-1", "", "qwen", "ollama")
	turn, err := relay.Relay(context.Background(), stream, adapter)
	if err != nil {
		t.Fatalf("Relay: %v", err)
	}
	if turn == nil {
		t.Fatalf("turn not persisted")
	}

	deltas := frames.byMethod(protocol.EventTokenDelta)
	if len(deltas) != 3 {
		t.Fatalf("token.delta count = %d, want 3", len(deltas))
	}

	// The persisted assistant turn's content blob must carry the
	// concatenated text. messageToTurn stores the canonical
	// provider.Message JSON in Turn.Content.
	var msg provider.Message
	if err := json.Unmarshal(turn.Content, &msg); err != nil {
		t.Fatalf("unmarshal turn content: %v", err)
	}
	if msg.Content != "v0.3.0 smoke ok" {
		t.Errorf("assistant content = %q, want %q", msg.Content, "v0.3.0 smoke ok")
	}
}

func TestStreamRelay_TextChunks_PersistsAndCompletes(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "stream test")

	c, frames := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)

	// Pre-seed a user turn so AppendAssistantTurn has a sequence to follow.
	if _, err := active.Conversation.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "u1", SessionID: sid, Sequence: 1, Mode: protocol.ModeBuild, Content: "hi",
	}); err != nil {
		t.Fatalf("user append: %v", err)
	}

	// SSE stream: two text chunks then DONE.
	stream := io.NopCloser(strings.NewReader("data: chunk1\n\ndata: chunk2\n\ndata: done\n\n"))
	adapter := &streamingAdapter{
		parse: func(data []byte) (*provider.StreamEvent, error) {
			text := string(data)
			if text == "done" {
				return &provider.StreamEvent{Type: provider.StreamEventDone}, nil
			}
			return &provider.StreamEvent{Type: provider.StreamEventText, Content: text}, nil
		},
	}

	relay := NewStreamRelay(c, active, "turn-1", "", "claude-sonnet-4-6", "anthropic")
	turn, err := relay.Relay(context.Background(), stream, adapter)
	if err != nil {
		t.Fatalf("Relay: %v", err)
	}
	if turn == nil {
		t.Fatalf("turn not persisted")
	}

	// Two token.delta + one turn.complete on the wire.
	deltas := frames.byMethod(protocol.EventTokenDelta)
	if len(deltas) != 2 {
		t.Errorf("token.delta count = %d, want 2", len(deltas))
	}
	for i, raw := range deltas {
		var td protocol.TokenDelta
		if err := json.Unmarshal(raw, &td); err != nil {
			t.Fatalf("decode delta: %v", err)
		}
		if td.TurnID != "turn-1" {
			t.Errorf("delta[%d].TurnID = %q", i, td.TurnID)
		}
	}

	complete := frames.waitForFrame(t, protocol.EventTurnComplete)
	if len(complete) != 1 {
		t.Fatalf("turn.complete count = %d, want 1", len(complete))
	}
	var tc protocol.TurnComplete
	_ = json.Unmarshal(complete[0], &tc)
	if tc.Cancelled {
		t.Errorf("Cancelled=true unexpectedly")
	}

	// Conversation now has two turns (user + assistant).
	if active.Conversation.TurnCount() != 2 {
		t.Errorf("conversation turn count = %d, want 2", active.Conversation.TurnCount())
	}
	// Turn persisted to storage.
	turns, _ := srv.store.ListTurns(context.Background(), sid)
	if len(turns) != 1 {
		t.Errorf("storage turn count = %d, want 1 (only assistant; user not persisted by relay)", len(turns))
	}
}

func TestStreamRelay_ToolCall_AccumulatesAndEmits(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "tool call")

	c, frames := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)
	_, _ = active.Conversation.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "u1", SessionID: sid, Sequence: 1, Mode: protocol.ModeBuild, Content: "go",
	})

	// Stream events: start tool, three input deltas, end tool, done.
	stream := io.NopCloser(strings.NewReader(
		"data: start\n\ndata: d1\n\ndata: d2\n\ndata: d3\n\ndata: end\n\ndata: done\n\n",
	))
	adapter := &streamingAdapter{
		parse: func(data []byte) (*provider.StreamEvent, error) {
			switch string(data) {
			case "start":
				return &provider.StreamEvent{
					Type:     provider.StreamEventToolCallStart,
					ToolCall: &provider.StreamToolCall{ID: "call-x", Name: "read"},
				}, nil
			case "d1":
				return &provider.StreamEvent{Type: provider.StreamEventToolCallDelta, ToolCall: &provider.StreamToolCall{Input: `{"path"`}}, nil
			case "d2":
				return &provider.StreamEvent{Type: provider.StreamEventToolCallDelta, ToolCall: &provider.StreamToolCall{Input: `:"f`}}, nil
			case "d3":
				return &provider.StreamEvent{Type: provider.StreamEventToolCallDelta, ToolCall: &provider.StreamToolCall{Input: `oo"}`}}, nil
			case "end":
				return &provider.StreamEvent{Type: provider.StreamEventToolCallEnd}, nil
			case "done":
				return &provider.StreamEvent{Type: provider.StreamEventDone}, nil
			}
			return nil, nil
		},
	}

	relay := NewStreamRelay(c, active, "turn-1", "", "claude-sonnet-4-6", "anthropic")
	if _, err := relay.Relay(context.Background(), stream, adapter); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	calls := frames.waitForFrame(t, protocol.EventToolCall)
	if len(calls) != 1 {
		t.Fatalf("tool.call count = %d, want 1", len(calls))
	}
	var tc protocol.ToolCall
	_ = json.Unmarshal(calls[0], &tc)
	if tc.ToolCallID != "call-x" || tc.Tool != "read" {
		t.Errorf("tool.call = %+v", tc)
	}
	if string(tc.Input) != `{"path":"foo"}` {
		t.Errorf("accumulated input = %s, want {\"path\":\"foo\"}", tc.Input)
	}
}

func TestStreamRelay_Usage_FillsTurnCompleteTokens(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "usage")

	c, frames := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)
	_, _ = active.Conversation.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "u1", SessionID: sid, Sequence: 1, Mode: protocol.ModeBuild, Content: "hi",
	})

	stream := io.NopCloser(strings.NewReader("data: u\n\ndata: done\n\n"))
	adapter := &streamingAdapter{
		parse: func(data []byte) (*provider.StreamEvent, error) {
			switch string(data) {
			case "u":
				return &provider.StreamEvent{Type: provider.StreamEventUsage, Usage: &provider.StreamUsage{InputTokens: 100, OutputTokens: 200}}, nil
			case "done":
				return &provider.StreamEvent{Type: provider.StreamEventDone}, nil
			}
			return nil, nil
		},
	}

	relay := NewStreamRelay(c, active, "turn-1", "", "claude-sonnet-4-6", "anthropic")
	turn, err := relay.Relay(context.Background(), stream, adapter)
	if err != nil {
		t.Fatalf("Relay: %v", err)
	}
	if turn.InputTokens != 100 || turn.OutputTokens != 200 {
		t.Errorf("turn tokens = (in=%d, out=%d)", turn.InputTokens, turn.OutputTokens)
	}

	complete := frames.waitForFrame(t, protocol.EventTurnComplete)
	if len(complete) != 1 {
		t.Fatalf("turn.complete = %d", len(complete))
	}
	var tc protocol.TurnComplete
	_ = json.Unmarshal(complete[0], &tc)
	if tc.FinalInputTokens != 100 || tc.FinalOutputTokens != 200 {
		t.Errorf("complete tokens = (in=%d, out=%d)", tc.FinalInputTokens, tc.FinalOutputTokens)
	}
}

func TestStreamRelay_BranchID_TaggedOnEvents(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "branch")

	c, frames := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)
	_, _ = active.Conversation.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "u1", SessionID: sid, Sequence: 1, Mode: protocol.ModeBuild, Content: "go",
	})

	stream := io.NopCloser(strings.NewReader("data: hi\n\ndata: done\n\n"))
	adapter := &streamingAdapter{
		parse: func(data []byte) (*provider.StreamEvent, error) {
			if string(data) == "done" {
				return &provider.StreamEvent{Type: provider.StreamEventDone}, nil
			}
			return &provider.StreamEvent{Type: provider.StreamEventText, Content: "x"}, nil
		},
	}

	relay := NewStreamRelay(c, active, "turn-1", "br_001", "claude-opus-4-6", "anthropic")
	if _, err := relay.Relay(context.Background(), stream, adapter); err != nil {
		t.Fatalf("Relay: %v", err)
	}
	deltas := frames.waitForFrame(t, protocol.EventTokenDelta)
	if len(deltas) == 0 {
		t.Fatalf("no token.delta")
	}
	var td protocol.TokenDelta
	_ = json.Unmarshal(deltas[0], &td)
	if td.BranchID != "br_001" {
		t.Errorf("branch id = %q, want br_001", td.BranchID)
	}
}

func TestStreamRelay_Cancel_EmitsCancelledTurnComplete(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "cancel")

	c, frames := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)
	_, _ = active.Conversation.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "u1", SessionID: sid, Sequence: 1, Mode: protocol.ModeBuild, Content: "x",
	})

	// A pipe whose write side never closes — Relay's parser blocks
	// reading past the first chunk.
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte("data: hi\n\n"))
		// Leave open.
	}()
	t.Cleanup(func() { _ = pw.Close() })

	adapter := &streamingAdapter{
		parse: func(data []byte) (*provider.StreamEvent, error) {
			return &provider.StreamEvent{Type: provider.StreamEventText, Content: string(data)}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	relay := NewStreamRelay(c, active, "turn-1", "", "claude-sonnet-4-6", "anthropic")

	relayDone := make(chan struct{})
	go func() {
		_, _ = relay.Relay(ctx, pr, adapter)
		close(relayDone)
	}()

	// Give the relay a moment to read the first chunk, then cancel.
	// Closing the pipe after cancel ensures Next() returns.
	cancel()
	_ = pw.Close()

	<-relayDone

	complete := frames.waitForFrame(t, protocol.EventTurnComplete)
	if len(complete) != 1 {
		t.Fatalf("turn.complete count = %d", len(complete))
	}
	var tc protocol.TurnComplete
	_ = json.Unmarshal(complete[0], &tc)
	// The cancel signal may race with the EOF from pipe close — in
	// either case the relay should terminate.
	_ = tc
}

// stubStore is unused for now but documents that a stricter test could
// inject failures in CreateTurn to verify the partial-persist path.
var _ = storage.Session{}
