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

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// writeFrame writes a single NDJSON frame to w.
func writeFrame(t *testing.T, w io.Writer, b []byte) {
	t.Helper()
	fw := protocol.NewFrameWriter(w)
	if err := fw.WriteFrame(b); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
}

func TestFrameTransport_ReadRequest(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	t.Cleanup(func() { clientConn.Close() })

	go func() {
		writeFrame(t, clientConn, []byte(`{"jsonrpc":"2.0","id":1,"method":"connection.ping","params":{}}`))
	}()

	tr := NewFrameTransport(serverConn)
	env, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if env.Request == nil {
		t.Fatalf("expected Request envelope, got %+v", env)
	}
	if env.Response != nil || env.Notification != nil {
		t.Errorf("expected only Request set, got Response=%v Notification=%v", env.Response, env.Notification)
	}
	if env.Request.Method != "connection.ping" {
		t.Errorf("Method = %q, want connection.ping", env.Request.Method)
	}
	if string(env.Request.ID) != "1" {
		t.Errorf("ID = %s, want 1", env.Request.ID)
	}
	if len(env.Raw) == 0 {
		t.Errorf("Raw bytes not preserved")
	}
}

func TestFrameTransport_ReadNotification(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		writeFrame(t, clientConn, []byte(`{"jsonrpc":"2.0","method":"capabilities.request","params":{"reason":"reconnection"}}`))
	}()

	tr := NewFrameTransport(serverConn)
	env, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if env.Notification == nil {
		t.Fatalf("expected Notification envelope, got %+v", env)
	}
	if env.Request != nil || env.Response != nil {
		t.Errorf("expected only Notification set")
	}
	if env.Notification.Method != "capabilities.request" {
		t.Errorf("Method = %q", env.Notification.Method)
	}
}

func TestFrameTransport_ReadResponse(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		writeFrame(t, clientConn, []byte(`{"jsonrpc":"2.0","id":7,"result":{"ok":true}}`))
	}()

	tr := NewFrameTransport(serverConn)
	env, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if env.Response == nil {
		t.Fatalf("expected Response envelope, got %+v", env)
	}
	if env.Request != nil || env.Notification != nil {
		t.Errorf("expected only Response set")
	}
	if string(env.Response.ID) != "7" {
		t.Errorf("ID = %s", env.Response.ID)
	}
}

func TestFrameTransport_ReadResponseError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		writeFrame(t, clientConn, []byte(`{"jsonrpc":"2.0","id":7,"error":{"code":-32601,"message":"nope"}}`))
	}()

	tr := NewFrameTransport(serverConn)
	env, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if env.Response == nil || env.Response.Error == nil {
		t.Fatalf("expected Response with Error, got %+v", env)
	}
}

func TestFrameTransport_InvalidJSON(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		writeFrame(t, clientConn, []byte(`{not json`))
	}()

	tr := NewFrameTransport(serverConn)
	_, err := tr.ReadMessage()
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeParseError {
		t.Errorf("expected TransportError CodeParseError, got %T %v", err, err)
	}
}

