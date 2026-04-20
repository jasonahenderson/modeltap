package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// recentTurnsKeepFloor is the floor of turns the default heuristic
// always keeps verbatim. Last-three-turns-pinned covers the typical
// "just asked a question" / "in the middle of fixing bug X" cases
// where losing immediate context would be confusing.
const recentTurnsKeepFloor = 3

// Compaction category names. The FEAT-0009 spec calls out semantic
// buckets like "Debugging session" / "Test iteration" but deriving
// those requires topic-modeling. v1 uses structural buckets instead:
// age + role. Future heuristics can slot in by implementing their
// own bucket fn.
const (
	CompactCategoryOlder      = "older_turns"
	CompactCategoryToolOutput = "tool_results"
	CompactCategoryRecent     = "recent"
)

// Compaction action names — mirror the suggested_action / action
// enum specified in protocol.CompactCategory.
const (
	CompactActionKeep      = "keep"
	CompactActionSummarize = "summarize"
	CompactActionDrop      = "drop"
	CompactActionPin       = "pin"
)

// buildCompactPlan analyses a session's turns and returns a plan
// with categories + suggested actions. Called from handleSessionCompact.
//
// v1 heuristic:
//   - last N turns (N = recentTurnsKeepFloor): "recent", suggested keep.
//   - older turns in role=tool_result: "tool_results", suggested drop.
//   - older turns in role=user/assistant: "older_turns", suggested summarize.
//   - fewer than (N+1) turns total: empty plan (nothing to gain).
//
// The heuristic is deliberately shallow. Real topic-aware bucketing
// is a follow-up once we have signal on what users actually want
// preserved.
func buildCompactPlan(turns []storage.Turn, contextWindow int) *protocol.CompactPlan {
	if len(turns) <= recentTurnsKeepFloor {
		return &protocol.CompactPlan{}
	}

	// Bucket turns. "recent" protects the tail regardless of role.
	recentStart := len(turns) - recentTurnsKeepFloor
	var recent, older, toolOutputs []storage.Turn
	for i, t := range turns {
		if i >= recentStart {
			recent = append(recent, t)
			continue
		}
		if t.Role == "tool" {
			toolOutputs = append(toolOutputs, t)
			continue
		}
		older = append(older, t)
	}

	totalBefore := sumTurnTokens(turns)
	contextPctBefore := float64(totalBefore) / float64(contextWindowOr(contextWindow))

	var categories []protocol.CompactCategory
	if n := len(older); n > 0 {
		tokens := sumTurnTokens(older)
		categories = append(categories, protocol.CompactCategory{
			Name:            CompactCategoryOlder,
			TokenCount:      tokens,
			ValueScore:      0.35,
			SuggestedAction: CompactActionSummarize,
			SummaryPreview:  summaryPreview(older),
		})
	}
	if n := len(toolOutputs); n > 0 {
		tokens := sumTurnTokens(toolOutputs)
		categories = append(categories, protocol.CompactCategory{
			Name:            CompactCategoryToolOutput,
			TokenCount:      tokens,
			ValueScore:      0.15,
			SuggestedAction: CompactActionDrop,
		})
	}
	if n := len(recent); n > 0 {
		tokens := sumTurnTokens(recent)
		categories = append(categories, protocol.CompactCategory{
			Name:            CompactCategoryRecent,
			TokenCount:      tokens,
			ValueScore:      0.95,
			SuggestedAction: CompactActionKeep,
		})
	}

	estimatedFreed := 0
	for _, c := range categories {
		switch c.SuggestedAction {
		case CompactActionDrop:
			estimatedFreed += c.TokenCount
		case CompactActionSummarize:
			// Summaries land around 10% of source tokens; the real
			// ratio depends on the summarizer we eventually wire.
			estimatedFreed += int(0.9 * float64(c.TokenCount))
		}
	}
	contextPctAfter := float64(totalBefore-estimatedFreed) / float64(contextWindowOr(contextWindow))
	if contextPctAfter < 0 {
		contextPctAfter = 0
	}

	return &protocol.CompactPlan{
		Categories:           categories,
		EstimatedTokensFreed: estimatedFreed,
		ContextPctBefore:     contextPctBefore,
		ContextPctAfter:      contextPctAfter,
	}
}

