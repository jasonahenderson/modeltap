package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

// newRequestsListCommand registers `modeltap requests list`. Uses the
// package-shared requestsStore (lazy-opened in production, injected in
// tests). Per PATCH-0020, this replaces the previous `modeltap logs`
// top-level command.
func newRequestsListCommand() *cobra.Command {
	var (
		provider string
		model    string
		since    string
		until    string
		status   int
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List captured upstream API request/response exchanges",
		Long: `List captured request/response entries with optional filtering.

Displays a table of captured API requests showing the request ID, timestamp,
provider, model, HTTP status, token counts, estimated cost, and latency.
Results are ordered by timestamp (newest first) and limited to 50 by default.

Time filters (--since, --until) accept either a duration shorthand relative
to now (e.g. "24h", "7d", "30m") or an RFC3339 timestamp.`,
		Example: `  # Show the 50 most recent captures
  modeltap requests list

  # Filter by provider
  modeltap requests list --provider anthropic

  # Filter by model and limit results
  modeltap requests list --model gpt-4 --limit 10

  # Show only failed captures from the last hour
  modeltap requests list --status 500 --since 1h

  # Show captures within a specific time window
  modeltap requests list --since 2026-03-01T00:00:00Z --until 2026-03-08T00:00:00Z`,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			requests, err := requestsStore.ListRequests(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing requests: %w", err)
			}

			w := cmd.OutOrStdout()

			if len(requests) == 0 {
				fmt.Fprintln(w, "No captures found.")
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
	cmd.Flags().StringVar(&since, "since", "", "Filter captures after this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Filter captures before this time (duration like 24h/7d or RFC3339)")
	cmd.Flags().IntVar(&status, "status", 0, "Filter by HTTP response status code")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results to return")

	return cmd
}
