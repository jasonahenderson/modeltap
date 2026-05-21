// Package testutil provides shared test scaffolding for harnesshost
// tests, primarily a minimal Runtime stub that listens on a unix socket
// and speaks just enough JSON-RPC for ProductionRuntime to exercise
// SubmitTurn + simple stream lifecycle events.
//
// The stub deliberately implements only the subset of methods the
// integration tests need; it is NOT a fake of the modeltap Runtime as a
// whole. Tests that need richer behavior should extend RuntimeStub or
// build a tailored fake.
package testutil

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// RuntimeStub is a unix-socket JSON-RPC server that handles enough of the
// Runtime protocol for ProductionRuntime SubmitTurn integration tests.
//
// Capabilities:
//
//   - capabilities.register: returns a synthetic ack.
//   - turn.submit: returns a server-assigned TurnID; the test can
//     observe submitted TurnSubmits via [Submits].
//   - turn.cancel: succeeds silently; observable via [Cancels].
//
// Usage:
//
//	stub, err := testutil.NewRuntimeStub()
//	defer stub.Close()
//	cfg := harness.ConnectionConfig{SocketPath: stub.SocketPath()}
//	rt, _ := harnesshost.NewProductionRuntime(harnesshost.ProductionRuntimeConfig{
//	    ConnConfig: cfg,
//	    ...
//	})
type RuntimeStub struct {
	socketPath string
	listener   net.Listener

	mu            sync.Mutex
	submits       []json.RawMessage
	cancels       []json.RawMessage
	calls         []RuntimeCall
	sessionDetail protocol.SessionDetail
	sessionResume protocol.SessionResumeResponse
	runs          []protocol.RunSummary
	runListError  string

	nextTurnID atomic.Uint64
	closed     atomic.Bool
}

// RuntimeCall records one JSON-RPC request observed by RuntimeStub.
type RuntimeCall struct {
	Method string
	Params json.RawMessage
}

// NewRuntimeStub starts a unix-socket Runtime stub on a temp path. The caller
// must Close() it.
func NewRuntimeStub() (*RuntimeStub, error) {
	dir, err := os.MkdirTemp("", "harnesshost-runtimestub-*")
	if err != nil {
		return nil, err
	}
	socket := filepath.Join(dir, "runtime.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("listen: %w", err)
	}
	s := &RuntimeStub{socketPath: socket, listener: l}
	go s.acceptLoop()
	return s, nil
}

// SocketPath returns the unix socket path the stub is listening on.
func (s *RuntimeStub) SocketPath() string { return s.socketPath }

// Submits returns a snapshot of every turn.submit payload received.
func (s *RuntimeStub) Submits() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]json.RawMessage, len(s.submits))
	for i, m := range s.submits {
		c := make(json.RawMessage, len(m))
		copy(c, m)
		out[i] = c
	}
	return out
}

// Cancels returns a snapshot of every turn.cancel payload received.
func (s *RuntimeStub) Cancels() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]json.RawMessage, len(s.cancels))
	for i, m := range s.cancels {
		c := make(json.RawMessage, len(m))
		copy(c, m)
		out[i] = c
	}
	return out
}

// Calls returns a snapshot of every JSON-RPC method and params payload
// received by the stub.
func (s *RuntimeStub) Calls() []RuntimeCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RuntimeCall, len(s.calls))
	for i, c := range s.calls {
		out[i] = RuntimeCall{Method: c.Method}
		if c.Params != nil {
			out[i].Params = append(json.RawMessage(nil), c.Params...)
		}
	}
	return out
}

// SetSessionDetail sets the response for session.details.
func (s *RuntimeStub) SetSessionDetail(detail protocol.SessionDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionDetail = detail
}

// SetSessionResume sets the response for session.resume.
func (s *RuntimeStub) SetSessionResume(resp protocol.SessionResumeResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionResume = resp
}

// SetRuns sets the response for run.list.
func (s *RuntimeStub) SetRuns(runs []protocol.RunSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = append([]protocol.RunSummary(nil), runs...)
}

// SetRunListError makes run.list return a JSON-RPC error with message.
func (s *RuntimeStub) SetRunListError(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runListError = message
}

// Close stops the listener and removes the socket dir.
func (s *RuntimeStub) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	err := s.listener.Close()
	_ = os.RemoveAll(filepath.Dir(s.socketPath))
	return err
}

func (s *RuntimeStub) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go s.handleConn(conn)
	}
}

