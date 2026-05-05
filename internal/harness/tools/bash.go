package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// BashTool runs shell commands with output capture. The permission
// layer enforces dangerous-pattern detection before Execute is called
// (design D3.3 / D4); by the time a command reaches Execute it has
// either been allow-listed, approved by the user, or belongs to an
// auto-allowed permission level.
type BashTool struct {
	projectRoot    string
	defaultTimeout time.Duration
	maxOutput      int // bytes; anything beyond is truncated to last maxOutput/2
}

// NewBashTool constructs a Bash tool rooted at projectRoot. The
// default timeout (120s) and output ceiling (100KB) match the Bundle
// 7 design; override via SetDefaultTimeout / SetMaxOutput for tests.
func NewBashTool(projectRoot string) *BashTool {
	return &BashTool{
		projectRoot:    projectRoot,
		defaultTimeout: 120 * time.Second,
		maxOutput:      100 * 1024,
	}
}

// SetDefaultTimeout overrides the timeout used when input omits it.
// Primarily for tests that need short timeouts to exercise the
// kill path.
func (b *BashTool) SetDefaultTimeout(d time.Duration) { b.defaultTimeout = d }

// SetMaxOutput overrides the truncation ceiling. Tests use this to
// drive truncation without generating huge output.
func (b *BashTool) SetMaxOutput(n int) { b.maxOutput = n }

func (b *BashTool) Name() string        { return ToolNameBash }
func (b *BashTool) Description() string { return "Execute a shell command" }
func (b *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "command":     {"type":"string","description":"Shell command to execute"},
    "timeout_ms":  {"type":"integer","description":"Timeout in milliseconds (default 120000, max 600000)"},
    "description": {"type":"string","description":"What this command does (for approval prompt)"}
  },
  "required": ["command"]
}`)
}
func (b *BashTool) OutputEnvelope() string { return "text" }
func (b *BashTool) RiskLevel() RiskLevel   { return RiskExecute }

type bashArgs struct {
	Command   *string `json:"command"`
	TimeoutMS int     `json:"timeout_ms"`
}

const bashMaxTimeoutMS = 600_000

func (b *BashTool) Execute(ctx context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in bashArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.Command == nil || *in.Command == "" {
		return ErrorResult("command is required"), nil
	}

	timeout := b.defaultTimeout
	if in.TimeoutMS > 0 {
		if in.TimeoutMS > bashMaxTimeoutMS {
			in.TimeoutMS = bashMaxTimeoutMS
		}
		timeout = time.Duration(in.TimeoutMS) * time.Millisecond
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", *in.Command)
	cmd.Dir = b.projectRoot
	// On Linux, /bin/sh (dash) may fork the trailing simple command
	// rather than exec'ing into it. CommandContext SIGKILLs sh, but the
	// forked child inherits the stdout/stderr pipes and gets reparented
	// to PID 1 — CombinedOutput's I/O reader goroutines then block on
	// EOF until the child exits naturally. WaitDelay (Go 1.20+) bounds
	// that wait so timeouts return promptly.
	cmd.WaitDelay = 100 * time.Millisecond
	output, runErr := cmd.CombinedOutput()

	truncated := truncateOutput(output, b.maxOutput)

	if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		return ErrorResult("command timed out after %s\n%s", timeout, truncated), nil
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return ErrorResult("exit %d\n%s", exitErr.ExitCode(), truncated), nil
		}
		return ErrorResult("run: %v\n%s", runErr, truncated), nil
	}

	return SuccessResult(truncated, "text"), nil
}

// truncateOutput caps combined output at max bytes. When the output
// exceeds the cap we keep the trailing half (since errors typically
// surface at the end) plus a visible marker.
func truncateOutput(output []byte, max int) string {
	if max <= 0 || len(output) <= max {
		return string(output)
	}
	keep := max / 2
	if keep <= 0 {
		keep = 1
	}
	return fmt.Sprintf("[output truncated — showing last %d bytes of %d]\n%s",
		keep, len(output), output[len(output)-keep:])
}
