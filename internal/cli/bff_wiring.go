package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jasonahenderson/modeltap/internal/bff"
	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// startBFFServer constructs a BFF server, populates its provider /
// adapter / model / routing surfaces from the loaded config, ensures
// the socket directory exists, and starts listening. Returns the
// running server so the caller can defer Shutdown.
//
// stderr is used for non-fatal warnings (e.g., "no BFF endpoints
// configured") so the BFF can still serve session.* / capability
// requests without provider config; turn.submit will fail clearly.
func startBFFServer(cfg *config.Config, store storage.Store, stderr io.Writer) (*bff.Server, error) {
	bffCfg := bff.DefaultServerConfig()
	bffCfg.SocketPath = cfg.BFF.SocketPath
	if cfg.BFF.SocketMode != 0 {
		bffCfg.SocketMode = os.FileMode(cfg.BFF.SocketMode)
	}
	bffCfg.TLSAddress = cfg.BFF.TLSAddress
	bffCfg.TLSCertFile = cfg.BFF.TLSCertFile
	bffCfg.TLSKeyFile = cfg.BFF.TLSKeyFile
	if cfg.BFF.MaxConnections > 0 {
		bffCfg.MaxConnections = cfg.BFF.MaxConnections
	}
	if cfg.BFF.MaxAttachmentSize > 0 {
		bffCfg.MaxAttachmentSize = cfg.BFF.MaxAttachmentSize
	}

	if bffCfg.SocketPath != "" {
		if err := os.MkdirAll(filepath.Dir(bffCfg.SocketPath), 0o755); err != nil {
			return nil, fmt.Errorf("creating BFF socket dir: %w", err)
		}
	}

	srv := bff.NewServer(store, bffCfg)

	// Provider adapters that ship in-tree.
	srv.Adapters().Register(provider.NewAnthropicProvider())
	srv.Adapters().Register(provider.NewOpenAIProvider())
	srv.Adapters().Register(provider.NewOllamaProvider())

	// Provider endpoints (those entries with Type set).
	endpointCount := 0
	for name, pc := range cfg.Providers {
		if pc.Type == "" {
			continue
		}
		ep := &bff.ProviderEndpoint{
			Name:     name,
			Type:     pc.Type,
			APIKey:   pc.APIKey,
			Host:     pc.Host,
			Discover: pc.Discover,
		}
		if err := srv.Providers().Add(ep); err != nil {
			fmt.Fprintf(stderr, "BFF provider %q: %v\n", name, err)
			continue
		}
		endpointCount++
	}
	srv.Models().Refresh()

	// Manual model overrides.
	if len(cfg.BFF.Models) > 0 {
		manual := make(map[string]bff.ModelOverrideConfig, len(cfg.BFF.Models))
		for name, mc := range cfg.BFF.Models {
			manual[name] = bff.ModelOverrideConfig{
				Provider:      mc.Provider,
				ContextWindow: mc.ContextWindow,
				Capabilities:  mc.Capabilities,
				Description:   mc.Description,
			}
		}
		srv.Models().SetManual(manual)
	}

	// Routing tree.
	if len(cfg.BFF.Routing) > 0 {
		tree := make(protocol.RoutingPolicy, len(cfg.BFF.Routing))
		for path, value := range cfg.BFF.Routing {
			raw, err := json.Marshal(value)
			if err != nil {
				fmt.Fprintf(stderr, "BFF routing %q: %v\n", path, err)
				continue
			}
			tree[path] = raw
		}
		srv.Routing().Replace(tree)
	}

	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("starting BFF server: %w", err)
	}

	if endpointCount == 0 {
		fmt.Fprintln(stderr, "BFF: no provider endpoints configured (turn.submit will fail until you add `providers:` entries with `type:` set)")
	}
	return srv, nil
}
