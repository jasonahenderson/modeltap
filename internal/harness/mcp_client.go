// Package harness — MCP stdio client.
//
// The Model Context Protocol (MCP) exposes tools from an external
// process over stdio. The transport is line-delimited JSON-RPC 2.0:
// one message per line, no Content-Length framing. The client here
// implements enough of that spec to run initialize / tools.list /
// tools.call against a server. Callbacks into the harness tool
// framework live in mcp_tool.go; orchestration (process launch,
// retry, registration) lives in mcp.go.
package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// MCPStream is the io pair the client drives. In production this is a
// subprocess's stdin / stdout wired through exec.Cmd; in tests it's
// a pair of in-process pipes so the unit tests don't fork processes.
type MCPStream struct {
	In  io.WriteCloser // client → server (stdin)
	Out io.ReadCloser  // server → client (stdout)
}

// MCPClient is a line-delimited JSON-RPC 2.0 client over an MCPStream.
// Safe for concurrent callers — each Call serializes one request to
// the stream and blocks on its matched response. Notifications from
// the server currently go to a single optional handler; richer
// routing can layer on top.
type MCPClient struct {
	stream  MCPStream
	encoder *json.Encoder
	writeMu sync.Mutex

	nextID atomic.Int64

	mu      sync.Mutex
	pending map[int64]chan mcpResponse

	readerDone chan struct{}
	closed     atomic.Bool

	// OnNotification, when non-nil, receives server-sent notifications
	// (messages with no id). The default is nil — unknown
	// notifications are silently dropped.
	OnNotification func(method string, params json.RawMessage)
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"` // set on notifications
	Params  json.RawMessage `json:"params,omitempty"` // set on notifications
}

type mcpError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *mcpError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
}

// NewMCPClient wires a stream into a client and starts the response
// reader goroutine. Callers should Close the client (which Closes the
// stream) to release the goroutine.
func NewMCPClient(stream MCPStream) *MCPClient {
	c := &MCPClient{
		stream:     stream,
		encoder:    json.NewEncoder(stream.In),
		pending:    make(map[int64]chan mcpResponse),
		readerDone: make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Close terminates the client. Pending callers unblock with
// io.ErrClosedPipe.
func (c *MCPClient) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	// Closing stdin signals the server to shut down cleanly; closing
	// stdout unblocks our reader.
	var errs []error
	if err := c.stream.In.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := c.stream.Out.Close(); err != nil {
		errs = append(errs, err)
	}
	<-c.readerDone

	// Fail any still-pending callers.
	c.mu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	return errors.Join(errs...)
}

// Notify sends a JSON-RPC notification (no id, no response expected).
func (c *MCPClient) Notify(method string, params any) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		raw = b
	}
	return c.writeMsg(mcpNotification{JSONRPC: "2.0", Method: method, Params: raw})
}

// Call sends a JSON-RPC request and blocks until the response
// arrives or ctx cancels. Returns the decoded result payload.
func (c *MCPClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c.closed.Load() {
		return nil, errors.New("mcp client closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	id := c.nextID.Add(1)
	ch := make(chan mcpResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			c.releasePending(id)
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		raw = b
	}

	// Write can block on a synchronous transport; run it off-thread so
	// ctx cancellation unblocks the caller even if the server is slow
	// to consume.
	writeErr := make(chan error, 1)
	go func() {
		writeErr <- c.writeMsg(mcpRequest{JSONRPC: "2.0", ID: id, Method: method, Params: raw})
	}()

	select {
	case <-ctx.Done():
		c.releasePending(id)
		return nil, ctx.Err()
	case err := <-writeErr:
		if err != nil {
			c.releasePending(id)
			return nil, err
		}
	}

	select {
	case <-ctx.Done():
		c.releasePending(id)
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, io.ErrClosedPipe
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (c *MCPClient) releasePending(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// writeMsg is the single serialized writer.
func (c *MCPClient) writeMsg(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.encoder.Encode(v)
}

// readLoop pulls frames off the server stream. Exits cleanly on
// io.EOF and on Close.
func (c *MCPClient) readLoop() {
	defer close(c.readerDone)

	scanner := bufio.NewScanner(c.stream.Out)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg mcpResponse
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // skip malformed frames
		}
		if msg.ID == nil {
			// Notification.
			if c.OnNotification != nil && msg.Method != "" {
				c.OnNotification(msg.Method, msg.Params)
			}
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[*msg.ID]
		if ok {
			delete(c.pending, *msg.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- msg
		}
	}
}

// -------------------------------------------------------------------
// MCP protocol helpers
// -------------------------------------------------------------------

// MCPInitializeParams is the payload for initialize.
type MCPInitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      MCPClientInfo  `json:"clientInfo"`
}

// MCPClientInfo identifies the harness to the server.
type MCPClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPInitializeResult is the returned server identity + caps.
type MCPInitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      MCPServerInfo  `json:"serverInfo"`
}

// MCPServerInfo identifies the remote server.
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPToolDescriptor matches the MCP spec's tool-descriptor shape.
// The InputSchema is a full JSON Schema the harness passes through
// unchanged when building its tool catalog.
type MCPToolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// MCPToolsListResult is the response to tools/list.
type MCPToolsListResult struct {
	Tools []MCPToolDescriptor `json:"tools"`
}

// MCPToolsCallParams is the payload for tools/call.
type MCPToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// MCPToolsCallResult is the response to tools/call. Content is an
// array of text / image / resource blocks in the spec; the harness
// collapses text blocks into a single string for its own tool output.
type MCPToolsCallResult struct {
	Content []MCPContentBlock `json:"content"`
	IsError bool              `json:"isError"`
}

// MCPContentBlock is one element of MCPToolsCallResult.Content.
type MCPContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"` // base64 for image/resource
	MimeType string `json:"mimeType,omitempty"`
}

// Initialize runs the MCP handshake. Returns the server's identity
// and capabilities.
func (c *MCPClient) Initialize(ctx context.Context, clientName, clientVersion string) (*MCPInitializeResult, error) {
	raw, err := c.Call(ctx, "initialize", MCPInitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]any{},
		ClientInfo:      MCPClientInfo{Name: clientName, Version: clientVersion},
	})
	if err != nil {
		return nil, err
	}
	var out MCPInitializeResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode initialize: %w", err)
	}
	// Per spec the client sends notifications/initialized after.
	if err := c.Notify("notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("notify initialized: %w", err)
	}
	return &out, nil
}

// ListTools calls tools/list.
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPToolDescriptor, error) {
	raw, err := c.Call(ctx, "tools/list", struct{}{})
	if err != nil {
		return nil, err
	}
	var out MCPToolsListResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode tools/list: %w", err)
	}
	return out.Tools, nil
}

// CallTool calls tools/call.
func (c *MCPClient) CallTool(ctx context.Context, name string, args json.RawMessage) (*MCPToolsCallResult, error) {
	raw, err := c.Call(ctx, "tools/call", MCPToolsCallParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	var out MCPToolsCallResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode tools/call: %w", err)
	}
	return &out, nil
}
