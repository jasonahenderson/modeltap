package harness

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// DefaultDialTimeout is the default timeout for the initial connection
// to the BFF socket / TLS endpoint.
const DefaultDialTimeout = 5 * time.Second

// EventHandler processes server-initiated notifications. Implementations
// must be non-blocking — long-running work should dispatch to a
// channel or goroutine so the read loop keeps draining frames.
type EventHandler interface {
	HandleEvent(method string, params json.RawMessage)
}

// EventHandlerFunc adapts an ordinary function to EventHandler.
type EventHandlerFunc func(method string, params json.RawMessage)

// HandleEvent satisfies EventHandler.
func (f EventHandlerFunc) HandleEvent(method string, params json.RawMessage) {
	f(method, params)
}

// DialOptions configures Dial. Exactly one of SocketPath or TLSAddress
// must be set.
type DialOptions struct {
	SocketPath string
	TLSAddress string
	TLSConfig  *tls.Config

	EventHandler EventHandler

	DialTimeout time.Duration
}

// RPCError carries a JSON-RPC error response in typed form so callers
// can distinguish recoverable application errors from transport errors.
type RPCError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

// Error satisfies the error interface.
func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// IsRPCError reports whether err wraps an RPCError with the given code.
func IsRPCError(err error, code int) bool {
	var rpc *RPCError
	if !errors.As(err, &rpc) {
		return false
	}
	return rpc.Code == code
}

// ProtocolClient is the harness-side JSON-RPC 2.0 client. It owns a
// single net.Conn, runs a read loop, correlates request/response
// pairs by id, and dispatches notifications to an EventHandler.
//
// The client is intentionally transport-agnostic above the net.Conn
// layer — DialOptions selects between Unix socket and TLS, but once
// connected the protocol is identical.
type ProtocolClient struct {
	conn   net.Conn
	reader *protocol.FrameReader
	writer *protocol.FrameWriter

	writeMu sync.Mutex // serializes outbound frames

	pendMu  sync.Mutex
	pending map[string]chan *protocol.Response
	nextID  int64

	eventHandler EventHandler

	ctx    context.Context
	cancel context.CancelFunc

	doneCh   chan struct{}
	doneOnce sync.Once

	exitErr atomic.Value // stores error
}