// applyCompactPlan mutates the persisted turns of a session per the
// user's actions map. Returns how many tokens were freed + a
// single-line human summary. Actions missing from the map default to
// the category's suggested action (best-effort). Categories absent
// from the current plan (stale client or rename) are ignored.
func applyCompactPlan(ctx context.Context, store storage.Store, sessionID string, actions map[string]string, contextWindow int) (*protocol.CompactApplyResponse, error) {
	turns, err := store.ListTurns(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load turns: %w", err)
	}
	if len(turns) <= recentTurnsKeepFloor {
		return &protocol.CompactApplyResponse{Applied: false, Summary: "nothing to compact"}, nil
	}

	recentStart := len(turns) - recentTurnsKeepFloor
	var older, toolOutputs []storage.Turn
	for i, t := range turns {
		if i >= recentStart {
			continue
		}
		if t.Role == "tool" {
			toolOutputs = append(toolOutputs, t)
		} else {
			older = append(older, t)
		}
	}

	plan := buildCompactPlan(turns, contextWindow)
	merged := map[string]string{}
	for _, c := range plan.Categories {
		merged[c.Name] = c.SuggestedAction
	}
	for k, v := range actions {
		merged[k] = v
	}

	freed := 0
	var summaryLines []string

	// Older-turn summarization.
	if act := merged[CompactCategoryOlder]; act != "" && len(older) > 0 {
		switch act {
		case CompactActionSummarize:
			placeholder := summarizeTurns(older)
			seq := older[0].Sequence
			originals := make([]int, 0, len(older))
			for _, t := range older {
				originals = append(originals, t.Sequence)
			}
			if err := replaceTurnsWithSummary(ctx, store, sessionID, older, seq, placeholder, originals); err != nil {
				return nil, err
			}
			before := sumTurnTokens(older)
			freed += int(0.9 * float64(before))
			summaryLines = append(summaryLines, fmt.Sprintf("summarized %d older turns", len(older)))
		case CompactActionDrop:
			if err := deleteTurns(ctx, store, older); err != nil {
				return nil, err
			}
			freed += sumTurnTokens(older)
			summaryLines = append(summaryLines, fmt.Sprintf("dropped %d older turns", len(older)))
		}
	}

	// Tool-output pass is always drop-or-keep; summarize is skipped
	// for v1 because tool output text is noisy to summarize.
	if act := merged[CompactCategoryToolOutput]; act == CompactActionDrop && len(toolOutputs) > 0 {
		if err := deleteTurns(ctx, store, toolOutputs); err != nil {
			return nil, err
		}
		freed += sumTurnTokens(toolOutputs)
		summaryLines = append(summaryLines, fmt.Sprintf("dropped %d tool outputs", len(toolOutputs)))
	}

	// Record a server-side session event so future session.details
	// calls surface the manual-compact entry. FreedTokens rides on
	// Payload to match the storage schema (v2 uses a JSON blob for
	// event-specific fields; see storage design doc §"session_events").
	payload, _ := json.Marshal(map[string]any{"freed_tokens": freed})
	_ = store.AppendServerEvent(ctx, &storage.ServerSessionEvent{
		SessionID: sessionID,
		Type:      "manual_compact",
		Detail:    strings.Join(summaryLines, "; "),
		Payload:   payload,
		At:        time.Now().UTC(),
	})

	remainingTurns, _ := store.ListTurns(ctx, sessionID)
	remainingTokens := sumTurnTokens(remainingTurns)
	pctAfter := float64(remainingTokens) / float64(contextWindowOr(contextWindow))

	summary := strings.Join(summaryLines, "; ")
	if summary == "" {
		summary = "no-op (all categories kept)"
	}
	return &protocol.CompactApplyResponse{
		Applied:         freed > 0,
		TokensFreed:     freed,
		ContextPctAfter: pctAfter,
		Summary:         summary,
	}, nil
}

// handleSessionCompact implements method session.compact. Returns a
// CompactPlan the client renders; the client then issues compact.apply
// with the action map it wants to run.
func handleSessionCompact(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.SessionCompact
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode session.compact", wrapped: err}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}

	sess, err := conn.server.store.GetSession(ctx, req.SessionID)
	if err != nil || sess == nil {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: "session not found"}
	}
	if err := verifySessionAccess(conn, sess); err != nil {
		return nil, err
	}
	turns, err := conn.server.store.ListTurns(ctx, req.SessionID)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "list turns: " + err.Error()}
	}
	// Context window from active session's model, or a sane default.
	window := contextWindowForSession(conn.server, req.SessionID)
	return buildCompactPlan(turns, window), nil
}

// handleCompactApply implements method compact.apply. Mutates turns
// according to the action map, records a manual_compact session
// event, returns a summary.
func handleCompactApply(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.CompactApply
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode compact.apply", wrapped: err}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}
	sess, err := conn.server.store.GetSession(ctx, req.SessionID)
	if err != nil || sess == nil {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: "session not found"}
	}
	if err := verifySessionAccess(conn, sess); err != nil {
		return nil, err
	}
	window := contextWindowForSession(conn.server, req.SessionID)
	return applyCompactPlan(ctx, conn.server.store, req.SessionID, req.Actions, window)
}

