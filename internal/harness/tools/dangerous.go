package tools

import (
	"regexp"
	"strings"
)

// DangerousPattern is one entry in the dangerous-command catalog.
type DangerousPattern struct {
	Pattern string // compiled regex
	Desc    string
	re      *regexp.Regexp
}

// dangerousPatterns is the seed list per Bundle 7 design D4. The
// regexes are compiled lazily on first use (init() below) so the
// package doesn't pay the compile cost at import time when the tool
// framework isn't exercised.
//
// The catalog is explicitly advisory — regex over a free-form shell
// command is fundamentally bypassable (obfuscation via quotes, env-var
// indirection, $'\...' escapes, base64, etc. per WU-094 H-7). The
// intent is to raise the bar on obvious destructive patterns so the
// model can't stumble into them without an explicit prompt. True
// defense belongs in the permission model layer.
var dangerousPatterns = []*DangerousPattern{
	// rm variants. -rf, -fR, -Rf, -fr detected together; also the
	// space-separated `-r -f` and `--recursive --force` forms.
	{Pattern: `(?i)rm\s+(-[a-z]*r[a-z]*f|(-[a-z]*f[a-z]*r))\b`, Desc: "recursive force delete"},
	{Pattern: `(?i)rm\s+.*\B-r\b.*\B-f\b`, Desc: "recursive force delete (split flags)"},
	{Pattern: `(?i)rm\s+.*--recursive.*--force`, Desc: "recursive force delete (long flags)"},
	{Pattern: `(?i)rm\s+-rf?\s+/`, Desc: "recursive force delete from root"},
	{Pattern: `(?i)\bfind\s+.*-delete\b`, Desc: "find -delete"},
	{Pattern: `\bshred\b`, Desc: "overwrite and delete"},
	{Pattern: `>\s*/dev/`, Desc: "redirect to device"},

	// Permissions / ownership / filesystem.
	{Pattern: `chmod\s+.*777\b`, Desc: "world-writable permissions"},
	{Pattern: `chown\s+-R\b`, Desc: "recursive ownership change"},
	{Pattern: `mkfs\b`, Desc: "format filesystem"},
	{Pattern: `\bdd\s+`, Desc: "disk/device write"},

	// Network download-and-execute. `curl -d` is the old POST check;
	// the new entries catch the "download + pipe to shell" pattern
	// that's the dominant way to bootstrap a malware payload.
	{Pattern: `curl\s+.*-d\b`, Desc: "curl with data (POST)"},
	{Pattern: `\bcurl\b.*\|\s*(sh|bash|zsh|ksh|fish|python|python3|node|ruby|perl)\b`, Desc: "curl | shell"},
	{Pattern: `\bwget\b.*\|\s*(sh|bash|zsh|ksh|fish|python|python3|node|ruby|perl)\b`, Desc: "wget | shell"},
	{Pattern: `\bwget\b`, Desc: "download"},

	// Arbitrary code execution via -c/-e flags. These are "shell by
	// another name" — any script that can be -c'd can also escape
	// every other guard.
	{Pattern: `\b(bash|sh|zsh|ksh|fish)\s+-c\b`, Desc: "shell -c"},
	{Pattern: `\b(python|python3)\s+-c\b`, Desc: "python -c"},
	{Pattern: `\bnode\s+-e\b`, Desc: "node -e"},
	{Pattern: `\b(ruby|perl)\s+-e\b`, Desc: "interpreter -e"},
	{Pattern: `\beval\s+`, Desc: "eval"},

	// Network tools that commonly appear in lateral movement.
	{Pattern: `\b(nc|ncat|netcat)\s`, Desc: "netcat"},
	{Pattern: `\bssh\s+.*@`, Desc: "ssh to remote host"},
	{Pattern: `\b(scp|rsync)\s+.*@`, Desc: "file transfer to remote host"},

	// Env-var shenanigans.
	{Pattern: `export\s+LD_PRELOAD`, Desc: "LD_PRELOAD injection"},
	{Pattern: `export\s+PATH=`, Desc: "PATH modification"},

	// Fork bomb — classic and unambiguous.
	{Pattern: `:\(\)\s*\{\s*:\|:\&\s*\}\s*;\s*:`, Desc: "fork bomb"},
}

// dangerousGitPatterns covers Git operations that are destructive
// regardless of where in the command line they appear.
var dangerousGitPatterns = []*DangerousPattern{
	{Pattern: `\bpush\b.*--force\b`, Desc: "force push"},
	{Pattern: `\bpush\b.*(^|\s)-f(\s|$)`, Desc: "force push (short flag)"},
	{Pattern: `\breset\s+--hard\b`, Desc: "hard reset"},
	{Pattern: `\bclean\s+.*-f`, Desc: "force clean"},
	{Pattern: `\bcheckout\s+.*--\s+\.`, Desc: "discard all changes"},
	{Pattern: `\bbranch\s+.*-D\b`, Desc: "force delete branch"},
}

func init() {
	for _, p := range dangerousPatterns {
		p.re = regexp.MustCompile(p.Pattern)
	}
	for _, p := range dangerousGitPatterns {
		p.re = regexp.MustCompile(p.Pattern)
	}
}

// IsDangerous reports whether a Bash command string matches any
// dangerous pattern. Returns the matching DangerousPattern's Desc on
// the first hit (or "" if none).
func IsDangerous(command string) bool {
	if strings.TrimSpace(command) == "" {
		return false
	}
	for _, p := range dangerousPatterns {
		if p.re.MatchString(command) {
			return true
		}
	}
	return false
}

// DangerReason returns the human-readable reason a Bash command was
// flagged dangerous, or "" if the command is fine.
func DangerReason(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	for _, p := range dangerousPatterns {
		if p.re.MatchString(command) {
			return p.Desc
		}
	}
	return ""
}

// IsDangerousGit reports whether a Git command (subcommand + args, e.g.
// "push --force origin main") matches a destructive Git pattern.
func IsDangerousGit(command string) bool {
	if strings.TrimSpace(command) == "" {
		return false
	}
	for _, p := range dangerousGitPatterns {
		if p.re.MatchString(command) {
			return true
		}
	}
	return false
}

// DangerReasonGit returns the human-readable reason a Git command was
// flagged dangerous, or "" if the command is fine.
func DangerReasonGit(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	for _, p := range dangerousGitPatterns {
		if p.re.MatchString(command) {
			return p.Desc
		}
	}
	return ""
}
