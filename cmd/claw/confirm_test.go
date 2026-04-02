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

	allowed, err := confirmer.Confirm(context.Background(), permissions.PermissionRequest{
		ToolName:    "bash",
		CurrentMode: permissions.ModeWorkspaceWrite,
		Required:    permissions.ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if !allowed {
		t.Fatal("expected confirmation to allow")
	}
	if !strings.Contains(out.String(), "Allow tool bash?") {
		t.Fatalf("unexpected prompt output: %q", out.String())
	}
}

func TestTerminalConfirmerDeniesDefault(t *testing.T) {
	in := strings.NewReader("\n")
	var out bytes.Buffer
	confirmer := newTerminalConfirmer(in, &out)

	allowed, err := confirmer.Confirm(context.Background(), permissions.PermissionRequest{
		ToolName:    "web_fetch",
		CurrentMode: permissions.ModeWorkspaceWrite,
		Required:    permissions.ModeDangerFull,
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if allowed {
		t.Fatal("expected empty confirmation to deny")
	}
}
