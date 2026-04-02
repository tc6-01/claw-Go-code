package tools

import (
	"context"
	"encoding/json"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type modeAwareTool interface {
	RequiredMode() permissions.Mode
}

type builtinTool struct {
	spec         types.ToolSpec
	requiredMode permissions.Mode
	exec         func(ctx context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error)
}

func (t builtinTool) Spec() types.ToolSpec {
	spec := t.spec
	if spec.Permission == "" && t.requiredMode != "" {
		spec.Permission = string(t.requiredMode)
	}
	return spec
}

func (t builtinTool) RequiredMode() permissions.Mode {
	if t.requiredMode == "" {
		return permissions.ModeWorkspaceWrite
	}
	return t.requiredMode
}

func (t builtinTool) Execute(ctx context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error) {
	return t.exec(ctx, input, env)
}

func BuiltinTools() []Tool {
	return []Tool{
		newReadFileTool(),
		newWriteFileTool(),
		newEditFileTool(),
		newGlobSearchTool(),
		newGrepSearchTool(),
		newBashTool(),
		newTodoWriteTool(),
		newWebFetchTool(),
		newWebSearchTool(),
	}
}

func requiredMode(tool Tool) permissions.Mode {
	if tool == nil {
		return permissions.ModeWorkspaceWrite
	}
	if aware, ok := tool.(modeAwareTool); ok {
		if mode := aware.RequiredMode(); mode != "" {
			return mode
		}
	}
	if spec := tool.Spec(); spec.Permission != "" {
		if mode, err := permissions.ParseMode(spec.Permission); err == nil {
			return mode
		}
	}
	return permissions.ModeWorkspaceWrite
}

func rawJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"error":"json_marshal_failed"}`)
	}
	return data
}
