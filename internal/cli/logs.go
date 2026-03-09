package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

// logsStore is a package-level variable that allows injecting a Store for
// the logs command. In production this is set via SetLogsStore before
// command execution; in tests it is set directly.
var logsStore storage.Store

// SetLogsStore sets the store used by the logs command.
func SetLogsStore(s storage.Store) {
	logsStore = s
}

func newLogsCommand() *cobra.Command {
	var (
		provider string
		model    string
		since    string
		until    string
		status   int
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "List captured request/response logs",
		Long:  `List captured request/response log entries with optional filtering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if logsStore == nil {
				return fmt.Errorf("no store configured")
			}

			filter := storage.ListFilter{
				Provider: provider,
				Model:    model,
				Limit:    limit,
			}

			if status != 0 {
				filter.StatusCode = &status
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

			ctx := cmd.Context()
			requests, err := logsStore.ListRequests(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing requests: %w", err)
			}

			w := cmd.OutOrStdout()

			if len(requests) == 0 {
				fmt.Fprintln(w, "No log entries found.")
				return nil
			}

			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tTIMESTAMP\tPROVIDER\tMODEL\tSTATUS\tIN TOKENS\tOUT TOKENS\tCOST\tLATENCY")

			for _, r := range requests {
				id := r.ID
				if len(id) > 8 {
					id = id[:8]
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t$%.4f\t%dms\n",
					id,
					r.Timestamp.Format(time.RFC3339),
					r.Provider,
					r.Model,
					r.ResponseStatus,
					r.InputTokens,
					r.OutputTokens,
					r.EstimatedCostUSD,
					r.LatencyMs,
				)
			}

			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Filter by provider name")
	cmd.Flags().StringVar(&model, "model", "", "Filter by model name")
	cmd.Flags().StringVar(&since, "since", "", "Filter requests after this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Filter requests before this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().IntVar(&status, "status", 0, "Filter by HTTP response status code")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results to return")

	return cmd
}
