package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDashboardCommandExists(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"dashboard"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("dashboard command returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Dashboard:") {
		t.Errorf("expected output to contain 'Dashboard:', got: %s", output)
	}
	if !strings.Contains(output, "http://") {
		t.Errorf("expected output to contain a URL, got: %s", output)
	}
}

func TestDashboardOutputContainsDefaultPort(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"dashboard"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("dashboard command returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "8081") {
		t.Errorf("expected output to contain default port 8081, got: %s", output)
	}
}

func TestStartDashboardFlagRecognized(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("start --help returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "--dashboard") {
		t.Errorf("expected start --help to show --dashboard flag, got: %s", output)
	}
}

func TestStartDashboardPortFlagRecognized(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("start --help returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "--dashboard-port") {
		t.Errorf("expected start --help to show --dashboard-port flag, got: %s", output)
	}
}
