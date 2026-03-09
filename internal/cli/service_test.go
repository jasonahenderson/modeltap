package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestServiceCommandExists(t *testing.T) {
	cmd := NewRootCommand("test")

	registered := make(map[string]bool)
	for _, c := range cmd.Commands() {
		registered[c.Name()] = true
	}

	if !registered["service"] {
		t.Fatal("service subcommand is not registered on root")
	}
}

func TestServiceSubcommandsExist(t *testing.T) {
	cmd := NewRootCommand("test")

	// Find the service command and verify subcommands.
	for _, c := range cmd.Commands() {
		if c.Name() == "service" {
			subcommands := make(map[string]bool)
			for _, sub := range c.Commands() {
				subcommands[sub.Name()] = true
			}
			if !subcommands["install"] {
				t.Error("service command missing 'install' subcommand")
			}
			if !subcommands["uninstall"] {
				t.Error("service command missing 'uninstall' subcommand")
			}
			if !subcommands["status"] {
				t.Error("service command missing 'status' subcommand")
			}
			if !subcommands["logs"] {
				t.Error("service command missing 'logs' subcommand")
			}
			return
		}
	}
	t.Fatal("service command not found")
}

func TestServiceHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"service", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("service --help returned error: %v", err)
	}

	output := buf.String()
	expected := []string{"install", "uninstall", "status", "logs", "background service"}
	for _, s := range expected {
		if !strings.Contains(output, s) {
			t.Errorf("service help output missing %q, got: %s", s, output)
		}
	}
}

func TestServiceInstallHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"service", "install", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("service install --help returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Install") {
		t.Errorf("expected install help to contain 'Install', got: %s", output)
	}
}

func TestServiceUninstallHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"service", "uninstall", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("service uninstall --help returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Uninstall") {
		t.Errorf("expected uninstall help to contain 'Uninstall', got: %s", output)
	}
}

func TestServiceStatusHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"service", "status", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("service status --help returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "status") {
		t.Errorf("expected status help to contain 'status', got: %s", output)
	}
	if !strings.Contains(output, "installed") {
		t.Errorf("expected status help to contain 'installed', got: %s", output)
	}
}

func TestServiceLogsHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"service", "logs", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("service logs --help returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "logs") {
		t.Errorf("expected logs help to contain 'logs', got: %s", output)
	}
	if !strings.Contains(output, "--lines") {
		t.Errorf("expected logs help to contain '--lines', got: %s", output)
	}
	if !strings.Contains(output, "-n") {
		t.Errorf("expected logs help to contain '-n', got: %s", output)
	}
}

func TestServiceLogsLinesFlag(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Just verify the flag is accepted by checking help shows the default.
	cmd.SetArgs([]string{"service", "logs", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("service logs --help returned error: %v", err)
	}

	output := buf.String()
	// Default value of 50 should appear in the help.
	if !strings.Contains(output, "50") {
		t.Errorf("expected logs help to show default value '50', got: %s", output)
	}
}