// handleConn reads line-delimited JSON-RPC requests and writes
// responses inline. The harness's Client uses a length-prefix framing
// in some configurations; this stub mirrors what `jsonrpc` framing
// the harness emits over unix sockets, which is line-delimited NDJSON
// per the protocol spec.
func (s *RuntimeStub) handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return
		}
		if len(line) == 0 {
			continue
		}
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		s.recordCall(req.Method, req.Params)
		switch req.Method {
		case "capabilities.register":
			s.respond(w, req.ID, map[string]any{
				"protocol_version":    "1",
				"max_frame_size":      1 << 20,
				"server_capabilities": map[string]any{},
			}, nil)
		case "session.create":
			// PATCH-0028: harness auto-calls session.create on
			// ConnStateReady. Return a stub session id so the
			// harness can populate r.mode.SessionID().
			s.respond(w, req.ID, map[string]any{
				"session_id": "stub-session",
				"project":    map[string]any{"root": "/tmp"},
			}, nil)
		case "session.list":
			s.respond(w, req.ID, map[string]any{
				"sessions": []any{},
			}, nil)
		case "session.resume":
			resume := s.currentSessionResume(req.Params)
			s.respond(w, req.ID, resume, nil)
		case "session.details":
			detail := s.currentSessionDetail(req.Params)
			s.respond(w, req.ID, detail, nil)
		case "run.list":
			runs, errMessage := s.currentRuns()
			if errMessage != "" {
				s.respond(w, req.ID, nil, &rpcError{Code: -32000, Message: errMessage})
			} else {
				s.respond(w, req.ID, protocol.RunListResponse{Runs: runs}, nil)
			}
		case "turn.submit":
			s.mu.Lock()
			s.submits = append(s.submits, append(json.RawMessage(nil), req.Params...))
			s.mu.Unlock()
			id := fmt.Sprintf("stub-turn-%d", s.nextTurnID.Add(1))
			s.respond(w, req.ID, map[string]any{
				"turn_id":    id,
				"session_id": "stub-session",
				"status":     "accepted",
			}, nil)
		case "turn.cancel":
			s.mu.Lock()
			s.cancels = append(s.cancels, append(json.RawMessage(nil), req.Params...))
			s.mu.Unlock()
			s.respond(w, req.ID, map[string]any{"status": "canceled"}, nil)
		case "ping":
			s.respond(w, req.ID, map[string]any{"ok": true}, nil)
		default:
			s.respond(w, req.ID, nil, &rpcError{
				Code:    -32601,
				Message: "method not implemented in RuntimeStub: " + req.Method,
			})
		}
		_ = w.Flush()
	}
}

func (s *RuntimeStub) recordCall(method string, params json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := RuntimeCall{Method: method}
	if params != nil {
		c.Params = append(json.RawMessage(nil), params...)
	}
	s.calls = append(s.calls, c)
}

func (s *RuntimeStub) currentSessionDetail(params json.RawMessage) protocol.SessionDetail {
	s.mu.Lock()
	detail := s.sessionDetail
	s.mu.Unlock()
	if detail.ID != "" {
		return detail
	}
	var req protocol.SessionDetails
	_ = json.Unmarshal(params, &req)
	return protocol.SessionDetail{
		ID:         req.SessionID,
		Summary:    "Stub session",
		CreatedAt:  "2026-05-21T00:00:00Z",
		LastActive: "2026-05-21T00:01:00Z",
	}
}

func (s *RuntimeStub) currentSessionResume(params json.RawMessage) protocol.SessionResumeResponse {
	s.mu.Lock()
	resume := s.sessionResume
	s.mu.Unlock()
	if resume.SessionID != "" {
		return resume
	}
	var req protocol.SessionResume
	_ = json.Unmarshal(params, &req)
	return protocol.SessionResumeResponse{
		SessionID:    req.SessionID,
		Project:      protocol.ProjectContext{Root: "/tmp"},
		NextSequence: 1,
	}
}

func (s *RuntimeStub) currentRuns() ([]protocol.RunSummary, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocol.RunSummary(nil), s.runs...), s.runListError
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *RuntimeStub) respond(w *bufio.Writer, id json.RawMessage, result any, errPayload *rpcError) {
	resp := map[string]any{"jsonrpc": "2.0"}
	if id != nil {
		resp["id"] = id
	}
	if errPayload != nil {
		resp["error"] = errPayload
	} else {
		resp["result"] = result
	}
	bts, _ := json.Marshal(resp)
	_, _ = w.Write(bts)
	_, _ = w.Write([]byte{'\n'})
}

// Compile-time noop to keep ctx import alive when the stub adds a
// context-aware operation in a follow-up.
var _ = context.Background