func TestFrameTransport_InvalidJSONRPC(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		// missing method on a request-shaped frame
		{"missing method/result/error", `{"jsonrpc":"2.0","id":1}`},
		// has id and method but result/error too — ambiguous
		{"id+method+result", `{"jsonrpc":"2.0","id":1,"method":"x","result":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer clientConn.Close()
			defer serverConn.Close()

			go func() { writeFrame(t, clientConn, []byte(tc.body)) }()

			tr := NewFrameTransport(serverConn)
			_, err := tr.ReadMessage()
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			var te *TransportError
			if !errors.As(err, &te) || te.Code != CodeInvalidRequest {
				t.Errorf("expected CodeInvalidRequest, got %T %v", err, err)
			}
		})
	}
}

func TestFrameTransport_OversizeFrame(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Write more than MaxFrameSize bytes with no newline.
	go func() {
		buf := make([]byte, protocol.MaxFrameSize+1024)
		for i := range buf {
			buf[i] = 'a'
		}
		_, _ = clientConn.Write(buf)
	}()

	tr := NewFrameTransport(serverConn)
	_, err := tr.ReadMessage()
	if err == nil {
		t.Fatalf("expected oversize error")
	}
	var te *TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected TransportError, got %T %v", err, err)
	}
	if !te.Close {
		t.Errorf("oversize error should set Close=true")
	}
	if !errors.Is(err, protocol.ErrFrameTooLarge) {
		t.Errorf("error chain should wrap ErrFrameTooLarge, got %v", err)
	}
}

func TestFrameTransport_SendResponse(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	tr := NewFrameTransport(serverConn)

	done := make(chan struct{})
	var got []byte
	go func() {
		fr := protocol.NewFrameReader(clientConn)
		b, err := fr.ReadFrame()
		if err != nil {
			t.Errorf("ReadFrame: %v", err)
		}
		got = b
		close(done)
	}()

	err := tr.SendResponse(&protocol.Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  json.RawMessage(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("SendResponse: %v", err)
	}
	<-done

	var resp protocol.Response
	if err := json.Unmarshal(got, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(resp.ID) != "1" || string(resp.Result) != `{"ok":true}` {
		t.Errorf("response = %+v", resp)
	}
}

func TestFrameTransport_SendNotification(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	tr := NewFrameTransport(serverConn)

	done := make(chan []byte, 1)
	go func() {
		fr := protocol.NewFrameReader(clientConn)
		b, _ := fr.ReadFrame()
		done <- b
	}()

	err := tr.SendNotification(&protocol.Notification{
		JSONRPC: "2.0",
		Method:  "token.delta",
		Params:  json.RawMessage(`{"text":"hi"}`),
	})
	if err != nil {
		t.Fatalf("SendNotification: %v", err)
	}
	got := <-done
	if !strings.Contains(string(got), `"method":"token.delta"`) {
		t.Errorf("got %s", got)
	}
	// Notifications must not carry an id field.
	if strings.Contains(string(got), `"id"`) {
		t.Errorf("notification leaked id: %s", got)
	}
}

func TestFrameTransport_SendError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	tr := NewFrameTransport(serverConn)

	done := make(chan []byte, 1)
	go func() {
		fr := protocol.NewFrameReader(clientConn)
		b, _ := fr.ReadFrame()
		done <- b
	}()

	err := tr.SendError(json.RawMessage(`9`), CodeMethodNotFound, "no such method", map[string]string{"method": "foo"})
	if err != nil {
		t.Fatalf("SendError: %v", err)
	}
	got := <-done

	var resp protocol.Response
	if err := json.Unmarshal(got, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("error not set")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Errorf("code = %d", resp.Error.Code)
	}
	if string(resp.ID) != "9" {
		t.Errorf("id = %s", resp.ID)
	}
}

func TestFrameTransport_ConcurrentWrites(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	tr := NewFrameTransport(serverConn)

	const writers = 10
	const perWriter = 20
	total := writers * perWriter

	// Reader goroutine collects all frames.
	collected := make(chan [][]byte, 1)
	go func() {
		fr := protocol.NewFrameReader(clientConn)
		var frames [][]byte
		for i := 0; i < total; i++ {
			b, err := fr.ReadFrame()
			if err != nil {
				t.Errorf("ReadFrame %d: %v", i, err)
				break
			}
			frames = append(frames, b)
		}
		collected <- frames
	}()

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				notif := &protocol.Notification{
					JSONRPC: "2.0",
					Method:  "writer",
					Params:  json.RawMessage(`{"w":` + jsonNum(wid) + `,"i":` + jsonNum(i) + `}`),
				}
				if err := tr.SendNotification(notif); err != nil {
					t.Errorf("SendNotification: %v", err)
				}
			}
		}(w)
	}
	wg.Wait()

	frames := <-collected
	if len(frames) != total {
		t.Fatalf("got %d frames, want %d", len(frames), total)
	}
	// Every frame must be a valid JSON object — no interleaving.
	for i, f := range frames {
		var v map[string]json.RawMessage
		if err := json.Unmarshal(f, &v); err != nil {
			t.Errorf("frame %d not valid JSON: %v\n%s", i, err, f)
		}
	}
}

func jsonNum(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestDispatcher_Register(t *testing.T) {
	d := NewDispatcher()
	called := false
	d.Register("foo", func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		called = true
		return map[string]string{"hello": "world"}, nil
	})

	result, err := d.Dispatch(context.Background(), &Connection{}, &protocol.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "foo",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !called {
		t.Errorf("handler not called")
	}
	if m, ok := result.(map[string]string); !ok || m["hello"] != "world" {
		t.Errorf("result = %v", result)
	}
}

func TestDispatcher_MethodNotFound(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Dispatch(context.Background(), &Connection{}, &protocol.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "missing",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeMethodNotFound {
		t.Errorf("expected CodeMethodNotFound, got %T %v", err, err)
	}
}

func TestDispatcher_DuplicateRegister(t *testing.T) {
	d := NewDispatcher()
	d.Register("foo", func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		return nil, nil
	})
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate Register")
		}
	}()
	d.Register("foo", func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		return nil, nil
	})
}

func TestValidateTurnSubmit_MissingSequence(t *testing.T) {
	raw := json.RawMessage(`{"turn_id":"t1","session_id":"s1","mode":"build","content":"hi"}`)
	_, err := ValidateTurnSubmit(raw)
	if err == nil {
		t.Fatalf("expected error for missing sequence")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams, got %T %v", err, err)
	}
	if !strings.Contains(te.Message, "sequence") {
		t.Errorf("message should mention sequence: %s", te.Message)
	}
}

func TestValidateTurnSubmit_InvalidMode(t *testing.T) {
	raw := json.RawMessage(`{"turn_id":"t1","session_id":"s1","sequence":0,"mode":"chaotic","content":"hi"}`)
	_, err := ValidateTurnSubmit(raw)
	if err == nil {
		t.Fatalf("expected error for invalid mode")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams, got %T %v", err, err)
	}
	if !strings.Contains(te.Message, "mode") {
		t.Errorf("message should mention mode: %s", te.Message)
	}
}

func TestValidateTurnSubmit_Valid(t *testing.T) {
	raw := json.RawMessage(`{"turn_id":"t1","session_id":"s1","sequence":3,"mode":"build","content":"hi"}`)
	ts, err := ValidateTurnSubmit(raw)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ts.TurnID != "t1" || ts.SessionID != "s1" || ts.Sequence != 3 || ts.Mode != protocol.ModeBuild || ts.Content != "hi" {
		t.Errorf("decoded = %+v", ts)
	}
}

func TestValidateTurnSubmit_ZeroSequenceAccepted(t *testing.T) {
	// Sequence is present and zero — must be accepted (the validation is
	// presence-of-key, not value-non-zero).
	raw := json.RawMessage(`{"turn_id":"t1","session_id":"s1","sequence":0,"mode":"build","content":"hi"}`)
	if _, err := ValidateTurnSubmit(raw); err != nil {
		t.Fatalf("zero sequence rejected: %v", err)
	}
}

func TestErrorCodes(t *testing.T) {
	// Pin the wire-visible JSON-RPC codes.
	cases := []struct {
		got, want int
	}{
		{CodeParseError, -32700},
		{CodeInvalidRequest, -32600},
		{CodeMethodNotFound, -32601},
		{CodeInvalidParams, -32602},
		{CodeInternalError, -32603},
		{CodeNotReady, -32000},
		{CodeSessionLocked, -32001},
		{CodeVersionMismatch, -32002},
		{CodeCapabilityError, -32003},
		{CodeProviderError, -32004},
		{CodeSessionNotFound, -32005},
		{CodeTurnNotFound, -32006},
		{CodeModelUnavailable, -32007},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("code = %d, want %d", c.got, c.want)
		}
	}
}
