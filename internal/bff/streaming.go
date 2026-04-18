package bff

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// SSEParser reads Server-Sent Events from an io.Reader, yielding the
// payload bytes of each `data:` line. Empty lines (event separators)
// and `event:` headers are consumed transparently. Returns io.EOF when
// the stream ends.
type SSEParser struct {
	r *bufio.Reader
}

// NewSSEParser wraps r in an SSEParser with a generously-sized buffer
// to accommodate large per-event JSON payloads.
func NewSSEParser(r io.Reader) *SSEParser {
	return &SSEParser{r: bufio.NewReaderSize(r, 64*1024)}
}

// Next returns the next data payload. Returns io.EOF when the upstream
// stream is exhausted. `event:` headers and other field types are
// silently skipped — the BFF only needs the data payload because both
// Anthropic and OpenAI embed event-type info in the JSON.
func (p *SSEParser) Next() ([]byte, error) {
	for {
		line, err := p.r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if line == "" {
					return nil, io.EOF
				}
				// Final line without trailing newline — try to interpret it.
			} else {
				return nil, err
			}
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// Event separator; keep reading.
			continue
		}
		if strings.HasPrefix(line, ":") {
			// SSE comment.
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			return []byte(payload), nil
		}
		// Other field types (event:, id:, retry:) — skip.
	}
}

// StreamRelay reads SSE chunks from a provider HTTP response, translates
// them into protocol notifications, accumulates the full assistant
// response, persists it to storage, and emits a turn.complete event.
//
// Created by handleTurnSubmit (one StreamRelay per turn / branch).
type StreamRelay struct {
	conn     *Connection
	session  *ActiveSession
	turnID   string
	branchID string
	model    string
	provider string
}

// NewStreamRelay constructs a relay bound to the given connection and
// session. branchID is empty for single-model turns; populated for
// per-branch streams in WU-060 multi-model.
func NewStreamRelay(conn *Connection, session *ActiveSession, turnID, branchID, model, providerName string) *StreamRelay {
	return &StreamRelay{
		conn:     conn,
		session:  session,
		turnID:   turnID,
		branchID: branchID,
		model:    model,
		provider: providerName,
	}
}

// relayResult is the in-memory accumulation of a streaming response.
// Returned by Relay alongside the persisted turn so the caller (cost
// tracker, multi-model aggregator) can see the final stats.
type relayResult struct {
	Content      string
	ToolCalls    []provider.ToolCall
	InputTokens  int64
	OutputTokens int64
}

