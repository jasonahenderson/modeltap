package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/harness/tools"
)

// fakeLauncher is an MCPLauncher backed by in-process pipes. Each
// Launch call spins up a goroutine that speaks just enough MCP to
// satisfy initialize + tools/list + tools/call with canned replies.
type fakeLauncher struct {
	tools       []MCPToolDescriptor
	toolResult  MCPToolsCallResult
	toolError   string // if non-empty, next tools/call returns this as a JSON-RPC error message
	initErr     string
	listErr     string
	launchErr   error
	callLog     []string
	callLogLock sync.Mutex
}

func (f *fakeLauncher) appendCall(m string) {
	f.callLogLock.Lock()
	f.callLog = append(f.callLog, m)
	f.callLogLock.Unlock()
}

func (f *fakeLauncher) Launch(ctx context.Context, cfg MCPServerConfig) (MCPStream, func(), error) {
	if f.launchErr != nil {
		return MCPStream{}, nil, f.launchErr
	}
	// Two pipe pairs: inR/inW  (client writes → we read)
	// outR/outW (we write → client reads)
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	stream := MCPStream{In: inW, Out: outR}
	stop := func() { _ = inR.Close(); _ = outW.Close() }

	// Run the fake server loop.
	go f.run(inR, outW)

	return stream, stop, nil
}

func (f *fakeLauncher) run(in io.ReadCloser, out io.WriteCloser) {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		f.appendCall(msg.Method)
		if msg.ID == nil {
			continue // notification
		}
		switch msg.Method {
		case "initialize":
			if f.initErr != "" {
				writeErr(out, *msg.ID, f.initErr)
				continue
			}
			writeResult(out, *msg.ID,
				`{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"fake","version":"0.0"}}`)
		case "tools/list":
			if f.listErr != "" {
				writeErr(out, *msg.ID, f.listErr)
				continue
			}
			raw, _ := json.Marshal(MCPToolsListResult{Tools: f.tools})
			writeResultRaw(out, *msg.ID, raw)
		case "tools/call":
			if f.toolError != "" {
				writeErr(out, *msg.ID, f.toolError)
				continue
			}
			raw, _ := json.Marshal(f.toolResult)
			writeResultRaw(out, *msg.ID, raw)
		default:
			writeErr(out, *msg.ID, "unknown method")
		}
	}
}

func writeResult(w io.Writer, id int64, result string) {
	_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+formatInt64(id)+`,"result":`+result+"}\n")
}
func writeResultRaw(w io.Writer, id int64, result json.RawMessage) {
	_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+formatInt64(id)+`,"result":`+string(result)+"}\n")
}
func writeErr(w io.Writer, id int64, msg string) {
	_, _ = io.WriteString(w,
		`{"jsonrpc":"2.0","id":`+formatInt64(id)+`,"error":{"code":-1,"message":"`+msg+`"}}`+"\n")
}

// waitFor polls a predicate for up to d, stepping every 10ms. Returns
// true if pred ever returned true. Used to wait on async MCP launch.
func waitFor(d time.Duration, pred func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if pred() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return pred()
}

