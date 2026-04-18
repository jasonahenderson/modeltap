package bff

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func makeRegisterParams(t *testing.T, req *protocol.CapabilitiesRegister) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal CapabilitiesRegister: %v", err)
	}
	return raw
}

func validTool(name string) protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:           name,
		Namespace:      "test",
		Description:    "a " + name + " tool",
		InputSchema:    json.RawMessage(`{"type":"object"}`),
		RiskLevel:      "read_only",
		OutputEnvelope: "text",
	}
}

func TestCapabilities_Register_Success(t *testing.T) {
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnRegistering)

	params := makeRegisterParams(t, &protocol.CapabilitiesRegister{
		ProtocolVersion: protocol.ProtocolVersion,
		HarnessVersion:  "0.0.1",
		HarnessPlatform: "test",
		Tools:           []protocol.ToolDefinition{validTool("read")},
		Project: protocol.ProjectContext{
			Root:          "/tmp/proj",
			ConfigFile:    "modeltap.yaml",
			ConfigContent: "model: dev",
		},
	})

	raw, err := handleCapabilitiesRegister(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleCapabilitiesRegister: %v", err)
	}
	resp := raw.(*protocol.CapabilitiesRegisterResponse)
	if len(resp.Registered) != 1 {
		t.Errorf("Registered = %d, want 1", len(resp.Registered))
	}
	if len(resp.Rejected) != 0 {
		t.Errorf("Rejected = %+v, want empty", resp.Rejected)
	}
	if resp.ServerCapabilities.ProtocolVersion != protocol.ProtocolVersion {
		t.Errorf("ServerCapabilities.ProtocolVersion = %q", resp.ServerCapabilities.ProtocolVersion)
	}
	if resp.ServerCapabilities.MaxFrameSize != protocol.MaxFrameSize {
		t.Errorf("MaxFrameSize = %d, want %d", resp.ServerCapabilities.MaxFrameSize, protocol.MaxFrameSize)
	}
	if resp.ServerCapabilities.MaxAttachmentSize <= 0 {
		t.Errorf("MaxAttachmentSize not populated: %d", resp.ServerCapabilities.MaxAttachmentSize)
	}
	if c.State() != ConnReady {
		t.Errorf("connection state = %v, want ConnReady", c.State())
	}
	tools := c.Capabilities().Tools()
	if len(tools) != 1 || tools[0].Name != "read" {
		t.Errorf("Capabilities.Tools = %+v", tools)
	}
	pc := c.Capabilities().ProjectContext()
	if pc.Root != "/tmp/proj" {
		t.Errorf("ProjectContext.Root = %q", pc.Root)
	}
	if v := c.Capabilities().NegotiatedVersion(); v != protocol.ProtocolVersion {
		t.Errorf("NegotiatedVersion = %q", v)
	}
}

func TestCapabilities_Register_VersionMismatch(t *testing.T) {
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnRegistering)

	params := makeRegisterParams(t, &protocol.CapabilitiesRegister{
		ProtocolVersion: "99",
		Tools:           []protocol.ToolDefinition{validTool("read")},
	})

	_, err := handleCapabilitiesRegister(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected version-mismatch error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeVersionMismatch {
		t.Errorf("expected CodeVersionMismatch, got %T %v", err, err)
	}
	if c.State() != ConnFailed {
		t.Errorf("after version mismatch, state = %v, want ConnFailed", c.State())
	}
}

func TestCapabilities_Register_PartialRejection(t *testing.T) {
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnRegistering)

	good := validTool("good")
	badRisk := validTool("bad_risk")
	badRisk.RiskLevel = "WHATEVER"
	badEnvelope := validTool("bad_envelope")
	badEnvelope.OutputEnvelope = "martian"
	missingName := validTool("")
	missingDesc := validTool("no_desc")
	missingDesc.Description = ""

	params := makeRegisterParams(t, &protocol.CapabilitiesRegister{
		ProtocolVersion: protocol.ProtocolVersion,
		Tools:           []protocol.ToolDefinition{good, badRisk, badEnvelope, missingName, missingDesc},
	})

	raw, err := handleCapabilitiesRegister(context.Background(), c, params)
	if err != nil {
		t.Fatalf("partial rejection must not error: %v", err)
	}
	resp := raw.(*protocol.CapabilitiesRegisterResponse)
	if len(resp.Registered) != 1 {
		t.Errorf("Registered = %d, want 1", len(resp.Registered))
	}
	if len(resp.Rejected) != 4 {
		t.Errorf("Rejected = %d, want 4: %+v", len(resp.Rejected), resp.Rejected)
	}
	if c.State() != ConnReady {
		t.Errorf("state = %v, want ConnReady (partial rejection is success)", c.State())
	}
}

