package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// maxCaptureBodySize is the maximum number of bytes read from request and
// response bodies during capture. Bodies larger than this are truncated.
// This prevents memory exhaustion from very large payloads (e.g., image
// uploads or massive completions). 10 MB matches the default config value.
const maxCaptureBodySize = 10 * 1024 * 1024 // 10 MB

// CaptureMiddleware intercepts proxied requests and responses, saving them
// to the Store for later inspection. It detects the provider using the
// registry and extracts metadata (model, tokens, etc.) from the bodies.
type CaptureMiddleware struct {
	store    storage.Store
	registry *provider.Registry
	pricing  *config.PricingTable

	// OnSaved is called after a request is saved to the store (or save fails).
	// Used for testing to synchronize with async middleware processing.
	OnSaved func()
}

// NewCaptureMiddleware creates a new CaptureMiddleware.
func NewCaptureMiddleware(store storage.Store, registry *provider.Registry, pricing *config.PricingTable) *CaptureMiddleware {
	return &CaptureMiddleware{
		store:    store,
		registry: registry,
		pricing:  pricing,
	}
}

// Wrap returns an http.Handler that captures the request/response and then
// delegates to next. The capture is saved asynchronously so it does not add
// latency to the response.
func (m *CaptureMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 1. Read and re-buffer the request body so the proxy can still use it.
		//    Limit read to maxCaptureBodySize to prevent memory exhaustion from
		//    very large payloads.
		var reqBody []byte
		if r.Body != nil {
			var err error
			reqBody, err = io.ReadAll(io.LimitReader(r.Body, maxCaptureBodySize))
			if err != nil {
				// If we can't read the body, just forward the request as-is.
				http.Error(w, "failed to read request body", http.StatusBadGateway)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// 2. Detect provider from the request.
		var prov provider.Provider
		if m.registry != nil {
			prov = m.registry.Detect(r)
		}

		// 3. Wrap the ResponseWriter to capture the response.
		rec := newResponseRecorder(w)

		// 4. Forward the request through the proxy.
		next.ServeHTTP(rec, r)

		// 5. Compute latency.
		latency := time.Since(start)

		// 6. Build the storage record.
		reqHeaders, _ := json.Marshal(sanitizeHeaders(r.Header))
		respHeaders, _ := json.Marshal(sanitizeHeaders(rec.Header()))

		record := &storage.Request{
			Timestamp:       start.UTC(),
			Method:          r.Method,
			URL:             r.URL.String(),
			RequestHeaders:  string(reqHeaders),
			RequestBody:     string(reqBody),
			ResponseStatus:  rec.statusCode,
			ResponseHeaders: string(respHeaders),
			LatencyMs:       latency.Milliseconds(),
		}

		// 7. Check if this is a streaming (SSE) response.
		contentType := rec.Header().Get("Content-Type")
		isSSE := strings.Contains(contentType, "text/event-stream")

		if isSSE {
			// Parse the buffered SSE data into StreamChunks and reassemble.
			chunks := parseSSEChunks(rec.body.Bytes())

			if prov != nil {
				record.Provider = prov.Name()

				// Parse request metadata.
				if reqMeta, err := prov.ParseRequest(reqBody, r.Header); err == nil && reqMeta != nil {
					record.Model = reqMeta.Model
				}

				// Reassemble stream to get full response text and metadata.
				if respMeta, text, err := prov.ReassembleStream(chunks); err == nil && respMeta != nil {
					record.ResponseBody = text
					if respMeta.Model != "" {
						record.Model = respMeta.Model
					}
					record.InputTokens = respMeta.InputTokens
					record.OutputTokens = respMeta.OutputTokens
				}
			} else {
				// No provider detected — save the raw SSE data.
				record.ResponseBody = rec.body.String()
			}
		} else {
			// Non-streaming response — use body as-is.
			record.ResponseBody = rec.body.String()

			// Extract provider name and metadata.
			if prov != nil {
				record.Provider = prov.Name()

				// Parse request metadata.
				if reqMeta, err := prov.ParseRequest(reqBody, r.Header); err == nil && reqMeta != nil {
					record.Model = reqMeta.Model
				}

				// Parse response metadata.
				if respMeta, err := prov.ParseResponse(rec.body.Bytes(), rec.Header(), rec.statusCode); err == nil && respMeta != nil {
					if respMeta.Model != "" {
						record.Model = respMeta.Model
					}
					record.InputTokens = respMeta.InputTokens
					record.OutputTokens = respMeta.OutputTokens
				}
			}
		}

		// Estimate cost using the pricing table.
		if m.pricing != nil && record.Provider != "" && record.Model != "" {
			record.EstimatedCostUSD = m.pricing.EstimateCost(
				record.Provider, record.Model,
				record.InputTokens, record.OutputTokens,
			)
		}

		// Save after response is complete. The response has already been sent
		// to the client, so this doesn't add latency to the user's request.
		go func() {
			if err := m.store.SaveRequest(context.Background(), record); err != nil {
				slog.Error("failed to save captured request", "error", err)
			}
			if m.OnSaved != nil {
				m.OnSaved()
			}
		}()
	})
}