func TestMCPManager_Launch_RegistersTools(t *testing.T) {
	launcher := &fakeLauncher{
		tools: []MCPToolDescriptor{
			{Name: "read_file", Description: "reads a file", InputSchema: json.RawMessage(`{"type":"object"}`)},
			{Name: "write_note", Description: "writes a note", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	}
	reg := tools.NewRegistry()
	mgr := NewMCPManager(reg, launcher)

	mgr.Launch(context.Background(), MCPServerConfig{Name: "demo", Command: "does-not-matter"})

	if !waitFor(2*time.Second, func() bool {
		s := mgr.Status()
		return len(s) == 1 && s[0].State == MCPStateConnected
	}) {
		t.Fatalf("server never reached connected; status=%+v", mgr.Status())
	}

	got := reg.Names()
	want := map[string]bool{
		"mcp/demo:read_file": false,
		"mcp/demo:write_note": false,
	}
	for _, n := range got {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, saw := range want {
		if !saw {
			t.Errorf("expected tool %q in registry; got %v", n, got)
		}
	}

	status := mgr.Status()
	if status[0].ToolCount != 2 {
		t.Errorf("ToolCount = %d, want 2", status[0].ToolCount)
	}
}

func TestMCPManager_Launch_InitError_StateFailed(t *testing.T) {
	launcher := &fakeLauncher{initErr: "nope"}
	mgr := NewMCPManager(tools.NewRegistry(), launcher)
	mgr.Launch(context.Background(), MCPServerConfig{Name: "demo", Command: "x"})

	if !waitFor(2*time.Second, func() bool {
		s := mgr.Status()
		return len(s) == 1 && s[0].State == MCPStateFailed
	}) {
		t.Fatalf("server never failed: %+v", mgr.Status())
	}
	if !strings.Contains(mgr.Status()[0].LastError, "initialize") {
		t.Errorf("LastError should mention initialize: %q", mgr.Status()[0].LastError)
	}
}

func TestMCPManager_Launch_LaunchError(t *testing.T) {
	launcher := &fakeLauncher{launchErr: errors.New("exec failed")}
	mgr := NewMCPManager(tools.NewRegistry(), launcher)
	mgr.Launch(context.Background(), MCPServerConfig{Name: "demo", Command: "x"})

	if !waitFor(2*time.Second, func() bool {
		s := mgr.Status()
		return len(s) == 1 && s[0].State == MCPStateFailed
	}) {
		t.Fatalf("server never failed: %+v", mgr.Status())
	}
}

func TestMCPTool_Execute_Success(t *testing.T) {
	launcher := &fakeLauncher{
		tools: []MCPToolDescriptor{
			{Name: "echo", Description: "echo", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
		toolResult: MCPToolsCallResult{
			Content: []MCPContentBlock{{Type: "text", Text: "pong"}},
		},
	}
	reg := tools.NewRegistry()
	mgr := NewMCPManager(reg, launcher)
	mgr.Launch(context.Background(), MCPServerConfig{Name: "demo", Command: "x"})

	if !waitFor(2*time.Second, func() bool {
		return reg.Get("mcp/demo:echo") != nil
	}) {
		t.Fatal("tool never registered")
	}
	tool := reg.Get("mcp/demo:echo")
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"msg":"hi"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != tools.StatusSuccess || res.Output != "pong" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestMCPTool_Execute_IsErrorReportsError(t *testing.T) {
	launcher := &fakeLauncher{
		tools: []MCPToolDescriptor{
			{Name: "broken", InputSchema: json.RawMessage(`{}`)},
		},
		toolResult: MCPToolsCallResult{
			Content: []MCPContentBlock{{Type: "text", Text: "something broke"}},
			IsError: true,
		},
	}
	reg := tools.NewRegistry()
	mgr := NewMCPManager(reg, launcher)
	mgr.Launch(context.Background(), MCPServerConfig{Name: "demo", Command: "x"})
	if !waitFor(2*time.Second, func() bool { return reg.Get("mcp/demo:broken") != nil }) {
		t.Fatal("tool never registered")
	}

	res, _ := reg.Get("mcp/demo:broken").Execute(context.Background(), json.RawMessage(`{}`))
	if res.Status != tools.StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(res.Error, "something broke") {
		t.Errorf("Error missing content: %q", res.Error)
	}
}

func TestFlattenMCPContent(t *testing.T) {
	cases := []struct {
		blocks []MCPContentBlock
		want   string
	}{
		{nil, "(no content)"},
		{[]MCPContentBlock{{Type: "text", Text: "hi"}}, "hi"},
		{[]MCPContentBlock{{Type: "text", Text: "a"}, {Type: "text", Text: "b"}}, "a\nb"},
		{[]MCPContentBlock{{Type: "image", Data: strings.Repeat("A", 2048), MimeType: "image/png"}}, "[image: image/png, 2 KB base64]"},
		{[]MCPContentBlock{{Type: "resource", MimeType: "application/json"}}, "[resource: application/json]"},
		{[]MCPContentBlock{{Type: "other"}}, "[other block]"},
	}
	for _, c := range cases {
		got := flattenMCPContent(c.blocks)
		if got != c.want {
			t.Errorf("flattenMCPContent(%+v) = %q, want %q", c.blocks, got, c.want)
		}
	}
}

// TestApp_MCPCommand_StatusWithoutManager is a sanity check that the
// /mcp command degrades gracefully when no MCPManager is wired.
func TestApp_MCPCommand_StatusWithoutManager(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "mcp", CommandArgs: "status"})
	b := drainCmdAny(cmd).(BannerMsg)
	if !strings.Contains(b.Text, "not wired") {
		t.Errorf("banner should announce missing manager: %q", b.Text)
	}
}

func TestApp_MCPCommand_Status_Wired(t *testing.T) {
	launcher := &fakeLauncher{tools: []MCPToolDescriptor{{Name: "x", InputSchema: json.RawMessage(`{}`)}}}
	mgr := NewMCPManager(tools.NewRegistry(), launcher)
	mgr.Launch(context.Background(), MCPServerConfig{Name: "demo", Command: "x"})
	if !waitFor(2*time.Second, func() bool {
		s := mgr.Status()
		return len(s) == 1 && s[0].State == MCPStateConnected
	}) {
		t.Fatal("server never connected")
	}

	app := NewApp(AppOptions{MCP: mgr})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "mcp", CommandArgs: "status"})
	loaded := drainCmdAny(cmd).(MCPStatusLoadedMsg)
	if len(loaded.Servers) != 1 {
		t.Fatalf("Servers = %d", len(loaded.Servers))
	}
	_, bc := app.Update(loaded)
	b := drainCmdAny(bc).(BannerMsg)
	for _, want := range []string{"demo", "connected", "1 tools"} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("banner missing %q: %q", want, b.Text)
		}
	}
}

