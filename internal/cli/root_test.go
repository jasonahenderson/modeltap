package cli

import (
	"bytes"
	"strings"
	"testing"
)

func setTestProviderKeys(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("MODELTAP_PROVIDERS_ANTHROPIC_API_KEY", "test-anthropic-key")
	t.Setenv("MODELTAP_PROVIDERS_OPENAI_API_KEY", "test-openai-key")
}

func TestRootCommandExecutes(t *testing.T) {
	// Bare `modeltap` invocation falls back to cobra's default help
	// behavior since the legacy harness CLI was scrapped in v0.2.1.
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root --help returned error: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	cmd := NewRootCommand("1.2.3")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "1.2.3") {
		t.Errorf("expected version output to contain '1.2.3', got: %s", output)
	}
}

func TestSubcommandsRegistered(t *testing.T) {
	tests := []struct {
		name   string
		subcmd string
	}{
		{"start", "start"},
		{"logs", "logs"},
		{"show", "show"},
		{"export", "export"},
		{"config", "config"},
		{"shell", "shell"},
		{"shell-demo", "shell-demo"},
		{"status", "status"},
		{"metrics", "metrics"},
		{"dashboard", "dashboard"},
		{"completion", "completion"},
		{"service", "service"},
	}

	cmd := NewRootCommand("test")

	// Build a set of registered command names.
	registered := make(map[string]bool)
	for _, c := range cmd.Commands() {
		registered[c.Name()] = true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !registered[tt.subcmd] {
				t.Errorf("subcommand %q is not registered on root", tt.subcmd)
			}
		})
	}
}

func TestSubcommandsAcceptHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"start --help", []string{"start", "--help"}},
		{"shell --help", []string{"shell", "--help"}},
		{"shell-demo --help", []string{"shell-demo", "--help"}},
		{"logs --help", []string{"logs", "--help"}},
		{"show --help", []string{"show", "--help"}},
		{"export --help", []string{"export", "--help"}},
		{"config --help", []string{"config", "--help"}},
		{"config show --help", []string{"config", "show", "--help"}},
		{"config set --help", []string{"config", "set", "--help"}},
		{"config path --help", []string{"config", "path", "--help"}},
		{"status --help", []string{"status", "--help"}},
		{"metrics --help", []string{"metrics", "--help"}},
		{"metrics rebuild --help", []string{"metrics", "rebuild", "--help"}},
		{"dashboard --help", []string{"dashboard", "--help"}},
		{"completion --help", []string{"completion", "--help"}},
		{"completion bash --help", []string{"completion", "bash", "--help"}},
		{"completion zsh --help", []string{"completion", "zsh", "--help"}},
		{"completion fish --help", []string{"completion", "fish", "--help"}},
		{"completion powershell --help", []string{"completion", "powershell", "--help"}},
		{"service --help", []string{"service", "--help"}},
		{"service install --help", []string{"service", "install", "--help"}},
		{"service uninstall --help", []string{"service", "uninstall", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "config show" {
				setTestProviderKeys(t)
			}
			cmd := NewRootCommand("test")
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err != nil {
				t.Errorf("%s returned error: %v", tt.name, err)
			}
		})
	}
}

func TestHelpListsAllSubcommands(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}

	output := buf.String()
	expected := []string{
		"start", "logs", "show", "export", "config",
		"shell", "shell-demo",
		"status", "metrics", "dashboard", "completion", "service",
	}
	for _, sub := range expected {
		if !strings.Contains(output, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestStubCommandsOutput(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		// start is no longer a stub; it launches the proxy server.
		// Its behavior is tested in internal/proxy/server_test.go.
		// logs is no longer a stub; it is tested in logs_test.go.
		// show is tested separately in show_test.go (requires a store)
		// export is tested separately in export_test.go (requires a store)
		// status is tested separately in status_test.go (requires a store/config)
		// metrics is tested separately in metrics_test.go (requires a store)
		// dashboard is tested separately in dashboard_test.go (loads config)
		{"config show", []string{"config", "show"}, "port:"},
		{"config set", []string{"config", "set", "key", "val"}, "not implemented yet"},
		{"config path", []string{"config", "path"}, "config.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "config show" {
				setTestProviderKeys(t)
			}
			cmd := NewRootCommand("test")
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err != nil {
				t.Fatalf("%s returned error: %v", tt.name, err)
			}
			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected output to contain %q, got: %s", tt.expected, buf.String())
			}
		})
	}
}
