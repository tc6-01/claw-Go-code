package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

const defaultBashTimeout = 10 * time.Second

type bashInput struct {
	Command   string `json:"command"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type bashOutput struct {
	Command  string `json:"command"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out,omitempty"`
}

func newBashTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeDangerFull,
		spec: types.ToolSpec{
			Name:        "bash",
			Description: "Run a shell command inside the current workspace",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"},"timeout_ms":{"type":"integer","minimum":1}},"required":["command"]}`),
		},
		exec: executeBash,
	}
}

func executeBash(ctx context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error) {
	var req bashInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("bash: invalid input: %w", err)
	}
	if req.Command == "" {
		return nil, fmt.Errorf("bash: command is required")
	}

	timeout := defaultBashTimeout
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, "/bin/sh", "-lc", req.Command)
	cmd.Dir = env.WorkingDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	output := bashOutput{
		Command:  req.Command,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
	if err == nil {
		return &types.ToolResult{Output: rawJSON(output)}, nil
	}

	var exitErr *exec.ExitError
	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		output.ExitCode = -1
		output.TimedOut = true
		return &types.ToolResult{Output: rawJSON(output), Error: fmt.Sprintf("bash: command timed out after %s", timeout)}, nil
	}
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			output.ExitCode = status.ExitStatus()
		} else {
			output.ExitCode = exitErr.ExitCode()
		}
		return &types.ToolResult{Output: rawJSON(output), Error: fmt.Sprintf("bash: command exited with code %d", output.ExitCode)}, nil
	}
	return nil, fmt.Errorf("bash: run command: %w", err)
}
