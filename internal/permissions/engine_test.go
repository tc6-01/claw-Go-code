package permissions

import (
	"context"
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
