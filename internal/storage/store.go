// Package storage defines the Store interface and types for persisting
// proxied API request/response data.
package storage

import (
	"context"
	"time"
)

// Request represents a captured API request/response pair.
type Request struct {
	ID               string
	Timestamp        time.Time
	Provider         string
	Model            string
	Method           string
	URL              string
	RequestHeaders   string // JSON
	RequestBody      string
	ResponseStatus   int
	ResponseHeaders  string // JSON
	ResponseBody     string
	InputTokens      int64
	OutputTokens     int64
	LatencyMs        int64
	EstimatedCostUSD float64
}

// ListFilter defines criteria for filtering and paginating request listings.
type ListFilter struct {
	Provider   string
	Model      string
	Since      *time.Time
	Until      *time.Time
	StatusCode *int
	Limit      int
	Offset     int
}

// UsageMetrics holds aggregated usage data for a time period.
type UsageMetrics struct {
	Period        string
	Provider      string
	Model         string
	RequestCount  int64
	InputTokens   int64
	OutputTokens  int64
	EstimatedCost float64
	AvgLatencyMs  int64
	ErrorCount    int64
}

// MetricsFilter defines criteria for querying aggregated metrics.
type MetricsFilter struct {
	Since    *time.Time
	Until    *time.Time
	Provider string
	Model    string
	GroupBy  string // "hour", "day", "provider", "model"
}

// Store is the interface for persisting and querying captured requests.
type Store interface {
	SaveRequest(ctx context.Context, req *Request) error
	GetRequest(ctx context.Context, id string) (*Request, error)
	ListRequests(ctx context.Context, filter ListFilter) ([]Request, error)
	CountRequests(ctx context.Context, filter ListFilter) (int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
	QueryHourlyMetrics(ctx context.Context, filter MetricsFilter) ([]UsageMetrics, error)
	QueryDailyMetrics(ctx context.Context, filter MetricsFilter) ([]UsageMetrics, error)
	RebuildMetrics(ctx context.Context) error
	Close() error
}
