package harness

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// History scope constants — subset of FEAT-0009 taxonomy the harness
// surfaces via `/history <scope>`. See also internal/protocol.
const (
	HistoryScopeUser    = "user"
	HistoryScopeProject = "project"
	HistoryScopeSession = "session"
)

// defaultHistoryLoadLimit is the page size used by the initial Load()
// call. 200 matches the Bundle 13 D8 spec; callers that want more
// entries page through via loadMore once the cache is exhausted.
const defaultHistoryLoadLimit = 200

// HistoryController implements HistorySource (from WU-070) backed by
// the BFF's history.list handler (WU-091). Entries are cached newest-
// first so Entry(0) returns the most recent command. When the user
// arrows past the cached entries, the controller asynchronously pages
// in the next window via the connection's HistoryList call.
//
// The controller is safe for concurrent use between the Bubbletea
// update goroutine (Entry / Len) and background paging goroutines
// (Load / SetScope / loadMore).
type HistoryController struct {
	conn ConnSurface

	mu      sync.RWMutex
	scope   string
	entries []string // newest-first; index 0 = most recent
	cursor  string
	hasMore bool
}

// NewHistoryController returns a controller rooted at conn and the
// default "user" scope. conn may be nil — the controller degrades to
// an empty source (Len == 0) so arrow-up is a no-op rather than a
// crash when the harness is launched without a wired manager.
func NewHistoryController(conn ConnSurface) *HistoryController {
	return &HistoryController{conn: conn, scope: HistoryScopeUser}
}

// Entry returns the cached entry at index (0 = most recent). Returns
// ("", false) when index is out of range.
func (hc *HistoryController) Entry(index int) (string, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	if index < 0 || index >= len(hc.entries) {
		return "", false
	}
	return hc.entries[index], true
}

// Len returns the count of currently-cached entries.
func (hc *HistoryController) Len() int {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return len(hc.entries)
}

// Scope returns the current history scope.
func (hc *HistoryController) Scope() string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.scope
}

// HasMore reports whether a paginated follow-up request would return
// additional entries. Used by the InputArea to decide whether to
// trigger a LoadMore when the user arrows past the cached end.
func (hc *HistoryController) HasMore() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.hasMore
}

// Load fetches the first page of entries at the current scope.
// Returns an error only when a connection exists and the RPC fails;
// no-op (and nil error) when no conn is wired.
func (hc *HistoryController) Load(ctx context.Context) error {
	return hc.fetch(ctx, "")
}

// LoadMore pages in the next window using the cursor retained from
// the previous page. No-op when HasMore() is false or no conn is
// wired.
func (hc *HistoryController) LoadMore(ctx context.Context) error {
	hc.mu.RLock()
	cursor := hc.cursor
	hasMore := hc.hasMore
	hc.mu.RUnlock()
	if !hasMore {
		return nil
	}
	return hc.fetch(ctx, cursor)
}

// SetScope changes the scope and returns a tea.Cmd that refreshes the
// cache. Safe to call from a Bubbletea Update handler; the returned
// Cmd emits a HistoryRefreshedMsg with the new scope on success, or a
// HistoryErrMsg on failure.
func (hc *HistoryController) SetScope(scope string) tea.Cmd {
	return func() tea.Msg {
		if scope != HistoryScopeUser && scope != HistoryScopeProject && scope != HistoryScopeSession {
			return HistoryErrMsg{Scope: scope, Err: fmt.Errorf("unknown scope %q", scope)}
		}
		hc.mu.Lock()
		hc.scope = scope
		hc.entries = nil
		hc.cursor = ""
		hc.hasMore = false
		hc.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := hc.Load(ctx); err != nil {
			return HistoryErrMsg{Scope: scope, Err: err}
		}
		return HistoryRefreshedMsg{Scope: scope, Count: hc.Len()}
	}
}

// HistoryRefreshedMsg is emitted after a successful scope change or
// Load. The App surfaces a transient banner confirming the refresh.
type HistoryRefreshedMsg struct {
	Scope string
	Count int
}

// HistoryErrMsg is emitted when a history fetch fails. The App
// surfaces a transient banner with the error.
type HistoryErrMsg struct {
	Scope string
	Err   error
}

// fetch issues one history.list RPC and merges the result into the
// cache. cursor == "" requests the first page (and resets the cache);
// a non-empty cursor appends the next window.
func (hc *HistoryController) fetch(ctx context.Context, cursor string) error {
	if hc.conn == nil {
		return nil
	}
	client := hc.conn.Client()
	if client == nil {
		return nil
	}
	hc.mu.RLock()
	scope := hc.scope
	hc.mu.RUnlock()

	resp, err := client.HistoryList(ctx, &protocol.HistoryList{
		Scope:  scope,
		Limit:  defaultHistoryLoadLimit,
		Before: cursor,
	})
	if err != nil {
		return err
	}
	contents := make([]string, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		contents = append(contents, e.Content)
	}

	hc.mu.Lock()
	if cursor == "" {
		hc.entries = contents
	} else {
		hc.entries = append(hc.entries, contents...)
	}
	hc.cursor = resp.Cursor
	hc.hasMore = resp.HasMore
	hc.mu.Unlock()
	return nil
}
