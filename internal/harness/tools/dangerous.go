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
var dangerousPatterns = []*DangerousPattern{
	// Case-insensitive matching for rm flag variants: -rf, -fR, -Rf, -fr.
	{Pattern: `(?i)rm\s+(-[a-z]*r[a-z]*f|(-[a-z]*f[a-z]*r))\b`, Desc: "recursive force delete"},
	{Pattern: `(?i)rm\s+-rf\s+/`, Desc: "recursive force delete from root"},
	{Pattern: `>\s*/dev/`, Desc: "redirect to device"},

	{Pattern: `chmod\s+.*777\b`, Desc: "world-writable permissions"},
	{Pattern: `chown\s+-R\b`, Desc: "recursive ownership change"},
	{Pattern: `mkfs\b`, Desc: "format filesystem"},
	{Pattern: `\bdd\s+`, Desc: "disk/device write"},

	{Pattern: `curl\s+.*-d\b`, Desc: "curl with data (POST)"},
	{Pattern: `\bwget\b`, Desc: "download"},

	{Pattern: `export\s+LD_PRELOAD`, Desc: "LD_PRELOAD injection"},
	{Pattern: `export\s+PATH=`, Desc: "PATH modification"},
}

// dangerousGitPatterns covers Git operations that are destructive
// regardless of where in the command line they appear.
var dangerousGitPatterns = []*DangerousPattern{
	{Pattern: `\bpush\s+.*--force\b`, Desc: "force push"},
	{Pattern: `\bpush\s+.*\s-f(\s|$)`, Desc: "force push (short flag)"},
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
