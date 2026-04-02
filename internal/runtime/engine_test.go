package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
	sharedprovider "claude-go-code/internal/provider"
	"claude-go-code/internal/session"
	"claude-go-code/internal/tools"
	"claude-go-code/pkg/types"
)

func TestEngineRunCreatesBootstrapSession(t *testing.T) {
	store := session.NewInMemoryStore()
	engine := NewEngine(Dependencies{
		Config:          config.DefaultConfig(t.TempDir()),
		SessionStore:    store,
		ProviderFactory: sharedprovider.NewStaticFactory(),
		ToolRegistry:    tools.NewRegistry(nil),
		Permission:      permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if sess.Model != "claude-sonnet-4-5" {
		t.Fatalf("unexpected session model: %s", sess.Model)
	}
	if len(sess.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(sess.Messages))
	}
	if sess.Messages[0].Role != types.RoleUser {
		t.Fatalf("unexpected bootstrap role: %s", sess.Messages[0].Role)
	}
	if sess.Messages[0].Content != "bootstrap:[status]" {
		t.Fatalf("unexpected bootstrap message: %q", sess.Messages[0].Content)
	}
	if sess.Messages[1].Role != types.RoleAssistant {
		t.Fatalf("unexpected assistant role: %s", sess.Messages[1].Role)
	}
	if sess.Messages[1].Usage.TotalTokens == 0 {
		t.Fatalf("expected assistant usage to persist")
	}
	if sess.Messages[1].StopReason != "stop" {
		t.Fatalf("unexpected stop reason: %q", sess.Messages[1].StopReason)
	}
	if len(sess.Usage) != 1 || sess.Usage[0].TotalTokens == 0 {
		t.Fatalf("expected session usage aggregate, got %#v", sess.Usage)
	}
}

func TestEngineRunExecutesToolCallsAndPersistsTrace(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	store := session.NewInMemoryStore()
	registry := tools.NewRegistry([]tools.Tool{stubTool{name: "echo"}})
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("calling tool"),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-1", Name: "echo", Input: json.RawMessage(`{"value":"hi"}`)}),
			sharedprovider.UsageEvent(types.Usage{InputTokens: 2, OutputTokens: 1, TotalTokens: 3}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.UsageEvent(types.Usage{InputTokens: 3, OutputTokens: 1, TotalTokens: 4}),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: registry,
		Permission:   permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.Messages) != 4 {
		t.Fatalf("message count = %d, want 4", len(sess.Messages))
	}
	if sess.Messages[1].Content != "calling tool" {
		t.Fatalf("unexpected assistant content: %q", sess.Messages[1].Content)
	}
	if len(sess.Messages[1].ToolCalls) != 1 || sess.Messages[1].ToolCalls[0].ID != "call-1" {
		t.Fatalf("assistant tool call trace mismatch: %#v", sess.Messages[1].ToolCalls)
	}
	if sess.Messages[1].Usage.TotalTokens != 3 {
		t.Fatalf("unexpected assistant usage: %#v", sess.Messages[1].Usage)
	}
	if sess.Messages[2].Role != types.RoleTool {
		t.Fatalf("expected tool role, got %s", sess.Messages[2].Role)
	}
	if sess.Messages[2].Content != `{"value":"hi"}` {
		t.Fatalf("unexpected tool output: %q", sess.Messages[2].Content)
	}
	if sess.Messages[2].ToolResult == nil || sess.Messages[2].ToolResult.ToolCallID != "call-1" {
		t.Fatalf("unexpected tool result trace: %#v", sess.Messages[2].ToolResult)
	}
	if sess.Messages[3].Content != "done" {
		t.Fatalf("unexpected final assistant content: %q", sess.Messages[3].Content)
	}
	if len(sess.Usage) != 2 || sess.Usage[0].TotalTokens != 3 || sess.Usage[1].TotalTokens != 4 {
		t.Fatalf("unexpected session usage trace: %#v", sess.Usage)
	}
	if len(sess.ToolTrace) != 1 {
		t.Fatalf("tool trace count = %d, want 1", len(sess.ToolTrace))
	}
	if !sess.ToolTrace[0].Success {
		t.Fatal("expected successful tool trace")
	}
	if sess.ToolTrace[0].Name != "echo" {
		t.Fatalf("unexpected tool trace name: %s", sess.ToolTrace[0].Name)
	}
	if sess.ToolTrace[0].ID != "call-1" {
		t.Fatalf("unexpected tool trace id: %s", sess.ToolTrace[0].ID)
	}
	if sess.ToolTrace[0].Result == nil || sess.ToolTrace[0].Result.ToolCallID != "call-1" {
		t.Fatalf("unexpected tool trace result: %#v", sess.ToolTrace[0].Result)
	}
}

type scriptedProvider struct {
	events [][]*sharedprovider.StreamEvent
	index  int
}

func (p *scriptedProvider) Send(context.Context, *types.MessageRequest) (*types.MessageResponse, error) {
	return &types.MessageResponse{}, nil
}

func (p *scriptedProvider) Stream(_ context.Context, _ *types.MessageRequest) (sharedprovider.StreamReader, error) {
	if p.index >= len(p.events) {
		return &sharedprovider.SliceStreamReader{Events: []*sharedprovider.StreamEvent{sharedprovider.MessageStartEvent(), sharedprovider.StopEvent()}}, nil
	}
	reader := &sharedprovider.SliceStreamReader{Events: p.events[p.index]}
	p.index++
	return reader, nil
}

func (p *scriptedProvider) NormalizeModel(model string) string { return model }

func (p *scriptedProvider) Capabilities() sharedprovider.ProviderCapabilities {
	return sharedprovider.ProviderCapabilities{Streaming: true, ToolCalls: true}
}

type stubTool struct {
	name string
}

func (t stubTool) Spec() types.ToolSpec {
	return types.ToolSpec{Name: t.name}
}

func (t stubTool) Execute(_ context.Context, input json.RawMessage, _ tools.ToolEnv) (*types.ToolResult, error) {
	return &types.ToolResult{ToolCallID: "call-1", Name: t.name, Output: append(json.RawMessage(nil), input...)}, nil
}
