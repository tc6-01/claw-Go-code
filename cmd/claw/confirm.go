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
	return permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (bool, error) {
		if _, err := fmt.Fprintf(out, "Allow tool %s? current=%s required=%s [y/N]: ", req.ToolName, req.CurrentMode, req.Required); err != nil {
			return false, err
		}
		reader := bufio.NewReader(in)
		answer, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
			return true, nil
		default:
			return false, nil
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
