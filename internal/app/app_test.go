package app

import (
	"context"
	"strings"
	"testing"

	"claude-go-code/internal/config"
)

func TestNewBuildsRunnableApp(t *testing.T) {
	cfg, err := config.Load(context.Background(), config.LoadOptions{WorkingDir: t.TempDir()})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if application.Runtime == nil {
		t.Fatal("expected runtime to be initialized")
	}
	if err := application.Run(context.Background(), []string{"status"}); err != nil {
		t.Fatalf("run app: %v", err)
	}

	sess, err := application.SessionStore.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(sess.Messages))
	}
	if sess.Messages[0].Role != "user" {
		t.Fatalf("unexpected bootstrap role %q", sess.Messages[0].Role)
	}
	if !strings.Contains(sess.Messages[1].Content, "anthropic stub response:") {
		t.Fatalf("unexpected default provider response %q", sess.Messages[1].Content)
	}
	if sess.Messages[1].Usage.TotalTokens == 0 {
		t.Fatal("expected persisted usage on assistant message")
	}
}

func TestNewUsesConfiguredOpenAIProvider(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "openai"
	cfg.Provider.DefaultModel = "gpt-4o-mini"

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if err := application.Run(context.Background(), []string{"status"}); err != nil {
		t.Fatalf("run app: %v", err)
	}

	sess, err := application.SessionStore.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(sess.Messages))
	}
	if sess.Messages[0].Role != "user" {
		t.Fatalf("unexpected bootstrap role %q", sess.Messages[0].Role)
	}
	if !strings.Contains(sess.Messages[1].Content, "openai stub response:") {
		t.Fatalf("unexpected openai provider response %q", sess.Messages[1].Content)
	}
	if sess.Messages[1].Usage.TotalTokens == 0 {
		t.Fatal("expected persisted usage on assistant message")
	}
}
