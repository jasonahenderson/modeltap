// Package dashboard provides REST API endpoints for the modeltap web dashboard.
package dashboard

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// APIHandler serves the dashboard JSON API endpoints.
type APIHandler struct {
	store  storage.Store
	config *config.Config
}

// NewAPIHandler creates a new API handler backed by the given store and config.
func NewAPIHandler(store storage.Store, cfg *config.Config) *APIHandler {
	return &APIHandler{
		store:  store,
		config: cfg,
	}
}

// RegisterRoutes registers all dashboard API routes on the given mux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// API routes.
	mux.HandleFunc("GET /api/logs", h.handleLogs)
	mux.HandleFunc("GET /api/logs/{id}", h.handleLogDetail)
	mux.HandleFunc("GET /api/metrics", h.handleMetrics)
	mux.HandleFunc("GET /api/status", h.handleStatus)

	// Serve embedded static assets.
	staticSub, err := fs.Sub(StaticFS, "static")
	if err != nil {
		panic("dashboard: failed to create static sub-filesystem: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(staticSub))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		fileServer.ServeHTTP(w, r)
	})))

	// Serve index.html at root.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := StaticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		securityHeaders(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
}

// securityHeaders sets common security headers on the response.
func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	securityHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// parseTime parses an RFC3339 time string, returning nil if empty.
func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// requestJSON is the JSON representation of a stored request.
type requestJSON struct {
	ID               string  `json:"id"`
	Timestamp        string  `json:"timestamp"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Method           string  `json:"method"`
	URL              string  `json:"url"`
	RequestHeaders   string  `json:"request_headers"`
	RequestBody      string  `json:"request_body"`
	ResponseStatus   int     `json:"response_status"`
	ResponseHeaders  string  `json:"response_headers"`
	ResponseBody     string  `json:"response_body"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	LatencyMs        int64   `json:"latency_ms"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

func toRequestJSON(r *storage.Request) requestJSON {
	return requestJSON{
		ID:               r.ID,
		Timestamp:        r.Timestamp.Format(time.RFC3339Nano),
		Provider:         r.Provider,
		Model:            r.Model,
		Method:           r.Method,
		URL:              r.URL,
		RequestHeaders:   r.RequestHeaders,
		RequestBody:      r.RequestBody,
		ResponseStatus:   r.ResponseStatus,
		ResponseHeaders:  r.ResponseHeaders,
		ResponseBody:     r.ResponseBody,
		InputTokens:      r.InputTokens,
		OutputTokens:     r.OutputTokens,
		LatencyMs:        r.LatencyMs,
		EstimatedCostUSD: r.EstimatedCostUSD,
	}
}

// handleLogs handles GET /api/logs with pagination and filtering.
func (h *APIHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse limit (default 50, max 1000).
	limit := 50
	if s := q.Get("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
		limit = v
	}
	if limit > 1000 {
		limit = 1000
	}

	// Parse offset.
	offset := 0
	if s := q.Get("offset"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset parameter")
			return
		}
		offset = v
	}

	// Parse time filters.
	since, err := parseTime(q.Get("since"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid since parameter: expected RFC3339 format")
		return
	}
	until, err := parseTime(q.Get("until"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid until parameter: expected RFC3339 format")
		return
	}

	// Parse status filter.
	var statusCode *int
	if s := q.Get("status"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid status parameter")
			return
		}
		statusCode = &v
	}

	filter := storage.ListFilter{
		Provider:   q.Get("provider"),
		Model:      q.Get("model"),
		Since:      since,
		Until:      until,
		StatusCode: statusCode,
		Limit:      limit,
		Offset:     offset,
	}

	ctx := r.Context()

	// Get total count (without pagination).
	countFilter := filter
	countFilter.Limit = 0
	countFilter.Offset = 0
	total, err := h.store.CountRequests(ctx, countFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count requests")
		return
	}

	// Get paginated results.
	requests, err := h.store.ListRequests(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}

	data := make([]requestJSON, 0, len(requests))
	for i := range requests {
		data = append(data, toRequestJSON(&requests[i]))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  data,
		"total": total,
	})
}

// handleLogDetail handles GET /api/logs/{id}.
func (h *APIHandler) handleLogDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing log ID")
		return
	}

	req, err := h.store.GetRequest(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get request")
		return
	}
	if req == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}

	writeJSON(w, http.StatusOK, toRequestJSON(req))
}

// metricsJSON is the JSON representation of usage metrics.
type metricsJSON struct {
	Period        string  `json:"period"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	RequestCount  int64   `json:"request_count"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	EstimatedCost float64 `json:"estimated_cost"`
	AvgLatencyMs  int64   `json:"avg_latency_ms"`
	ErrorCount    int64   `json:"error_count"`
}

func toMetricsJSON(m *storage.UsageMetrics) metricsJSON {
	return metricsJSON{
		Period:        m.Period,
		Provider:      m.Provider,
		Model:         m.Model,
		RequestCount:  m.RequestCount,
		InputTokens:   m.InputTokens,
		OutputTokens:  m.OutputTokens,
		EstimatedCost: m.EstimatedCost,
		AvgLatencyMs:  m.AvgLatencyMs,
		ErrorCount:    m.ErrorCount,
	}
}

// handleMetrics handles GET /api/metrics.
func (h *APIHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	groupBy := q.Get("group_by")
	if groupBy == "" {
		groupBy = "day"
	}

	validGroupBy := map[string]bool{
		"hour": true, "day": true, "provider": true, "model": true,
	}
	if !validGroupBy[groupBy] {
		writeError(w, http.StatusBadRequest, "invalid group_by: must be one of hour, day, provider, model")
		return
	}

	since, err := parseTime(q.Get("since"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid since parameter: expected RFC3339 format")
		return
	}
	until, err := parseTime(q.Get("until"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid until parameter: expected RFC3339 format")
		return
	}

	filter := storage.MetricsFilter{
		Since:    since,
		Until:    until,
		Provider: q.Get("provider"),
		Model:    q.Get("model"),
		GroupBy:  groupBy,
	}

	ctx := r.Context()
	var metrics []storage.UsageMetrics

	switch {
	case groupBy == "hour":
		metrics, err = h.store.QueryHourlyMetrics(ctx, filter)
	default:
		// day, provider, model all use daily metrics
		metrics, err = h.store.QueryDailyMetrics(ctx, filter)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query metrics")
		return
	}

	// For provider/model grouping, aggregate across periods.
	if groupBy == "provider" || groupBy == "model" {
		metrics = aggregateMetrics(metrics, groupBy)
	}

	data := make([]metricsJSON, 0, len(metrics))
	for i := range metrics {
		data = append(data, toMetricsJSON(&metrics[i]))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
	})
}

// aggregateMetrics groups metrics by provider or model, summing across periods.
func aggregateMetrics(metrics []storage.UsageMetrics, groupBy string) []storage.UsageMetrics {
	type key struct{ a, b string }
	agg := make(map[key]*storage.UsageMetrics)
	var order []key

	for i := range metrics {
		m := &metrics[i]
		var k key
		switch groupBy {
		case "provider":
			k = key{a: m.Provider}
		case "model":
			k = key{a: m.Model}
		}

		if existing, ok := agg[k]; ok {
			existing.RequestCount += m.RequestCount
			existing.InputTokens += m.InputTokens
			existing.OutputTokens += m.OutputTokens
			existing.EstimatedCost += m.EstimatedCost
			existing.ErrorCount += m.ErrorCount
			// Recalculate average latency as weighted average
			totalRequests := existing.RequestCount
			if totalRequests > 0 {
				existing.AvgLatencyMs = (existing.AvgLatencyMs*(totalRequests-m.RequestCount) + m.AvgLatencyMs*m.RequestCount) / totalRequests
			}
		} else {
			entry := *m
			switch groupBy {
			case "provider":
				entry.Period = m.Provider
				entry.Model = ""
			case "model":
				entry.Period = m.Model
				entry.Provider = ""
			}
			agg[k] = &entry
			order = append(order, k)
		}
	}

	result := make([]storage.UsageMetrics, 0, len(agg))
	for _, k := range order {
		result = append(result, *agg[k])
	}
	return result
}

// handleStatus handles GET /api/status.
func (h *APIHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Count total records.
	total, err := h.store.CountRequests(r.Context(), storage.ListFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count records")
		return
	}

	// Determine upstream URL. Check providers map first, fall back to config.
	upstream := h.config.Upstream

	writeJSON(w, http.StatusOK, map[string]any{
		"proxy": map[string]any{
			"port":     h.config.Port,
			"upstream": upstream,
		},
		"database": map[string]any{
			"records": total,
		},
		"retention": map[string]any{
			"days": h.config.RetentionDays,
		},
	})
}

// ListenAndServe starts the dashboard HTTP server on the configured bind
// address and port. It blocks until the context is cancelled or an error
// occurs.
func (h *APIHandler) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	bind := h.config.Dashboard.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	port := h.config.Dashboard.Port
	if port == 0 {
		port = 8081
	}

	addr := bind + ":" + strconv.Itoa(port)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Shut down gracefully when context is cancelled.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
