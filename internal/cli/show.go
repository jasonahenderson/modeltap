package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

// showStore is a package-level variable that allows injecting a Store for
// the show command. In production this is set via SetShowStore before
// command execution; in tests it is set directly.
var showStore storage.Store

// SetShowStore sets the store used by the show command.
func SetShowStore(s storage.Store) {
	showStore = s
}

func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show detail of a captured request",
		Long: `Show the full detail of a captured request/response pair by its ID.

Prints a comprehensive view of a single captured request including headers,
body content (pretty-printed JSON), token usage, cost estimate, and latency.
Use "modeltap logs" to find request IDs, then pass one to this command for
the complete picture.

The request ID can be the full UUID or the short 8-character prefix shown
in the logs table output.`,
		Example: `  # Show full detail for a request by its short ID
  modeltap show abc12345

  # Show detail using a full UUID
  modeltap show 550e8400-e29b-41d4-a716-446655440000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if showStore == nil {
				s, err := openStoreFromConfig()
				if err != nil {
					return err
				}
				defer s.Close()
				showStore = s
				defer func() { showStore = nil }()
			}

			id := args[0]
			ctx := context.Background()
			req, err := showStore.GetRequest(ctx, id)
			if err != nil {
				return fmt.Errorf("fetching request: %w", err)
			}
			if req == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Request %s not found\n", id)
				return fmt.Errorf("request %s not found", id)
			}

			w := cmd.OutOrStdout()

			// Header section
			fmt.Fprintln(w, "=== Request Detail ===")
			fmt.Fprintf(w, "ID:        %s\n", req.ID)
			fmt.Fprintf(w, "Timestamp: %s\n", req.Timestamp.Format(time.RFC3339))
			fmt.Fprintf(w, "Provider:  %s\n", req.Provider)
			fmt.Fprintf(w, "Model:     %s\n", req.Model)
			fmt.Fprintf(w, "Status:    %d\n", req.ResponseStatus)
			fmt.Fprintf(w, "Latency:   %dms\n", req.LatencyMs)
			fmt.Fprintf(w, "Tokens:    %d input / %d output\n", req.InputTokens, req.OutputTokens)
			fmt.Fprintf(w, "Cost:      $%.4f\n", req.EstimatedCostUSD)

			// Request section
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "--- Request ---")
			fmt.Fprintf(w, "Method: %s\n", req.Method)
			fmt.Fprintf(w, "URL:    %s\n", req.URL)
			if req.RequestHeaders != "" {
				fmt.Fprintln(w, "Headers:")
				fmt.Fprintln(w, formatHeaders(req.RequestHeaders))
			}
			if req.RequestBody != "" {
				fmt.Fprintln(w, "Body:")
				fmt.Fprintln(w, prettyJSON(req.RequestBody))
			}

			// Response section
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "--- Response ---")
			fmt.Fprintf(w, "Status: %d\n", req.ResponseStatus)
			if req.ResponseHeaders != "" {
				fmt.Fprintln(w, "Headers:")
				fmt.Fprintln(w, formatHeaders(req.ResponseHeaders))
			}
			if req.ResponseBody != "" {
				fmt.Fprintln(w, "Body:")
				fmt.Fprintln(w, prettyJSON(req.ResponseBody))
			}

			return nil
		},
	}

	return cmd
}

// prettyJSON attempts to pretty-print a JSON string. If the input is not
// valid JSON, it is returned as-is.
func prettyJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "  ", "  "); err != nil {
		return s
	}
	return "  " + buf.String()
}

// formatHeaders parses a JSON object of headers and formats them as
// indented "Key: Value" lines. If the input is not valid JSON, it is
// returned as-is.
func formatHeaders(s string) string {
	var headers map[string]string
	if err := json.Unmarshal([]byte(s), &headers); err != nil {
		return "  " + s
	}
	var lines []string
	for k, v := range headers {
		lines = append(lines, fmt.Sprintf("  %s: %s", k, v))
	}
	// Sort for deterministic output is not strictly required but let's
	// keep it simple - the map iteration order is non-deterministic.
	return strings.Join(lines, "\n")
}
