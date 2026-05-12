package cli

import (
	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

// requestsStore is the package-level storage.Store shared by the three
// `requests` subcommands (list, show, export). In production, the
// store is lazy-opened from config in each subcommand's RunE when this
// variable is nil; in tests it is set directly via SetRequestsStore.
//
// Per PATCH-0020: replaces the three per-command vars (logsStore,
// showStore, exportStore). All three subcommands operate on the same
// SQLite request capture table; one shared store is the right shape.
var requestsStore storage.Store

// SetRequestsStore sets the store used by the `requests` subcommands.
// Test injection seam.
func SetRequestsStore(s storage.Store) {
	requestsStore = s
}

// newRequestsCommand registers the `requests` parent command and its
// list / show / export subcommands. Per PATCH-0020 this replaces the
// three previous top-level commands (`logs`, `show`, `export`) with a
// noun-verb structure that names the underlying domain primitive
// (captured request/response exchanges per ADR-0005).
func newRequestsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "requests",
		Short: "Inspect captured upstream API request/response exchanges",
		Long: `Inspect captured upstream API request/response exchanges.

modeltap captures every proxied request and its upstream response into
a local SQLite table (ADR-0005, "always full capture, retention-based
pruning"). Use these subcommands to list captures, view detail for a
single capture, or export the captured set in JSONL or CSV.`,
		Example: `  # List the most recent captures
  modeltap requests list --limit 20

  # Show full detail for one capture
  modeltap requests show <id>

  # Export the last 24 hours as CSV
  modeltap requests export --format csv --since 24h`,
	}

	cmd.AddCommand(newRequestsListCommand())
	cmd.AddCommand(newRequestsShowCommand())
	cmd.AddCommand(newRequestsExportCommand())
	return cmd
}
