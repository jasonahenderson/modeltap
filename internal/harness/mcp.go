package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harness/tools"
)

// MCP server state labels. Matched on the wire by harness UX (banner,
// /mcp status table). Keep these strings stable — dashboards consume
// them.
const (
	MCPStateStarting  = "starting"
	MCPStateConnected = "connected"
	MCPStateRetrying  = "retrying"
	MCPStateFailed    = "failed"
)

// MCPServerConfig is the user-facing config knob for one MCP server.
// Mapstructure tags let Viper load it from config.yaml; the harness
// plucks it off AppOptions.MCPServers.
type MCPServerConfig struct {
	Name    string            `mapstructure:"name"`
	Command string            `mapstructure:"command"`
	Args    []string          `mapstructure:"args"`
	Env     map[string]string `mapstructure:"env"`
	Timeout time.Duration     `mapstructure:"timeout"` // default: 5s
}

// MCPLauncher abstracts "run an MCP server subprocess." The default
// impl uses os/exec; tests inject a fake that returns an in-process
// pipe pair and a stub stop func. This keeps the MCPManager
// test-free of real subprocess state.
type MCPLauncher interface {
	Launch(ctx context.Context, cfg MCPServerConfig) (stream MCPStream, stop func(), err error)
}

// ExecLauncher is the production MCPLauncher: fork the configured
// command, pipe stdin/stdout, and return a stop func that terminates
// the process.
type ExecLauncher struct{}

// Launch starts the configured binary with its stdio piped.
//
// Environment handling (WU-094 H-6): MCP servers are third-party
// code installed from npm / pip / random git repos. Passing the
// parent's full env (which includes ANTHROPIC_API_KEY, OPENAI_API_KEY,
// AWS_*, GITHUB_TOKEN, etc.) to every MCP server lets a malicious
// one exfiltrate credentials. We pass only a minimal allowlist plus
// anything the user explicitly configured via cfg.Env.
func (ExecLauncher) Launch(ctx context.Context, cfg MCPServerConfig) (MCPStream, func(), error) {
	if cfg.Command == "" {
		return MCPStream{}, nil, errors.New("mcp: command required")
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = filteredEnv(os.Environ())
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return MCPStream{}, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return MCPStream{}, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return MCPStream{}, nil, fmt.Errorf("start: %w", err)
	}
	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return MCPStream{In: stdin, Out: stdout}, stop, nil
}

// mcpEnvAllowlist is the minimal set of environment variable names
// forwarded to MCP subprocesses by default. Anything outside this
// list has to be explicitly opted in via MCPServerConfig.Env.
// Covers what's needed for a typical binary to locate dependencies
// and format text — nothing touching credentials or config.
var mcpEnvAllowlist = map[string]struct{}{
	"PATH":       {},
	"HOME":       {},
	"USER":       {},
	"LANG":       {},
	"TMPDIR":     {},
	"SHELL":      {},
	"LOGNAME":    {},
	"TZ":         {},
	"TERM":       {},
	"XDG_CONFIG_HOME": {},
	"XDG_DATA_HOME":   {},
	"XDG_CACHE_HOME":  {},
}

// filteredEnv returns only the env vars whose names match the MCP
// allowlist. `LC_*` variables are forwarded as a prefix match because
// their set is POSIX-defined but open-ended (LC_TIME, LC_NUMERIC…).
// Credential-looking variables (anything ending in _KEY / _TOKEN /
// _SECRET, anything under known provider namespaces) are explicitly
// rejected even if they happen to land in the allowlist.
func filteredEnv(input []string) []string {
	out := make([]string, 0, len(mcpEnvAllowlist)+4)
	for _, entry := range input {
		eq := strings.IndexByte(entry, '=')
		if eq <= 0 {
			continue
		}
		name := entry[:eq]
		if isSensitiveEnvName(name) {
			continue
		}
		if _, allow := mcpEnvAllowlist[name]; allow {
			out = append(out, entry)
			continue
		}
		if strings.HasPrefix(name, "LC_") {
			out = append(out, entry)
		}
	}
	return out
}

// isSensitiveEnvName catches names that look like credentials, even
// if they happen to match the positive allowlist. Defense in depth.
func isSensitiveEnvName(name string) bool {
	upper := strings.ToUpper(name)
	for _, suf := range []string{"_KEY", "_TOKEN", "_SECRET", "_PASSWORD", "_CREDENTIAL", "_CREDENTIALS"} {
		if strings.HasSuffix(upper, suf) {
			return true
		}
	}
	for _, pre := range []string{"ANTHROPIC_", "OPENAI_", "AWS_", "AZURE_", "GCP_", "GITHUB_", "GITLAB_", "BITBUCKET_", "MODELTAP_", "SSH_"} {
		if strings.HasPrefix(upper, pre) {
			return true
		}
	}
	return false
}

