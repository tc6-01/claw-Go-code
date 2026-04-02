package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
)

func TestHandlePermissionRuleCommandList(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	engine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      permissions.ModeWorkspaceWrite,
		EscalationPolicy: permissions.EscalationPrompt,
		RuleCachePath:    cfg.Permission.RulesPath,
		Confirmer: permissions.ConfirmFunc(func(_ context.Context, _ permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), permissions.RequestForToolCall(
		"bash",
		permissions.ModeWorkspaceWrite,
		permissions.ModeDangerFull,
		[]byte(`{"command":"git status"}`),
	)); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{"permissions", "rules", "list"}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if !strings.Contains(out.String(), "command_prefix=git") {
		t.Fatalf("unexpected list output: %s", out.String())
	}
}

func TestHandlePermissionRuleCommandClear(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	engine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      permissions.ModeWorkspaceWrite,
		EscalationPolicy: permissions.EscalationPrompt,
		RuleCachePath:    cfg.Permission.RulesPath,
		Confirmer: permissions.ConfirmFunc(func(_ context.Context, _ permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionDeny, Scope: permissions.ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), permissions.RequestForToolCall(
		"web_fetch",
		permissions.ModeWorkspaceWrite,
		permissions.ModeDangerFull,
		[]byte(`{"url":"https://example.com/page"}`),
	)); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{"permissions", "rules", "clear"}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected rules to be cleared, got %#v", rules)
	}
}
