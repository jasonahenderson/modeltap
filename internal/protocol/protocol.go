// Package protocol defines the on-the-wire types and framing used by the
// Modeltap BFF server and terminal harness.
//
// The protocol is JSON-RPC 2.0 over NDJSON (newline-delimited JSON) carried
// by either a Unix-domain socket or TLS. Every wire field uses snake_case;
// Go field identifiers use CamelCase and carry an explicit JSON tag.
//
// This package contains only type definitions and serialization helpers.
// Dispatch, validation, and business logic live in higher-level packages
// (internal/bff and internal/harness).
//
// Scope for WU-039: protocol version constants, JSON-RPC 2.0 envelope,
// NDJSON framing reader/writer, the Mode enum, and the 20 harness->server
// request types (declared in messages.go). Server->harness streaming events,
// session/tool/model/health/error/compact payloads, and cross-track
// conformance fixtures are added by WU-040, WU-041, and WU-093 respectively.
//
// Canonical field names (snake_case):
//
//	turn_id, session_id, tool_call_id, content_type, output_type,
//	raw_content, max_output_tokens, protocol_version, harness_version,
//	harness_platform, config_file, config_content, input_schema,
//	output_envelope, risk_level, capabilities_required, added_tools,
//	removed_tools, tool_results, attachments, actions.
//
// Go-side field identifiers use CamelCase; every struct field carries an
// explicit `json:"..."` tag so default lowercasing cannot leak a CamelCase
// form onto the wire.
//
// References:
//   - Feature spec: docs/features/0008-bff-server.md (Protocol Specification,
//     Protocol Messages, Protocol Payload Schemas, Canonical Field Names).
//   - JSON-RPC 2.0: https://www.jsonrpc.org/specification
//   - Cross-track conformance (golden fixtures, round-trip tests against
//     frozen wire samples): see WU-093 deliverables in
//     docs/releases/v0.2.0/track-0-shared.md.
package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// ProtocolVersion is the wire-format protocol version advertised in
// capabilities.register and connection.health. Bumped on incompatible
// changes to the protocol message catalog.
const ProtocolVersion = "1"

// MaxFrameSize is the maximum size in bytes of a single NDJSON frame.
//
// The cap bounds attacker-controlled memory use: turn.submit payloads may
// carry attachments (base64-encoded raw bytes + extracted text) and large
// pastes, so the limit accommodates typical documents and screenshots
// without allowing trivial exhaustion. Enforced on the reader side only;
// writer-side policing is the caller's responsibility.
const MaxFrameSize = 10 * 1024 * 1024 // 10 MiB

// ErrFrameTooLarge is returned by FrameReader.ReadFrame when the next
// frame exceeds MaxFrameSize. The caller should treat this as a terminal
// protocol error and close the connection.
var ErrFrameTooLarge = errors.New("protocol: frame exceeds max size")

// ErrInvalidFrame is returned by FrameReader.ReadFrame when the next frame
// is malformed (for example, the stream ends mid-frame).
var ErrInvalidFrame = errors.New("protocol: invalid frame")

// ErrEmbeddedNewline is returned by FrameWriter.WriteFrame if the supplied
// bytes contain a literal newline. json.Marshal output never contains a
// literal newline byte, so callers must not supply pre-indented JSON.
var ErrEmbeddedNewline = errors.New("protocol: frame contains embedded newline")

// Request is the JSON-RPC 2.0 request envelope.
//
// Method selects the handler; Params is the method-specific payload held as
// raw JSON so callers can decode into the correct typed struct. ID is raw
// JSON because JSON-RPC permits string, number, or null identifiers.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is the JSON-RPC 2.0 response envelope. Exactly one of Result or
// Error is set on any given response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

// ErrorObject is the JSON-RPC 2.0 error shape. Data carries optional
// diagnostic details (e.g., an MT-CONN-* diagnostic code and cause).
type ErrorObject struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Mode is the harness conversational mode submitted on every turn.
type Mode string

const (
	ModePlan  Mode = "plan"
	ModeBuild Mode = "build"
	ModeAuto  Mode = "auto"
)

// Valid reports whether m is one of the defined Mode values.
func (m Mode) Valid() bool {
	switch m {
	case ModePlan, ModeBuild, ModeAuto:
		return true
	default:
		return false
	}
}

// FrameReader reads NDJSON frames from an io.Reader.
//
// Each call to ReadFrame returns a single frame's raw JSON bytes (without
// the trailing newline). io.EOF is returned cleanly between frames.
// ErrFrameTooLarge is returned without buffering the full oversized input.
type FrameReader struct {
	br *bufio.Reader
}

// NewFrameReader returns a FrameReader wrapping r.
func NewFrameReader(r io.Reader) *FrameReader {
	// Use a modest initial buffer; bufio grows as needed. We enforce the
	// MaxFrameSize cap ourselves so we can return the typed error without
	// relying on bufio's "token too long" semantics.
	return &FrameReader{br: bufio.NewReaderSize(r, 64*1024)}
}

// ReadFrame reads the next NDJSON frame from the underlying reader.
//
// On success, returns the raw JSON bytes of the frame, without the trailing
// newline. Returns io.EOF if the stream ended cleanly between frames.
// Returns ErrInvalidFrame (joined with io.ErrUnexpectedEOF) if the stream
// ends mid-frame. Returns ErrFrameTooLarge without consuming more than
// MaxFrameSize bytes if the frame would exceed the cap.
//
// On ErrFrameTooLarge the caller MUST close the connection; the underlying
// reader is left positioned mid-frame and cannot be resynchronized safely
// (finding SR-039-01). This package deliberately does not attempt to
// drain the oversize frame, because an attacker controls when the next
// newline appears and could block the reader indefinitely.
func (fr *FrameReader) ReadFrame() ([]byte, error) {
	var buf bytes.Buffer
	for {
		b, err := fr.br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if buf.Len() == 0 {
					return nil, io.EOF
				}
				// Stream ended mid-frame.
				return nil, errors.Join(ErrInvalidFrame, io.ErrUnexpectedEOF)
			}
			return nil, err
		}
		if b == '\n' {
			// Return a defensive copy so the caller can retain it.
			out := make([]byte, buf.Len())
			copy(out, buf.Bytes())
			return out, nil
		}
		if buf.Len() >= MaxFrameSize {
			return nil, ErrFrameTooLarge
		}
		buf.WriteByte(b)
	}
}

// FrameWriter writes NDJSON frames to an io.Writer.
type FrameWriter struct {
	w io.Writer
}

// NewFrameWriter returns a FrameWriter wrapping w.
func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{w: w}
}

// WriteFrame writes b as a single NDJSON frame (appends a trailing newline).
// b must be a complete JSON object with no literal newline bytes; callers
// should use json.Marshal, not json.MarshalIndent.
//
// Returns ErrEmbeddedNewline if b contains a \n byte. Write errors from
// the underlying io.Writer are returned as-is.
func (fw *FrameWriter) WriteFrame(b []byte) error {
	if bytes.IndexByte(b, '\n') >= 0 {
		return ErrEmbeddedNewline
	}
	if _, err := fw.w.Write(b); err != nil {
		return err
	}
	_, err := fw.w.Write([]byte{'\n'})
	return err
}
