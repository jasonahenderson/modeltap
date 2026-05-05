package tools

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"
)

// PermissionLevel controls automatic tool approval per Bundle 7
// design D3.1.
type PermissionLevel int

const (
	// PermDefault: read-only auto-allowed, all others prompt on first
	// use per session.
	PermDefault PermissionLevel = iota

	// PermAcceptEdits: read-only and write/edit auto-allowed; bash /
	// git mutations and web tools still prompt.
	PermAcceptEdits

	// PermAutonomous: all tools auto-allowed except absolute-deny
	// patterns (dangerous commands, SSRF risks).
	PermAutonomous
)

// PermissionDecision is the outcome of a permission check.
type PermissionDecision int

const (
	PermAllow  PermissionDecision = iota // run without prompting
	PermPrompt                           // ask the user
	PermDeny                             // never allow (no prompt)
)

// PermissionEnforcer decides whether a tool execution requires user
// approval. Approvals are remembered per tool name for the lifetime
// of the enforcer (typically the session). Web domains track
// separately so approving example.com doesn't blanket-approve the
// whole WebFetch tool.
type PermissionEnforcer struct {
	level PermissionLevel

	mu       sync.Mutex
	approved map[string]bool // tool name → approved this session
	domains  map[string]bool // domain → approved this session
}

// NewPermissionEnforcer constructs an enforcer at the given level.
func NewPermissionEnforcer(level PermissionLevel) *PermissionEnforcer {
	return &PermissionEnforcer{
		level:    level,
		approved: make(map[string]bool),
		domains:  make(map[string]bool),
	}
}

// Level returns the configured permission level.
func (p *PermissionEnforcer) Level() PermissionLevel { return p.level }

// SetLevel updates the permission level. Existing approvals are kept.
func (p *PermissionEnforcer) SetLevel(l PermissionLevel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.level = l
}

// Approve marks toolName as approved for this session.
func (p *PermissionEnforcer) Approve(toolName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.approved[toolName] = true
}

// IsApproved reports whether toolName was approved earlier in this session.
func (p *PermissionEnforcer) IsApproved(toolName string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.approved[toolName]
}

// ApproveDomain marks a host as approved (used by WebFetch).
func (p *PermissionEnforcer) ApproveDomain(domain string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.domains[strings.ToLower(domain)] = true
}

// IsDomainApproved reports whether a domain has been approved.
func (p *PermissionEnforcer) IsDomainApproved(domain string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.domains[strings.ToLower(domain)]
}

// Check applies the FEAT-0009 permission decision matrix per design
// D3.2 plus the absolute deny-list per D3.3.
//
// Tools may carry input-dependent risk (Bash commands matched against
// IsDangerous, Git subcommands inspected for force/reset semantics).
// The check inspects input for known tool names with that property.
func (p *PermissionEnforcer) Check(tool Tool, input json.RawMessage) PermissionDecision {
	if tool == nil {
		return PermDeny
	}
	risk := tool.RiskLevel()
	name := tool.Name()

	// Dangerous patterns always prompt regardless of permission level.
	if isDangerous := alwaysPrompt(tool, input); isDangerous {
		return PermPrompt
	}

	// Risk-based decision matrix.
	switch risk {
	case RiskReadOnly:
		return PermAllow

	case RiskWrite:
		switch p.Level() {
		case PermAcceptEdits, PermAutonomous:
			return PermAllow
		}
		// PermDefault: prompt on first use, allow thereafter.
		if p.IsApproved(name) {
			return PermAllow
		}
		return PermPrompt

	case RiskExecute:
		// Git read-only subcommands (status / log / diff / …) are
		// auto-allowed regardless of mode. Dangerous-git was already
		// caught by alwaysPrompt above, so anything reaching here that
		// classifies as read is safe.
		if name == ToolNameGit && IsGitRead(gitCommandFromInput(input)) {
			return PermAllow
		}
		// WebFetch handles per-domain approval below.
		if name == ToolNameWebFetch {
			if domainAllowed := p.checkWebFetchDomain(input); domainAllowed {
				return PermAllow
			}
		}
		switch p.Level() {
		case PermAutonomous:
			return PermAllow
		}
		if p.IsApproved(name) {
			return PermAllow
		}
		return PermPrompt

	case RiskDestructive:
		// Always prompt — even Autonomous.
		return PermPrompt
	}

	return PermPrompt
}

// ToolNameWebFetch is the canonical WebFetch tool name. Defined here
// (not in webfetch.go) so the permission layer can refer to it
// without depending on the implementation file.
const ToolNameWebFetch = "WebFetch"

// ToolNameBash is the canonical Bash tool name.
const ToolNameBash = "Bash"

// ToolNameGit is the canonical Git tool name.
const ToolNameGit = "Git"

// alwaysPrompt returns true for tool+input combinations that must
// always be confirmed, even in PermAutonomous mode.
func alwaysPrompt(tool Tool, input json.RawMessage) bool {
	switch tool.Name() {
	case ToolNameBash:
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &args); err == nil {
			if IsDangerous(args.Command) {
				return true
			}
		}
	case ToolNameGit:
		if IsDangerousGit(gitCommandFromInput(input)) {
			return true
		}
	case ToolNameWebFetch:
		// Disallow internal/private hosts entirely (SSRF prevention).
		// The web fetch tool will run if a public URL is supplied;
		// internal URLs are absolute deny rather than prompt.
		if isInternalURL(input) {
			return true
		}
	}
	return false
}

// gitCommandFromInput extracts the git subcommand + args string from
// the Git tool's JSON input. Accepts either `command` (single string,
// preferred per WU-078 schema) or the legacy `args` array, joining
// them with spaces. Returns "" on unmarshal failure.
func gitCommandFromInput(input json.RawMessage) string {
	var args struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ""
	}
	combined := strings.Join(args.Args, " ")
	if args.Command != "" {
		if combined == "" {
			return args.Command
		}
		combined = args.Command + " " + combined
	}
	return combined
}

// checkWebFetchDomain inspects WebFetch input for a previously-
// approved host. Returns true when the request can run without a
// prompt.
func (p *PermissionEnforcer) checkWebFetchDomain(input json.RawMessage) bool {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(input, &args); err != nil || args.URL == "" {
		return false
	}
	u, err := url.Parse(args.URL)
	if err != nil {
		return false
	}
	return p.IsDomainApproved(u.Host)
}

// isInternalURL reports whether the WebFetch input targets a private
// network address (loopback, RFC1918, link-local). Used for the
// absolute-deny list (SSRF prevention).
func isInternalURL(input json.RawMessage) bool {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(input, &args); err != nil || args.URL == "" {
		return false
	}
	u, err := url.Parse(args.URL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	}
	// Block 10.x, 192.168.x, 172.16-31.x, 169.254.x (link-local).
	parts := strings.Split(host, ".")
	if len(parts) == 4 {
		switch parts[0] {
		case "10":
			return true
		case "169":
			if parts[1] == "254" {
				return true
			}
		case "172":
			if len(parts[1]) > 0 {
				// 172.16.x – 172.31.x
				if parts[1] >= "16" && parts[1] <= "31" {
					return true
				}
			}
		case "192":
			if parts[1] == "168" {
				return true
			}
		}
	}
	return false
}
