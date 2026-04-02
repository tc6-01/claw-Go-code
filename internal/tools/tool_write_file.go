package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type writeFileOutput struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

func newWriteFileTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeWorkspaceWrite,
		spec: types.ToolSpec{
			Name:        "write_file",
			Description: "Write a UTF-8 text file inside the current workspace",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		},
		exec: executeWriteFile,
	}
}

func executeWriteFile(_ context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error) {
	var req writeFileInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("write_file: invalid input: %w", err)
	}
	if req.Path == "" {
		return nil, fmt.Errorf("write_file: path is required")
	}

	absPath, relPath, err := resolveWorkspacePath(env.WorkingDir, req.Path)
	if err != nil {
		return &types.ToolResult{Error: err.Error()}, nil
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("write_file: create parent dirs: %w", err)
	}
	if err := os.WriteFile(absPath, []byte(req.Content), 0o644); err != nil {
		return &types.ToolResult{Output: rawJSON(map[string]any{"path": relPath}), Error: err.Error()}, nil
	}
	return &types.ToolResult{Output: rawJSON(writeFileOutput{Path: relPath, BytesWritten: len(req.Content)})}, nil
}
