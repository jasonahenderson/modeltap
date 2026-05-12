package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

// newRequestsExportCommand registers `modeltap requests export`. Uses
// the package-shared requestsStore (lazy-opened in production,
// injected in tests). Per PATCH-0020, this replaces the previous
// `modeltap export` top-level command.
func newRequestsExportCommand() *cobra.Command {
	var (
		format  string
		runID   string
		traceID string
		since   string
		until   string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export captured requests to JSONL or CSV",
		Long: `Export captured request/response entries to JSONL or CSV format.

Writes all matching captures to stdout in the chosen format. JSONL
(JSON Lines) outputs one JSON object per line, suitable for streaming
ingestion. CSV outputs a header row followed by one row per request.

Both formats include: id, timestamp, provider, model, status, input_tokens,
output_tokens, latency_ms, cost, run_id, and trace_id. Redirect stdout to a
file to save the output.

Time filters (--since, --until) accept either a duration shorthand relative
to now (e.g. "24h", "7d") or an RFC3339 timestamp.`,
		Example: `  # Export all captures as JSONL (default format)
  modeltap requests export > captures.jsonl

  # Export as CSV
  modeltap requests export --format csv > captures.csv

  # Export only the last 7 days
  modeltap requests export --since 7d > recent.jsonl

  # Export captures for a durable run
  modeltap requests export --run run-123 > run-captures.jsonl

  # Export a specific time window as CSV
  modeltap requests export --format csv --since 2026-03-01T00:00:00Z --until 2026-03-08T00:00:00Z`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "jsonl" && format != "csv" {
				return fmt.Errorf("invalid format %q: must be jsonl or csv", format)
			}

			if requestsStore == nil {
				s, err := openStoreFromConfig()
				if err != nil {
					return err
				}
				defer s.Close()
				requestsStore = s
				defer func() { requestsStore = nil }()
			}

			filter := storage.ListFilter{
				RunID:   runID,
				TraceID: traceID,
			}

			if since != "" {
				t, err := parseTimeFlag(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				filter.Since = &t
			}

			if until != "" {
				t, err := parseTimeFlag(until)
				if err != nil {
					return fmt.Errorf("invalid --until value %q: %w", until, err)
				}
				filter.Until = &t
			}

			ctx := context.Background()
			requests, err := requestsStore.ListRequests(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing requests: %w", err)
			}

			w := cmd.OutOrStdout()

			switch format {
			case "jsonl":
				return writeJSONL(w, requests)
			case "csv":
				return writeCSV(w, requests)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "jsonl", "Output format: jsonl or csv")
	cmd.Flags().StringVar(&runID, "run", "", "Filter by durable run id")
	cmd.Flags().StringVar(&traceID, "trace", "", "Filter by trace id")
	cmd.Flags().StringVar(&since, "since", "", "Filter requests after this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Filter requests before this time (duration like 24h/7d or RFC3339)")

	return cmd
}

// exportRecord is the JSON-serializable representation of a Request for JSONL output.
type exportRecord struct {
	ID           string  `json:"id"`
	Timestamp    string  `json:"timestamp"`
	RunID        string  `json:"run_id"`
	TraceID      string  `json:"trace_id"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Status       int     `json:"status"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	LatencyMs    int64   `json:"latency_ms"`
	Cost         float64 `json:"cost"`
}

func toExportRecord(r storage.Request) exportRecord {
	return exportRecord{
		ID:           r.ID,
		Timestamp:    r.Timestamp.Format(time.RFC3339Nano),
		RunID:        r.RunID,
		TraceID:      r.TraceID,
		Provider:     r.Provider,
		Model:        r.Model,
		Status:       r.ResponseStatus,
		InputTokens:  r.InputTokens,
		OutputTokens: r.OutputTokens,
		LatencyMs:    r.LatencyMs,
		Cost:         r.EstimatedCostUSD,
	}
}

func writeJSONL(w io.Writer, requests []storage.Request) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, r := range requests {
		if err := enc.Encode(toExportRecord(r)); err != nil {
			return fmt.Errorf("encoding JSONL: %w", err)
		}
	}
	return nil
}

var csvHeader = []string{
	"id", "timestamp", "run_id", "trace_id", "provider", "model", "status",
	"input_tokens", "output_tokens", "latency_ms", "cost",
}

func writeCSV(w io.Writer, requests []storage.Request) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(csvHeader); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, r := range requests {
		row := []string{
			r.ID,
			r.Timestamp.Format(time.RFC3339Nano),
			r.RunID,
			r.TraceID,
			r.Provider,
			r.Model,
			strconv.Itoa(r.ResponseStatus),
			strconv.FormatInt(r.InputTokens, 10),
			strconv.FormatInt(r.OutputTokens, 10),
			strconv.FormatInt(r.LatencyMs, 10),
			strconv.FormatFloat(r.EstimatedCostUSD, 'f', -1, 64),
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}
	return nil
}

// durationPattern matches durations like "7d", "24h", "30m", "60s".
var durationPattern = regexp.MustCompile(`^(\d+)([dhms])$`)

// parseTimeFlag parses a time flag value as either a duration shorthand
// (e.g., "7d", "24h") interpreted as that much time ago from now, or as an
// RFC3339 timestamp.
func parseTimeFlag(val string) (time.Time, error) {
	// Try duration pattern first.
	if m := durationPattern.FindStringSubmatch(val); m != nil {
		n, _ := strconv.Atoi(m[1])
		var d time.Duration
		switch m[2] {
		case "d":
			d = time.Duration(n) * 24 * time.Hour
		case "h":
			d = time.Duration(n) * time.Hour
		case "m":
			d = time.Duration(n) * time.Minute
		case "s":
			d = time.Duration(n) * time.Second
		}
		return time.Now().UTC().Add(-d), nil
	}

	// Try RFC3339.
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected duration (e.g. 24h, 7d) or RFC3339 timestamp")
	}
	return t, nil
}
