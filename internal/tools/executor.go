package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

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
	Result  *types.ToolResult
}

func NewExecutor(registry Registry, permission permissions.Engine) *toolExecutor {
	return &toolExecutor{registry: registry, permission: permission}
}

func (e *toolExecutor) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	tool, ok := e.registry.Get(req.Call.Name)
	if !ok {
		return nil, fmt.Errorf("tool %s not found", req.Call.Name)
	}

	decision, err := e.permission.Decide(ctx, permissions.RequestForToolCall(
		req.Call.Name,
		req.Env.Mode,
		requiredMode(tool),
		req.Call.Input,
	))
	if err != nil {
		return nil, err
	}
	if decision == nil || decision.Decision != permissions.DecisionAllow {
		reason := "permission decision unavailable"
		if decision != nil && decision.Reason != "" {
			reason = decision.Reason
		}
		toolResult := &types.ToolResult{
			ToolCallID: req.Call.ID,
			Name:       req.Call.Name,
			Error:      reason,
		}
		message := types.Message{
			Role:       types.RoleTool,
			Name:       req.Call.Name,
			Content:    mustJSON(toolResult),
			Metadata:   map[string]string{"tool_call_id": req.Call.ID},
			ToolCalls:  []types.ToolCall{*cloneToolCall(&req.Call)},
			ToolResult: cloneToolResult(toolResult),
		}
		return &ExecuteResult{
			Message: message,
			Result:  cloneToolResult(toolResult),
		}, nil
	}

	result, err := tool.Execute(ctx, req.Call.Input, req.Env)
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

	message := types.Message{
		Role:       types.RoleTool,
		Name:       req.Call.Name,
		Content:    mustJSON(result),
		Metadata:   map[string]string{"tool_call_id": req.Call.ID},
		ToolCalls:  []types.ToolCall{*cloneToolCall(&req.Call)},
		ToolResult: cloneToolResult(result),
	}
	if result.Error != "" {
		message.Metadata["tool_error"] = result.Error
	}

	return &ExecuteResult{
		Message: message,
		Result:  cloneToolResult(result),
	}, nil
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func cloneToolCall(call *types.ToolCall) *types.ToolCall {
	if call == nil {
		return nil
	}
	cloned := *call
	cloned.Input = append(json.RawMessage(nil), call.Input...)
	return &cloned
}

func cloneToolResult(result *types.ToolResult) *types.ToolResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.Output = append(json.RawMessage(nil), result.Output...)
	return &cloned
}

func SpecNames(specs []types.ToolSpec) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Name)
	}
	sort.Strings(out)
	return out
}
