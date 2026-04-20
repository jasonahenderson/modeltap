package storage_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
)

// WU-095 micro-benchmarks for storage hot paths — the write path is
// on every turn, and the session-lock path fires on every
// turn.submit. Run via:
//
//	go test ./internal/storage/ -bench=. -benchmem -run=^$

func benchStore(b *testing.B) *storage.SQLiteStore {
	b.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		b.Fatalf("NewSQLiteStore: %v", err)
	}
	b.Cleanup(func() { _ = store.Close() })
	return store
}

func BenchmarkCreateTurn(b *testing.B) {
	store := benchStore(b)
	ctx := context.Background()
	if err := store.CreateSession(ctx, &storage.Session{
		ID: "bench-sess", UserID: "u", Project: "/tmp/p", Status: "active",
	}); err != nil {
		b.Fatalf("CreateSession: %v", err)
	}
	content, _ := json.Marshal(map[string]string{"role": "user", "content": "hi"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := &storage.Turn{
			ID:        "t-" + time.Now().UTC().Format(time.RFC3339Nano) + "-" + itoa(i),
			SessionID: "bench-sess",
			Sequence:  i + 1,
			Role:      "user",
			Content:   content,
			CreatedAt: time.Now().UTC(),
		}
		if err := store.CreateTurn(ctx, t); err != nil {
			b.Fatalf("CreateTurn: %v", err)
		}
	}
}

func BenchmarkListTurns(b *testing.B) {
	store := benchStore(b)
	ctx := context.Background()
	_ = store.CreateSession(ctx, &storage.Session{ID: "s", UserID: "u", Status: "active"})
	content, _ := json.Marshal(map[string]string{"role": "user", "content": "hi"})
	for i := 0; i < 200; i++ {
		_ = store.CreateTurn(ctx, &storage.Turn{
			ID: "t-" + itoa(i), SessionID: "s", Sequence: i + 1,
			Role: "user", Content: content, CreatedAt: time.Now().UTC(),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.ListTurns(ctx, "s"); err != nil {
			b.Fatalf("ListTurns: %v", err)
		}
	}
}

func BenchmarkAppendCommandHistory(b *testing.B) {
	store := benchStore(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.AppendCommandHistory(ctx, &storage.CommandHistoryEntry{
			UserID: "u", Project: "/tmp/p", Content: "test cmd",
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			b.Fatalf("AppendCommandHistory: %v", err)
		}
	}
}

// itoa is a minimal helper to keep imports tight. Avoids strconv for
// a single-digit conversion on the benchmark hot path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
