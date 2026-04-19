package harness

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// mockServer is a minimal in-memory JSON-RPC server backed by a
// net.Pipe. It echoes scripted responses for incoming requests and
// can push notifications on demand.
type mockServer struct {
	conn   net.Conn
	reader *protocol.FrameReader
	writer *protocol.FrameWriter

	mu       sync.Mutex
	handlers map[string]func(req *protocol.Request) *protocol.Response

	t *testing.T
}

func newMockServer(t *testing.T) (clientConn net.Conn, srv *mockServer) {
	t.Helper()
	a, b := net.Pipe()
	srv = &mockServer{
		conn:     a,
		reader:   protocol.NewFrameReader(a),
		writer:   protocol.NewFrameWriter(a),
		handlers: make(map[string]func(*protocol.Request) *protocol.Response),
		t:        t,
	}
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	go srv.serve()
	return b, srv
}

func (s *mockServer) handle(method string, h func(req *protocol.Request) *protocol.Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

func (s *mockServer) serve() {
	for {
		raw, err := s.reader.ReadFrame()
		if err != nil {
			return
		}
		var req protocol.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}
		s.mu.Lock()
		h, ok := s.handlers[req.Method]
		s.mu.Unlock()
		if !ok {
			continue
		}
		resp := h(&req)
		if resp == nil {
			continue
		}
		out, _ := json.Marshal(resp)
		_ = s.writer.WriteFrame(out)
	}
}

func (s *mockServer) sendNotification(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	notif := protocol.Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	}
	frame, err := json.Marshal(&notif)
	if err != nil {
		return err
	}
	return s.writer.WriteFrame(frame)
}

// dialPipeClient adapts the mockServer's pipe into a *ProtocolClient
// without going through net.Listen.
func dialPipeClient(t *testing.T, conn net.Conn, handler EventHandler) *ProtocolClient {
	t.Helper()
	clientCtx, cancel := context.WithCancel(context.Background())
	c := &ProtocolClient{
		conn:         conn,
		reader:       protocol.NewFrameReader(conn),
		writer:       protocol.NewFrameWriter(conn),
		pending:      make(map[string]chan *protocol.Response),
		eventHandler: handler,
		ctx:          clientCtx,
		cancel:       cancel,
		doneCh:       make(chan struct{}),
	}
	go c.readLoop()
	t.Cleanup(func() {
		_ = c.Close()
	})
	return c
}

