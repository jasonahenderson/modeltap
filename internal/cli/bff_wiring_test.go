package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// shortSocketPath returns a /tmp-rooted socket path that fits inside
// the 104-byte unix-socket name limit on macOS. Uses an atomic
// counter for uniqueness rather than an OS-allocated TempDir whose
// path is too long.
var sockSeq atomic.Uint32

func shortSocketPath(t *testing.T) string {
	t.Helper()
	n := sockSeq.Add(1)
	p := filepath.Join(os.TempDir(), fmt.Sprintf("mt-%d-%d.sock", os.Getpid(), n))
	t.Cleanup(func() { _ = os.Remove(p) })
	return p
}

func newCfgWithBFF(t *testing.T, sock string) *config.Config {
	t.Helper()
	return &config.Config{
		BFF: config.BFFConfig{
			SocketPath:        sock,
			SocketMode:        0o600,
			MaxConnections:    100,
			MaxAttachmentSize: 5 * 1024 * 1024,
		},
	}
}

func newCfgStore(t *testing.T) storage.Store {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStartBFFServer_NoEndpoints_WarnsButStarts(t *testing.T) {
	sock := shortSocketPath(t)
	cfg := newCfgWithBFF(t, sock)

	var stderr bytes.Buffer
	srv, err := startBFFServer(cfg, newCfgStore(t), &stderr)
	if err != nil {
		t.Fatalf("startBFFServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	if !strings.Contains(stderr.String(), "no provider endpoints configured") {
		t.Errorf("expected warning, got %q", stderr.String())
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
}

func TestStartBFFServer_EndpointsRegistered(t *testing.T) {
	sock := shortSocketPath(t)
	cfg := newCfgWithBFF(t, sock)
	cfg.Providers = map[string]config.ProviderConfig{
		// proxy-only entry — should be skipped by the BFF.
		"anthropic": {Upstream: "https://api.anthropic.com"},
		// BFF endpoint entries.
		"anthropic-prod": {Type: "anthropic", APIKey: "k"},
		"ollama-local":   {Type: "ollama", Host: "http://localhost:11434", Discover: true},
	}

	var stderr bytes.Buffer
	srv, err := startBFFServer(cfg, newCfgStore(t), &stderr)
	if err != nil {
		t.Fatalf("startBFFServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	names := srv.Providers().Names()
	if len(names) != 2 {
		t.Errorf("got %d endpoints, want 2: %v", len(names), names)
	}
}

func TestStartBFFServer_InvalidEndpointTypeReported(t *testing.T) {
	sock := shortSocketPath(t)
	cfg := newCfgWithBFF(t, sock)
	cfg.Providers = map[string]config.ProviderConfig{
		"weird": {Type: "not-a-real-type", APIKey: "k"},
	}

	var stderr bytes.Buffer
	srv, err := startBFFServer(cfg, newCfgStore(t), &stderr)
	if err != nil {
		t.Fatalf("startBFFServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	if !strings.Contains(stderr.String(), "BFF provider \"weird\"") {
		t.Errorf("expected provider error in stderr; got %q", stderr.String())
	}
}

func TestStartBFFServer_RoutingTreeAndManualModels(t *testing.T) {
	sock := shortSocketPath(t)
	cfg := newCfgWithBFF(t, sock)
	cfg.Providers = map[string]config.ProviderConfig{
		"ollama-local": {Type: "ollama", Host: "http://localhost:11434"},
	}
	cfg.BFF.Models = map[string]config.ModelOverrideConfig{
		"llama-3.1-8b": {
			Provider:      "ollama-local",
			ContextWindow: 8192,
			Capabilities:  []string{"tool_use"},
			Description:   "fast local",
		},
	}
	cfg.BFF.Routing = map[string]any{
		"default":       "llama-3.1-8b",
		"coding.review": []any{"claude-opus-4-6", "gpt-5"},
	}

	var stderr bytes.Buffer
	srv, err := startBFFServer(cfg, newCfgStore(t), &stderr)
	if err != nil {
		t.Fatalf("startBFFServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	if e := srv.Models().Get("llama-3.1-8b"); e == nil {
		t.Errorf("manual model override missing")
	}

	tree := srv.Routing().Tree()
	if _, ok := tree["default"]; !ok {
		t.Errorf("routing default missing")
	}
	// The "coding.review" path should resolve to a multi-model array.
	models, isMulti, _ := srv.Routing().Resolve("coding.review")
	if !isMulti || len(models) != 2 {
		t.Errorf("multi-model routing not parsed correctly: %v isMulti=%v", models, isMulti)
	}

	// Round-trip the JSON for the multi-model entry to confirm it
	// survived the YAML→any→json.RawMessage path.
	raw := tree["coding.review"]
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Errorf("multi-model raw not a JSON array: %v", err)
	}
}
