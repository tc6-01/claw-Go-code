package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type toolExecutor struct {
	registry   Registry
	permission permissions.Engine
}

type ExecuteRequest struct {
	Call types.ToolCall
	Env  ToolEnv
}

type ExecuteResult struct {
	Message types.Message
	Trace   types.ToolTraceEntry
}

func NewExecutor(registry Registry, permission permissions.Engine) *toolExecutor {
	return &toolExecutor{registry: registry, permission: permission}
}

func (e *toolExecutor) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	tool, ok := e.registry.Get(req.Call.Name)
	if !ok {
		return nil, fmt.Errorf("tool %s not found", req.Call.Name)
	}

	startedAt := time.Now().UTC()
	trace := types.ToolTraceEntry{Name: req.Call.Name, StartedAt: startedAt}

	decision, err := e.permission.Decide(ctx, permissions.PermissionRequest{
		ToolName:    req.Call.Name,
		CurrentMode: req.Env.Mode,
		Required:    permissions.ModeWorkspaceWrite,
	})
	if err != nil {
		trace.EndedAt = time.Now().UTC()
		trace.Success = false
		return nil, err
	}
	if decision.Decision != permissions.DecisionAllow {
		trace.EndedAt = time.Now().UTC()
		trace.Success = false
		return &ExecuteResult{
			Message: types.Message{
				Role:    types.RoleTool,
				Name:    req.Call.Name,
				Content: mustJSON(map[string]any{"error": decision.Reason}),
			},
			Trace: trace,
		}, nil
	}

	result, err := tool.Execute(ctx, req.Call.Input, req.Env)
	trace.EndedAt = time.Now().UTC()
	trace.Success = err == nil && result != nil && result.Error == ""
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("tool %s returned nil result", req.Call.Name)
	}
	if result.Name == "" {
		result.Name = req.Call.Name
	}
	if result.ToolCallID == "" {
		result.ToolCallID = req.Call.ID
	}

	return &ExecuteResult{
		Message: types.Message{
			Role:    types.RoleTool,
			Name:    req.Call.Name,
			Content: mustJSON(result),
		},
		Trace: trace,
	}, nil
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func SpecNames(specs []types.ToolSpec) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Name)
	}
	sort.Strings(out)
	return out
}
