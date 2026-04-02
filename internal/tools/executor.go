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

	startedAt := time.Now().UTC()
	trace := types.ToolTraceEntry{
		ID:        req.Call.ID,
		Name:      req.Call.Name,
		Input:     stringifyJSON(req.Call.Input),
		StartedAt: startedAt,
	}

	decision, err := e.permission.Decide(ctx, permissions.RequestForToolCall(
		req.Call.Name,
		req.Env.Mode,
		requiredMode(tool),
		req.Call.Input,
	))
	if err != nil {
		trace.EndedAt = time.Now().UTC()
		trace.Success = false
		return nil, err
	}
	if decision != nil {
		trace.Permission = string(decision.Decision)
	}
	if decision == nil || decision.Decision != permissions.DecisionAllow {
		reason := "permission decision unavailable"
		if decision != nil && decision.Reason != "" {
			reason = decision.Reason
		}
		trace.EndedAt = time.Now().UTC()
		trace.Success = false
		trace.Error = reason
		toolResult := &types.ToolResult{
			ToolCallID: req.Call.ID,
			Name:       req.Call.Name,
			Error:      reason,
		}
		message := types.Message{
			Role:       types.RoleTool,
			Name:       req.Call.Name,
			Content:    mustJSON(toolResult),
			CreatedAt:  startedAt,
			Metadata:   map[string]string{"tool_call_id": req.Call.ID},
			ToolCalls:  []types.ToolCall{*cloneToolCall(&req.Call)},
			ToolResult: cloneToolResult(toolResult),
		}
		trace.Result = cloneToolResult(toolResult)
		trace.Output = message.Content
		return &ExecuteResult{
			Message: message,
			Trace:   trace,
			Result:  cloneToolResult(toolResult),
		}, nil
	}

	result, err := tool.Execute(ctx, req.Call.Input, req.Env)
	trace.EndedAt = time.Now().UTC()
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
	trace.Success = result.Error == ""
	trace.Result = cloneToolResult(result)

	message := types.Message{
		Role:       types.RoleTool,
		Name:       req.Call.Name,
		Content:    mustJSON(result),
		CreatedAt:  startedAt,
		Metadata:   map[string]string{"tool_call_id": req.Call.ID},
		ToolCalls:  []types.ToolCall{*cloneToolCall(&req.Call)},
		ToolResult: cloneToolResult(result),
	}
	if result.Error != "" {
		trace.Error = result.Error
		message.Metadata["tool_error"] = result.Error
	}
	trace.Output = message.Content

	return &ExecuteResult{
		Message: message,
		Trace:   trace,
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

func stringifyJSON(raw json.RawMessage) string {
	trimmed := string(raw)
	if len(raw) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return trimmed
	}
	formatted, err := json.Marshal(value)
	if err != nil {
		return trimmed
	}
	return string(formatted)
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
