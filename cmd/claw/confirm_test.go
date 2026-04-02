package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"claude-go-code/internal/permissions"
)

func TestTerminalConfirmerAllowsYes(t *testing.T) {
	in := strings.NewReader("y\n")
	var out bytes.Buffer
	confirmer := newTerminalConfirmer(in, &out)

	outcome, err := confirmer.Confirm(context.Background(), permissions.PermissionRequest{
		ToolName:    "bash",
		CurrentMode: permissions.ModeWorkspaceWrite,
		Required:    permissions.ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if outcome.Decision != permissions.DecisionAllow || outcome.Scope != permissions.ConfirmationScopeOnce {
		t.Fatalf("unexpected confirmation outcome: %#v", outcome)
	}
	if !strings.Contains(out.String(), "Allow tool bash?") {
		t.Fatalf("unexpected prompt output: %q", out.String())
	}
}

func TestTerminalConfirmerDeniesDefault(t *testing.T) {
	in := strings.NewReader("\n")
	var out bytes.Buffer
	confirmer := newTerminalConfirmer(in, &out)

	outcome, err := confirmer.Confirm(context.Background(), permissions.PermissionRequest{
		ToolName:    "web_fetch",
		CurrentMode: permissions.ModeWorkspaceWrite,
		Required:    permissions.ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if outcome.Decision != permissions.DecisionDeny || outcome.Scope != permissions.ConfirmationScopeOnce {
		t.Fatalf("unexpected denial outcome: %#v", outcome)
	}
}

func TestTerminalConfirmerAllowsSession(t *testing.T) {
	in := strings.NewReader("a\n")
	var out bytes.Buffer
	confirmer := newTerminalConfirmer(in, &out)

	outcome, err := confirmer.Confirm(context.Background(), permissions.PermissionRequest{
		ToolName:    "bash",
		CurrentMode: permissions.ModeWorkspaceWrite,
		Required:    permissions.ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if outcome.Decision != permissions.DecisionAllow || outcome.Scope != permissions.ConfirmationScopeSession {
		t.Fatalf("unexpected session-allow outcome: %#v", outcome)
	}
}

func TestTerminalConfirmerDeniesSession(t *testing.T) {
	in := strings.NewReader("d\n")
	var out bytes.Buffer
	confirmer := newTerminalConfirmer(in, &out)

	outcome, err := confirmer.Confirm(context.Background(), permissions.PermissionRequest{
		ToolName:    "web_fetch",
		CurrentMode: permissions.ModeWorkspaceWrite,
		Required:    permissions.ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if outcome.Decision != permissions.DecisionDeny || outcome.Scope != permissions.ConfirmationScopeSession {
		t.Fatalf("unexpected session-deny outcome: %#v", outcome)
	}
}
