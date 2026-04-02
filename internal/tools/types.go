package tools

import (
	"context"
	"encoding/json"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type Tool interface {
	Spec() types.ToolSpec
	Execute(ctx context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error)
}

type ToolEnv struct {
	WorkingDir string
	Mode       permissions.Mode
}
