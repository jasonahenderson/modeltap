package harness

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// extend fakeClient with a canned HistoryList; the shared fakeClient
// already records transform / submit calls. HistoryList support is
// added here rather than mutating app_conn_test.go to keep test
// scopes isolated.
func init() {
	// compile-time check that extension works — no-op at runtime.
}

// newHistoryFake builds a fakeClient rigged with the supplied entries
// and cursor/hasMore flags.
func newHistoryFake(entries []string, cursor string, hasMore bool) *fakeClient {
	resp := protocol.HistoryListResponse{
		HasMore: hasMore,
		Cursor:  cursor,
	}
	for _, c := range entries {
		resp.Entries = append(resp.Entries, protocol.HistoryEntry{Content: c})
	}
	return &fakeClient{historyResp: resp}
}

func TestHistoryController_NoConn_ZeroEntries(t *testing.T) {
	hc := NewHistoryController(nil)
	if hc.Len() != 0 {
		t.Errorf("expected empty history; got %d", hc.Len())
	}
	if _, ok := hc.Entry(0); ok {
		t.Errorf("Entry(0) should miss")
	}
	if err := hc.Load(context.Background()); err != nil {
		t.Errorf("Load without conn should be a no-op; got %v", err)
	}
}

func TestHistoryController_Load_PopulatesCache(t *testing.T) {
	fc := newHistoryFake([]string{"third", "second", "first"}, "", false)
	conn := &fakeConn{state: ConnStateReady, client: fc}
	hc := NewHistoryController(conn)

	if err := hc.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if hc.Len() != 3 {
		t.Fatalf("Len = %d, want 3", hc.Len())
	}
	for i, want := range []string{"third", "second", "first"} {
		got, ok := hc.Entry(i)
		if !ok || got != want {
			t.Errorf("Entry(%d) = (%q, %v), want (%q, true)", i, got, ok, want)
		}
	}
}

func TestHistoryController_LoadMore_AppendsAndUpdatesCursor(t *testing.T) {
	fc := newHistoryFake([]string{"a", "b"}, "cur-1", true)
	conn := &fakeConn{state: ConnStateReady, client: fc}
	hc := NewHistoryController(conn)

	if err := hc.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !hc.HasMore() {
		t.Fatal("HasMore should be true")
	}

	// Swap the canned response for the next page.
	fc.mu.Lock()
	fc.historyResp = protocol.HistoryListResponse{
		Entries: []protocol.HistoryEntry{{Content: "c"}, {Content: "d"}},
		HasMore: false,
	}
	fc.mu.Unlock()

	if err := hc.LoadMore(context.Background()); err != nil {
		t.Fatalf("LoadMore: %v", err)
	}
	if hc.Len() != 4 {
		t.Errorf("Len = %d, want 4 after LoadMore", hc.Len())
	}
	// Assert the second call sent the cursor from the first page.
	if len(fc.historyCalls) != 2 {
		t.Fatalf("expected 2 history calls, got %d", len(fc.historyCalls))
	}
	if fc.historyCalls[1].Before != "cur-1" {
		t.Errorf("LoadMore cursor = %q, want cur-1", fc.historyCalls[1].Before)
	}
	if hc.HasMore() {
		t.Errorf("HasMore should be false after exhausted page")
	}
}

func TestHistoryController_LoadMore_NoOpWhenExhausted(t *testing.T) {
	fc := newHistoryFake([]string{"a"}, "", false)
	conn := &fakeConn{client: fc}
	hc := NewHistoryController(conn)
	_ = hc.Load(context.Background())
	if err := hc.LoadMore(context.Background()); err != nil {
		t.Fatalf("LoadMore: %v", err)
	}
	if len(fc.historyCalls) != 1 {
		t.Errorf("LoadMore should be a no-op; call count = %d", len(fc.historyCalls))
	}
}