func TestCapabilities_Register_ReplayPrevented(t *testing.T) {
	// After a successful register, the connection is in ConnReady. Any
	// subsequent capabilities.register goes through the dispatch gate
	// (which rejects non-register/ping methods in non-ready states), but
	// for ready state all methods pass. The handler itself must refuse
	// re-registration because the state is no longer ConnRegistering.
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnRegistering)

	params := makeRegisterParams(t, &protocol.CapabilitiesRegister{
		ProtocolVersion: protocol.ProtocolVersion,
		Tools:           []protocol.ToolDefinition{validTool("read")},
	})
	if _, err := handleCapabilitiesRegister(context.Background(), c, params); err != nil {
		t.Fatalf("first register: %v", err)
	}

	// Second call: state is ConnReady now.
	_, err := handleCapabilitiesRegister(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected replay to be rejected")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeNotReady {
		t.Errorf("expected CodeNotReady on replay, got %T %v", err, err)
	}
}

func TestCapabilities_Update_AddAndRemove(t *testing.T) {
	cm := NewCapabilityManager()
	cm.setVersion(protocol.ProtocolVersion)
	cm.replaceTools([]protocol.ToolDefinition{validTool("one"), validTool("two")})

	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)
	c.capabilities = cm

	// Add "three", remove "one".
	update := &protocol.CapabilitiesUpdate{
		AddedTools:   []protocol.ToolDefinition{validTool("three")},
		RemovedTools: []string{"one"},
	}
	raw, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	respRaw, err := handleCapabilitiesUpdate(context.Background(), c, raw)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	resp := respRaw.(*protocol.CapabilitiesUpdateResponse)
	if resp.AddedCount != 1 || resp.RemovedCount != 1 {
		t.Errorf("counts = %+v", resp)
	}
	names := make(map[string]bool)
	for _, t := range cm.Tools() {
		names[t.Name] = true
	}
	if !names["two"] || !names["three"] || names["one"] {
		t.Errorf("tool catalog after update = %+v", names)
	}
}

func TestCapabilities_Update_AtomicReject(t *testing.T) {
	cm := NewCapabilityManager()
	cm.replaceTools([]protocol.ToolDefinition{validTool("one")})

	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	c := NewConnection("c1", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)
	c.capabilities = cm

	bad := validTool("bad")
	bad.RiskLevel = "WHATEVER"

	update := &protocol.CapabilitiesUpdate{
		AddedTools:   []protocol.ToolDefinition{validTool("good"), bad},
		RemovedTools: []string{"one"},
	}
	raw, _ := json.Marshal(update)
	_, err := handleCapabilitiesUpdate(context.Background(), c, raw)
	if err == nil {
		t.Fatalf("expected error for invalid added tool")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeCapabilityError {
		t.Errorf("expected CodeCapabilityError, got %T %v", err, err)
	}

	// Catalog must be unchanged: still just "one".
	tools := cm.Tools()
	if len(tools) != 1 || tools[0].Name != "one" {
		t.Errorf("catalog was mutated on atomic-reject: %+v", tools)
	}
}

func TestCapabilities_ProjectContext_Refresh(t *testing.T) {
	cm := NewCapabilityManager()
	cm.UpdateProjectContext(protocol.ProjectContext{Root: "/a"})
	if cm.ProjectContext().Root != "/a" {
		t.Errorf("initial: %+v", cm.ProjectContext())
	}
	cm.UpdateProjectContext(protocol.ProjectContext{Root: "/b", ConfigFile: "f.yaml"})
	if cm.ProjectContext().Root != "/b" || cm.ProjectContext().ConfigFile != "f.yaml" {
		t.Errorf("after refresh: %+v", cm.ProjectContext())
	}
}

func TestCapabilities_Tools_SnapshotIsCopy(t *testing.T) {
	cm := NewCapabilityManager()
	cm.replaceTools([]protocol.ToolDefinition{validTool("one")})

	snap := cm.Tools()
	if len(snap) != 1 {
		t.Fatalf("snap len = %d", len(snap))
	}
	// Mutating the returned slice must not affect the manager.
	snap[0].Name = "MUTATED"
	internal := cm.Tools()
	if internal[0].Name == "MUTATED" {
		t.Errorf("Tools() returned aliased slice — internal state was mutated")
	}
}

func TestCapabilities_RequestReregistration_SendsNotification(t *testing.T) {
	srv := NewServer(&recordingStore{}, shortServerConfig(""))
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close() })
	t.Cleanup(func() { _ = serverConn.Close() })

	c := NewConnection("c1", NewFrameTransport(serverConn), srv, false)

	// Reader to capture the notification.
	got := make(chan []byte, 1)
	go func() {
		fr := protocol.NewFrameReader(clientConn)
		b, _ := fr.ReadFrame()
		got <- b
	}()

	if err := c.Capabilities().RequestReregistration(c, "reconnection"); err != nil {
		t.Fatalf("RequestReregistration: %v", err)
	}
	select {
	case frame := <-got:
		var notif protocol.Notification
		if err := json.Unmarshal(frame, &notif); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if notif.Method != protocol.EventCapabilitiesRequest {
			t.Errorf("method = %q, want %q", notif.Method, protocol.EventCapabilitiesRequest)
		}
		if !strings.Contains(string(notif.Params), "reconnection") {
			t.Errorf("params missing reason: %s", notif.Params)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("no frame received")
	}
}
