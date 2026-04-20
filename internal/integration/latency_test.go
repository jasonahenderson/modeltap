package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/bff"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// WU-095 B: end-to-end latency suite. Each test exercises the real
// harness ConnectionManager against a real BFF over unix socket,
// records latencies across N iterations, and reports p50/p95/p99.
//
// These are `go test` tests, not `go test -bench`, because the
// measurements we care about are per-iteration latencies under a
// fixed N, not the ns/op average of -bench. `-v` prints the numbers;
// they're baselines rather than pass/fail assertions (yet — we can
// set budgets from the recorded numbers once we have more signal).
//
// Run via:
//
//	go test ./internal/integration/ -run LatencyE2E -v -count=3

// latencyIters is the number of samples per latency test.
// 50 gives a readable p99 in ~seconds on an M-series laptop;
// bump to 500 for tighter intervals.
const latencyIters = 50

// latencySummary prints p50 / p95 / p99 for a set of durations.
// Format is one line per metric so -v output is greppable.
func latencySummary(t *testing.T, name string, samples []time.Duration) {
	t.Helper()
	if len(samples) == 0 {
		t.Errorf("%s: no samples", name)
		return
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p50 := samples[len(samples)*50/100]
	p95 := samples[len(samples)*95/100]
	p99 := samples[len(samples)*99/100]
	if len(samples) >= 100 {
		p99 = samples[len(samples)*99/100]
	} else {
		p99 = samples[len(samples)-1] // max as proxy
	}
	t.Logf("%s  n=%d  p50=%v  p95=%v  p99=%v  min=%v  max=%v",
		name, len(samples), p50, p95, p99, samples[0], samples[len(samples)-1])
}

// startLatencyBFF stands up a real BFF with a mock upstream for
// turn-submit latency tests. The mock returns minimal Anthropic SSE
// (one text delta + message_stop) synchronously so latency is
// dominated by framing / persistence, not upstream.
func startLatencyBFF(t *testing.T) (string, *httptest.Server) {
	t.Helper()
	sockPath := shortSocketPath(t)

	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}

event: message_stop
data: {"type":"message_stop"}

`))
	}))
	t.Cleanup(upstream.Close)

	cfg := bff.DefaultServerConfig()
	cfg.SocketPath = sockPath
	cfg.HeartbeatInterval = 50 * time.Millisecond
	cfg.HeartbeatTimeout = 500 * time.Millisecond

	srv := bff.NewServer(store, cfg)
	srv.Adapters().Register(provider.NewAnthropicProvider())
	_ = srv.Providers().Add(&bff.ProviderEndpoint{
		Name: "mock", Type: bff.ProviderTypeAnthropic, APIKey: "k", Host: upstream.URL,
	})
	srv.Dispatch().SetHTTPClient(upstream.Client())
	srv.Models().SetManual(map[string]bff.ModelOverrideConfig{
		"claude-sonnet-4-6": {Provider: "mock", ContextWindow: 200_000, Capabilities: []string{"text"}},
	})
	srv.Models().Refresh()
	srv.Routing().Replace(protocol.RoutingPolicy{"default": rawDefault(t, "claude-sonnet-4-6")})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	return sockPath, upstream
}

// TestLatencyE2E_SocketDialHandshake measures the time from
// ConnectSync entry to reaching ConnStateReady. Covers: dial,
// register RPC round-trip, event bridge startup.
func TestLatencyE2E_SocketDialHandshake(t *testing.T) {
	sockPath, _ := startLatencyBFF(t)

	samples := make([]time.Duration, 0, latencyIters)
	for i := 0; i < latencyIters; i++ {
		cm := harness.NewConnectionManager(harness.ConnectionConfig{
			SocketPath: sockPath,
			Registration: &protocol.CapabilitiesRegister{
				ProtocolVersion: "1",
				HarnessVersion:  "test",
				HarnessPlatform: "test",
			},
		}, &recordingSender{})

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := cm.ConnectSync(ctx); err != nil {
			cancel()
			cm.Disconnect()
			t.Fatalf("iter %d ConnectSync: %v", i, err)
		}
		cancel()
		samples = append(samples, time.Since(start))
		cm.Disconnect()
	}
	latencySummary(t, "dial+handshake", samples)
}

// TestLatencyE2E_ConnectionPing measures steady-state RPC latency.
// Ping is the minimal round-trip — framing + dispatch + handler +
// framing back. Good baseline for "how fast can an idle RPC go."
func TestLatencyE2E_ConnectionPing(t *testing.T) {
	sockPath, _ := startLatencyBFF(t)
	cm := harness.NewConnectionManager(harness.ConnectionConfig{
		SocketPath: sockPath,
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  "test",
			HarnessPlatform: "test",
		},
	}, &recordingSender{})
	t.Cleanup(cm.Disconnect)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cm.ConnectSync(ctx); err != nil {
		t.Fatalf("ConnectSync: %v", err)
	}
	client := cm.Client()

	samples := make([]time.Duration, 0, latencyIters)
	for i := 0; i < latencyIters; i++ {
		start := time.Now()
		if err := client.Ping(ctx); err != nil {
			t.Fatalf("iter %d Ping: %v", i, err)
		}
		samples = append(samples, time.Since(start))
	}
	latencySummary(t, "connection.ping", samples)
}

// TestLatencyE2E_SessionListRoundTrip measures a typical read-path
// RPC. Covers storage read + serialization + framing.
func TestLatencyE2E_SessionListRoundTrip(t *testing.T) {
	sockPath, _ := startLatencyBFF(t)
	cm := harness.NewConnectionManager(harness.ConnectionConfig{
		SocketPath: sockPath,
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  "test",
			HarnessPlatform: "test",
		},
	}, &recordingSender{})
	t.Cleanup(cm.Disconnect)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = cm.ConnectSync(ctx)
	client := cm.Client()

	samples := make([]time.Duration, 0, latencyIters)
	for i := 0; i < latencyIters; i++ {
		start := time.Now()
		if _, err := client.SessionList(ctx); err != nil {
			t.Fatalf("iter %d SessionList: %v", i, err)
		}
		samples = append(samples, time.Since(start))
	}
	latencySummary(t, "session.list", samples)
}

// TestLatencyE2E_TurnSubmitFirstToken is the one that maps to
// user-felt snappiness: time from SubmitMsg → first StreamTokenMsg.
// Latency spans: SubmitTurn RPC, BFF dispatch, TurnDispatcher HTTP
// to mock upstream, SSE parse + stream relay, notification back to
// harness, event bridge → tea.Msg delivery.
func TestLatencyE2E_TurnSubmitFirstToken(t *testing.T) {
	sockPath, _ := startLatencyBFF(t)

	samples := make([]time.Duration, 0, latencyIters/2) // turn.submit is heavier; fewer iters
	for i := 0; i < cap(samples); i++ {
		sender := &recordingSender{}
		cm := harness.NewConnectionManager(harness.ConnectionConfig{
			SocketPath: sockPath,
			Registration: &protocol.CapabilitiesRegister{
				ProtocolVersion: "1",
				HarnessVersion:  "test",
				HarnessPlatform: "test",
			},
		}, sender)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := cm.ConnectSync(ctx); err != nil {
			cancel()
			cm.Disconnect()
			t.Fatalf("iter %d ConnectSync: %v", i, err)
		}

		sessionID := "latency-sess-" + itoaLatency(i)
		client := cm.Client()
		// Seed a session so turn.submit doesn't need to create one
		// on the fly (noise we don't want in the latency sample).
		_, _ = client.SessionResume(ctx, sessionID, protocol.ProjectContext{Root: "/tmp/p"})

		app := harness.NewApp(harness.AppOptions{Conn: harness.WrapConnectionManager(cm)})
		app.State().SessionID = sessionID

		start := time.Now()
		_, cmdFn := app.Update(harness.SubmitMsg{Content: "hi"})
		if cmdFn != nil {
			cmdFn() // drain; returns TurnSubmittedMsg, not the first token
		}
		// Poll sender for the first StreamTokenMsg.
		deadline := time.Now().Add(3 * time.Second)
		var firstToken time.Time
		for time.Now().Before(deadline) && firstToken.IsZero() {
			for _, m := range sender.snapshot() {
				if _, ok := m.(harness.StreamTokenMsg); ok {
					firstToken = time.Now()
					break
				}
			}
			if firstToken.IsZero() {
				time.Sleep(2 * time.Millisecond)
			}
		}
		cancel()
		cm.Disconnect()

		if firstToken.IsZero() {
			t.Fatalf("iter %d: no StreamTokenMsg within deadline", i)
		}
		samples = append(samples, firstToken.Sub(start))
	}
	latencySummary(t, "turn.submit→firstToken", samples)
}

// itoaLatency is a local copy of the integer formatter used in the
// storage benchmarks — avoids the cost of fmt.Sprint in test setup.
func itoaLatency(n int) string {
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
