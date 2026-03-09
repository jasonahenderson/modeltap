package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	p := DetectPlatform()

	switch runtime.GOOS {
	case "darwin":
		if p != PlatformMacOS {
			t.Errorf("expected PlatformMacOS on darwin, got %s", p)
		}
	case "linux":
		if p != PlatformLinux {
			t.Errorf("expected PlatformLinux on linux, got %s", p)
		}
	default:
		if p != PlatformUnsupported {
			t.Errorf("expected PlatformUnsupported on %s, got %s", runtime.GOOS, p)
		}
	}
}

func TestGenerateLaunchdPlist(t *testing.T) {
	cfg := Config{
		BinaryPath:  "/usr/local/bin/modeltap",
		ConfigPath:  "/home/user/.config/modeltap/config.yaml",
		ServiceName: "com.modeltap.proxy",
	}

	content, err := generateLaunchdPlist(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"<key>Label</key>",
		"<string>com.modeltap.proxy</string>",
		"<key>ProgramArguments</key>",
		"<string>/usr/local/bin/modeltap</string>",
		"<string>start</string>",
		"<key>KeepAlive</key>",
		"<true/>",
		"<key>RunAtLoad</key>",
		"<key>StandardOutPath</key>",
		"modeltap.log",
		"<key>StandardErrorPath</key>",
		"modeltap.err.log",
		"<key>EnvironmentVariables</key>",
		"<key>WorkingDirectory</key>",
	}

	for _, s := range expected {
		if !strings.Contains(content, s) {
			t.Errorf("plist missing expected content: %q", s)
		}
	}
}

func TestGenerateSystemdUnit(t *testing.T) {
	cfg := Config{
		BinaryPath:  "/usr/local/bin/modeltap",
		ConfigPath:  "/home/user/.config/modeltap/config.yaml",
		ServiceName: "modeltap",
	}

	content, err := generateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"[Unit]",
		"Description=Modeltap API Proxy",
		"After=network.target",
		"[Service]",
		"ExecStart=/usr/local/bin/modeltap start",
		"Restart=on-failure",
		"RestartSec=5",
		"Environment=MODELTAP_CONFIG=/home/user/.config/modeltap/config.yaml",
		"[Install]",
		"WantedBy=default.target",
	}

	for _, s := range expected {
		if !strings.Contains(content, s) {
			t.Errorf("systemd unit missing expected content: %q", s)
		}
	}
}

func TestServiceFilePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	tests := []struct {
		platform Platform
		expected string
		wantErr  bool
	}{
		{
			platform: PlatformMacOS,
			expected: filepath.Join(home, "Library", "LaunchAgents", "com.modeltap.proxy.plist"),
		},
		{
			platform: PlatformLinux,
			expected: filepath.Join(home, ".config", "systemd", "user", "modeltap.service"),
		},
		{
			platform: PlatformUnsupported,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.platform), func(t *testing.T) {
			path, err := ServiceFilePath(tt.platform)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for unsupported platform, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, path)
			}
		})
	}
}

func TestGenerateServiceFile_UnsupportedPlatform(t *testing.T) {
	cfg := Config{
		BinaryPath: "/usr/local/bin/modeltap",
		ConfigPath: "/home/user/.config/modeltap/config.yaml",
	}

	_, err := GenerateServiceFile(PlatformUnsupported, cfg)
	if err == nil {
		t.Error("expected error for unsupported platform, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("expected 'unsupported platform' in error, got: %v", err)
	}
}

func TestGenerateServiceFile_DefaultServiceName(t *testing.T) {
	cfg := Config{
		BinaryPath: "/usr/local/bin/modeltap",
		ConfigPath: "/home/user/.config/modeltap/config.yaml",
	}

	// macOS should use default service name.
	content, err := GenerateServiceFile(PlatformMacOS, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "com.modeltap.proxy") {
		t.Error("macOS service file missing default service name com.modeltap.proxy")
	}

	// Linux should use default service name.
	content, err = GenerateServiceFile(PlatformLinux, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "/usr/local/bin/modeltap start") {
		t.Error("Linux service file missing ExecStart directive")
	}
}
