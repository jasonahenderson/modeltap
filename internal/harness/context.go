package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harness/tools"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// FileAttacher resolves raw @file references (paths and globs captured
// by ExtractFileRefs) into typed protocol.Attachment values ready to
// ride on a turn.submit. Resolution strategy is implementation-
// defined: the default ContextManager uses the harness's own Read /
// Glob tools and so honors the tool framework's project-root scope.
// Tests can inject a fake.
type FileAttacher interface {
	Resolve(ctx context.Context, refs []string) ([]protocol.Attachment, error)
}

// ContextManager is the default FileAttacher. It wires the harness
// Read / Glob tools so @file refs resolve through the same scope rules
// the tool framework enforces (project-root-bounded path resolution,
// magic-byte format detection, image base64 envelope, etc.).
//
// The manager is intentionally stateless — each Resolve call is a
// fresh pass. Per-session attachment bookkeeping (for /drop and
// /context merges) is a follow-up that will layer on top.
type ContextManager struct {
	projectRoot string
	read        *tools.ReadTool
	glob        *tools.GlobTool
}

// NewContextManager constructs a manager rooted at projectRoot. The
// tracker plumbs into ReadTool so subsequent Edit/Write operations
// see the attached files as already-read.
func NewContextManager(projectRoot string, tracker *tools.FileTracker) *ContextManager {
	if tracker == nil {
		tracker = tools.NewFileTracker()
	}
	return &ContextManager{
		projectRoot: projectRoot,
		read:        tools.NewReadTool(projectRoot, tracker),
		glob:        tools.NewGlobTool(projectRoot),
	}
}

// Resolve expands each raw @file ref into zero-or-more
// protocol.Attachment structs. A ref containing '*' / '?' / '[' is
// treated as a glob (expanded via GlobTool); anything else is read as
// a single file. Errors resolving individual refs are surfaced as
// `Err` on the resulting Attachment (path preserved, Content empty)
// rather than failing the whole resolution — a bad attachment
// shouldn't block a turn.
func (m *ContextManager) Resolve(ctx context.Context, refs []string) ([]protocol.Attachment, error) {
	if m == nil || len(refs) == 0 {
		return nil, nil
	}
	out := make([]protocol.Attachment, 0, len(refs))
	for _, ref := range refs {
		if containsGlobMeta(ref) {
			paths, err := m.expandGlob(ctx, ref)
			if err != nil {
				out = append(out, errAttachment(ref, err))
				continue
			}
			for _, p := range paths {
				out = append(out, m.readAttachment(ctx, p))
			}
			continue
		}
		out = append(out, m.readAttachment(ctx, ref))
	}
	return out, nil
}

// readAttachment runs the Read tool on a single path and returns an
// Attachment. Failures become err-laden attachments so the caller
// can see the bad ref without losing the others.
func (m *ContextManager) readAttachment(ctx context.Context, path string) protocol.Attachment {
	input, _ := json.Marshal(map[string]any{"file_path": path})
	res, err := m.read.Execute(ctx, input)
	if err != nil {
		return errAttachment(path, err)
	}
	if res.Status != tools.StatusSuccess {
		return errAttachment(path, errors.New(nonEmpty(res.Error, "read failed")))
	}
	att := protocol.Attachment{
		Path:        path,
		Content:     res.Output,
		ContentType: "text/plain",
		Transform:   "read",
	}
	if res.OutputType == "image" {
		// Read emits "mime\nbase64"; split so Raw carries the base64
		// payload and ContentType captures the MIME type.
		parts := strings.SplitN(res.Output, "\n", 2)
		if len(parts) == 2 {
			att.ContentType = parts[0]
			att.Raw = parts[1]
			att.Content = ""
			att.Transform = "image"
		}
	}
	return att
}

// expandGlob invokes the Glob tool and returns the relative matches.
// The Glob tool's output format is "N match(es):\n<rel>\n..." — we
// strip the header and split the remainder.
func (m *ContextManager) expandGlob(ctx context.Context, pattern string) ([]string, error) {
	input, _ := json.Marshal(map[string]any{"pattern": pattern})
	res, err := m.glob.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	if res.Status != tools.StatusSuccess {
		return nil, errors.New(nonEmpty(res.Error, "glob failed"))
	}
	lines := strings.Split(res.Output, "\n")
	paths := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.Contains(l, "match(es)") || l == "no matches" {
			continue
		}
		paths = append(paths, l)
	}
	return paths, nil
}

func errAttachment(path string, err error) protocol.Attachment {
	return protocol.Attachment{
		Path:        path,
		Content:     fmt.Sprintf("[attachment error: %v]", err),
		ContentType: "text/plain",
		Transform:   "error",
	}
}

// containsGlobMeta reports whether ref uses doublestar-style wildcard
// characters. Keeps the Glob vs Read branch cheap.
func containsGlobMeta(ref string) bool {
	return strings.ContainsAny(ref, "*?[")
}

// ContextListLoadedMsg fires when /context successfully fetches the
// server-side context breakdown. The App surfaces a multi-line
// banner.
type ContextListLoadedMsg struct {
	Response *protocol.ContextListResponse
}

// ContextErrMsg carries a failure from /context.
type ContextErrMsg struct {
	Err error
}

// handleContextCommand dispatches /context (no args → list).
func (a *App) handleContextCommand(msg SubmitMsg) tea.Cmd {
	args := strings.TrimSpace(msg.CommandArgs)
	if args != "" {
		return func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("/context takes no arguments (got %q)", args),
				Duration: 4 * time.Second,
			}
		}
	}
	return a.dispatchContextList()
}

func (a *App) dispatchContextList() tea.Cmd {
	conn := a.conn
	sessionID := a.state.SessionID
	return func() tea.Msg {
		if sessionID == "" {
			return ContextErrMsg{Err: errNoActiveSession}
		}
		if conn == nil {
			return ContextErrMsg{Err: errNoConnection}
		}
		client := conn.Client()
		if client == nil {
			return ContextErrMsg{Err: errNotConnected}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := client.ContextList(ctx, sessionID)
		if err != nil {
			return ContextErrMsg{Err: err}
		}
		return ContextListLoadedMsg{Response: resp}
	}
}

// formatContextList renders a multi-line banner summarizing files +
// knowledge injections + token budget. Keeps rendering out of the
// controller so the App can evolve the surface (dedicated pane,
// sidebar) without rewriting the data path.
func formatContextList(resp *protocol.ContextListResponse) string {
	if resp == nil {
		return "No context data."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Context: %d tokens (%.0f%% of %d) — system prompt %d, injections %d",
		resp.ContextTokens, resp.ContextPct*100, resp.ContextWindow,
		resp.SystemPromptTokens, resp.KnowledgeInjectionTokens)
	if len(resp.Files) > 0 {
		b.WriteString("\nFiles:")
		for _, f := range resp.Files {
			stale := ""
			if f.Stale {
				stale = " [stale]"
			}
			fmt.Fprintf(&b, "\n  %s (%d bytes, turn %d)%s",
				f.Path, f.SizeBytes, f.AttachedTurn, stale)
		}
	}
	if len(resp.KnowledgeInjections) > 0 {
		b.WriteString("\nKnowledge:")
		for _, k := range resp.KnowledgeInjections {
			fmt.Fprintf(&b, "\n  [%.2f] %s (%s)", k.Relevance, k.Summary, k.SourceDate)
		}
	}
	if len(resp.PinnedItems) > 0 {
		fmt.Fprintf(&b, "\nPinned: %s", strings.Join(resp.PinnedItems, ", "))
	}
	return b.String()
}