func TestApp_MCPCommand_Reconnect_UnknownServer(t *testing.T) {
	launcher := &fakeLauncher{}
	mgr := NewMCPManager(tools.NewRegistry(), launcher)
	app := NewApp(AppOptions{MCP: mgr})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "mcp", CommandArgs: "reconnect ghost"})
	msg := drainCmdAny(cmd)
	e, ok := msg.(MCPErrMsg)
	if !ok {
		t.Fatalf("expected MCPErrMsg, got %T", msg)
	}
	if !strings.Contains(e.Err.Error(), "unknown") {
		t.Errorf("Err = %v", e.Err)
	}
}

// TestFilteredEnv_StripsCredentials pins WU-094 H-6: MCP subprocesses
// must not inherit the full parent env. API keys and credentials
// get stripped; a minimal allowlist (PATH, HOME, etc.) + LC_* is
// forwarded. Anything the user explicitly adds via MCPServerConfig.Env
// is passed through downstream.
func TestFilteredEnv_StripsCredentials(t *testing.T) {
	got := filteredEnv([]string{
		"PATH=/usr/bin",
		"HOME=/home/u",
		"LC_TIME=en_US",
		"ANTHROPIC_API_KEY=sk-ant-xxx",
		"OPENAI_API_KEY=sk-xxx",
		"AWS_SECRET_ACCESS_KEY=zzz",
		"GITHUB_TOKEN=ghp_xxx",
		"MODELTAP_SECRET=y",
		"SOME_API_TOKEN=z",
		"DATABASE_PASSWORD=p",
		"MY_CUSTOM_CREDENTIAL=c",
		"SSH_AUTH_SOCK=/tmp/s",
		"RANDOM_VAR=value", // not allowed, not sensitive — still stripped
	})
	want := map[string]bool{
		"PATH=/usr/bin":     false,
		"HOME=/home/u":      false,
		"LC_TIME=en_US":     false,
	}
	for _, e := range got {
		if _, ok := want[e]; ok {
			want[e] = true
			continue
		}
		t.Errorf("filteredEnv leaked %q", e)
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("filteredEnv dropped expected %q", k)
		}
	}
}

func TestIsSensitiveEnvName(t *testing.T) {
	sensitive := []string{
		"ANTHROPIC_API_KEY", "anthropic_api_key",
		"OPENAI_KEY", "AWS_SECRET_ACCESS_KEY",
		"GITHUB_TOKEN", "MY_SECRET", "CUSTOM_CREDENTIAL",
		"DATABASE_PASSWORD", "ssh_auth_sock",
	}
	for _, n := range sensitive {
		if !isSensitiveEnvName(n) {
			t.Errorf("%q should be sensitive", n)
		}
	}
	safe := []string{
		"PATH", "HOME", "LANG", "TERM", "LC_TIME", "MY_VAR", "CONFIG_DIR",
	}
	for _, n := range safe {
		if isSensitiveEnvName(n) {
			t.Errorf("%q should not be sensitive", n)
		}
	}
}

func TestApp_MCPCommand_UsageWhenMissingName(t *testing.T) {
	mgr := NewMCPManager(tools.NewRegistry(), &fakeLauncher{})
	app := NewApp(AppOptions{MCP: mgr})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "mcp", CommandArgs: "reconnect"})
	b := drainCmdAny(cmd).(BannerMsg)
	if !strings.Contains(b.Text, "Usage") {
		t.Errorf("banner should show usage: %q", b.Text)
	}
}
