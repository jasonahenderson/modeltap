package bff

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return json.RawMessage(b)
}

func TestRoutingPolicy_Resolve_ExactMatch(t *testing.T) {
	rp := NewRoutingPolicy()
	rp.Replace(protocol.RoutingPolicy{
		"coding.review": rawJSON(t, []string{"opus", "gpt-5"}),
	})
	models, multi, ok := rp.Resolve("coding.review")
	if !ok || !multi || len(models) != 2 {
		t.Errorf("Resolve(coding.review) = %v, multi=%v ok=%v", models, multi, ok)
	}
}

func TestRoutingPolicy_Resolve_FallbackToCategoryDefault(t *testing.T) {
	rp := NewRoutingPolicy()
	rp.Replace(protocol.RoutingPolicy{
		"coding.default": rawJSON(t, "claude-sonnet"),
	})
	models, multi, ok := rp.Resolve("coding.review")
	if !ok || multi || len(models) != 1 || models[0] != "claude-sonnet" {
		t.Errorf("unexpected: models=%v multi=%v ok=%v", models, multi, ok)
	}
}

func TestRoutingPolicy_Resolve_FallbackToRootDefault(t *testing.T) {
	rp := NewRoutingPolicy()
	rp.Replace(protocol.RoutingPolicy{
		"default": rawJSON(t, "root-model"),
	})
	models, _, ok := rp.Resolve("coding.review")
	if !ok || len(models) != 1 || models[0] != "root-model" {
		t.Errorf("fallback: models=%v ok=%v", models, ok)
	}
}

func TestRoutingPolicy_Resolve_NotFound(t *testing.T) {
	rp := NewRoutingPolicy()
	rp.Replace(protocol.RoutingPolicy{
		"other.default": rawJSON(t, "x"),
	})
	_, _, ok := rp.Resolve("coding.review")
	if ok {
		t.Errorf("should not resolve without default")
	}
}

func TestRoutingPolicy_ResolveForTurn_SessionOverride(t *testing.T) {
	rp := NewRoutingPolicy()
	rp.Replace(protocol.RoutingPolicy{
		"default": rawJSON(t, "policy-model"),
	})
	sess := &ActiveSession{ModelOverride: "override-model"}
	models, multi := rp.ResolveForTurn(sess, protocol.ModeBuild)
	if multi || len(models) != 1 || models[0] != "override-model" {
		t.Errorf("override not honored: models=%v multi=%v", models, multi)
	}
}

func TestRoutingPolicy_ResolveForTurn_RouteByMode(t *testing.T) {
	rp := NewRoutingPolicy()
	rp.Replace(protocol.RoutingPolicy{
		"build":   rawJSON(t, "build-model"),
		"default": rawJSON(t, "root-model"),
	})
	sess := &ActiveSession{}
	models, _ := rp.ResolveForTurn(sess, protocol.ModeBuild)
	if len(models) != 1 || models[0] != "build-model" {
		t.Errorf("mode routing: models=%v", models)
	}
}

func TestHandleModelList_EmptyServer(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)

	raw, err := handleModelList(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleModelList: %v", err)
	}
	resp := raw.(*protocol.ModelListResponse)
	if len(resp.Models) != 0 {
		t.Errorf("no providers configured → no models")
	}
	if resp.RoutingPolicy == nil {
		t.Errorf("RoutingPolicy should always be populated (even empty)")
	}
}

func TestHandleModelList_WithProvidersAndRouting(t *testing.T) {
	srv := newServerWithRealStore(t)
	_ = srv.providers.Add(&ProviderEndpoint{Name: "a1", Type: ProviderTypeAnthropic, APIKey: "k"})
	srv.models.Refresh()
	srv.routing.Replace(protocol.RoutingPolicy{
		"default": rawJSON(t, "claude-sonnet-4-6"),
	})

	c := newReadyConnection(t, srv)
	raw, err := handleModelList(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("handleModelList: %v", err)
	}
	resp := raw.(*protocol.ModelListResponse)
	if len(resp.Models) == 0 {
		t.Errorf("expected Anthropic builtins")
	}
	if _, ok := resp.RoutingPolicy["default"]; !ok {
		t.Errorf("routing tree not surfaced")
	}
}

func TestHandleModelSwitch_Apply(t *testing.T) {
	srv := newServerWithRealStore(t)
	_ = srv.providers.Add(&ProviderEndpoint{Name: "a1", Type: ProviderTypeAnthropic, APIKey: "k"})
	srv.models.Refresh()

	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "switch test")

	c := newReadyConnection(t, srv)
	c.SetSessionID(sid)
	srv.sessions.EnsureActive(sid, c)

	params, _ := json.Marshal(&protocol.ModelSwitch{SessionID: sid, Model: "claude-sonnet-4-6"})
	raw, err := handleModelSwitch(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleModelSwitch: %v", err)
	}
	resp := raw.(*protocol.ModelSwitchResponse)
	if !resp.OverrideSet || resp.Model != "claude-sonnet-4-6" {
		t.Errorf("response = %+v", resp)
	}
	active := srv.sessions.GetActiveSession(sid)
	if active.ModelOverride != "claude-sonnet-4-6" {
		t.Errorf("active.ModelOverride = %q", active.ModelOverride)
	}

	// Persisted to storage.
	persisted, _ := srv.store.GetSession(context.Background(), sid)
	if persisted.ModelOverride == nil || *persisted.ModelOverride != "claude-sonnet-4-6" {
		t.Errorf("persisted override = %v", persisted.ModelOverride)
	}
}

func TestHandleModelSwitch_Clear(t *testing.T) {
	srv := newServerWithRealStore(t)
	_ = srv.providers.Add(&ProviderEndpoint{Name: "a1", Type: ProviderTypeAnthropic, APIKey: "k"})
	srv.models.Refresh()

	override := "claude-sonnet-4-6"
	sess := &storage.Session{
		ID:            "sess-clear",
		UserID:        SoloUserID,
		Project:       "/tmp/proj",
		Status:        "active",
		ModelOverride: &override,
	}
	if err := srv.store.CreateSession(context.Background(), sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	c := newReadyConnection(t, srv)
	srv.sessions.EnsureActive(sess.ID, c)

	params, _ := json.Marshal(&protocol.ModelSwitch{SessionID: sess.ID, Model: "auto"})
	raw, err := handleModelSwitch(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleModelSwitch(auto): %v", err)
	}
	resp := raw.(*protocol.ModelSwitchResponse)
	if resp.OverrideSet {
		t.Errorf("OverrideSet should be false after clear")
	}
	persisted, _ := srv.store.GetSession(context.Background(), sess.ID)
	if persisted.ModelOverride != nil {
		t.Errorf("persisted override not cleared: %v", persisted.ModelOverride)
	}
}

func TestHandleModelSwitch_UnknownModel(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "bad model")

	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.ModelSwitch{SessionID: sid, Model: "does-not-exist"})
	_, err := handleModelSwitch(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected model-unavailable error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeModelUnavailable {
		t.Errorf("expected CodeModelUnavailable, got %T %v", err, err)
	}
}

func TestHandleModelSwitch_SessionNotFound(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.ModelSwitch{SessionID: "missing", Model: "x"})
	_, err := handleModelSwitch(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected not-found")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeSessionNotFound {
		t.Errorf("expected CodeSessionNotFound, got %T %v", err, err)
	}
}