// sensitiveHeaders lists header names whose values must be redacted before
// storage. Comparison is case-insensitive (http.Header canonicalises keys).
var sensitiveHeaders = map[string]bool{
	"Authorization":   true,
	"X-Api-Key":       true,
	"Api-Key":         true,
	"Proxy-Authorization": true,
}

// sanitizeHeaders returns a copy of the headers as map[string]string (first
// value only). Values of sensitive headers (Authorization, X-Api-Key, etc.)
// are redacted to prevent credential exposure in stored data.
func sanitizeHeaders(h http.Header) map[string]string {
	result := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			if sensitiveHeaders[http.CanonicalHeaderKey(k)] {
				result[k] = "[REDACTED]"
			} else {
				result[k] = v[0]
			}
		}
	}
	return result
}

// responseRecorder wraps an http.ResponseWriter to capture the status code,
// headers, and body while still writing everything through to the client.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	wroteHeader bool
}

// newResponseRecorder creates a responseRecorder wrapping the given writer.
func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default if WriteHeader is never called
	}
}

// WriteHeader captures the status code and passes it through.
func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.statusCode = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Write captures the body bytes and passes them through to the client.
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Flush implements http.Flusher if the underlying writer supports it.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// parseSSEChunks parses raw SSE bytes into StreamChunks.
//
// SSE format: events are separated by blank lines (\n\n). Each event
// may have an "event:" line (the event type) and a "data:" line (the payload).
//
// For providers that use "event:" lines (e.g., Anthropic), the EventType is
// populated and Data contains only the JSON payload. For providers that don't
// use "event:" lines (e.g., OpenAI), EventType is empty and Data contains the
// raw "data: ..." line as the provider's ReassembleStream expects it.
func parseSSEChunks(raw []byte) []provider.StreamChunk {
	var chunks []provider.StreamChunk

	// Split on double-newline boundaries to get individual events.
	events := bytes.Split(raw, []byte("\n\n"))

	for _, event := range events {
		event = bytes.TrimSpace(event)
		if len(event) == 0 {
			continue
		}

		var eventType string
		var dataLine []byte

		lines := bytes.Split(event, []byte("\n"))
		for _, line := range lines {
			if bytes.HasPrefix(line, []byte("event: ")) || bytes.HasPrefix(line, []byte("event:")) {
				eventType = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
			} else if bytes.HasPrefix(line, []byte("data: ")) || bytes.HasPrefix(line, []byte("data:")) {
				dataLine = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			}
		}

		if dataLine == nil {
			continue
		}

		if eventType != "" {
			// Provider uses event types (e.g., Anthropic) — Data is the raw JSON payload.
			chunks = append(chunks, provider.StreamChunk{
				EventType: eventType,
				Data:      dataLine,
			})
		} else {
			// No event type (e.g., OpenAI) — Data is the raw "data: ..." line
			// as OpenAI's ReassembleStream expects.
			chunks = append(chunks, provider.StreamChunk{
				Data: []byte("data: " + string(dataLine)),
			})
		}
	}

	return chunks
}
