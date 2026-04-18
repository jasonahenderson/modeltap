package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// statusMockProvider implements provider.Provider for testing.
type statusMockProvider struct {
	name string
}

func (m *statusMockProvider) Name() string                { return m.name }
func (m *statusMockProvider) Detect(_ *http.Request) bool { return false }
func (m *statusMockProvider) ParseRequest(_ []byte, _ http.Header) (*provider.RequestMetadata, error) {
	return &provider.RequestMetadata{}, nil
}
func (m *statusMockProvider) ParseResponse(_ []byte, _ http.Header, _ int) (*provider.ResponseMetadata, error) {
	return &provider.ResponseMetadata{}, nil
}
func (m *statusMockProvider) ReassembleStream(_ []provider.StreamChunk) (*provider.ResponseMetadata, string, error) {
	return &provider.ResponseMetadata{}, "", nil
}
func (m *statusMockProvider) FormatMessages(_ provider.FormatMessagesOpts) ([]byte, error) {
	return nil, nil
}
func (m *statusMockProvider) FormatToolDefinitions(_ []protocol.ToolDefinition) ([]byte, error) {
	return nil, nil
}

// seedStatusTestStore creates an in-memory SQLite store populated with test data.
func seedStatusTestStore(t *testing.T, count int) storage.Store {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Now().UTC()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		req := &storage.Request{
			ID:             fmt.Sprintf("req-%04d", i),
			Timestamp:      now.Add(-time.Duration(i) * time.Hour),
			Provider:       "anthropic",
			Model:          "claude-3",
			Method:         "POST",
			URL:            "https://api.anthropic.com/v1/messages",
			ResponseStatus: 200,
			InputTokens:    100,
			OutputTokens:   50,
			LatencyMs:      250,
		}
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("seeding request %d: %v", i, err)
		}
	}
	return store
}

func testStatusConfig() *config.Config {
	return &config.Config{
		Port:          8080,
		Upstream:      "https://api.anthropic.com",
		DBPath:        "~/.config/modeltap/modeltap.db",
		RetentionDays: 30,
	}
}

func executeStatus(t *testing.T, store storage.Store, cfg *config.Config, reg *provider.Registry) (string, error) {
	t.Helper()

	prevStore := statusStore
	prevCfg := statusConfig
	prevReg := statusRegistry
	statusStore = store
	statusConfig = cfg
	statusRegistry = reg
	t.Cleanup(func() {
		statusStore = prevStore
		statusConfig = prevCfg
		statusRegistry = prevReg
	})

	rootCmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"status"})

	err := rootCmd.Execute()
	return buf.String(), err
}

func TestStatusDisplaysProxyConfig(t *testing.T) {
	cfg := testStatusConfig()
	cfg.Port = 9090
	cfg.Upstream = "https://api.openai.com"

	output, err := executeStatus(t, nil, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Proxy") {
		t.Error("expected 'Proxy' section header in output")
	}
	if !strings.Contains(output, "9090") {
		t.Error("expected port 9090 in output")
	}
	if !strings.Contains(output, "https://api.openai.com") {
		t.Error("expected upstream URL in output")
	}
}

func TestStatusDisplaysDatabaseInfo(t *testing.T) {
	store := seedStatusTestStore(t, 5)
	cfg := testStatusConfig()
	cfg.DBPath = "/tmp/test.db"

	output, err := executeStatus(t, store, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Database") {
		t.Error("expected 'Database' section header in output")
	}
	if !strings.Contains(output, "/tmp/test.db") {
		t.Error("expected database path in output")
	}
	if !strings.Contains(output, "5") {
		t.Error("expected record count 5 in output")
	}
}

func TestStatusDisplaysRetentionSettings(t *testing.T) {
	cfg := testStatusConfig()
	cfg.RetentionDays = 90

	output, err := executeStatus(t, nil, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Retention") {
		t.Error("expected 'Retention' section header in output")
	}
	if !strings.Contains(output, "90") {
		t.Error("expected retention days 90 in output")
	}
}

func TestStatusDisplaysProviders(t *testing.T) {
	cfg := testStatusConfig()

	output, err := executeStatus(t, nil, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Providers") {
		t.Error("expected 'Providers' section header in output")
	}
	if !strings.Contains(output, "anthropic") {
		t.Error("expected 'anthropic' provider in output")
	}
	if !strings.Contains(output, "openai") {
		t.Error("expected 'openai' provider in output")
	}
}

func TestStatusWithRegistry(t *testing.T) {
	cfg := &config.Config{
		Port:          8080,
		Upstream:      "https://example.com",
		DBPath:        "/tmp/test.db",
		RetentionDays: 30,
	}
	reg := provider.NewRegistry()
	reg.Register(&statusMockProvider{name: "custom-provider"})

	output, err := executeStatus(t, nil, cfg, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "custom-provider") {
		t.Error("expected 'custom-provider' in output")
	}
	// Providers section should only list registry providers, not defaults.
	// Check that "- anthropic" (the provider list line) is NOT present.
	if strings.Contains(output, "- anthropic") {
		t.Error("should not show default providers when registry is set")
	}
}

func TestStatusRecordCountFormatted(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := formatCount(tt.n)
		if got != tt.want {
			t.Errorf("formatCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestStatusNoStoreShowsNA(t *testing.T) {
	cfg := testStatusConfig()

	output, err := executeStatus(t, nil, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "N/A") {
		t.Errorf("expected 'N/A' for records when no store, got: %s", output)
	}
}

func TestStatusDashboardFormat(t *testing.T) {
	store := seedStatusTestStore(t, 3)
	cfg := testStatusConfig()

	output, err := executeStatus(t, store, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all sections are present and in the right order.
	sections := []string{"Proxy", "Database", "Retention", "Providers"}
	lastIdx := -1
	for _, section := range sections {
		idx := strings.Index(output, section)
		if idx == -1 {
			t.Errorf("missing section %q in output", section)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("section %q should appear after previous section", section)
		}
		lastIdx = idx
	}
}
