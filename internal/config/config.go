// Package config provides configuration loading for modeltap.
//
// Configuration is loaded with three-tier precedence (highest to lowest):
//
//	flags > environment variables > config file > defaults
//
// Environment variables use the MODELTAP_ prefix (e.g., MODELTAP_PORT).
// The config file path defaults to ~/.modeltap/config.yaml (PATCH-0006),
// with ~/.config/modeltap/config.yaml honored as a legacy fallback for
// one release cycle.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// DashboardConfig holds dashboard-specific settings.
type DashboardConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Port    int    `yaml:"port" mapstructure:"port"`
	Bind    string `yaml:"bind" mapstructure:"bind"`
}

// ProviderConfig holds per-provider settings. Fields fall into two
// groups:
//
//   - Upstream is used by the v0.1 reverse proxy (`modeltap start`)
//     when this entry's key matches a well-known provider name
//     ("anthropic", "openai", ...).
//   - Type / APIKey / Host / Discover are used by the v0.2 BFF
//     server's ProviderRegistry. The map key is the user-assigned
//     endpoint name (e.g., "anthropic-prod").
//
// Both groups can coexist in one `providers:` map; the proxy uses
// entries with Upstream set, the BFF uses entries with Type set.
type ProviderConfig struct {
	Upstream string `yaml:"upstream,omitempty" mapstructure:"upstream"`

	// BFF endpoint fields (WU-057).
	Type     string `yaml:"type,omitempty" mapstructure:"type"`
	APIKey   string `yaml:"api_key,omitempty" mapstructure:"api_key"`
	Host     string `yaml:"host,omitempty" mapstructure:"host"`
	Discover bool   `yaml:"discover,omitempty" mapstructure:"discover"`
}

// ModelOverrideConfig is one entry in BFFConfig.Models, applied as a
// manual catalog override on the BFF's ModelRegistry per WU-058.
type ModelOverrideConfig struct {
	Provider      string   `yaml:"provider" mapstructure:"provider"`
	ContextWindow int      `yaml:"context_window" mapstructure:"context_window"`
	Capabilities  []string `yaml:"capabilities,omitempty" mapstructure:"capabilities"`
	Description   string   `yaml:"description,omitempty" mapstructure:"description"`
}

// BFFConfig holds settings for the v0.2 BFF JSON-RPC server.
//
// Listener fields default to a Unix socket only; TLS is opt-in by
// setting TLSAddress + cert/key paths.
type BFFConfig struct {
	SocketPath        string `yaml:"socket_path,omitempty" mapstructure:"socket_path"`
	SocketMode        uint32 `yaml:"socket_mode,omitempty" mapstructure:"socket_mode"`
	TLSAddress        string `yaml:"tls_address,omitempty" mapstructure:"tls_address"`
	TLSCertFile       string `yaml:"tls_cert_file,omitempty" mapstructure:"tls_cert_file"`
	TLSKeyFile        string `yaml:"tls_key_file,omitempty" mapstructure:"tls_key_file"`
	TLSClientCAFile   string `yaml:"tls_client_ca_file,omitempty" mapstructure:"tls_client_ca_file"`
	MaxConnections    int    `yaml:"max_connections,omitempty" mapstructure:"max_connections"`
	MaxAttachmentSize int    `yaml:"max_attachment_size,omitempty" mapstructure:"max_attachment_size"`

	// Models adds manual catalog overrides on top of the built-in
	// catalog and discovered Ollama/MLX models. Map key is the model
	// name (e.g., "llama-3.1-8b").
	Models map[string]ModelOverrideConfig `yaml:"models,omitempty" mapstructure:"models"`

	// Routing is the hierarchical routing tree per WU-059. Values may
	// be a single model name string or an array of names (multi-model).
	// Stored as map[string]any so YAML loading handles both shapes; the
	// CLI converts each value to json.RawMessage when handing the tree
	// to the BFF's RoutingPolicy.
	Routing map[string]any `yaml:"routing,omitempty" mapstructure:"routing"`
}

// HarnessConfig holds harness-specific settings per FEAT-0009.
type HarnessConfig struct {
	SubmitKey      string `yaml:"submit_key,omitempty" mapstructure:"submit_key"`
	Permissions    string `yaml:"permissions,omitempty" mapstructure:"permissions"`
	PasteThreshold int    `yaml:"paste_threshold,omitempty" mapstructure:"paste_threshold"`
	Theme          string `yaml:"theme,omitempty" mapstructure:"theme"`
	ShowCost       bool   `yaml:"show_cost,omitempty" mapstructure:"show_cost"`
	ShowContext    bool   `yaml:"show_context,omitempty" mapstructure:"show_context"`
}

