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

// Store is the interface for persisting and querying captured requests.
type Store interface {
	SaveRequest(ctx context.Context, req *Request) error
	GetRequest(ctx context.Context, id string) (*Request, error)
	ListRequests(ctx context.Context, filter ListFilter) ([]Request, error)
	CountRequests(ctx context.Context, filter ListFilter) (int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
	Close() error
}
