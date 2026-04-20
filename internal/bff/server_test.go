package bff

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// shortServerConfig returns a ServerConfig with aggressive timings so
// timer-driven tests run in milliseconds rather than seconds.
func shortServerConfig(socketPath string) ServerConfig {
	return ServerConfig{
		SocketPath:        socketPath,
		SocketMode:        0o600,
		MaxConnections:    100,
		HeartbeatInterval: 10 * time.Millisecond,
		HeartbeatTimeout:  200 * time.Millisecond,
		GracePeriod:       50 * time.Millisecond,
	}
}

func TestServer_SocketListener(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "s.sock")

	srv := NewServer(&recordingStore{}, shortServerConfig(sockPath))
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	if _, err := os.Stat(sockPath); err != nil {
		t.Fatalf("socket not created: %v", err)
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
}

func TestServer_SocketPermissions(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "s.sock")

	cfg := shortServerConfig(sockPath)
	cfg.SocketMode = 0o600
	srv := NewServer(&recordingStore{}, cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("socket perm = %o, want 0600", info.Mode().Perm())
	}
}

func TestServer_StaleSocketRemoval(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "s.sock")

	// Create a stale socket file (no listener).
	if err := os.WriteFile(sockPath, []byte(""), 0o600); err != nil {
		t.Fatalf("prepare stale socket: %v", err)
	}

	srv := NewServer(&recordingStore{}, shortServerConfig(sockPath))
	if err := srv.Start(); err != nil {
		t.Fatalf("Start (should remove stale socket): %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial after stale removal: %v", err)
	}
	_ = conn.Close()
}

func TestServer_ActiveSocketReject(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "s.sock")

	// First server binds and holds the socket.
	first := NewServer(&recordingStore{}, shortServerConfig(sockPath))
	if err := first.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	t.Cleanup(func() { _ = first.Shutdown(context.Background()) })

	// Second server must refuse to bind — an active listener is present.
	second := NewServer(&recordingStore{}, shortServerConfig(sockPath))
	if err := second.Start(); err == nil {
		_ = second.Shutdown(context.Background())
		t.Fatalf("second Start should reject an active socket")
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "s.sock")

	srv := NewServer(&recordingStore{}, shortServerConfig(sockPath))
	srv.dispatcher.Register("test.slow", func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		time.Sleep(50 * time.Millisecond)
		return map[string]string{"ok": "yes"}, nil
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Open a client, drive it to Ready so gating lets test.slow through.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	// Give the server a moment to set up the Connection.
	time.Sleep(20 * time.Millisecond)
	srv.forceAllReadyForTest()

	// Shutdown should drain active connections.
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- srv.Shutdown(context.Background())
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Shutdown did not complete")
	}

	// Socket must no longer accept.
	if _, err := net.DialTimeout("unix", sockPath, 100*time.Millisecond); err == nil {
		t.Errorf("socket still accepting after shutdown")
	}
}

func TestServer_MaxConnections(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "s.sock")

	cfg := shortServerConfig(sockPath)
	cfg.MaxConnections = 1
	srv := NewServer(&recordingStore{}, cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	// First connection succeeds.
	c1, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("c1 dial: %v", err)
	}
	defer c1.Close()
	// Let the server register it.
	time.Sleep(30 * time.Millisecond)

	// Second connection: the server accepts the unix socket at kernel
	// level, then immediately closes because MaxConnections is reached.
	c2, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("c2 dial: %v", err)
	}
	defer c2.Close()
	// A read on c2 should return EOF quickly (server closed it).
	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 16)
	if _, err := c2.Read(buf); err == nil {
		t.Errorf("expected c2 closed, got data")
	} else if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
		// net.Pipe returns io.EOF; unix socket may return different EOF-
		// shaped errors — accept any read error as acceptable evidence.
		t.Logf("c2 read err (acceptable): %v", err)
	}
}

func TestServer_ConcurrentAccept(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "s.sock")

	srv := NewServer(&recordingStore{}, shortServerConfig(sockPath))
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	const n = 10
	var wg sync.WaitGroup
	errCount := int32(0)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("unix", sockPath)
			if err != nil {
				atomic.AddInt32(&errCount, 1)
				return
			}
			_ = conn.Close()
		}()
	}
	wg.Wait()
	if errCount > 0 {
		t.Errorf("%d of %d concurrent dials failed", errCount, n)
	}
}

func TestServer_TLSListener(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	generateSelfSignedCert(t, certFile, keyFile)

	// mTLS is required now (WU-094 H-14). Reuse the server cert as
	// its own client CA for the test — exercises the verification
	// code path without maintaining a second fixture.
	cfg := ServerConfig{
		TLSAddress:        "127.0.0.1:0", // ephemeral port; resolved after Start
		TLSCertFile:       certFile,
		TLSKeyFile:        keyFile,
		TLSClientCAFile:   certFile,
		MaxConnections:    100,
		HeartbeatInterval: 10 * time.Millisecond,
		HeartbeatTimeout:  200 * time.Millisecond,
		GracePeriod:       50 * time.Millisecond,
	}
	srv := NewServer(&recordingStore{}, cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	addr := srv.TLSAddr()
	if addr == "" {
		t.Fatalf("TLS address empty after Start")
	}

	// Client presents the same cert as its identity to satisfy
	// RequireAndVerifyClientCert.
	clientCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("load client cert: %v", err)
	}
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{clientCert},
	})
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	_ = conn.Close()
}