func TestClient_Call_Success(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	srv.handle("echo", func(req *protocol.Request) *protocol.Response {
		return &protocol.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"ok":true}`),
		}
	})

	raw, err := c.Call(context.Background(), "echo", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Errorf("result = %s", raw)
	}
}

func TestClient_Call_RPCError(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	srv.handle("fail", func(req *protocol.Request) *protocol.Response {
		return &protocol.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorObject{
				Code:    -32000,
				Message: "boom",
			},
		}
	})

	_, err := c.Call(context.Background(), "fail", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsRPCError(err, -32000) {
		t.Errorf("expected RPCError(-32000), got %T %v", err, err)
	}
}

func TestClient_Call_Timeout(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	srv.handle("hang", func(req *protocol.Request) *protocol.Response { return nil })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Call(ctx, "hang", nil)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestClient_Call_Concurrent_CorrelatesByID(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	srv.handle("echo", func(req *protocol.Request) *protocol.Response {
		return &protocol.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  req.Params, // echo back the params so we can verify
		}
	})

	const n = 25
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload := map[string]int{"i": i}
			raw, err := c.Call(context.Background(), "echo", payload)
			if err != nil {
				errs <- err
				return
			}
			var got map[string]int
			if err := json.Unmarshal(raw, &got); err != nil {
				errs <- err
				return
			}
			if got["i"] != i {
				errs <- errors.New("response did not correlate to caller's payload")
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("concurrent call: %v", e)
	}
}

func TestClient_Notification_Dispatched(t *testing.T) {
	clientConn, srv := newMockServer(t)
	var received atomic.Int32
	handler := EventHandlerFunc(func(method string, params json.RawMessage) {
		if method == protocol.EventTokenDelta {
			received.Add(1)
		}
	})
	_ = dialPipeClient(t, clientConn, handler)

	if err := srv.sendNotification(protocol.EventTokenDelta, &protocol.TokenDelta{TurnID: "t1", Text: "hi"}); err != nil {
		t.Fatalf("sendNotification: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if received.Load() == 1 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("notification not dispatched (received=%d)", received.Load())
}

func TestClient_ReadLoop_EOF_ClosesDone(t *testing.T) {
	clientConn, _ := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	// Close the client side; read loop sees EOF.
	_ = clientConn.Close()

	select {
	case <-c.Done():
		// Expected — Err should be nil for clean EOF.
		if err := c.Err(); err != nil && !errors.Is(err, io.EOF) {
			t.Logf("Err on EOF: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Done not closed after EOF")
	}
}

func TestClient_Close_StopsReadLoop(t *testing.T) {
	clientConn, _ := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	if err := c.Close(); err != nil {
		t.Logf("Close err (acceptable): %v", err)
	}
	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatalf("Done not closed after Close()")
	}
}

func TestClient_SubmitTurn_TypedHelper(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	srv.handle(protocol.MethodTurnSubmit, func(req *protocol.Request) *protocol.Response {
		var sub protocol.TurnSubmit
		_ = json.Unmarshal(req.Params, &sub)
		ack := protocol.TurnSubmitResponse{TurnID: sub.TurnID, Status: "accepted"}
		out, _ := json.Marshal(ack)
		return &protocol.Response{JSONRPC: "2.0", ID: req.ID, Result: out}
	})

	ack, err := c.SubmitTurn(context.Background(), &protocol.TurnSubmit{
		TurnID: "turn-99", SessionID: "s", Sequence: 1, Mode: protocol.ModeBuild, Content: "go",
	})
	if err != nil {
		t.Fatalf("SubmitTurn: %v", err)
	}
	if ack.TurnID != "turn-99" || ack.Status != "accepted" {
		t.Errorf("ack = %+v", ack)
	}
}

func TestClient_Ping_TypedHelper(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	called := false
	srv.handle(protocol.MethodConnectionPing, func(req *protocol.Request) *protocol.Response {
		called = true
		return &protocol.Response{
			JSONRPC: "2.0", ID: req.ID,
			Result: json.RawMessage(`{}`),
		}
	})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if !called {
		t.Errorf("server handler not called")
	}
}

func TestClient_Register_TypedHelper(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	srv.handle(protocol.MethodCapabilitiesRegister, func(req *protocol.Request) *protocol.Response {
		resp := protocol.CapabilitiesRegisterResponse{
			ServerCapabilities: protocol.ServerCapabilities{
				ProtocolVersion:   "1",
				MaxFrameSize:      10 * 1024 * 1024,
				MaxAttachmentSize: 5 * 1024 * 1024,
			},
		}
		out, _ := json.Marshal(resp)
		return &protocol.Response{JSONRPC: "2.0", ID: req.ID, Result: out}
	})

	rr, err := c.Register(context.Background(), &protocol.CapabilitiesRegister{
		ProtocolVersion: "1",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if rr.NegotiatedVersion != "1" || rr.MaxFrameSize != 10*1024*1024 || rr.MaxAttachmentSize != 5*1024*1024 {
		t.Errorf("RegisterResponse = %+v", rr)
	}
}

func TestClient_CallInto_DecodesIntoDest(t *testing.T) {
	clientConn, srv := newMockServer(t)
	c := dialPipeClient(t, clientConn, nil)

	srv.handle("ans", func(req *protocol.Request) *protocol.Response {
		return &protocol.Response{
			JSONRPC: "2.0", ID: req.ID,
			Result: json.RawMessage(`{"answer":42}`),
		}
	})

	var dest struct {
		Answer int `json:"answer"`
	}
	if err := c.CallInto(context.Background(), "ans", nil, &dest); err != nil {
		t.Fatalf("CallInto: %v", err)
	}
	if dest.Answer != 42 {
		t.Errorf("Answer = %d", dest.Answer)
	}
}

func TestDial_Validation(t *testing.T) {
	_, err := Dial(context.Background(), DialOptions{})
	if err == nil {
		t.Errorf("expected error when neither SocketPath nor TLSAddress set")
	}
}

// TestDial_RealUnixSocket smoke-tests the actual Dial path against a
// listening unix socket (no protocol exchange — just verify that
// Dial succeeds and the read loop sees EOF when the listener closes).
func TestDial_RealUnixSocket(t *testing.T) {
	sock := shortSockPath(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err == nil {
			// Hold connection, then close.
			time.Sleep(50 * time.Millisecond)
			_ = conn.Close()
		}
	}()

	c, err := Dial(context.Background(), DialOptions{SocketPath: sock})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatalf("Done not closed after server closed connection")
	}
}

// shortSockPath returns a /tmp socket path within macOS's 104-byte
// limit; t.TempDir() paths are too long.
func shortSockPath(t *testing.T) string {
	t.Helper()
	dir := "/tmp"
	srv := httptest.NewUnstartedServer(nil)
	defer srv.Close()
	// Use a deterministic-ish but unique-per-test name.
	return dir + "/mt-cli-" + t.Name() + ".sock"
}