// Relay reads the SSE stream, emits per-chunk notifications, and on
// completion persists the assistant turn and emits turn.complete. The
// caller is responsible for closing httpResp.Body if Relay returns an
// error before consuming it; on success Relay closes the body.
func (sr *StreamRelay) Relay(ctx context.Context, httpResp io.ReadCloser, adapter provider.Provider) (*storage.Turn, error) {
	startedAt := time.Now()
	defer httpResp.Close()

	parser := NewSSEParser(httpResp)
	acc := &relayResult{}

	// Tool-call accumulator. Tool calls arrive in pieces (start +
	// argument deltas + end); collect the running input bytes per call.
	type toolBuilder struct {
		id    string
		name  string
		input strings.Builder
	}
	var pendingTools []*toolBuilder

	var streamErr string

readLoop:
	for {
		select {
		case <-ctx.Done():
			streamErr = "cancelled"
			break readLoop
		default:
		}

		raw, err := parser.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			streamErr = err.Error()
			break
		}

		ev, err := adapter.ParseStreamEvent(raw)
		if err != nil {
			// Forward-compat: parse errors on individual events should
			// not poison the whole stream — log via streamErr but keep
			// reading. If the upstream is genuinely broken the next
			// Next() will fail too.
			continue
		}
		if ev == nil {
			continue
		}

		switch ev.Type {
		case provider.StreamEventText:
			acc.Content += ev.Content
			delta := protocol.TokenDelta{TurnID: sr.turnID, BranchID: sr.branchID, Text: ev.Content}
			sr.sendNotification(protocol.EventTokenDelta, delta)

		case provider.StreamEventToolCallStart:
			tb := &toolBuilder{id: ev.ToolCall.ID, name: ev.ToolCall.Name}
			tb.input.WriteString(ev.ToolCall.Input)
			pendingTools = append(pendingTools, tb)

		case provider.StreamEventToolCallDelta:
			if len(pendingTools) > 0 {
				pendingTools[len(pendingTools)-1].input.WriteString(ev.ToolCall.Input)
			}

		case provider.StreamEventToolCallEnd:
			if len(pendingTools) == 0 {
				continue
			}
			tb := pendingTools[len(pendingTools)-1]
			tc := provider.ToolCall{
				ID:    tb.id,
				Name:  tb.name,
				Input: json.RawMessage(tb.input.String()),
			}
			acc.ToolCalls = append(acc.ToolCalls, tc)
			tcEv := protocol.ToolCall{
				TurnID:     sr.turnID,
				ToolCallID: tb.id,
				Tool:       tb.name,
				Input:      tc.Input,
			}
			sr.sendNotification(protocol.EventToolCall, tcEv)

		case provider.StreamEventUsage:
			if ev.Usage == nil {
				continue
			}
			if ev.Usage.InputTokens > 0 {
				acc.InputTokens = int64(ev.Usage.InputTokens)
			}
			if ev.Usage.OutputTokens > 0 {
				acc.OutputTokens = int64(ev.Usage.OutputTokens)
			}

		case provider.StreamEventDone:
			break readLoop

		case provider.StreamEventError:
			streamErr = ev.Error
			break readLoop
		}
	}

	// Persist the assistant turn unconditionally — even if the stream
	// errored mid-flight, partial content is preserved (design D2.2 §4).
	turnPersisted, persistErr := sr.persistAssistant(ctx, acc)
	if persistErr != nil {
		// Surface the persist error if we don't already have a stream
		// error to report.
		if streamErr == "" {
			streamErr = persistErr.Error()
		}
	}

	cancelled := streamErr == "cancelled"
	complete := protocol.TurnComplete{
		TurnID:            sr.turnID,
		FinalInputTokens:  int(acc.InputTokens),
		FinalOutputTokens: int(acc.OutputTokens),
		Model:             sr.model,
		Provider:          sr.provider,
		LatencyMs:         int(time.Since(startedAt).Milliseconds()),
		Cancelled:         cancelled,
	}
	if sr.session != nil {
		// TotalCost is updated by the cost tracker (WU-056); mirror its
		// most recent value into the per-turn complete event.
		complete.TotalCost = sr.session.TotalCost
	}
	sr.sendNotification(protocol.EventTurnComplete, complete)

	if streamErr != "" && !cancelled {
		errEv := protocol.ServerError{
			TurnID:  sr.turnID,
			Code:    "provider_error",
			Message: streamErr,
			Diagnostic: protocol.Diagnostic{
				Code:     protocol.DiagProviderUnavailable,
				Category: "provider",
				Cause:    streamErr,
			},
		}
		sr.sendNotification(protocol.EventError, errEv)
		return turnPersisted, fmt.Errorf("relay: %s", streamErr)
	}
	return turnPersisted, nil
}

// persistAssistant appends the accumulated response to the conversation
// and writes the resulting Turn via the store.
func (sr *StreamRelay) persistAssistant(ctx context.Context, acc *relayResult) (*storage.Turn, error) {
	if sr.session == nil {
		return nil, nil
	}
	resp := AssistantResponse{
		Content:      acc.Content,
		ToolCalls:    acc.ToolCalls,
		Model:        sr.model,
		Provider:     sr.provider,
		InputTokens:  acc.InputTokens,
		OutputTokens: acc.OutputTokens,
	}
	turn, err := sr.session.Conversation.AppendAssistantTurn(resp)
	if err != nil {
		return nil, fmt.Errorf("append assistant turn: %w", err)
	}
	if sr.conn != nil && sr.conn.server != nil && sr.conn.server.store != nil {
		if err := sr.conn.server.store.CreateTurn(ctx, turn); err != nil {
			return turn, fmt.Errorf("persist turn: %w", err)
		}
	}
	return turn, nil
}

// sendNotification marshals payload and writes it to the connection
// transport. Best-effort — write errors are swallowed because the
// connection's read loop will independently observe a closed transport
// and tear down.
func (sr *StreamRelay) sendNotification(method string, payload any) {
	if sr.conn == nil {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = sr.conn.transport.SendNotification(&protocol.Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	})
}