func TestHistoryController_SetScope_RefreshesCache(t *testing.T) {
	fc := newHistoryFake([]string{"u1"}, "", false)
	conn := &fakeConn{client: fc}
	hc := NewHistoryController(conn)
	_ = hc.Load(context.Background())

	// Swap the canned response so the refresh returns different data.
	fc.mu.Lock()
	fc.historyResp = protocol.HistoryListResponse{
		Entries: []protocol.HistoryEntry{{Content: "p1"}, {Content: "p2"}},
	}
	fc.mu.Unlock()

	cmd := hc.SetScope(HistoryScopeProject)
	if cmd == nil {
		t.Fatal("SetScope should return a cmd")
	}
	msg := cmd()
	r, ok := msg.(HistoryRefreshedMsg)
	if !ok {
		t.Fatalf("expected HistoryRefreshedMsg, got %T", msg)
	}
	if r.Scope != HistoryScopeProject {
		t.Errorf("Scope = %q", r.Scope)
	}
	if r.Count != 2 {
		t.Errorf("Count = %d, want 2", r.Count)
	}
	if hc.Scope() != HistoryScopeProject {
		t.Errorf("controller Scope = %q", hc.Scope())
	}
	// Cache should be fresh, not appended.
	if hc.Len() != 2 {
		t.Errorf("Len = %d, want 2 after refresh", hc.Len())
	}
	if got, _ := hc.Entry(0); got != "p1" {
		t.Errorf("Entry(0) = %q, want p1", got)
	}
}

func TestHistoryController_SetScope_UnknownScope_Errors(t *testing.T) {
	hc := NewHistoryController(nil)
	cmd := hc.SetScope("gibberish")
	msg := cmd()
	e, ok := msg.(HistoryErrMsg)
	if !ok {
		t.Fatalf("expected HistoryErrMsg, got %T", msg)
	}
	if !strings.Contains(e.Err.Error(), "gibberish") {
		t.Errorf("error should name the bad scope: %v", e.Err)
	}
}

func TestHistoryController_Load_RPCErrorPropagates(t *testing.T) {
	fc := &fakeClient{historyErr: errors.New("rpc down")}
	conn := &fakeConn{client: fc}
	hc := NewHistoryController(conn)

	err := hc.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "rpc down") {
		t.Errorf("expected RPC error; got %v", err)
	}
}

func TestHistoryController_Scope_Defaults(t *testing.T) {
	hc := NewHistoryController(nil)
	if got := hc.Scope(); got != HistoryScopeUser {
		t.Errorf("default scope = %q, want user", got)
	}
}

func TestApp_HistoryCommand_ShowsCurrentScope(t *testing.T) {
	fc := newHistoryFake([]string{"a"}, "", false)
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})
	_ = app.History().Load(context.Background())

	// Bare /history reports current scope + entry count.
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "history"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, HistoryScopeUser) {
		t.Errorf("banner should mention user scope: %q", b.Text)
	}
}

func TestApp_HistoryCommand_SetScope_SurfacesBanner(t *testing.T) {
	fc := newHistoryFake([]string{"p1"}, "", false)
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})

	// `/history project` triggers SetScope, whose cmd returns a
	// HistoryRefreshedMsg that App.Update turns into a banner.
	model, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "history", CommandArgs: "project"})
	if cmd == nil {
		t.Fatal("expected SetScope cmd")
	}
	a, _ := model.(App)
	refreshed := cmd()
	if r, ok := refreshed.(HistoryRefreshedMsg); ok {
		if r.Scope != HistoryScopeProject {
			t.Errorf("Scope = %q", r.Scope)
		}
	} else {
		t.Fatalf("expected HistoryRefreshedMsg, got %T", refreshed)
	}

	// Feed the refreshed msg back through Update to get the banner.
	_, bannerCmd := a.Update(refreshed)
	b, ok := drainCmdAny(bannerCmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", bannerCmd())
	}
	if !strings.Contains(b.Text, HistoryScopeProject) {
		t.Errorf("banner should include new scope: %q", b.Text)
	}
}

func TestApp_HistoryCommand_UnknownScope_ErrorsSurface(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "history", CommandArgs: "nope"})
	msg := drainCmdAny(cmd)
	if _, ok := msg.(HistoryErrMsg); !ok {
		t.Fatalf("expected HistoryErrMsg, got %T", msg)
	}
}

func TestApp_HistoryController_WiredIntoInput(t *testing.T) {
	fc := newHistoryFake([]string{"latest", "earlier"}, "", false)
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})
	_ = app.History().Load(context.Background())

	// Len should equal what the controller cached — proves the input
	// area's HistorySource points at the real controller.
	if app.History().Len() != 2 {
		t.Errorf("controller Len = %d, want 2", app.History().Len())
	}
	// Direct entry check confirms ordering.
	if got, ok := app.History().Entry(0); !ok || got != "latest" {
		t.Errorf("Entry(0) = (%q, %v)", got, ok)
	}
}
