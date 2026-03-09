// Package config provides configuration loading for modeltap.
//
// Configuration is loaded with three-tier precedence (highest to lowest):
//
//	flags > environment variables > config file > defaults
//
// Environment variables use the MODELTAP_ prefix (e.g., MODELTAP_PORT).
// The config file path defaults to ~/.config/modeltap/config.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// DashboardConfig holds dashboard-specific settings.
type DashboardConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Port    int    `yaml:"port" mapstructure:"port"`
	Bind    string `yaml:"bind" mapstructure:"bind"`
}

// ProviderConfig holds per-provider settings.
type ProviderConfig struct {
	Upstream string `yaml:"upstream" mapstructure:"upstream"`
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
}

// DefaultConfigDir returns the default configuration directory path.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, ".config", "modeltap")
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
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

// Load reads configuration from defaults, the config file, and environment
// variables. It returns a Config struct with all resolved values.
// The configPath parameter is optional; if empty, the default path is used.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults.
	v.SetDefault("port", 8080)
	v.SetDefault("upstream", "https://api.anthropic.com")
	v.SetDefault("db_path", filepath.Join("~", ".config", "modeltap", "modeltap.db"))
	v.SetDefault("retention_days", 30)
	v.SetDefault("max_body_size", "10MB")
	v.SetDefault("dashboard.enabled", false)
	v.SetDefault("dashboard.port", 8081)
	v.SetDefault("dashboard.bind", "127.0.0.1")

	// Set up config file.
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	configPath = expandHome(configPath)

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read config file (ignore file-not-found errors).
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// If the file simply doesn't exist, that's fine.
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	// Set up environment variables.
	v.SetEnvPrefix("MODELTAP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into Config struct.
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Expand ~ in paths.
	cfg.DBPath = expandHome(cfg.DBPath)

	return &cfg, nil
}

// LoadWithViper is like Load but also returns the viper instance, allowing
// callers to bind CLI flags to it for flag > env > file precedence.
func LoadWithViper(configPath string) (*Config, *viper.Viper, error) {
	v := viper.New()

	// Set defaults.
	v.SetDefault("port", 8080)
	v.SetDefault("upstream", "https://api.anthropic.com")
	v.SetDefault("db_path", filepath.Join("~", ".config", "modeltap", "modeltap.db"))
	v.SetDefault("retention_days", 30)
	v.SetDefault("max_body_size", "10MB")
	v.SetDefault("dashboard.enabled", false)
	v.SetDefault("dashboard.port", 8081)
	v.SetDefault("dashboard.bind", "127.0.0.1")

	// Set up config file.
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	configPath = expandHome(configPath)

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

	// Set up environment variables.
	v.SetEnvPrefix("MODELTAP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into Config struct.
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Expand ~ in paths.
	cfg.DBPath = expandHome(cfg.DBPath)

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