// Dial opens a connection to the BFF and starts the read loop.
func Dial(ctx context.Context, opts DialOptions) (*ProtocolClient, error) {
	if opts.SocketPath == "" && opts.TLSAddress == "" {
		return nil, errors.New("dial: SocketPath or TLSAddress required")
	}
	timeout := opts.DialTimeout
	if timeout <= 0 {
		timeout = DefaultDialTimeout
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var conn net.Conn
	var err error
	switch {
	case opts.SocketPath != "":
		var d net.Dialer
		conn, err = d.DialContext(dialCtx, "unix", opts.SocketPath)
	case opts.TLSAddress != "":
		dialer := &tls.Dialer{Config: opts.TLSConfig}
		conn, err = dialer.DialContext(dialCtx, "tcp", opts.TLSAddress)
	}
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	clientCtx, clientCancel := context.WithCancel(context.Background())
	c := &ProtocolClient{
		conn:         conn,
		reader:       protocol.NewFrameReader(conn),
		writer:       protocol.NewFrameWriter(conn),
		pending:      make(map[string]chan *protocol.Response),
		eventHandler: opts.EventHandler,
		ctx:          clientCtx,
		cancel:       clientCancel,
		doneCh:       make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

// Close stops the read loop and closes the underlying connection.
// Safe to call multiple times.
func (c *ProtocolClient) Close() error {
	c.cancel()
	err := c.conn.Close()
	c.doneOnce.Do(func() { close(c.doneCh) })
	return err
}

// Done returns a channel closed when the read loop exits (either via
// Close, a network error, or io.EOF).
func (c *ProtocolClient) Done() <-chan struct{} { return c.doneCh }

// Err returns the read-loop exit error, or nil for a clean Close.
func (c *ProtocolClient) Err() error {
	if v := c.exitErr.Load(); v != nil {
		if err, ok := v.(error); ok {
			return err
		}
	}
	return nil
}

// Call sends a request and waits for the matching response. Returns
// the Result payload, an *RPCError for JSON-RPC error responses, or
// the context error on cancel/timeout.
func (c *ProtocolClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextRequestID()
	idBytes, _ := json.Marshal(id)

	var paramBytes json.RawMessage
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		paramBytes = raw
	}

	req := &protocol.Request{
		JSONRPC: "2.0",
		ID:      idBytes,
		Method:  method,
		Params:  paramBytes,
	}
	frame, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respCh := make(chan *protocol.Response, 1)
	c.pendMu.Lock()
	c.pending[id] = respCh
	c.pendMu.Unlock()

	cleanup := func() {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
	}

	c.writeMu.Lock()
	werr := c.writer.WriteFrame(frame)
	c.writeMu.Unlock()
	if werr != nil {
		cleanup()
		return nil, fmt.Errorf("write frame: %w", werr)
	}

	select {
	case <-ctx.Done():
		cleanup()
		return nil, ctx.Err()
	case <-c.doneCh:
		cleanup()
		if err := c.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("client: connection closed")
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, &RPCError{
				Code:    resp.Error.Code,
				Message: resp.Error.Message,
				Data:    resp.Error.Data,
			}
		}
		return resp.Result, nil
	}
}

// CallInto is Call + json.Unmarshal into dest. Returns transport /
// RPC errors via Call; unmarshal failures wrap the underlying err.
func (c *ProtocolClient) CallInto(ctx context.Context, method string, params, dest any) error {
	raw, err := c.Call(ctx, method, params)
	if err != nil {
		return err
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// nextRequestID atomically generates the next request id as a string.
func (c *ProtocolClient) nextRequestID() string {
	n := atomic.AddInt64(&c.nextID, 1)
	return strconv.FormatInt(n, 10)
}

// readLoop reads frames in a dedicated goroutine. Responses are routed
// to the matching pending channel; notifications are dispatched to
// the EventHandler.
func (c *ProtocolClient) readLoop() {
	defer c.doneOnce.Do(func() { close(c.doneCh) })

	for {
		raw, err := c.reader.ReadFrame()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.exitErr.Store(err)
			}
			return
		}

		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			// Malformed frame — drop and continue.
			continue
		}

		_, hasMethod := probe["method"]
		_, hasID := probe["id"]
		_, hasResult := probe["result"]
		_, hasError := probe["error"]

		switch {
		case hasID && (hasResult || hasError) && !hasMethod:
			var resp protocol.Response
			if err := json.Unmarshal(raw, &resp); err != nil {
				continue
			}
			id := stripQuotesAndSpaces(string(resp.ID))
			c.pendMu.Lock()
			ch, ok := c.pending[id]
			if ok {
				delete(c.pending, id)
			}
			c.pendMu.Unlock()
			if ok {
				ch <- &resp
			}

		case hasMethod && !hasID:
			var notif protocol.Notification
			if err := json.Unmarshal(raw, &notif); err != nil {
				continue
			}
			if c.eventHandler != nil {
				c.eventHandler.HandleEvent(notif.Method, notif.Params)
			}
		}
	}
}