// replaceTurnsWithSummary inserts a compacted summary row at
// insertSequence, copies original-turn sequences onto it, and
// deletes the originals in a single best-effort pass. On storage
// errors the caller sees a failure; partial writes are possible but
// rare (and the manual_compact session event log records what
// happened for postmortem).
func replaceTurnsWithSummary(ctx context.Context, store storage.Store, sessionID string, originals []storage.Turn, insertSequence int, summary string, originalSeqs []int) error {
	payload, _ := json.Marshal(provider.Message{
		Role:    "assistant",
		Content: summary,
	})
	summaryTurn := &storage.Turn{
		ID:               "compact-" + sessionID + "-" + fmt.Sprintf("%d", insertSequence),
		SessionID:        sessionID,
		Sequence:         insertSequence,
		Role:             "assistant",
		Content:          payload,
		Compacted:        true,
		CompactedSummary: summary,
		OriginalTurns:    originalSeqs,
		CreatedAt:        time.Now().UTC(),
	}

	// Delete originals first so the sequence slot is free; then
	// insert the summary. If the store does transactional multi-write
	// later, wrap this block.
	if err := deleteTurns(ctx, store, originals); err != nil {
		return err
	}
	if err := store.CreateTurn(ctx, summaryTurn); err != nil {
		return fmt.Errorf("insert summary turn: %w", err)
	}
	return nil
}

func deleteTurns(ctx context.Context, store storage.Store, turns []storage.Turn) error {
	for _, t := range turns {
		if err := store.DeleteTurn(ctx, t.SessionID, t.ID); err != nil {
			return fmt.Errorf("delete turn %s: %w", t.ID, err)
		}
	}
	return nil
}

// summarizeTurns is v1's placeholder summarizer: concatenate the
// first ~160 chars of every turn's text content, joined with
// newlines, and truncate the whole thing at 800 chars so the
// summary turn doesn't rival the originals in length. Real
// summarization via content.transform is a follow-up — wiring it
// here would require a model + streaming relay, which deserves its
// own scope.
func summarizeTurns(turns []storage.Turn) string {
	var parts []string
	for _, t := range turns {
		text := firstText(t.Content)
		if text == "" {
			continue
		}
		if len(text) > 160 {
			text = text[:160] + "…"
		}
		parts = append(parts, "- "+text)
	}
	out := strings.Join(parts, "\n")
	if len(out) > 800 {
		out = out[:800] + "\n…(truncated)"
	}
	if out == "" {
		return "[empty summary]"
	}
	return "Compacted summary of earlier turns:\n" + out
}

// summaryPreview is the short teaser shown in the plan UI before the
// user approves. Uses the same truncation as summarizeTurns but caps
// earlier so the banner stays compact.
func summaryPreview(turns []storage.Turn) string {
	if len(turns) == 0 {
		return ""
	}
	text := firstText(turns[0].Content)
	if len(text) > 120 {
		text = text[:120] + "…"
	}
	return text
}

// firstText pulls the text body from a canonical provider.Message
// JSON payload. Missing / malformed payloads return "" — the caller
// decides how to surface that. provider.Message.Content is a string
// in the v0.2 canonical shape; this helper exists so the compaction
// layer doesn't have to know that detail.
func firstText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var msg provider.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ""
	}
	return msg.Content
}

// sumTurnTokens adds up InputTokens + OutputTokens across turns.
// Handles the common case of zero values on legacy turns by falling
// back to a content-length heuristic: 1 token ≈ 4 bytes of text.
func sumTurnTokens(turns []storage.Turn) int {
	total := 0
	for _, t := range turns {
		declared := int(t.InputTokens + t.OutputTokens)
		if declared > 0 {
			total += declared
			continue
		}
		// Fallback heuristic so fresh sessions without token counts
		// still produce a non-trivial plan.
		total += len(t.Content) / 4
	}
	return total
}

// contextWindowOr returns window when positive, otherwise a safe
// default (200k matches Claude Sonnet 4+). Used to compute
// context_pct_before / after when the session's active model isn't
// resolvable (early sessions, provider registry not loaded yet).
func contextWindowOr(window int) int {
	if window > 0 {
		return window
	}
	return 200_000
}

// contextWindowForSession looks up the context window of the active
// session's effective model. Prefers the session-level ModelOverride
// when set; otherwise asks the routing policy for the default.
// Returns 0 when nothing resolves; the plan builder falls back to a
// safe default in that case.
func contextWindowForSession(srv *Server, sessionID string) int {
	if srv == nil || srv.sessions == nil || srv.models == nil {
		return 0
	}
	name := ""
	if active := srv.sessions.GetActiveSession(sessionID); active != nil {
		name = active.ModelOverride
	}
	if name == "" && srv.routing != nil {
		if models, _, found := srv.routing.Resolve("default"); found && len(models) > 0 {
			name = models[0]
		}
	}
	if name == "" {
		return 0
	}
	if entry := srv.models.Get(name); entry != nil {
		return entry.Info.ContextWindow
	}
	return 0
}
