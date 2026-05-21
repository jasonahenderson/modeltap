package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/runtime"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// resolveProviderHost applies the PATCH-0005 default-to-proxy
// behavior. When the v0.1 reverse proxy is configured to listen
// (proxyPort > 0) and the provider has no explicit host override,
// default the Runtime's provider endpoint at http://127.0.0.1:<port>
// so harness conversations flow through the proxy's capture
// pipeline. Ollama/MLX are skipped — those are already local.
//
// Explicit overrides win: a user who sets host verbatim (including
// https://api.anthropic.com) keeps the direct path. This lets
// operators route around the proxy for latency or isolation when
// they know what they're doing.
func resolveProviderHost(pc config.ProviderConfig, proxyPort int) string {
	if pc.Host != "" {
		return pc.Host
	}
	if proxyPort <= 0 {
		return ""
	}
	switch pc.Type {
	case runtime.ProviderTypeOllama, runtime.ProviderTypeMLX:
		return ""
	}
	return "http://127.0.0.1:" + strconv.Itoa(proxyPort)
}

// resolveProviderUpstream applies PATCH-0025: cloud-provider health
// checks probe a canonical upstream URL rather than the dispatch
// host (which may be the local capture proxy). pc.Upstream wins when
// set; otherwise the per-type canonical default
// (https://api.anthropic.com, https://api.openai.com, etc.) is used.
// For local providers (Ollama, MLX) the per-type default is the
// local service URL; the probe paths for those providers consult
// ep.Host directly so the value here is unused but harmless.
func resolveProviderUpstream(pc config.ProviderConfig) string {
	if pc.Upstream != "" {
		return pc.Upstream
	}
	switch pc.Type {
	case runtime.ProviderTypeAnthropic:
		return "https://api.anthropic.com"
	case runtime.ProviderTypeOpenAI:
		return "https://api.openai.com"
	}
	return ""
}

// startRuntimeServer constructs a Runtime server, populates its provider /
// adapter / model / routing surfaces from the loaded config, ensures
// the socket directory exists, and starts listening. Returns the
// running server so the caller can defer Shutdown.
//
// stderr is used for non-fatal warnings (e.g., "no Runtime endpoints
// configured") so the Runtime can still serve session.* / capability
// requests without provider config; turn.submit will fail clearly.
func startRuntimeServer(cfg *config.Config, store storage.Store, stderr io.Writer) (*runtime.Server, error) {
	runtimeCfg := runtime.DefaultServerConfig()
	runtimeCfg.SocketPath = cfg.Runtime.SocketPath
	if cfg.Runtime.SocketMode != 0 {
		runtimeCfg.SocketMode = os.FileMode(cfg.Runtime.SocketMode)
	}
	runtimeCfg.TLSAddress = cfg.Runtime.TLSAddress
	runtimeCfg.TLSCertFile = cfg.Runtime.TLSCertFile
	runtimeCfg.TLSKeyFile = cfg.Runtime.TLSKeyFile
	runtimeCfg.TLSClientCAFile = cfg.Runtime.TLSClientCAFile
	if cfg.Runtime.MaxConnections > 0 {
		runtimeCfg.MaxConnections = cfg.Runtime.MaxConnections
	}
	if cfg.Runtime.MaxAttachmentSize > 0 {
		runtimeCfg.MaxAttachmentSize = cfg.Runtime.MaxAttachmentSize
	}

	if runtimeCfg.SocketPath != "" {
		if err := os.MkdirAll(filepath.Dir(runtimeCfg.SocketPath), 0o755); err != nil {
			return nil, fmt.Errorf("creating runtime server socket dir: %w", err)
		}
	}

	srv := runtime.NewServer(store, runtimeCfg)

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
		host := resolveProviderHost(pc, cfg.Port)
		ep := &runtime.ProviderEndpoint{
			Name:     name,
			Type:     pc.Type,
			APIKey:   pc.APIKey,
			Host:     host,
			Upstream: resolveProviderUpstream(pc),
			Discover: pc.Discover,
		}
		if err := srv.Providers().Add(ep); err != nil {
			fmt.Fprintf(stderr, "runtime provider %q: %v\n", name, err)
			continue
		}
		endpointCount++
	}

	// Run an initial health check pass and start the background poll
	// loop. Without this, every endpoint stays at the zero-value status
	// (reported as "unavailable"), Ollama/MLX discovery never runs, and
	// the registry's built-in catalog reports every model as
	// unavailable on model.list. StartHealthChecks runs CheckAll
	// synchronously before returning, so the Refresh below sees current
	// status and discovered model lists.
	srv.Providers().StartHealthChecks(0)
	srv.Models().Refresh()

	// Manual model overrides.
	if len(cfg.Runtime.Models) > 0 {
		manual := make(map[string]runtime.ModelOverrideConfig, len(cfg.Runtime.Models))
		for name, mc := range cfg.Runtime.Models {
			manual[name] = runtime.ModelOverrideConfig{
				Provider:      mc.Provider,
				ContextWindow: mc.ContextWindow,
				Capabilities:  mc.Capabilities,
				Description:   mc.Description,
			}
		}
		srv.Models().SetManual(manual)
	}

	// Routing tree.
	if len(cfg.Runtime.Routing) > 0 {
		tree := make(protocol.RoutingPolicy, len(cfg.Runtime.Routing))
		for path, value := range cfg.Runtime.Routing {
			raw, err := json.Marshal(value)
			if err != nil {
				fmt.Fprintf(stderr, "runtime routing %q: %v\n", path, err)
				continue
			}
			tree[path] = raw
		}
		srv.Routing().Replace(tree)
	}

	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("starting runtime server server: %w", err)
	}

	if endpointCount == 0 {
		fmt.Fprintln(stderr, "runtime server: no provider endpoints configured (turn.submit will fail until you add `providers:` entries with `type:` set)")
	}
	return srv, nil
}
