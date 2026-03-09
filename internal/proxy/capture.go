package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// CaptureMiddleware intercepts proxied requests and responses, saving them
// to the Store for later inspection. It detects the provider using the
// registry and extracts metadata (model, tokens, etc.) from the bodies.
type CaptureMiddleware struct {
	store    storage.Store
	registry *provider.Registry
}

// NewCaptureMiddleware creates a new CaptureMiddleware.
func NewCaptureMiddleware(store storage.Store, registry *provider.Registry) *CaptureMiddleware {
	return &CaptureMiddleware{
		store:    store,
		registry: registry,
	}
}

// Wrap returns an http.Handler that captures the request/response and then
// delegates to next. The capture is saved asynchronously so it does not add
// latency to the response.
func (m *CaptureMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 1. Read and re-buffer the request body so the proxy can still use it.
		var reqBody []byte
		if r.Body != nil {
			var err error
			reqBody, err = io.ReadAll(r.Body)
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

		// 6. Check if this is a streaming response — skip capture for SSE (WU-015).
		contentType := rec.Header().Get("Content-Type")
		if strings.Contains(contentType, "text/event-stream") {
			return
		}

		// 7. Build the storage record and save asynchronously.
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
			ResponseBody:    rec.body.String(),
			LatencyMs:       latency.Milliseconds(),
		}

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

		// Fire and forget — save asynchronously to avoid blocking the response.
		go func() {
			_ = m.store.SaveRequest(context.Background(), record)
		}()
	})
}

// sanitizeHeaders returns a copy of the headers as map[string]string (first value only).
func sanitizeHeaders(h http.Header) map[string]string {
	result := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			result[k] = v[0]
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
