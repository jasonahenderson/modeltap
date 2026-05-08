package cli

import (
	"fmt"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// openStoreFromConfig loads the user's modeltap configuration and opens
// a SQLite store at the configured DBPath. The caller is responsible
// for calling Close on the returned store.
//
// Per PATCH-0019: the four read commands (logs, show, export, metrics)
// each declare a package-level `xxxStore storage.Store` for test
// injection, but no production code path was wiring them. This helper
// is the production wiring; commands call it lazily from their RunE
// when the package variable is nil so the test-injection seam remains
// intact.
func openStoreFromConfig() (storage.Store, error) {
	cfg, _, err := config.LoadWithViper("")
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	store, err := storage.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("opening store at %s: %w", cfg.DBPath, err)
	}
	return store, nil
}
