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
	expected := []string{"install", "uninstall", "background service"}
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
