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

// exportStore is a package-level variable that allows injecting a Store for
// the export command. In production this is set via SetExportStore before
// command execution; in tests it is set directly.
var exportStore storage.Store

// SetExportStore sets the store used by the export command.
func SetExportStore(s storage.Store) {
	exportStore = s
}

func newExportCommand() *cobra.Command {
	var (
		format string
		since  string
		until  string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export logs to JSONL or CSV",
		Long: `Export captured request/response logs to JSONL or CSV format.

Writes all matching log entries to stdout in the chosen format. JSONL
(JSON Lines) outputs one JSON object per line, suitable for streaming
ingestion. CSV outputs a header row followed by one row per request.

Both formats include: id, timestamp, provider, model, status, input_tokens,
output_tokens, latency_ms, and cost. Redirect stdout to a file to save
the output.

Time filters (--since, --until) accept either a duration shorthand relative
to now (e.g. "24h", "7d") or an RFC3339 timestamp.`,
		Example: `  # Export all logs as JSONL (default format)
  modeltap export > logs.jsonl

  # Export as CSV
  modeltap export --format csv > logs.csv

  # Export only the last 7 days
  modeltap export --since 7d > recent.jsonl

  # Export a specific time window as CSV
  modeltap export --format csv --since 2026-03-01T00:00:00Z --until 2026-03-08T00:00:00Z`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "jsonl" && format != "csv" {
				return fmt.Errorf("invalid format %q: must be jsonl or csv", format)
			}

			if exportStore == nil {
				return fmt.Errorf("no store configured")
			}

			filter := storage.ListFilter{}

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
			requests, err := exportStore.ListRequests(ctx, filter)
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
	cmd.Flags().StringVar(&since, "since", "", "Filter requests after this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Filter requests before this time (duration like 24h/7d or RFC3339)")

	return cmd
}

// exportRecord is the JSON-serializable representation of a Request for JSONL output.
type exportRecord struct {
	ID           string  `json:"id"`
	Timestamp    string  `json:"timestamp"`
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
	"id", "timestamp", "provider", "model", "status",
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