// Config holds all modeltap configuration values.
type Config struct {
	Port          int                       `yaml:"port" mapstructure:"port"`
	Upstream      string                    `yaml:"upstream" mapstructure:"upstream"`
	DBPath        string                    `yaml:"db_path" mapstructure:"db_path"`
	RetentionDays int                       `yaml:"retention_days" mapstructure:"retention_days"`
	MaxBodySize   string                    `yaml:"max_body_size" mapstructure:"max_body_size"`
	Dashboard     DashboardConfig           `yaml:"dashboard" mapstructure:"dashboard"`
	Providers     map[string]ProviderConfig `yaml:"providers" mapstructure:"providers"`
	Pricing       PricingConfig             `yaml:"pricing" mapstructure:"pricing"`
	BFF           BFFConfig                 `yaml:"bff" mapstructure:"bff"`
	Harness       HarnessConfig             `yaml:"harness" mapstructure:"harness"`
}

// homeDir is a small wrapper so every path helper degrades identically
// when os.UserHomeDir fails (returning "~" produces a visibly-wrong
// path that is still obvious in error messages).
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~"
	}
	return home
}

// DefaultConfigDir returns the directory in which `config.yaml` lives
// for a fresh install. Honors $XDG_CONFIG_HOME when explicitly set
// (Linux power-user opt-in); otherwise defaults to ~/.modeltap
// (PATCH-0006).
func DefaultConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "modeltap")
	}
	return filepath.Join(homeDir(), ".modeltap")
}

// DefaultConfigPath returns the canonical config file path a fresh
// install writes to.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// LegacyConfigPath returns the pre-PATCH-0006 config file path
// (~/.config/modeltap/config.yaml). Used as a fallback when the new
// default does not exist — see ResolveConfigPath.
func LegacyConfigPath() string {
	return filepath.Join(homeDir(), ".config", "modeltap", "config.yaml")
}

// DefaultDataDir returns the directory in which the SQLite DB, BFF
// socket, and service log live for a fresh install. Honors
// $XDG_DATA_HOME when explicitly set; otherwise ~/.modeltap
// (PATCH-0006).
func DefaultDataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "modeltap")
	}
	return filepath.Join(homeDir(), ".modeltap")
}

// legacyDBPath / legacySocketPath return the v0.1-era defaults. Kept as
// package-private helpers because they are only relevant when the config
// resolver detects a legacy install.
func legacyDBPath() string {
	return filepath.Join(homeDir(), ".config", "modeltap", "modeltap.db")
}
func legacySocketPath() string {
	// $XDG_DATA_HOME was honored pre-PATCH-0006; preserve that so a
	// Linux user with XDG_DATA_HOME set and a legacy config keeps the
	// socket at the same place it was before the patch.
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "modeltap", "server.sock")
	}
	return filepath.Join(homeDir(), ".local", "share", "modeltap", "server.sock")
}

// DefaultBFFSocketPath returns the canonical Unix-domain socket path
// for the BFF JSON-RPC listener.
func DefaultBFFSocketPath() string {
	return filepath.Join(DefaultDataDir(), "server.sock")
}

// ConfigPathResolution records where a config file was located and
// whether the resolver fell back to a legacy path. Legacy=true means
// the caller should also apply legacy defaults for DB / socket / log
// so the existing install keeps working with zero edits.
type ConfigPathResolution struct {
	Path    string
	Legacy  bool
	Warning string // stderr notice to print once; "" when no fallback occurred
}

// ResolveConfigPath picks where to read config from when the caller
// hasn't specified one. Prefers the new canonical default and falls
// back to ~/.config/modeltap/config.yaml iff that path exists but the
// new one does not. Explicit CLI / env paths bypass this entirely.
func ResolveConfigPath() ConfigPathResolution {
	newPath := DefaultConfigPath()
	if _, err := os.Stat(newPath); err == nil {
		return ConfigPathResolution{Path: newPath}
	}
	legacy := LegacyConfigPath()
	if _, err := os.Stat(legacy); err == nil && legacy != newPath {
		return ConfigPathResolution{
			Path:   legacy,
			Legacy: true,
			Warning: fmt.Sprintf(
				"modeltap: using legacy config at %s\n"+
					"          move to %s to silence this warning\n"+
					"          (and relocate modeltap.db + server.sock alongside it)\n",
				legacy, newPath,
			),
		}
	}
	return ConfigPathResolution{Path: newPath}
}

// legacyWarnOnce prints a deprecation warning at most once per
// process. Silent when stderr is nil (tests, -h output).
var legacyWarnOnce sync.Once

func emitLegacyWarning(stderr io.Writer, msg string) {
	if stderr == nil || msg == "" {
		return
	}
	legacyWarnOnce.Do(func() {
		fmt.Fprint(stderr, msg)
	})
}

