package permissions

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStaticEngineAllowsWhenModeIsSufficient(t *testing.T) {
	engine := NewStaticEngine(ModeWorkspaceWrite)
	decision, err := engine.Decide(context.Background(), PermissionRequest{
		ToolName:    "write_file",
		CurrentMode: ModeWorkspaceWrite,
		Required:    ModeWorkspaceWrite,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionAllow {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestStaticEngineDeniesWhenEscalationPolicyIsDeny(t *testing.T) {
	engine := NewStaticEngineWithOptions(Options{DefaultMode: ModeWorkspaceWrite, EscalationPolicy: EscalationDeny})
	decision, err := engine.Decide(context.Background(), PermissionRequest{
		ToolName:    "bash",
		CurrentMode: ModeWorkspaceWrite,
		Required:    ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionDeny {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if decision.Reason == "" {
		t.Fatal("expected denial reason")
	}
}

func TestStaticEnginePromptsWhenEscalationPolicyIsPrompt(t *testing.T) {
	engine := NewStaticEngineWithOptions(Options{DefaultMode: ModeWorkspaceWrite, EscalationPolicy: EscalationPrompt})
	decision, err := engine.Decide(context.Background(), PermissionRequest{
		ToolName:    "bash",
		CurrentMode: ModeWorkspaceWrite,
		Required:    ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionPrompt {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestStaticEngineUsesConfirmerToAllow(t *testing.T) {
	callCount := 0
	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		Confirmer: ConfirmFunc(func(_ context.Context, req PermissionRequest) (ConfirmationOutcome, error) {
			callCount++
			if req.ToolName != "bash" || req.Required != ModeDangerFull {
				t.Fatalf("unexpected request: %#v", req)
			}
			return ConfirmationOutcome{Decision: DecisionAllow, Scope: ConfirmationScopeOnce}, nil
		}),
	})
	decision, err := engine.Decide(context.Background(), PermissionRequest{
		ToolName:    "bash",
		CurrentMode: ModeWorkspaceWrite,
		Required:    ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionAllow {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if callCount != 1 {
		t.Fatalf("call count = %d, want 1", callCount)
	}
}

func TestStaticEngineUsesConfirmerToDeny(t *testing.T) {
	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			return ConfirmationOutcome{Decision: DecisionDeny, Scope: ConfirmationScopeOnce}, nil
		}),
	})
	decision, err := engine.Decide(context.Background(), PermissionRequest{
		ToolName:    "bash",
		CurrentMode: ModeWorkspaceWrite,
		Required:    ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionDeny {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestStaticEngineCachesSessionAllowDecision(t *testing.T) {
	callCount := 0
	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			callCount++
			return ConfirmationOutcome{Decision: DecisionAllow, Scope: ConfirmationScopeSession}, nil
		}),
	})
	req := PermissionRequest{ToolName: "bash", CurrentMode: ModeWorkspaceWrite, Required: ModeDangerFull}
	for i := 0; i < 2; i++ {
		decision, err := engine.Decide(context.Background(), req)
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		if decision == nil || decision.Decision != DecisionAllow {
			t.Fatalf("unexpected decision on pass %d: %#v", i, decision)
		}
	}
	if callCount != 1 {
		t.Fatalf("call count = %d, want 1", callCount)
	}
}

func TestStaticEngineCachesSessionDenyDecision(t *testing.T) {
	callCount := 0
	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			callCount++
			return ConfirmationOutcome{Decision: DecisionDeny, Scope: ConfirmationScopeSession}, nil
		}),
	})
	req := PermissionRequest{ToolName: "bash", CurrentMode: ModeWorkspaceWrite, Required: ModeDangerFull}
	for i := 0; i < 2; i++ {
		decision, err := engine.Decide(context.Background(), req)
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		if decision == nil || decision.Decision != DecisionDeny {
			t.Fatalf("unexpected decision on pass %d: %#v", i, decision)
		}
	}
	if callCount != 1 {
		t.Fatalf("call count = %d, want 1", callCount)
	}
}

func TestStaticEnginePersistsRuleAllowDecisionAcrossInstances(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	callCount := 0
	req := PermissionRequest{ToolName: "bash", CurrentMode: ModeWorkspaceWrite, Required: ModeDangerFull}

	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			callCount++
			return ConfirmationOutcome{Decision: DecisionAllow, Scope: ConfirmationScopeRule}, nil
		}),
	})
	decision, err := engine.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionAllow {
		t.Fatalf("unexpected persisted allow decision: %#v", decision)
	}

	reloaded := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
	})
	decision, err = reloaded.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() on reloaded engine error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionAllow {
		t.Fatalf("unexpected reloaded decision: %#v", decision)
	}
	if callCount != 1 {
		t.Fatalf("call count = %d, want 1", callCount)
	}
}

