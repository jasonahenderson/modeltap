package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// pipeStream returns an MCPStream backed by two io.Pipe pairs plus a
// server side that the test drives. Returns (client, server) where
// server exposes the inverted pair: server reads what the client
// writes, and vice versa.
type pipeServer struct {
	in  io.WriteCloser // tests write here → client reads
	out io.ReadCloser  // tests read here ← client wrote
}

func newPipeClient(t *testing.T) (*MCPClient, *pipeServer) {
	t.Helper()
	clientInR, clientInW := io.Pipe()   // client stdin pipe: server writes in, client reads
	clientOutR, clientOutW := io.Pipe() // client stdout pipe: client writes out, server reads

	stream := MCPStream{In: clientOutW, Out: clientInR}
	client := NewMCPClient(stream)
	server := &pipeServer{in: clientInW, out: clientOutR}
	return client, server
}

// readServerFrame reads one line from the client→server direction.
func readServerFrame(t *testing.T, s *pipeServer) string {
	t.Helper()
	scanner := bufio.NewScanner(s.out)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			t.Fatalf("scan: %v", err)
		}
		t.Fatal("scanner closed before reading a frame")
	}
	return scanner.Text()
}

// writeServerFrame writes one line from server → client.
func writeServerFrame(t *testing.T, s *pipeServer, payload string) {
	t.Helper()
	if _, err := s.in.Write([]byte(payload + "\n")); err != nil {
		t.Fatalf("write server frame: %v", err)
	}
}

func TestMCPClient_Call_HappyPath(t *testing.T) {
	client, server := newPipeClient(t)
	defer client.Close()

	done := make(chan struct{})
	var gotResult json.RawMessage
	var gotErr error
	go func() {
		defer close(done)
		gotResult, gotErr = client.Call(context.Background(), "ping", map[string]string{"hello": "world"})
	}()

	// Read the request, assert shape, respond.
	req := readServerFrame(t, server)
	if !strings.Contains(req, `"method":"ping"`) {
		t.Errorf("method not in request: %s", req)
	}
	if !strings.Contains(req, `"hello":"world"`) {
		t.Errorf("params not serialized: %s", req)
	}
	var parsed struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(req), &parsed); err != nil {
		t.Fatalf("decode req: %v", err)
	}
	writeServerFrame(t, server, `{"jsonrpc":"2.0","id":`+itoa(parsed.ID)+`,"result":{"pong":true}}`)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Call did not return")
	}
	if gotErr != nil {
		t.Fatalf("Call err: %v", gotErr)
	}
	if !strings.Contains(string(gotResult), "pong") {
		t.Errorf("result missing pong: %s", gotResult)
	}
}

func TestMCPClient_Call_ReturnsError(t *testing.T) {
	client, server := newPipeClient(t)
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Call(context.Background(), "busted", nil)
		errCh <- err
	}()

	req := readServerFrame(t, server)
	var parsed struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal([]byte(req), &parsed)
	writeServerFrame(t, server,
		`{"jsonrpc":"2.0","id":`+itoa(parsed.ID)+`,"error":{"code":-32601,"message":"unknown"}}`)

	err := <-errCh
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("err = %v", err)
	}
}

