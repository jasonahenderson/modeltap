// Package bff implements the BFF (Backend-for-Frontend) protocol server
// for FEAT-0008. The server speaks JSON-RPC 2.0 over NDJSON on a Unix
// domain socket or TLS, and brokers harness->provider conversation flow,
// session storage, model routing, and capability registration.
//
// File scope (WU-046, JSON-RPC transport layer):
//   - FrameTransport: NDJSON reader/writer over net.Conn with JSON-RPC
//     envelope classification (Request / Response / Notification).
//   - Dispatcher: method-name -> Handler routing, with MethodNotFound
//     errors and duplicate-registration panics.
//   - JSON-RPC error codes: standard codes plus FEAT-0008 application
//     codes (-32000..-32099 range).
//   - ValidateTurnSubmit: edge validation for turn.submit (presence of
//     required sequence field, mode enum value).
//   - TransportError: typed error returned by the transport so callers
//     can decide whether to send a JSON-RPC error response or close the
//     connection (Close=true on unrecoverable framing errors).
//
// Files for the server, connection state machine, and capability
// registration land in WU-047, WU-048, and WU-049 respectively.
package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// JSON-RPC 2.0 standard error codes plus FEAT-0008 application codes.
// The application range -32000..-32099 is reserved by JSON-RPC 2.0 for
// implementation-defined server errors. Values are wire-visible.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603

	CodeNotReady         = -32000
	CodeSessionLocked    = -32001
	CodeVersionMismatch  = -32002
	CodeCapabilityError  = -32003
	CodeProviderError    = -32004
	CodeSessionNotFound  = -32005
	CodeTurnNotFound     = -32006
	CodeModelUnavailable = -32007
)

// TransportError is returned by the transport and dispatcher when a
// problem must be surfaced as a JSON-RPC error response. Close=true
// indicates the underlying stream is unrecoverable (e.g., oversized
// frame) and the caller MUST close the connection rather than continue
// reading.
type TransportError struct {
	Code    int
	Message string
	Data    any
	Close   bool
	wrapped error
}

func (e *TransportError) Error() string {
	if e.wrapped != nil {
		return fmt.Sprintf("transport: %s (code %d): %v", e.Message, e.Code, e.wrapped)
	}
	return fmt.Sprintf("transport: %s (code %d)", e.Message, e.Code)
}

func (e *TransportError) Unwrap() error { return e.wrapped }

// Envelope is the result of decoding a single NDJSON frame. Exactly one
// of Request, Response, or Notification is non-nil. Raw retains the
// original frame bytes for capture/audit (ADR-0005).
type Envelope struct {
	Request      *protocol.Request
	Response     *protocol.Response
	Notification *protocol.Notification
	Raw          json.RawMessage
}

// FrameTransport provides JSON-RPC 2.0 message exchange over a net.Conn.
// Reads are not safe for concurrent callers (one read goroutine per
// connection per FEAT-0008). Writes ARE safe for concurrent callers; the
// transport serializes them with an internal mutex so streaming events
// (token.delta) and request responses cannot interleave on the wire.
type FrameTransport struct {
	conn   net.Conn
	reader *protocol.FrameReader
	writer *protocol.FrameWriter
	mu     sync.Mutex // guards writer
}

// NewFrameTransport wraps conn with NDJSON framing.
func NewFrameTransport(conn net.Conn) *FrameTransport {
	return &FrameTransport{
		conn:   conn,
		reader: protocol.NewFrameReader(conn),
		writer: protocol.NewFrameWriter(conn),
	}
}

// ReadMessage reads one NDJSON frame and decodes the JSON-RPC envelope.
// Returns io.EOF cleanly between frames. Returns *TransportError for
// malformed input; callers inspect Close to decide whether to close the
// connection.
func (t *FrameTransport) ReadMessage() (*Envelope, error) {
	raw, err := t.reader.ReadFrame()
	if err != nil {
		if errors.Is(err, protocol.ErrFrameTooLarge) {
			return nil, &TransportError{
				Code:    CodeInvalidRequest,
				Message: "frame exceeds max size",
				Close:   true,
				wrapped: err,
			}
		}
		// Other read errors (io.EOF, net errors) are passed through
		// untouched so callers can detect close conditions.
		return nil, err
	}

	// Classify the frame by which fields are present. Per JSON-RPC 2.0:
	//   request:      method + id (+ optional params)
	//   notification: method (no id)
	//   response:     id + (result XOR error)
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, &TransportError{
			Code:    CodeParseError,
			Message: "invalid JSON",
			wrapped: err,
		}
	}

	_, hasMethod := probe["method"]
	_, hasID := probe["id"]
	_, hasResult := probe["result"]
	_, hasError := probe["error"]

	switch {
	case hasMethod && hasID && !hasResult && !hasError:
		var req protocol.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, &TransportError{Code: CodeParseError, Message: "decode request", wrapped: err}
		}
		return &Envelope{Request: &req, Raw: append([]byte(nil), raw...)}, nil

	case hasMethod && !hasID && !hasResult && !hasError:
		var notif protocol.Notification
		if err := json.Unmarshal(raw, &notif); err != nil {
			return nil, &TransportError{Code: CodeParseError, Message: "decode notification", wrapped: err}
		}
		return &Envelope{Notification: &notif, Raw: append([]byte(nil), raw...)}, nil

	case hasID && (hasResult || hasError) && !hasMethod:
		var resp protocol.Response
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, &TransportError{Code: CodeParseError, Message: "decode response", wrapped: err}
		}
		return &Envelope{Response: &resp, Raw: append([]byte(nil), raw...)}, nil

	default:
		return nil, &TransportError{
			Code:    CodeInvalidRequest,
			Message: "frame is not a valid JSON-RPC 2.0 message",
		}
	}
}

