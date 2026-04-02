package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type editFileInput struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type editFileOutput struct {
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
	BytesWritten int    `json:"bytes_written"`
}

func newEditFileTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeWorkspaceWrite,
		spec: types.ToolSpec{
			Name:        "edit_file",
			Description: "Replace exact text in a UTF-8 text file inside the current workspace",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"},"replace_all":{"type":"boolean"}},"required":["path","old_string","new_string"]}`),
		},
		exec: executeEditFile,
	}
}

func executeEditFile(_ context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error) {
	var req editFileInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("edit_file: invalid input: %w", err)
	}
	if req.Path == "" {
		return nil, fmt.Errorf("edit_file: path is required")
	}
	if req.OldString == "" {
		return nil, fmt.Errorf("edit_file: old_string is required")
	}

	absPath, relPath, err := resolveWorkspacePath(env.WorkingDir, req.Path)
	if err != nil {
		return &types.ToolResult{Error: err.Error()}, nil
	}
	data, _, err := readWorkspaceTextFile(env.WorkingDir, req.Path)
	if err != nil {
		return &types.ToolResult{Output: rawJSON(map[string]any{"path": relPath}), Error: err.Error()}, nil
	}
	contents := string(data)
	count := strings.Count(contents, req.OldString)
	if count == 0 {
		return &types.ToolResult{Output: rawJSON(map[string]any{"path": relPath}), Error: fmt.Sprintf("edit_file: target string not found in %s", relPath)}, nil
	}

	replacements := 1
	updated := strings.Replace(contents, req.OldString, req.NewString, 1)
	if req.ReplaceAll {
		replacements = count
		updated = strings.ReplaceAll(contents, req.OldString, req.NewString)
	}

	if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
		return &types.ToolResult{Output: rawJSON(map[string]any{"path": relPath}), Error: err.Error()}, nil
	}
	return &types.ToolResult{Output: rawJSON(editFileOutput{Path: relPath, Replacements: replacements, BytesWritten: len(updated)})}, nil
}
