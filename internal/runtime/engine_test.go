package runtime

import (
	"context"
	"testing"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
	"claude-go-code/internal/provider"
	"claude-go-code/internal/session"
	"claude-go-code/internal/tools"
)

func TestEngineRunCreatesBootstrapSession(t *testing.T) {
	store := session.NewInMemoryStore()
	engine := NewEngine(Dependencies{
		Config:          config.DefaultConfig(t.TempDir()),
		SessionStore:    store,
		ProviderFactory: provider.NewStaticFactory(),
		ToolRegistry:    tools.NewRegistry(nil),
		Permission:      permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sessions, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "bootstrap-session" {
		t.Fatalf("unexpected session id: %s", sessions[0].ID)
	}
}
