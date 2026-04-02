package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type readFileInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type readFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Offset    int    `json:"offset,omitempty"`
	BytesRead int    `json:"bytes_read"`
	Truncated bool   `json:"truncated,omitempty"`
}

func newReadFileTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeReadOnly,
		spec: types.ToolSpec{
			Name:        "read_file",
			Description: "Read a UTF-8 text file from the current workspace",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":0}},"required":["path"]}`),
		},
		exec: executeReadFile,
	}
}

func executeReadFile(_ context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error) {
	var req readFileInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("read_file: invalid input: %w", err)
	}
	if req.Path == "" {
		return nil, fmt.Errorf("read_file: path is required")
	}
	if req.Offset < 0 || req.Limit < 0 {
		return nil, fmt.Errorf("read_file: offset and limit must be >= 0")
	}

	data, relPath, err := readWorkspaceTextFile(env.WorkingDir, req.Path)
	if err != nil {
		return &types.ToolResult{
			Output: rawJSON(map[string]any{"path": relPath}),
			Error:  err.Error(),
		}, nil
	}

	start := req.Offset
	if start > len(data) {
		start = len(data)
	}
	end := len(data)
	truncated := false
	if req.Limit > 0 && start+req.Limit < end {
		end = start + req.Limit
		truncated = true
	}
	chunk := data[start:end]

	return &types.ToolResult{Output: rawJSON(readFileOutput{
		Path:      relPath,
		Content:   string(chunk),
		Offset:    start,
		BytesRead: len(chunk),
		Truncated: truncated,
	})}, nil
}