// stripQuotesAndSpaces strips JSON quoting around a numeric/string id
// so the same id matches whether it came back as `1` or `"1"`. The
// client only emits unquoted numeric ids; this is defensive against
// servers that quote ids on the wire.
func stripQuotesAndSpaces(s string) string {
	s = trimSpaces(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func trimSpaces(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// -----------------------------------------------------------------------
// Typed helpers (D2.5)
// -----------------------------------------------------------------------

// TurnSubmitAck mirrors protocol.TurnSubmitResponse for the harness side.
type TurnSubmitAck struct {
	TurnID string          `json:"turn_id"`
	Status string          `json:"status"`
	Sync   json.RawMessage `json:"sync,omitempty"`
}

// SubmitTurn sends turn.submit and returns the ack. Streaming events
// flow asynchronously through the EventHandler.
func (c *ProtocolClient) SubmitTurn(ctx context.Context, submit *protocol.TurnSubmit) (*TurnSubmitAck, error) {
	var ack TurnSubmitAck
	if err := c.CallInto(ctx, protocol.MethodTurnSubmit, submit, &ack); err != nil {
		return nil, err
	}
	return &ack, nil
}

// CancelTurn sends turn.cancel.
func (c *ProtocolClient) CancelTurn(ctx context.Context, turnID string) error {
	_, err := c.Call(ctx, protocol.MethodTurnCancel, &protocol.TurnCancel{TurnID: turnID})
	return err
}

// SendToolResult sends tool.result.
func (c *ProtocolClient) SendToolResult(ctx context.Context, result *protocol.ToolResult) error {
	_, err := c.Call(ctx, protocol.MethodToolResult, result)
	return err
}

// RegisterResponse mirrors the bits of CapabilitiesRegisterResponse the
// harness needs at hand. The full response is also returned via the
// underlying call for callers that want richer fields.
type RegisterResponse struct {
	NegotiatedVersion string `json:"protocol_version"`
	MaxFrameSize      int    `json:"max_frame_size"`
	MaxAttachmentSize int    `json:"max_attachment_size"`
	ServerCapabilities protocol.ServerCapabilities `json:"server_capabilities"`
}

// Register sends capabilities.register and returns the parsed server
// capabilities. The wire response carries Registered/Rejected tool
// arrays too — callers that need those should use Call directly.
func (c *ProtocolClient) Register(ctx context.Context, reg *protocol.CapabilitiesRegister) (*RegisterResponse, error) {
	var resp protocol.CapabilitiesRegisterResponse
	if err := c.CallInto(ctx, protocol.MethodCapabilitiesRegister, reg, &resp); err != nil {
		return nil, err
	}
	return &RegisterResponse{
		NegotiatedVersion:  resp.ServerCapabilities.ProtocolVersion,
		MaxFrameSize:       resp.ServerCapabilities.MaxFrameSize,
		MaxAttachmentSize:  resp.ServerCapabilities.MaxAttachmentSize,
		ServerCapabilities: resp.ServerCapabilities,
	}, nil
}

// Ping sends connection.ping. Returns the server's pong (an empty
// struct on success per protocol).
func (c *ProtocolClient) Ping(ctx context.Context) error {
	_, err := c.Call(ctx, protocol.MethodConnectionPing, &protocol.ConnectionPing{})
	return err
}

// Health sends connection.health and returns the raw response payload
// for the connection manager to decode.
func (c *ProtocolClient) Health(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, protocol.MethodConnectionHealth, &protocol.ConnectionHealth{})
}

// SessionResume sends session.resume and decodes the typed response.
func (c *ProtocolClient) SessionResume(ctx context.Context, sessionID string, project protocol.ProjectContext) (*protocol.SessionResumeResponse, error) {
	var out protocol.SessionResumeResponse
	if err := c.CallInto(ctx, protocol.MethodSessionResume, &protocol.SessionResume{SessionID: sessionID, Project: project}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SessionList sends session.list and decodes the typed response.
func (c *ProtocolClient) SessionList(ctx context.Context) (*protocol.SessionListResponse, error) {
	var out protocol.SessionListResponse
	if err := c.CallInto(ctx, protocol.MethodSessionList, &protocol.SessionList{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SessionDetails sends session.details.
func (c *ProtocolClient) SessionDetails(ctx context.Context, sessionID string) (*protocol.SessionDetail, error) {
	var out protocol.SessionDetail
	if err := c.CallInto(ctx, protocol.MethodSessionDetails, &protocol.SessionDetails{SessionID: sessionID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SessionClear sends session.clear.
func (c *ProtocolClient) SessionClear(ctx context.Context, sessionID string) (*protocol.SessionClearResponse, error) {
	var out protocol.SessionClearResponse
	if err := c.CallInto(ctx, protocol.MethodSessionClear, &protocol.SessionClear{SessionID: sessionID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SessionFork sends session.fork.
func (c *ProtocolClient) SessionFork(ctx context.Context, sessionID string) (*protocol.SessionForkResponse, error) {
	var out protocol.SessionForkResponse
	if err := c.CallInto(ctx, protocol.MethodSessionFork, &protocol.SessionFork{SessionID: sessionID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ContentTransform sends content.transform.
func (c *ProtocolClient) ContentTransform(ctx context.Context, req *protocol.ContentTransform) (json.RawMessage, error) {
	return c.Call(ctx, protocol.MethodContentTransform, req)
}

// HistoryList sends history.list and decodes the typed response. Used
// by the harness HistoryController (WU-092) to populate cross-session
// command history for input-area arrow-up traversal.
func (c *ProtocolClient) HistoryList(ctx context.Context, req *protocol.HistoryList) (*protocol.HistoryListResponse, error) {
	var out protocol.HistoryListResponse
	if err := c.CallInto(ctx, protocol.MethodHistoryList, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ModelList sends model.list and decodes the typed response. Used by
// the WU-085 /models command and model-switch UX.
func (c *ProtocolClient) ModelList(ctx context.Context) (*protocol.ModelListResponse, error) {
	var out protocol.ModelListResponse
	if err := c.CallInto(ctx, protocol.MethodModelList, &protocol.ModelList{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ModelSwitch sends model.switch. Pass "auto" as model to clear the
// session-level override.
func (c *ProtocolClient) ModelSwitch(ctx context.Context, req *protocol.ModelSwitch) (*protocol.ModelSwitchResponse, error) {
	var out protocol.ModelSwitchResponse
	if err := c.CallInto(ctx, protocol.MethodModelSwitch, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
