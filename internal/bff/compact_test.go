package bff

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// seedTurn inserts a turn with the given role + text content into
// the store. Returns the persisted row.
func seedTurn(t *testing.T, store storage.Store, sessionID, role, text string, seq int, input, output int64) {
	t.Helper()
	raw, _ := json.Marshal(provider.Message{Role: role, Content: text})
	turn := &storage.Turn{
		ID:           "turn-" + sessionID + "-" + role + "-" + string(rune('0'+seq)),
		SessionID:    sessionID,
		Sequence:     seq,
		Role:         role,
		Content:      raw,
		InputTokens:  input,
		OutputTokens: output,
		CreatedAt:    time.Now().UTC(),
	}
	if err := store.CreateTurn(context.Background(), turn); err != nil {
		t.Fatalf("CreateTurn %d: %v", seq, err)
	}
}

func TestBuildCompactPlan_SmallSession_Empty(t *testing.T) {
	// Fewer than recentTurnsKeepFloor+1 turns = nothing worth compacting.
	turns := []storage.Turn{
		{Sequence: 1, Role: "user", Content: json.RawMessage(`{"content":"hi"}`)},
		{Sequence: 2, Role: "assistant", Content: json.RawMessage(`{"content":"hello"}`)},
	}
	plan := buildCompactPlan(turns, 200_000)
	if len(plan.Categories) != 0 {
		t.Errorf("expected empty plan for small session; got %+v", plan.Categories)
	}
}

func TestBuildCompactPlan_OlderTurnsGetSummarize(t *testing.T) {
	// 7 turns: last 3 "recent" (keep), 4 older user/assistant (summarize).
	var turns []storage.Turn
	for i := 1; i <= 7; i++ {
		raw, _ := json.Marshal(provider.Message{Role: "user", Content: "turn " + strings.Repeat("x", 200)})
		turns = append(turns, storage.Turn{
			Sequence:     i,
			Role:         "user",
			Content:      raw,
			InputTokens:  100,
			OutputTokens: 0,
		})
	}
	plan := buildCompactPlan(turns, 200_000)
	if len(plan.Categories) == 0 {
		t.Fatal("expected non-empty plan")
	}
	var sawOlder, sawRecent bool
	for _, c := range plan.Categories {
		switch c.Name {
		case CompactCategoryOlder:
			sawOlder = true
			if c.SuggestedAction != CompactActionSummarize {
				t.Errorf("older should be summarize; got %q", c.SuggestedAction)
			}
		case CompactCategoryRecent:
			sawRecent = true
			if c.SuggestedAction != CompactActionKeep {
				t.Errorf("recent should be keep; got %q", c.SuggestedAction)
			}
		}
	}
	if !sawOlder || !sawRecent {
		t.Errorf("expected older + recent categories; got %+v", plan.Categories)
	}
	if plan.EstimatedTokensFreed <= 0 {
		t.Errorf("estimate should be positive; got %d", plan.EstimatedTokensFreed)
	}
}

func TestBuildCompactPlan_ToolOutputsGetDrop(t *testing.T) {
	var turns []storage.Turn
	for i := 1; i <= 6; i++ {
		role := "user"
		if i == 3 || i == 4 {
			role = "tool"
		}
		raw, _ := json.Marshal(provider.Message{Role: role, Content: "t"})
		turns = append(turns, storage.Turn{
			Sequence: i, Role: role, Content: raw, OutputTokens: 50,
		})
	}
	plan := buildCompactPlan(turns, 200_000)

	var toolCat *protocol.CompactCategory
	for i := range plan.Categories {
		if plan.Categories[i].Name == CompactCategoryToolOutput {
			toolCat = &plan.Categories[i]
		}
	}
	if toolCat == nil {
		t.Fatal("expected tool_results category")
	}
	if toolCat.SuggestedAction != CompactActionDrop {
		t.Errorf("tool_results should suggest drop; got %q", toolCat.SuggestedAction)
	}
}

func TestApplyCompactPlan_SummarizeOlder(t *testing.T) {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	session := &storage.Session{ID: "sess-apply", UserID: "u", Project: "/tmp/p", Status: "active"}
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	for i := 1; i <= 6; i++ {
		seedTurn(t, store, session.ID, "user", "older turn "+string(rune('0'+i)), i, 100, 50)
	}
	// 3 recent are implicitly the last 3 by sequence; since we want
	// to trigger "older" summarization we have 3 older turns here.

	resp, err := applyCompactPlan(ctx, store, session.ID,
		map[string]string{CompactCategoryOlder: CompactActionSummarize},
		200_000,
	)
	if err != nil {
		t.Fatalf("applyCompactPlan: %v", err)
	}
	if !resp.Applied {
		t.Fatalf("expected Applied=true; got %+v", resp)
	}
	if resp.TokensFreed <= 0 {
		t.Errorf("TokensFreed should be positive; got %d", resp.TokensFreed)
	}
	if !strings.Contains(resp.Summary, "summarized") {
		t.Errorf("summary should mention summarization; got %q", resp.Summary)
	}

	// After apply, the store should hold fewer turns (3 recent + 1
	// summary = 4 < 6 originals).
	rest, err := store.ListTurns(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListTurns: %v", err)
	}
	if len(rest) >= 6 {
		t.Errorf("turns should shrink after compact; got %d", len(rest))
	}
	// One of the remaining should be marked Compacted.
	var sawCompacted bool
	for _, r := range rest {
		if r.Compacted {
			sawCompacted = true
			if r.CompactedSummary == "" {
				t.Error("compacted turn should carry a summary")
			}
		}
	}
	if !sawCompacted {
		t.Error("expected at least one Compacted=true turn after apply")
	}
}

func TestApplyCompactPlan_DropToolOutputs(t *testing.T) {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	session := &storage.Session{ID: "sess-tool", UserID: "u", Project: "/tmp/p", Status: "active"}
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	seedTurn(t, store, session.ID, "user", "q1", 1, 100, 0)
	seedTurn(t, store, session.ID, "assistant", "a1", 2, 0, 100)
	seedTurn(t, store, session.ID, "tool", "file body", 3, 0, 300)
	seedTurn(t, store, session.ID, "user", "q2", 4, 100, 0)
	seedTurn(t, store, session.ID, "assistant", "a2", 5, 0, 100)
	seedTurn(t, store, session.ID, "assistant", "a3", 6, 0, 100)

	resp, err := applyCompactPlan(ctx, store, session.ID,
		map[string]string{CompactCategoryToolOutput: CompactActionDrop},
		200_000,
	)
	if err != nil {
		t.Fatalf("applyCompactPlan: %v", err)
	}
	if !resp.Applied {
		t.Fatalf("expected Applied=true; got %+v", resp)
	}
	rest, _ := store.ListTurns(ctx, session.ID)
	for _, r := range rest {
		if r.Role == "tool" {
			t.Errorf("tool turn should be dropped; still present %+v", r)
		}
	}
}

func TestApplyCompactPlan_EmptySession(t *testing.T) {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	session := &storage.Session{ID: "sess-empty", UserID: "u", Status: "active"}
	_ = store.CreateSession(ctx, session)

	resp, err := applyCompactPlan(ctx, store, session.ID, map[string]string{}, 200_000)
	if err != nil {
		t.Fatalf("applyCompactPlan: %v", err)
	}
	if resp.Applied {
		t.Errorf("empty session should not apply; got %+v", resp)
	}
}
