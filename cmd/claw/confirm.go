package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"claude-go-code/internal/permissions"
)

func newTerminalConfirmer(in io.Reader, out io.Writer) permissions.Confirmer {
	return permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
		if _, err := fmt.Fprintf(out, "Allow tool %s? current=%s required=%s [y]es/[a]llow session/[n]o/[d]eny session: ", req.ToolName, req.CurrentMode, req.Required); err != nil {
			return permissions.ConfirmationOutcome{}, err
		}
		reader := bufio.NewReader(in)
		answer, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return permissions.ConfirmationOutcome{}, err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeOnce}, nil
		case "a", "always":
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeSession}, nil
		case "d", "deny-session", "never":
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionDeny, Scope: permissions.ConfirmationScopeSession}, nil
		default:
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionDeny, Scope: permissions.ConfirmationScopeOnce}, nil
		}
	})
}

func isTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