// MCPServerStatus is the user-facing view of one server.
type MCPServerStatus struct {
	Name       string
	State      string
	LastError  string
	ToolCount  int
	StartedAt  time.Time
	ToolPrefix string // "mcp/<name>:" — prefix matching registered tools
}

// MCPManager owns zero-or-more MCP server connections and wires their
// discovered tools into the harness tools.Registry so the rest of the
// harness treats MCP tools as first-class. Startup is non-blocking
// per design D2: Launch returns immediately and spins up a goroutine
// per server.
type MCPManager struct {
	launcher MCPLauncher
	registry *tools.Registry
	logger   func(format string, args ...any)

	mu      sync.Mutex
	servers map[string]*managedServer
}

// managedServer is the private state MCPManager keeps per server.
type managedServer struct {
	cfg       MCPServerConfig
	client    *MCPClient
	stop      func()
	state     string
	lastError string
	toolNames []string // registered names this server contributed
	startedAt time.Time
}

// NewMCPManager returns a fresh manager. Pass nil launcher for the
// default ExecLauncher (production); tests inject a fake.
func NewMCPManager(registry *tools.Registry, launcher MCPLauncher) *MCPManager {
	if launcher == nil {
		launcher = ExecLauncher{}
	}
	return &MCPManager{
		launcher: launcher,
		registry: registry,
		servers:  make(map[string]*managedServer),
		logger:   func(string, ...any) {},
	}
}

// SetLogger wires a logging function (stderr, slog, etc.) for
// transient status messages. Default is a silent no-op.
func (m *MCPManager) SetLogger(fn func(format string, args ...any)) {
	if fn != nil {
		m.logger = fn
	}
}

// Launch starts one MCP server asynchronously. The goroutine runs
// through subprocess start, initialize, tools/list, and registers
// each discovered tool. Errors at any stage transition the server
// to MCPStateFailed with LastError set; callers can use Reconnect to
// retry.
func (m *MCPManager) Launch(parent context.Context, cfg MCPServerConfig) {
	if cfg.Name == "" {
		cfg.Name = "mcp-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	m.setState(cfg.Name, cfg, MCPStateStarting, "")

	go func() {
		if err := m.runOnce(parent, cfg); err != nil {
			m.setState(cfg.Name, cfg, MCPStateFailed, err.Error())
			m.logger("mcp %s failed: %v", cfg.Name, err)
		}
	}()
}

// Reconnect tears down the named server (if running) and re-launches
// it. Returns an error only when the server was never configured.
func (m *MCPManager) Reconnect(ctx context.Context, name string) error {
	m.mu.Lock()
	srv, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("mcp: unknown server %q", name)
	}
	cfg := srv.cfg
	m.unwireLocked(srv)
	m.mu.Unlock()
	m.Launch(ctx, cfg)
	return nil
}

// Shutdown terminates all servers. Safe to call multiple times.
func (m *MCPManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		m.unwireLocked(s)
	}
}