// applyDefaults sets every Viper default value. Split out so Load and
// LoadWithViper stay in sync, and so the legacy-vs-new-default decision
// lives in one place.
func applyDefaults(v *viper.Viper, legacy bool) {
	v.SetDefault("port", 8080)
	v.SetDefault("upstream", "https://api.anthropic.com")
	if legacy {
		v.SetDefault("db_path", legacyDBPath())
		v.SetDefault("bff.socket_path", legacySocketPath())
	} else {
		v.SetDefault("db_path", filepath.Join(DefaultDataDir(), "modeltap.db"))
		v.SetDefault("bff.socket_path", DefaultBFFSocketPath())
	}
	v.SetDefault("retention_days", 30)
	v.SetDefault("max_body_size", "10MB")
	v.SetDefault("dashboard.enabled", false)
	v.SetDefault("dashboard.port", 8081)
	v.SetDefault("dashboard.bind", "127.0.0.1")
	v.SetDefault("bff.socket_mode", uint32(0o600))
	v.SetDefault("bff.max_connections", 100)
	v.SetDefault("bff.max_attachment_size", 5*1024*1024)
}

// expandHome expands a leading ~ in a path to the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// ResolveSecret resolves a secret value that may carry a prefix
// indicating where the actual value lives. Supported prefixes
// (PATCH-0004):
//
//	env:VAR      — os.Getenv("VAR"); empty value is an error
//	file:PATH    — read file contents, trim trailing whitespace;
//	               non-existent path is an error. Leading ~ in PATH
//	               expands to the user's home dir.
//	no prefix    — returned verbatim (preserves the legacy paste-key-
//	               directly ergonomic).
//
// Apply at config load time, not on read, so the rest of the codebase
// continues to see a plain string. Callers that carry a secret
// through more than one field should call this once per field during
// load.
func ResolveSecret(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	switch {
	case strings.HasPrefix(raw, "env:"):
		name := strings.TrimPrefix(raw, "env:")
		if name == "" {
			return "", fmt.Errorf("secret: env: prefix requires a variable name")
		}
		val := os.Getenv(name)
		if val == "" {
			return "", fmt.Errorf("secret: env:%s is empty or unset", name)
		}
		return val, nil
	case strings.HasPrefix(raw, "file:"):
		path := expandHome(strings.TrimPrefix(raw, "file:"))
		if path == "" {
			return "", fmt.Errorf("secret: file: prefix requires a path")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("secret: read %s: %w", path, err)
		}
		return strings.TrimRight(string(b), " \t\r\n"), nil
	}
	return raw, nil
}

// resolveProviderSecrets applies ResolveSecret to every provider's
// APIKey field. Called from Load / LoadWithViper / UnmarshalFrom so
// every config-consuming entry point gets the same behavior.
func resolveProviderSecrets(cfg *Config) error {
	for name, pc := range cfg.Providers {
		resolved, err := ResolveSecret(pc.APIKey)
		if err != nil {
			return fmt.Errorf("provider %q api_key: %w", name, err)
		}
		pc.APIKey = resolved
		cfg.Providers[name] = pc
	}
	return nil
}

// Load reads configuration from defaults, the config file, and environment
// variables. It returns a Config struct with all resolved values.
// The configPath parameter is optional; if empty, the resolver picks
// between the new canonical default and the legacy fallback.
func Load(configPath string) (*Config, error) {
	cfg, _, err := loadInternal(configPath, os.Stderr)
	return cfg, err
}

// LoadWithViper is like Load but also returns the viper instance, allowing
// callers to bind CLI flags to it for flag > env > file precedence.
func LoadWithViper(configPath string) (*Config, *viper.Viper, error) {
	return loadInternal(configPath, os.Stderr)
}

// loadInternal is the shared implementation behind Load and
// LoadWithViper. stderr is exposed as a parameter so tests can capture
// the legacy-fallback deprecation warning.
func loadInternal(configPath string, stderr io.Writer) (*Config, *viper.Viper, error) {
	v := viper.New()

	legacy := false
	if configPath == "" {
		res := ResolveConfigPath()
		configPath = res.Path
		legacy = res.Legacy
		emitLegacyWarning(stderr, res.Warning)
	}
	configPath = expandHome(configPath)

	applyDefaults(v, legacy)

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read config file (ignore file-not-found errors).
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if !os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	v.SetEnvPrefix("MODELTAP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	cfg.DBPath = expandHome(cfg.DBPath)
	cfg.BFF.SocketPath = expandHome(cfg.BFF.SocketPath)
	cfg.BFF.TLSCertFile = expandHome(cfg.BFF.TLSCertFile)
	cfg.BFF.TLSKeyFile = expandHome(cfg.BFF.TLSKeyFile)

	if err := resolveProviderSecrets(&cfg); err != nil {
		return nil, nil, err
	}

	return &cfg, v, nil
}

// UnmarshalFrom creates a Config from an existing viper instance. This is
// useful after binding CLI flags to viper.
func UnmarshalFrom(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	cfg.DBPath = expandHome(cfg.DBPath)
	cfg.BFF.SocketPath = expandHome(cfg.BFF.SocketPath)
	cfg.BFF.TLSCertFile = expandHome(cfg.BFF.TLSCertFile)
	cfg.BFF.TLSKeyFile = expandHome(cfg.BFF.TLSKeyFile)
	if err := resolveProviderSecrets(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// YAML returns the config as a YAML-formatted string.
func (c *Config) YAML() (string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshalling config to YAML: %w", err)
	}
	return string(data), nil
}