func TestStaticEnginePersistsRuleDenyDecisionAcrossInstances(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	callCount := 0
	req := PermissionRequest{ToolName: "bash", CurrentMode: ModeWorkspaceWrite, Required: ModeDangerFull}

	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			callCount++
			return ConfirmationOutcome{Decision: DecisionDeny, Scope: ConfirmationScopeRule}, nil
		}),
	})
	decision, err := engine.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionDeny {
		t.Fatalf("unexpected persisted deny decision: %#v", decision)
	}

	reloaded := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
	})
	decision, err = reloaded.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() on reloaded engine error = %v", err)
	}
	if decision == nil || decision.Decision != DecisionDeny {
		t.Fatalf("unexpected reloaded decision: %#v", decision)
	}
	if callCount != 1 {
		t.Fatalf("call count = %d, want 1", callCount)
	}
}

func TestStaticEngineRuleApprovalIsScopedByCommandPrefix(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			return ConfirmationOutcome{Decision: DecisionAllow, Scope: ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), RequestForToolCall("bash", ModeWorkspaceWrite, ModeDangerFull, []byte(`{"command":"git status"}`))); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	reloaded := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
	})
	allowDecision, err := reloaded.Decide(context.Background(), RequestForToolCall("bash", ModeWorkspaceWrite, ModeDangerFull, []byte(`{"command":"git diff"}`)))
	if err != nil {
		t.Fatalf("Decide() allow error = %v", err)
	}
	if allowDecision == nil || allowDecision.Decision != DecisionAllow {
		t.Fatalf("unexpected allow decision: %#v", allowDecision)
	}
	promptDecision, err := reloaded.Decide(context.Background(), RequestForToolCall("bash", ModeWorkspaceWrite, ModeDangerFull, []byte(`{"command":"npm test"}`)))
	if err != nil {
		t.Fatalf("Decide() prompt error = %v", err)
	}
	if promptDecision == nil || promptDecision.Decision != DecisionPrompt {
		t.Fatalf("unexpected prompt decision: %#v", promptDecision)
	}
}

func TestStaticEngineRuleDenialIsScopedByHost(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			return ConfirmationOutcome{Decision: DecisionDeny, Scope: ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), RequestForToolCall("web_fetch", ModeWorkspaceWrite, ModeDangerFull, []byte(`{"url":"https://example.com/docs"}`))); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	reloaded := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeWorkspaceWrite,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
	})
	denyDecision, err := reloaded.Decide(context.Background(), RequestForToolCall("web_fetch", ModeWorkspaceWrite, ModeDangerFull, []byte(`{"url":"https://example.com/blog"}`)))
	if err != nil {
		t.Fatalf("Decide() deny error = %v", err)
	}
	if denyDecision == nil || denyDecision.Decision != DecisionDeny {
		t.Fatalf("unexpected deny decision: %#v", denyDecision)
	}
	promptDecision, err := reloaded.Decide(context.Background(), RequestForToolCall("web_fetch", ModeWorkspaceWrite, ModeDangerFull, []byte(`{"url":"https://other.example.net/"}`)))
	if err != nil {
		t.Fatalf("Decide() prompt error = %v", err)
	}
	if promptDecision == nil || promptDecision.Decision != DecisionPrompt {
		t.Fatalf("unexpected prompt decision: %#v", promptDecision)
	}
}

func TestStaticEngineRuleApprovalIsScopedByPath(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	engine := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeReadOnly,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
		Confirmer: ConfirmFunc(func(_ context.Context, _ PermissionRequest) (ConfirmationOutcome, error) {
			return ConfirmationOutcome{Decision: DecisionAllow, Scope: ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), RequestForToolCall("write_file", ModeReadOnly, ModeWorkspaceWrite, []byte(`{"path":"docs/README.md","content":"hi"}`))); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	reloaded := NewStaticEngineWithOptions(Options{
		DefaultMode:      ModeReadOnly,
		EscalationPolicy: EscalationPrompt,
		RuleCachePath:    rulesPath,
	})
	allowDecision, err := reloaded.Decide(context.Background(), RequestForToolCall("write_file", ModeReadOnly, ModeWorkspaceWrite, []byte(`{"path":"./docs/README.md","content":"hello"}`)))
	if err != nil {
		t.Fatalf("Decide() allow error = %v", err)
	}
	if allowDecision == nil || allowDecision.Decision != DecisionAllow {
		t.Fatalf("unexpected allow decision: %#v", allowDecision)
	}
	promptDecision, err := reloaded.Decide(context.Background(), RequestForToolCall("write_file", ModeReadOnly, ModeWorkspaceWrite, []byte(`{"path":"docs/guide.md","content":"hello"}`)))
	if err != nil {
		t.Fatalf("Decide() prompt error = %v", err)
	}
	if promptDecision == nil || promptDecision.Decision != DecisionPrompt {
		t.Fatalf("unexpected prompt decision: %#v", promptDecision)
	}
}
