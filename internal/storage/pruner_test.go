package storage

import (
	"context"
	"testing"
	"time"
)

func TestPruner_DeletesOldRecords(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Insert a record 60 days old (should be pruned with 30-day retention).
	old := makeRequest(func(r *Request) {
		r.Timestamp = now.AddDate(0, 0, -60)
	})
	// Insert a record 10 days old (should be preserved with 30-day retention).
	recent := makeRequest(func(r *Request) {
		r.Timestamp = now.AddDate(0, 0, -10)
	})

	if err := store.SaveRequest(ctx, old); err != nil {
		t.Fatalf("SaveRequest (old): %v", err)
	}
	if err := store.SaveRequest(ctx, recent); err != nil {
		t.Fatalf("SaveRequest (recent): %v", err)
	}

	// Verify both exist before pruning.
	count, err := store.CountRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("CountRequests: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 records before pruning, got %d", count)
	}

	// Create pruner with 30-day retention and a short interval.
	pruner := NewPruner(store, 30, 50*time.Millisecond)
	pruner.Start(ctx)

	// Wait for at least one prune cycle.
	time.Sleep(150 * time.Millisecond)
	pruner.Stop()

	// The old record should be deleted, the recent one preserved.
	count, err = store.CountRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("CountRequests after prune: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record after pruning, got %d", count)
	}

	// Verify the preserved record is the recent one.
	got, err := store.GetRequest(ctx, recent.ID)
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if got == nil {
		t.Error("recent record was unexpectedly deleted")
	}

	// Verify the old record is gone.
	got, err = store.GetRequest(ctx, old.ID)
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if got != nil {
		t.Error("old record was not deleted by pruner")
	}
}

func TestPruner_PreservesRecentRecords(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Insert records that are all within the retention period.
	for i := 0; i < 5; i++ {
		req := makeRequest(func(r *Request) {
			r.Timestamp = now.AddDate(0, 0, -i) // 0 to 4 days old
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	pruner := NewPruner(store, 30, 50*time.Millisecond)
	pruner.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	pruner.Stop()

	count, err := store.CountRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("CountRequests: %v", err)
	}
	if count != 5 {
		t.Errorf("expected all 5 records preserved, got %d", count)
	}
}

func TestPruner_StopsCleanlyOnContextCancel(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())

	pruner := NewPruner(store, 30, 50*time.Millisecond)
	pruner.Start(ctx)

	// Cancel the parent context.
	cancel()

	// Stop should return promptly (not hang).
	done := make(chan struct{})
	go func() {
		pruner.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success: pruner stopped cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("pruner.Stop() did not return within 2 seconds after context cancellation")
	}
}

func TestPruner_ZeroRetentionDays_NoPruning(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Insert a record 100 days old.
	old := makeRequest(func(r *Request) {
		r.Timestamp = now.AddDate(0, 0, -100)
	})
	if err := store.SaveRequest(ctx, old); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	// With retentionDays=0, the pruner should not start and not delete anything.
	pruner := NewPruner(store, 0, 50*time.Millisecond)
	pruner.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	pruner.Stop()

	count, err := store.CountRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("CountRequests: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record (no pruning with retentionDays=0), got %d", count)
	}
}

func TestPruner_NegativeRetentionDays_NoPruning(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()

	old := makeRequest(func(r *Request) {
		r.Timestamp = now.AddDate(0, 0, -200)
	})
	if err := store.SaveRequest(ctx, old); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	// Negative retentionDays should also mean "keep forever".
	pruner := NewPruner(store, -1, 50*time.Millisecond)
	pruner.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	pruner.Stop()

	count, err := store.CountRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("CountRequests: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record (no pruning with negative retentionDays), got %d", count)
	}
}
