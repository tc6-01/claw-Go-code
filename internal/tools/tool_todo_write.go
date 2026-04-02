package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type todoWriteInput struct {
	Todos []types.TodoItem `json:"todos"`
}

type todoWriteOutput struct {
	Todos []types.TodoItem `json:"todos"`
	Count int              `json:"count"`
}

func newTodoWriteTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeWorkspaceWrite,
		spec: types.ToolSpec{
			Name:        "todo_write",
			Description: "Replace the current session todo list",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"todos":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"done":{"type":"boolean"}},"required":["content"]}}},"required":["todos"]}`),
		},
		exec: executeTodoWrite,
	}
}

func executeTodoWrite(_ context.Context, input json.RawMessage, _ ToolEnv) (*types.ToolResult, error) {
	var req todoWriteInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("todo_write: invalid input: %w", err)
	}
	for i := range req.Todos {
		req.Todos[i].Content = strings.TrimSpace(req.Todos[i].Content)
		if req.Todos[i].Content == "" {
			return nil, fmt.Errorf("todo_write: todos[%d].content is required", i)
		}
	}
	return &types.ToolResult{Output: rawJSON(todoWriteOutput{Todos: req.Todos, Count: len(req.Todos)})}, nil
}