// TestServer_TLSListener_RefusesWithoutClientCA pins WU-094 H-14:
// binding the TLS listener without tls_client_ca_file must fail
// loudly rather than accept arbitrary peers as the solo user.
func TestServer_TLSListener_RefusesWithoutClientCA(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	generateSelfSignedCert(t, certFile, keyFile)

	cfg := ServerConfig{
		TLSAddress:        "127.0.0.1:0",
		TLSCertFile:       certFile,
		TLSKeyFile:        keyFile,
		// No TLSClientCAFile — must refuse.
		MaxConnections:    100,
		HeartbeatInterval: 10 * time.Millisecond,
		HeartbeatTimeout:  200 * time.Millisecond,
		GracePeriod:       50 * time.Millisecond,
	}
	srv := NewServer(&recordingStore{}, cfg)
	err := srv.Start()
	if err == nil {
		_ = srv.Shutdown(context.Background())
		t.Fatal("expected Start to fail without TLSClientCAFile")
	}
	if !strings.Contains(err.Error(), "tls_client_ca_file") {
		t.Errorf("error should name the missing config: %v", err)
	}
}

func TestHandleConnectionHealth_Populated(t *testing.T) {
	store := &recordingStore{}
	srv := NewServer(store, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)

	raw, err := handleConnectionHealth(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleConnectionHealth: %v", err)
	}
	hr, ok := raw.(*protocol.HealthResponse)
	if !ok {
		t.Fatalf("result type = %T, want *protocol.HealthResponse", raw)
	}
	if hr.ProtocolVersion != protocol.ProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", hr.ProtocolVersion, protocol.ProtocolVersion)
	}
	if hr.ServerVersion == "" {
		t.Errorf("ServerVersion empty")
	}
	if hr.Storage.Status != "ready" {
		t.Errorf("Storage.Status = %q, want ready", hr.Storage.Status)
	}
	if hr.ActiveSession != nil {
		t.Errorf("ActiveSession should be nil when no session is bound; got %+v", hr.ActiveSession)
	}
}

func TestHandleConnectionHealth_StorageUnavailable(t *testing.T) {
	store := &recordingStore{}
	store.setPingErr(errors.New("db down"))
	srv := NewServer(store, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)

	raw, err := handleConnectionHealth(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleConnectionHealth: %v", err)
	}
	hr := raw.(*protocol.HealthResponse)
	if hr.Storage.Status != "unavailable" {
		t.Errorf("Storage.Status = %q, want unavailable", hr.Storage.Status)
	}
	if hr.Storage.Reason == "" {
		t.Errorf("Storage.Reason should be set when unavailable")
	}
}

func TestHandleConnectionHealth_ActiveSession(t *testing.T) {
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)
	c.SetSessionID("sess-xyz")

	raw, err := handleConnectionHealth(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleConnectionHealth: %v", err)
	}
	hr := raw.(*protocol.HealthResponse)
	if hr.ActiveSession == nil {
		t.Fatalf("ActiveSession should be populated")
	}
	if hr.ActiveSession.ID != "sess-xyz" {
		t.Errorf("ActiveSession.ID = %q", hr.ActiveSession.ID)
	}
	if hr.ActiveSession.Owner != "c1" {
		t.Errorf("ActiveSession.Owner = %q", hr.ActiveSession.Owner)
	}
}

func TestHandleConnectionReady_Ready(t *testing.T) {
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)

	raw, err := handleConnectionReady(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleConnectionReady: %v", err)
	}
	rr := raw.(*protocol.ReadyResponse)
	if !rr.Ready {
		t.Errorf("Ready should be true")
	}
}

func TestHandleConnectionReady_NotReadyStateWrong(t *testing.T) {
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnRegistering)

	raw, err := handleConnectionReady(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleConnectionReady: %v", err)
	}
	rr := raw.(*protocol.ReadyResponse)
	if rr.Ready {
		t.Errorf("Ready should be false in Registering state")
	}
}

func TestHandleConnectionReady_NotReadyStorageDown(t *testing.T) {
	store := &recordingStore{}
	store.setPingErr(errors.New("db down"))
	srv := NewServer(store, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)

	raw, err := handleConnectionReady(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleConnectionReady: %v", err)
	}
	rr := raw.(*protocol.ReadyResponse)
	if rr.Ready {
		t.Errorf("Ready should be false when storage is unavailable")
	}
}

func TestServer_RegistersConnectionHandlers(t *testing.T) {
	// After NewServer, ping/health/ready must be dispatchable. We do not
	// register stubs for the 19 application methods (later WUs own those).
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	for _, m := range []string{
		protocol.MethodConnectionPing,
		protocol.MethodConnectionHealth,
		protocol.MethodConnectionReady,
	} {
		if _, ok := srv.dispatcher.handlers[m]; !ok {
			t.Errorf("handler for %q not registered", m)
		}
	}
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// nopConn returns a net.Conn whose writes are continuously drained by
// a background goroutine. Useful for constructing a FrameTransport
// purely to reach a handler under test without a real peer; handlers
// that emit notifications (model.switch, etc.) won't block on write.
func nopConn() net.Conn {
	server, client := net.Pipe()
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := client.Read(buf); err != nil {
				return
			}
		}
	}()
	return server
}

// generateSelfSignedCert writes a fresh self-signed ECDSA cert and key
// to the given paths for TLS tests. The cert has a 1-hour validity and
// is for 127.0.0.1 only.
func generateSelfSignedCert(t *testing.T, certFile, keyFile string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "modeltap-test"},
		NotBefore:    time.Now().Add(-1 * time.Minute),
		NotAfter:     time.Now().Add(1 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certOut, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("open cert file: %v", err)
	}
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	_ = certOut.Close()

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyOut, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("open key file: %v", err)
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	_ = keyOut.Close()
}

// Ensure unused storage import in tests; consumed indirectly via recordingStore.
var _ storage.Store = (*recordingStore)(nil)
