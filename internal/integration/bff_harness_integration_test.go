package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/bff"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

var shortSocketCounter atomic.Int64

// shortSocketPath returns a unix socket path in /tmp keyed to the test
// PID + a counter — keeps the path well under the 104-byte sun_path
// cap on darwin, which long t.TempDir() paths overflow.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	n := shortSocketCounter.Add(1)
	p := filepath.Join("/tmp", fmt.Sprintf("mt-%d-%d.sock", os.Getpid(), n))
	t.Cleanup(func() { _ = os.Remove(p) })
	return p
}

// TestBFFHarness_RegisterHealthHistoryRoundTrip drives a real BFF
// server over its unix socket with the harness ProtocolClient, to
// satisfy WU-067: "E2E with real BFF + in-memory storage". Focus is
// on the wire-protocol contract — handshake, health, and request/
// response round-tripping through the framing / dispatcher layers.
//
// Provider-dependent paths (turn.submit streaming, tool round-trip)
// require a live provider and are covered in WU-088.
func TestBFFHarness_RegisterHealthHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// macOS caps sun_path at 104 bytes; the default t.TempDir() for
	// long test names overflows. Use /tmp with a short unique suffix.
	sockPath := shortSocketPath(t)

	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := bff.DefaultServerConfig()
	cfg.SocketPath = sockPath
	cfg.HeartbeatInterval = 50 * time.Millisecond
	cfg.HeartbeatTimeout = 500 * time.Millisecond
	cfg.GracePeriod = 100 * time.Millisecond

	srv := bff.NewServer(store, cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	// Dial via the harness ProtocolClient so the test exercises the
	// real framing layer end-to-end.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := harness.Dial(ctx, harness.DialOptions{SocketPath: sockPath})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// Handshake: capabilities.register.
	reg, err := client.Register(ctx, &protocol.CapabilitiesRegister{
		ProtocolVersion: "1",
		HarnessVersion:  "test",
		HarnessPlatform: "test",
		Project:         protocol.ProjectContext{Root: dir},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if reg == nil || reg.NegotiatedVersion == "" {
		t.Errorf("Register response empty: %+v", reg)
	}

	// Ping (connection.ping — the solo-profile heartbeat).
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Health returns structured dependency status.
	if _, err := client.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	// SessionList round-trip (empty initially).
	sessions, err := client.SessionList(ctx)
	if err != nil {
		t.Fatalf("SessionList: %v", err)
	}
	if sessions == nil {
		t.Error("SessionList returned nil response")
	}

	// HistoryList round-trip: append, then list.
	appendRaw, err := client.Call(ctx, protocol.MethodHistoryAppend, &protocol.HistoryAppend{
		Content: "hello history",
	})
	if err != nil {
		t.Fatalf("history.append: %v", err)
	}
	if appendRaw == nil {
		t.Error("history.append returned nil result")
	}

	listed, err := client.HistoryList(ctx, &protocol.HistoryList{
		Scope: harness.HistoryScopeUser,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("HistoryList: %v", err)
	}
	var sawAppended bool
	for _, e := range listed.Entries {
		if e.Content == "hello history" {
			sawAppended = true
			break
		}
	}
	if !sawAppended {
		t.Errorf("history.list did not include the appended entry; got %+v", listed.Entries)
	}

	// ModelList round-trip (empty catalog is fine).
	mresp, err := client.ModelList(ctx)
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	if mresp == nil {
		t.Error("ModelList returned nil")
	}
}

// TestBFFHarness_SessionResume covers the session.resume path for a
// pre-existing session: seed a session into storage, then assert the
// resume RPC returns it.
func TestBFFHarness_SessionResume(t *testing.T) {
	dir := t.TempDir()
	// macOS caps sun_path at 104 bytes; the default t.TempDir() for
	// long test names overflows. Use /tmp with a short unique suffix.
	sockPath := shortSocketPath(t)

	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Seed a session that the harness will resume.
	seededID := "sess-integration-resume"
	seeded := &storage.Session{
		ID:      seededID,
		UserID:  "solo",
		Project: dir,
		Summary: "seeded",
		Status:  "active",
	}
	if err := store.CreateSession(context.Background(), seeded); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	cfg := bff.DefaultServerConfig()
	cfg.SocketPath = sockPath
	cfg.HeartbeatInterval = 50 * time.Millisecond
	cfg.HeartbeatTimeout = 500 * time.Millisecond

	srv := bff.NewServer(store, cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := harness.Dial(ctx, harness.DialOptions{SocketPath: sockPath})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := client.Register(ctx, &protocol.CapabilitiesRegister{
		ProtocolVersion: "1",
		HarnessVersion:  "test",
		HarnessPlatform: "test",
		Project:         protocol.ProjectContext{Root: dir},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	resp, err := client.SessionResume(ctx, seededID, protocol.ProjectContext{Root: dir})
	if err != nil {
		t.Fatalf("SessionResume: %v", err)
	}
	if resp.SessionID != seededID {
		t.Errorf("SessionID = %q, want %q", resp.SessionID, seededID)
	}
}
