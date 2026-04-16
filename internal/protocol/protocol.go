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
// NDJSON framing reader/writer, the Mode enum, and the 19 harness->server
// request types (declared in messages.go). Server->harness streaming events,
// session/tool/model/health/error/compact payloads, and cross-track
// conformance fixtures are added by WU-040, WU-041, and WU-093 respectively.
package protocol

import (
	"encoding/json"
	"errors"
	"io"
)

// ProtocolVersion is the wire-format protocol version advertised in
// capabilities.register and connection.health. Bumped on incompatible
// changes to the protocol message catalog.
const ProtocolVersion = ""

// MaxFrameSize is the maximum size in bytes of a single NDJSON frame.
//
// The cap exists to bound attacker-controlled memory use: turn.submit
// payloads may carry attachments (base64-encoded raw bytes + extracted
// text) and large pastes, so the limit is generous enough to accommodate
// typical documents and screenshots but small enough to prevent trivial
// exhaustion. Enforced on the reader side only; writer-side policing is
// the caller's responsibility.
const MaxFrameSize = 0

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

// errNotImplemented is the red-phase stub sentinel; replaced in the
// green phase with real framing logic.
var errNotImplemented = errors.New("protocol: not implemented")

// Request is the JSON-RPC 2.0 request envelope.
//
// Method selects the handler; Params is the method-specific payload held as
// raw JSON so callers can decode into the correct typed struct. ID is raw
// JSON because JSON-RPC permits string, number, or null identifiers.
type Request struct {
	JSONRPC string          // JSON tags added in the green phase
	ID      json.RawMessage
	Method  string
	Params  json.RawMessage
}

// Response is the JSON-RPC 2.0 response envelope. Exactly one of Result or
// Error is set.
type Response struct {
	JSONRPC string
	ID      json.RawMessage
	Result  json.RawMessage
	Error   *ErrorObject
}

// ErrorObject is the JSON-RPC 2.0 error shape. Data carries optional
// diagnostic details (e.g., an MT-CONN-* diagnostic code and cause).
type ErrorObject struct {
	Code    int
	Message string
	Data    json.RawMessage
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
	// Red-phase stub: always false so Mode tests fail until green phase.
	return false
}

// FrameReader reads NDJSON frames from an io.Reader.
//
// Each call to ReadFrame returns a single frame's raw JSON bytes (without
// the trailing newline). EOF is returned cleanly between frames.
type FrameReader struct {
	r io.Reader
}

// NewFrameReader returns a FrameReader wrapping r.
func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{r: r}
}

// ReadFrame reads the next NDJSON frame from the underlying reader.
func (fr *FrameReader) ReadFrame() ([]byte, error) {
	return nil, errNotImplemented
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
func (fw *FrameWriter) WriteFrame(b []byte) error {
	return errNotImplemented
}

// compile-time type check that json.RawMessage is reachable from this
// package so that envelope types can reference it in the green phase.
var _ = json.RawMessage{}