func TestMCPClient_Notify_NoID(t *testing.T) {
	client, server := newPipeClient(t)
	defer client.Close()

	// io.Pipe is synchronous — start the reader before writing so the
	// Notify call doesn't deadlock waiting for a consumer on stdout.
	frameCh := make(chan string, 1)
	go func() { frameCh <- readServerFrame(t, server) }()

	if err := client.Notify("status", map[string]any{"msg": "hi"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	select {
	case frame := <-frameCh:
		if strings.Contains(frame, `"id"`) {
			t.Errorf("notification should have no id; got %s", frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame received")
	}
}

func TestMCPClient_Notification_DispatchesCallback(t *testing.T) {
	client, server := newPipeClient(t)
	defer client.Close()

	var mu sync.Mutex
	received := map[string]json.RawMessage{}
	client.OnNotification = func(method string, params json.RawMessage) {
		mu.Lock()
		received[method] = params
		mu.Unlock()
	}

	writeServerFrame(t, server,
		`{"jsonrpc":"2.0","method":"ping","params":{"tick":1}}`)

	// Give the reader goroutine a moment; poll up to 1s.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		_, ok := received["ping"]
		mu.Unlock()
		if ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if p, ok := received["ping"]; !ok || !strings.Contains(string(p), "tick") {
		t.Errorf("notification not dispatched; got %v", received)
	}
}

func TestMCPClient_Call_ContextCancels(t *testing.T) {
	client, _ := newPipeClient(t)
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Call(ctx, "slow", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want canceled", err)
	}
}

func TestMCPClient_Initialize(t *testing.T) {
	client, server := newPipeClient(t)
	defer client.Close()

	done := make(chan struct{})
	var info *MCPInitializeResult
	go func() {
		defer close(done)
		var err error
		info, err = client.Initialize(context.Background(), "modeltap", "1.0")
		if err != nil {
			t.Errorf("Initialize: %v", err)
		}
	}()

	// Respond to initialize.
	req := readServerFrame(t, server)
	var parsed struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal([]byte(req), &parsed)
	writeServerFrame(t, server,
		`{"jsonrpc":"2.0","id":`+itoa(parsed.ID)+
			`,"result":{"protocolVersion":"2024-11-05","serverInfo":{"name":"demo","version":"0.1"},"capabilities":{}}}`)

	// Expect the client to send notifications/initialized next.
	ntf := readServerFrame(t, server)
	if !strings.Contains(ntf, "notifications/initialized") {
		t.Errorf("expected initialized notification; got %s", ntf)
	}

	<-done
	if info == nil || info.ServerInfo.Name != "demo" {
		t.Errorf("unexpected init result: %+v", info)
	}
}

func TestMCPClient_ListTools(t *testing.T) {
	client, server := newPipeClient(t)
	defer client.Close()

	done := make(chan struct{})
	var tools []MCPToolDescriptor
	go func() {
		defer close(done)
		var err error
		tools, err = client.ListTools(context.Background())
		if err != nil {
			t.Errorf("ListTools: %v", err)
		}
	}()

	req := readServerFrame(t, server)
	if !strings.Contains(req, `"method":"tools/list"`) {
		t.Errorf("expected tools/list; got %s", req)
	}
	var parsed struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal([]byte(req), &parsed)
	writeServerFrame(t, server,
		`{"jsonrpc":"2.0","id":`+itoa(parsed.ID)+
			`,"result":{"tools":[{"name":"echo","description":"echoes","inputSchema":{"type":"object"}}]}}`)

	<-done
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Errorf("tools = %+v", tools)
	}
}

func TestMCPClient_CallTool(t *testing.T) {
	client, server := newPipeClient(t)
	defer client.Close()

	done := make(chan struct{})
	var out *MCPToolsCallResult
	go func() {
		defer close(done)
		var err error
		out, err = client.CallTool(context.Background(), "echo", json.RawMessage(`{"msg":"hi"}`))
		if err != nil {
			t.Errorf("CallTool: %v", err)
		}
	}()

	req := readServerFrame(t, server)
	if !strings.Contains(req, `"method":"tools/call"`) {
		t.Errorf("expected tools/call; got %s", req)
	}
	var parsed struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal([]byte(req), &parsed)
	writeServerFrame(t, server,
		`{"jsonrpc":"2.0","id":`+itoa(parsed.ID)+
			`,"result":{"content":[{"type":"text","text":"pong"}],"isError":false}}`)

	<-done
	if out == nil || len(out.Content) != 1 || out.Content[0].Text != "pong" {
		t.Errorf("call result = %+v", out)
	}
}

func itoa(n int64) string {
	return strings.TrimSpace(formatInt(n))
}

func formatInt(n int64) string {
	// minimal avoid-strconv helper to keep imports tight
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
