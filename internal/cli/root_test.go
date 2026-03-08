package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandExecutes(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("root command returned error: %v", err)
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
		name    string
		subcmd  string
	}{
		{"start", "start"},
		{"logs", "logs"},
		{"show", "show"},
		{"export", "export"},
		{"config", "config"},
		{"status", "status"},
		{"metrics", "metrics"},
		{"dashboard", "dashboard"},
		{"completion", "completion"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		"status", "metrics", "dashboard", "completion",
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
		{"logs", []string{"logs"}, "not implemented yet"},
		{"show", []string{"show", "abc123"}, "not implemented yet"},
		// export is tested separately in export_test.go (requires a store)
		{"config show", []string{"config", "show"}, "port:"},
		{"config set", []string{"config", "set", "key", "val"}, "not implemented yet"},
		{"config path", []string{"config", "path"}, "config.yaml"},
		{"status", []string{"status"}, "not implemented yet"},
		{"metrics", []string{"metrics"}, "not implemented yet"},
		{"metrics rebuild", []string{"metrics", "rebuild"}, "not implemented yet"},
		{"dashboard", []string{"dashboard"}, "not implemented yet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