// SendResponse writes a JSON-RPC 2.0 Response.
func (t *FrameTransport) SendResponse(resp *protocol.Response) error {
	if resp.JSONRPC == "" {
		resp.JSONRPC = "2.0"
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writer.WriteFrame(b)
}

// SendNotification writes a JSON-RPC 2.0 Notification.
func (t *FrameTransport) SendNotification(notif *protocol.Notification) error {
	if notif.JSONRPC == "" {
		notif.JSONRPC = "2.0"
	}
	b, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writer.WriteFrame(b)
}

// SendError writes a JSON-RPC 2.0 error response for the given request id.
// data may be nil; if non-nil it is JSON-encoded into the error object's
// data field.
func (t *FrameTransport) SendError(id json.RawMessage, code int, message string, data any) error {
	errObj := &protocol.ErrorObject{Code: code, Message: message}
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal error data: %w", err)
		}
		errObj.Data = raw
	}
	return t.SendResponse(&protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   errObj,
	})
}

// Close closes the underlying net.Conn.
func (t *FrameTransport) Close() error { return t.conn.Close() }

// Handler processes a single JSON-RPC request and returns a result
// payload (which the dispatcher marshals into Response.Result) or an
// error. Returning a *TransportError lets the handler choose the wire
// error code; any other error is surfaced as CodeInternalError.
type Handler func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)

// Dispatcher routes JSON-RPC requests to registered handlers by method
// name. It is safe for concurrent Dispatch calls (handlers map is read-
// only after registration), but Register is NOT safe for concurrent use
// and is expected to be called from a single setup goroutine.
type Dispatcher struct {
	handlers map[string]Handler
}

// NewDispatcher returns an empty Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]Handler)}
}

// Register binds a handler to a method name. Panics on duplicate
// registration — a duplicate is always a programming error and silently
// overwriting the prior handler would mask wire-visible behavior changes.
func (d *Dispatcher) Register(method string, h Handler) {
	if _, exists := d.handlers[method]; exists {
		panic(fmt.Sprintf("bff: handler already registered for method %q", method))
	}
	d.handlers[method] = h
}

// Dispatch routes a request to its registered handler. Returns a
// *TransportError with CodeMethodNotFound if no handler is registered.
func (d *Dispatcher) Dispatch(ctx context.Context, conn *Connection, req *protocol.Request) (any, error) {
	h, ok := d.handlers[req.Method]
	if !ok {
		return nil, &TransportError{
			Code:    CodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}
	return h(ctx, conn, req.Params)
}

// ValidateTurnSubmit performs edge validation on a turn.submit params
// payload before dispatch. The two checks not expressible in the Go type
// system are:
//
//  1. The sequence field must be PRESENT (not just zero) — a missing
//     sequence cannot be distinguished from sequence=0 after decoding,
//     and the protocol requires the harness to assign a real sequence.
//  2. The mode value must be one of plan/build/auto.
//
// Returns the decoded *protocol.TurnSubmit on success, or a
// *TransportError with CodeInvalidParams.
func ValidateTurnSubmit(raw json.RawMessage) (*protocol.TurnSubmit, error) {
	// Presence check via map decode — avoids a duplicate struct with *int.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "invalid turn.submit JSON", wrapped: err}
	}
	if _, ok := probe["sequence"]; !ok {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "turn.submit requires sequence field"}
	}

	var ts protocol.TurnSubmit
	if err := json.Unmarshal(raw, &ts); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode turn.submit", wrapped: err}
	}
	if !ts.Mode.Valid() {
		return nil, &TransportError{Code: CodeInvalidParams, Message: fmt.Sprintf("invalid mode: %s", ts.Mode)}
	}
	return &ts, nil
}