// Status returns a snapshot of every configured server, sorted by
// name. Used by /mcp status banner rendering.
func (m *MCPManager) Status() []MCPServerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MCPServerStatus, 0, len(m.servers))
	for name, s := range m.servers {
		out = append(out, MCPServerStatus{
			Name:       name,
			State:      s.state,
			LastError:  s.lastError,
			ToolCount:  len(s.toolNames),
			StartedAt:  s.startedAt,
			ToolPrefix: "mcp/" + name + ":",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// runOnce drives one server through its startup sequence. Returns
// the error that killed the run (nil if Shutdown was called cleanly).
func (m *MCPManager) runOnce(parent context.Context, cfg MCPServerConfig) error {
	initCtx, cancel := context.WithTimeout(parent, cfg.Timeout)
	defer cancel()

	stream, stop, err := m.launcher.Launch(parent, cfg)
	if err != nil {
		return fmt.Errorf("launch: %w", err)
	}

	client := NewMCPClient(stream)
	if _, err := client.Initialize(initCtx, "modeltap", "v0.2.0"); err != nil {
		stop()
		_ = client.Close()
		return fmt.Errorf("initialize: %w", err)
	}

	descs, err := client.ListTools(initCtx)
	if err != nil {
		stop()
		_ = client.Close()
		return fmt.Errorf("tools/list: %w", err)
	}

	names := make([]string, 0, len(descs))
	for _, d := range descs {
		t := NewMCPTool(cfg.Name, d, client)
		if m.registry != nil {
			// Registry.Register panics on duplicate names — guard with
			// a recover so a single dup doesn't kill the goroutine.
			func() {
				defer func() { _ = recover() }()
				m.registry.Register(t)
				names = append(names, t.Name())
			}()
		}
	}

	m.mu.Lock()
	if existing, ok := m.servers[cfg.Name]; ok && existing.stop != nil {
		existing.stop()
	}
	m.servers[cfg.Name] = &managedServer{
		cfg:       cfg,
		client:    client,
		stop:      stop,
		state:     MCPStateConnected,
		toolNames: names,
		startedAt: time.Now(),
	}
	m.mu.Unlock()
	return nil
}

// setState updates (or inserts) a managedServer with a new state and
// optional error. Keeps the rest of the fields intact so callers
// report "retrying" without losing their previous tool count.
func (m *MCPManager) setState(name string, cfg MCPServerConfig, state, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[name]
	if !ok {
		s = &managedServer{cfg: cfg}
		m.servers[name] = s
	}
	s.state = state
	s.lastError = errMsg
}

// unwireLocked terminates a server and removes its tools. Must be
// called with m.mu held.
func (m *MCPManager) unwireLocked(s *managedServer) {
	if s == nil {
		return
	}
	if s.stop != nil {
		s.stop()
	}
	if s.client != nil {
		_ = s.client.Close()
	}
	if m.registry != nil {
		for _, name := range s.toolNames {
			m.registry.Deregister(name)
		}
	}
	s.toolNames = nil
	s.client = nil
	s.stop = nil
}

// -----------------------------------------------------------------------
// /mcp slash command
// -----------------------------------------------------------------------

// MCPStatusLoadedMsg carries the snapshot shown by /mcp status.
type MCPStatusLoadedMsg struct {
	Servers []MCPServerStatus
}

// MCPReconnectedMsg fires when /mcp reconnect <name> completes.
type MCPReconnectedMsg struct {
	Name string
}

// MCPErrMsg carries failures from /mcp slash commands.
type MCPErrMsg struct {
	Command string
	Err     error
}

// handleMCPCommand routes /mcp status and /mcp reconnect <name>.
// Unknown forms produce a usage banner.
func (a *App) handleMCPCommand(msg SubmitMsg) tea.Cmd {
	args := strings.TrimSpace(msg.CommandArgs)
	if a.mcp == nil {
		return func() tea.Msg {
			return BannerMsg{Text: "MCP manager not wired", Duration: 4 * time.Second}
		}
	}
	parts := strings.SplitN(args, " ", 2)
	sub := ""
	if len(parts) > 0 {
		sub = strings.ToLower(parts[0])
	}
	rest := ""
	if len(parts) == 2 {
		rest = strings.TrimSpace(parts[1])
	}
	switch sub {
	case "", "status":
		return func() tea.Msg { return MCPStatusLoadedMsg{Servers: a.mcp.Status()} }
	case "reconnect":
		if rest == "" {
			return func() tea.Msg {
				return BannerMsg{Text: "Usage: /mcp reconnect <server-name>", Duration: 4 * time.Second}
			}
		}
		mgr := a.mcp
		name := rest
		return func() tea.Msg {
			if err := mgr.Reconnect(context.Background(), name); err != nil {
				return MCPErrMsg{Command: "mcp reconnect", Err: err}
			}
			return MCPReconnectedMsg{Name: name}
		}
	}
	return func() tea.Msg {
		return BannerMsg{Text: "Unknown /mcp subcommand: " + sub, Duration: 4 * time.Second}
	}
}

// formatMCPStatus renders the /mcp status banner. Keeps each server
// on its own line: "<name> [<state>] N tools" with an error line for
// failed servers.
func formatMCPStatus(servers []MCPServerStatus) string {
	if len(servers) == 0 {
		return "No MCP servers configured."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "MCP servers (%d):", len(servers))
	for _, s := range servers {
		fmt.Fprintf(&b, "\n  %s [%s] %d tools", s.Name, s.State, s.ToolCount)
		if s.LastError != "" {
			fmt.Fprintf(&b, " — %s", s.LastError)
		}
	}
	return b.String()
}
