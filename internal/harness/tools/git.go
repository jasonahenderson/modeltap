package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// GitTool runs git subcommands with classification-aware risk
// reporting. The static RiskLevel is RiskExecute so that by default a
// Git call prompts in Default / AcceptEdits; the permission layer
// downgrades safe-read subcommands (status / log / diff / …) to
// PermAllow, and dangerous ones (push --force / reset --hard / …) to
// PermPrompt via `alwaysPrompt` regardless of mode.
type GitTool struct {
	projectRoot    string
	defaultTimeout time.Duration
	maxOutput      int
}

// NewGitTool constructs a Git tool rooted at projectRoot.
func NewGitTool(projectRoot string) *GitTool {
	return &GitTool{
		projectRoot:    projectRoot,
		defaultTimeout: 60 * time.Second,
		maxOutput:      100 * 1024,
	}
}

// SetDefaultTimeout overrides the per-invocation timeout.
func (g *GitTool) SetDefaultTimeout(d time.Duration) { g.defaultTimeout = d }

// SetMaxOutput overrides the truncation ceiling.
func (g *GitTool) SetMaxOutput(n int) { g.maxOutput = n }

func (g *GitTool) Name() string        { return ToolNameGit }
func (g *GitTool) Description() string { return "Run a git subcommand" }
func (g *GitTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {"type":"string","description":"Git command without the leading 'git' (e.g. 'status', 'log --oneline -n 5')"}
  },
  "required": ["command"]
}`)
}
func (g *GitTool) OutputEnvelope() string { return "text" }
func (g *GitTool) RiskLevel() RiskLevel   { return RiskExecute }

type gitArgs struct {
	Command *string `json:"command"`
}

func (g *GitTool) Execute(ctx context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in gitArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.Command == nil || strings.TrimSpace(*in.Command) == "" {
		return ErrorResult("command is required"), nil
	}

	execCtx, cancel := context.WithTimeout(ctx, g.defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", "git "+*in.Command)
	cmd.Dir = g.projectRoot
	output, runErr := cmd.CombinedOutput()

	truncated := truncateOutput(output, g.maxOutput)

	if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		return ErrorResult("git command timed out after %s\n%s", g.defaultTimeout, truncated), nil
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

// gitReadCommands is the allow-list of subcommands that are safe
// reads in their bare form. Subcommands whose mutation/read behaviour
// is ambiguous based on positional args (remote / config) are
// deliberately absent — they're treated as RiskExecute so the first
// use prompts.
var gitReadCommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true,
	"branch":   true, "tag": true,
	"ls-files": true, "ls-tree": true, "rev-parse": true,
	"blame": true, "shortlog": true, "describe": true,
	"rev-list": true, "cat-file": true, "name-rev": true,
	"grep": true, "fsck": true,
}

// mutationFlags turn a read subcommand into a mutation (branch -d / -m
// create, delete, or rename). Destructive-force variants (-D) are
// caught earlier by IsDangerousGit.
var mutationFlags = map[string]bool{
	"-d": true, "-m": true, "-M": true, "-c": true, "-C": true,
	"--delete": true, "--move": true, "--copy": true,
}

// positionalMutationSubs lists subcommands that are read-only in their
// bare form but become mutations the moment a positional (non-flag)
// argument is supplied — `git branch newname` creates a branch, `git
// tag v1.0` creates a tag.
var positionalMutationSubs = map[string]bool{
	"branch": true,
	"tag":    true,
}

// ClassifyGit inspects a git command (without the "git" prefix) and
// returns the risk level that should govern the invocation. Precedence:
// destructive > mutation > read-only > execute (default).
func ClassifyGit(command string) RiskLevel {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return RiskExecute
	}
	if IsDangerousGit(trimmed) {
		return RiskDestructive
	}

	fields := strings.Fields(trimmed)
	sub := fields[0]
	if !gitReadCommands[sub] {
		return RiskExecute
	}
	for _, f := range fields[1:] {
		if mutationFlags[f] {
			return RiskExecute
		}
	}
	if positionalMutationSubs[sub] {
		for _, f := range fields[1:] {
			if !strings.HasPrefix(f, "-") {
				return RiskExecute
			}
		}
	}
	return RiskReadOnly
}

// IsGitRead reports whether a git command (without the "git" prefix)
// is a pure read per ClassifyGit. Exposed as a shorthand for callers
// that only care about the allow/prompt split.
func IsGitRead(command string) bool {
	return ClassifyGit(command) == RiskReadOnly
}
