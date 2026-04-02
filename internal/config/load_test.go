package config

import (
	"context"
	"testing"

	"claude-go-code/internal/permissions"
)

func TestLoadReadsPermissionEscalationPolicy(t *testing.T) {
	t.Setenv("CLAW_PERMISSION_MODE", string(permissions.ModeReadOnly))
	t.Setenv("CLAW_PERMISSION_ESCALATION_POLICY", string(permissions.EscalationPrompt))

	cfg, err := Load(context.Background(), LoadOptions{WorkingDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Permission.Mode != permissions.ModeReadOnly {
		t.Fatalf("mode = %q", cfg.Permission.Mode)
	}
	if cfg.Permission.EscalationPolicy != permissions.EscalationPrompt {
		t.Fatalf("escalation policy = %q", cfg.Permission.EscalationPolicy)
	}
}

func TestLoadRejectsInvalidPermissionEscalationPolicy(t *testing.T) {
	t.Setenv("CLAW_PERMISSION_ESCALATION_POLICY", "surprise")
	if _, err := Load(context.Background(), LoadOptions{WorkingDir: t.TempDir()}); err == nil {
		t.Fatal("expected invalid escalation policy error")
	}
}
