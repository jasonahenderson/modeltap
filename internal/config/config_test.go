package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Load with a non-existent config file to get pure defaults.
	cfg, err := Load("/tmp/modeltap-test-nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"port", cfg.Port, 8080},
		{"upstream", cfg.Upstream, "https://api.anthropic.com"},
		{"retention_days", cfg.RetentionDays, 30},
		{"max_body_size", cfg.MaxBodySize, "10MB"},
		{"dashboard.enabled", cfg.Dashboard.Enabled, false},
		{"dashboard.port", cfg.Dashboard.Port, 8081},
		{"dashboard.bind", cfg.Dashboard.Bind, "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}

	// db_path has ~ expanded, so just check it ends correctly.
	if !filepath.IsAbs(cfg.DBPath) {
		t.Errorf("DBPath should be absolute after expansion, got %s", cfg.DBPath)
	}
	if filepath.Base(cfg.DBPath) != "modeltap.db" {
		t.Errorf("DBPath should end with modeltap.db, got %s", cfg.DBPath)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp config file.
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	content := []byte(`port: 9090
upstream: "https://custom.api.example.com"
retention_days: 60
max_body_size: "50MB"
db_path: "/tmp/test.db"
dashboard:
  enabled: true
  port: 3000
  bind: "0.0.0.0"
`)
	if err := os.WriteFile(configFile, content, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"port", cfg.Port, 9090},
		{"upstream", cfg.Upstream, "https://custom.api.example.com"},
		{"retention_days", cfg.RetentionDays, 60},
		{"max_body_size", cfg.MaxBodySize, "50MB"},
		{"db_path", cfg.DBPath, "/tmp/test.db"},
		{"dashboard.enabled", cfg.Dashboard.Enabled, true},
		{"dashboard.port", cfg.Dashboard.Port, 3000},
		{"dashboard.bind", cfg.Dashboard.Bind, "0.0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("MODELTAP_PORT", "9999")
	t.Setenv("MODELTAP_UPSTREAM", "https://env.example.com")

	cfg, err := Load("/tmp/modeltap-test-nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"port from env", cfg.Port, 9999},
		{"upstream from env", cfg.Upstream, "https://env.example.com"},
		// Defaults should still apply for unset values.
		{"retention_days default", cfg.RetentionDays, 30},
		{"dashboard.port default", cfg.Dashboard.Port, 8081},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestEnvOverridesFile(t *testing.T) {
	// Create a config file with port=9090.
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	content := []byte(`port: 9090
upstream: "https://file.example.com"
`)
	if err := os.WriteFile(configFile, content, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// Set env var to override port.
	t.Setenv("MODELTAP_PORT", "7777")

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"port from env overrides file", cfg.Port, 7777},
		{"upstream from file (no env override)", cfg.Upstream, "https://file.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestDashboardEnvOverride(t *testing.T) {
	t.Setenv("MODELTAP_DASHBOARD_ENABLED", "true")
	t.Setenv("MODELTAP_DASHBOARD_PORT", "5000")

	cfg, err := Load("/tmp/modeltap-test-nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if !cfg.Dashboard.Enabled {
		t.Error("dashboard.enabled should be true from env")
	}
	if cfg.Dashboard.Port != 5000 {
		t.Errorf("dashboard.port got %d, want 5000", cfg.Dashboard.Port)
	}
}

func TestConfigYAML(t *testing.T) {
	cfg, err := Load("/tmp/modeltap-test-nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	yamlStr, err := cfg.YAML()
	if err != nil {
		t.Fatalf("YAML() returned error: %v", err)
	}

	if yamlStr == "" {
		t.Error("YAML() returned empty string")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		name string
		input string
		want  string
	}{
		{"tilde path", "~/foo/bar", filepath.Join(home, "foo", "bar")},
		{"absolute path", "/usr/local/bin", "/usr/local/bin"},
		{"relative path", "relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandHome(tt.input)
			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultConfigPath() should return absolute path, got %s", path)
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultConfigPath() should end with config.yaml, got %s", path)
	}
}

// TestResolveSecret_PassThrough confirms a value without any prefix
// is returned verbatim — preserves the "paste the key directly"
// ergonomic so existing configs keep working.
func TestResolveSecret_PassThrough(t *testing.T) {
	cases := []string{
		"sk-ant-abcdef",
		"",
		"no-prefix-here",
		"envbar:not-env-prefix", // prefix is "envbar:", not "env:"
	}
	for _, c := range cases {
		got, err := ResolveSecret(c)
		if err != nil {
			t.Errorf("ResolveSecret(%q) err = %v, want nil", c, err)
		}
		if got != c {
			t.Errorf("ResolveSecret(%q) = %q, want %q", c, got, c)
		}
	}
}

func TestResolveSecret_EnvPrefix(t *testing.T) {
	t.Setenv("MODELTAP_TEST_KEY", "resolved-value")
	got, err := ResolveSecret("env:MODELTAP_TEST_KEY")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "resolved-value" {
		t.Errorf("got %q, want %q", got, "resolved-value")
	}
}

func TestResolveSecret_EnvPrefix_MissingVar(t *testing.T) {
	os.Unsetenv("MODELTAP_DEFINITELY_NOT_SET")
	_, err := ResolveSecret("env:MODELTAP_DEFINITELY_NOT_SET")
	if err == nil {
		t.Fatal("expected error for unset env var")
	}
	if !contains(err.Error(), "MODELTAP_DEFINITELY_NOT_SET") {
		t.Errorf("error should name the var: %v", err)
	}
}

func TestResolveSecret_EnvPrefix_Empty(t *testing.T) {
	_, err := ResolveSecret("env:")
	if err == nil {
		t.Error("expected error for empty env var name")
	}
}

func TestResolveSecret_FilePrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, []byte("sk-ant-from-file\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ResolveSecret("file:" + path)
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "sk-ant-from-file" {
		t.Errorf("got %q, want trimmed %q", got, "sk-ant-from-file")
	}
}

func TestResolveSecret_FilePrefix_Missing(t *testing.T) {
	_, err := ResolveSecret("file:/tmp/modeltap-test-missing-" + t.Name())
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// contains is a tiny helper — strings.Contains via a thin wrapper so
// the test file doesn't sprout the import just for one check.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestResolveProviderSecrets_EndToEnd confirms the loader applies
// ResolveSecret to every provider's APIKey at load time.
func TestResolveProviderSecrets_EndToEnd(t *testing.T) {
	t.Setenv("MODELTAP_E2E_KEY", "env-resolved")

	dir := t.TempDir()
	fileKey := filepath.Join(dir, "other-key")
	if err := os.WriteFile(fileKey, []byte("file-resolved"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	configPath := filepath.Join(dir, "config.yaml")
	yaml := `providers:
  anthropic:
    type: anthropic
    api_key: env:MODELTAP_E2E_KEY
  openai:
    type: openai
    api_key: file:` + fileKey + `
  legacy:
    type: anthropic
    api_key: sk-plain-legacy
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Providers["anthropic"].APIKey != "env-resolved" {
		t.Errorf("anthropic key = %q, want env-resolved", cfg.Providers["anthropic"].APIKey)
	}
	if cfg.Providers["openai"].APIKey != "file-resolved" {
		t.Errorf("openai key = %q, want file-resolved", cfg.Providers["openai"].APIKey)
	}
	if cfg.Providers["legacy"].APIKey != "sk-plain-legacy" {
		t.Errorf("legacy key should pass through; got %q", cfg.Providers["legacy"].APIKey)
	}
}
