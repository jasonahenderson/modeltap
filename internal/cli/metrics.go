package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

// metricsStore is a package-level variable that allows injecting a Store for
// the metrics command. In production this is set via SetMetricsStore before
// command execution; in tests it is set directly.
var metricsStore storage.Store

// SetMetricsStore sets the store used by the metrics command.
func SetMetricsStore(s storage.Store) {
	metricsStore = s
}

func newMetricsCommand() *cobra.Command {
	var (
		since   string
		until   string
		groupBy string
		format  string
	)

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show usage metrics",
		Long: `Display aggregated usage metrics for captured API traffic.

Shows request counts, token usage, estimated costs, average latency, and
error counts. By default, metrics cover the last 30 days and are displayed
as a table. Use --group-by to segment results by provider, model, day, or
hour.

Output can be formatted as a table (default), JSON, or CSV for integration
with other tools or dashboards.`,
		Example: `  # Show metrics for the last 30 days (default)
  modeltap metrics

  # Group by provider
  modeltap metrics --group-by provider

  # Group by model for the last 7 days, output as JSON
  modeltap metrics --group-by model --since 7d --format json

  # Hourly breakdown as CSV
  modeltap metrics --group-by hour --format csv > hourly.csv

  # Metrics for a specific time window
  modeltap metrics --since 2026-03-01T00:00:00Z --until 2026-03-08T00:00:00Z`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if metricsStore == nil {
				return fmt.Errorf("no store configured")
			}

			filter := storage.MetricsFilter{}

			if groupBy != "" {
				switch groupBy {
				case "provider", "model", "day", "hour":
					filter.GroupBy = groupBy
				default:
					return fmt.Errorf("invalid --group-by value %q: must be provider, model, day, or hour", groupBy)
				}
			}

			if since != "" {
				t, err := parseTimeFlag(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				filter.Since = &t
			} else {
				// Default: last 30 days.
				t, _ := parseTimeFlag("30d")
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

			var metrics []storage.UsageMetrics
			var err error

			if groupBy == "hour" {
				metrics, err = metricsStore.QueryHourlyMetrics(ctx, filter)
			} else {
				metrics, err = metricsStore.QueryDailyMetrics(ctx, filter)
			}
			if err != nil {
				return fmt.Errorf("querying metrics: %w", err)
			}

			w := cmd.OutOrStdout()

			switch format {
			case "json":
				return writeMetricsJSON(w, metrics)
			case "csv":
				return writeMetricsCSV(w, metrics)
			case "table":
				return writeMetricsTable(w, metrics)
			default:
				return fmt.Errorf("invalid --format value %q: must be table, json, or csv", format)
			}
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Filter metrics after this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Filter metrics before this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "Group output by: provider, model, day, or hour")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, or csv")

	cmd.AddCommand(newMetricsRebuildCommand())

	return cmd
}

func newMetricsRebuildCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild metrics from stored logs",
		Long: `Rebuild aggregated metrics by re-processing all stored log entries.

This is useful after a database migration, manual data correction, or if
metrics appear out of sync with the raw logs. The operation scans every
stored request and recomputes all aggregated metric rows.`,
		Example: `  # Rebuild all metrics from stored logs
  modeltap metrics rebuild`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if metricsStore == nil {
				return fmt.Errorf("no store configured")
			}

			ctx := context.Background()
			if err := metricsStore.RebuildMetrics(ctx); err != nil {
				return fmt.Errorf("rebuilding metrics: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Metrics rebuilt successfully.")
			return nil
		},
	}
}

func writeMetricsTable(w interface{ Write([]byte) (int, error) }, metrics []storage.UsageMetrics) error {
	if len(metrics) == 0 {
		fmt.Fprintln(w, "No metrics found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PERIOD\tPROVIDER\tMODEL\tREQUESTS\tINPUT TOKENS\tOUTPUT TOKENS\tCOST\tAVG LATENCY\tERRORS")

	for _, m := range metrics {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t$%.4f\t%dms\t%d\n",
			m.Period,
			m.Provider,
			m.Model,
			m.RequestCount,
			m.InputTokens,
			m.OutputTokens,
			m.EstimatedCost,
			m.AvgLatencyMs,
			m.ErrorCount,
		)
	}

	return tw.Flush()
}

func writeMetricsJSON(w interface{ Write([]byte) (int, error) }, metrics []storage.UsageMetrics) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(metrics)
}

var metricsCSVHeader = []string{
	"period", "provider", "model", "requests",
	"input_tokens", "output_tokens", "cost", "avg_latency_ms", "errors",
}

func writeMetricsCSV(w interface{ Write([]byte) (int, error) }, metrics []storage.UsageMetrics) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(metricsCSVHeader); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, m := range metrics {
		row := []string{
			m.Period,
			m.Provider,
			m.Model,
			strconv.FormatInt(m.RequestCount, 10),
			strconv.FormatInt(m.InputTokens, 10),
			strconv.FormatInt(m.OutputTokens, 10),
			strconv.FormatFloat(m.EstimatedCost, 'f', 4, 64),
			strconv.FormatInt(m.AvgLatencyMs, 10),
			strconv.FormatInt(m.ErrorCount, 10),
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}
	return nil
}
