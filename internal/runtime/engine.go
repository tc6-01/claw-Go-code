package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
	"claude-go-code/internal/provider"
	"claude-go-code/internal/session"
	"claude-go-code/internal/tools"
	"claude-go-code/pkg/types"
)

const maxTurns = 8

type Invocation struct {
	Args []string
}

type Engine interface {
	Run(ctx context.Context, invocation Invocation) error
}

type Dependencies struct {
	Config          config.Config
	SessionStore    session.Store
	ProviderFactory provider.Factory
	ToolRegistry    tools.Registry
	Permission      permissions.Engine
}

type engine struct {
	deps Dependencies
}

type turnResult struct {
	assistant types.Message
	toolCalls []*types.ToolCall
}

func NewEngine(deps Dependencies) Engine {
	return &engine{deps: deps}
}

func (e *engine) Run(ctx context.Context, invocation Invocation) error {
	providerClient, model, err := e.resolveProvider()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	bootstrap := e.bootstrapMessage(invocation, now)
	sess := &types.Session{
		ID:             "bootstrap-session",
		Version:        types.CurrentSessionVersion,
		CreatedAt:      now,
		UpdatedAt:      now,
		CWD:            e.deps.Config.WorkingDir,
		Model:          model,
		PermissionMode: string(e.deps.Config.Permission.Mode),
		Messages:       []types.Message{bootstrap},
	}
	if err := e.deps.SessionStore.Create(ctx, sess); err != nil {
		return err
	}

	requestMessages := []types.Message{bootstrap}
	toolSpecs := e.deps.ToolRegistry.Specs()

	for turn := 0; turn < maxTurns; turn++ {
		result, err := e.runTurn(ctx, providerClient, model, requestMessages, toolSpecs)
		if err != nil {
			return err
		}

		if shouldPersistAssistant(result.assistant) {
			sess.Messages = append(sess.Messages, result.assistant)
			requestMessages = append(requestMessages, result.assistant)
			if hasUsage(result.assistant.Usage) {
				sess.Usage = append(sess.Usage, result.assistant.Usage)
			}
		}

		if len(result.toolCalls) == 0 {
			sess.UpdatedAt = time.Now().UTC()
			return e.deps.SessionStore.Save(ctx, sess)
		}

		for _, call := range result.toolCalls {
			toolMessage, trace := e.executeTool(ctx, call)
			sess.ToolTrace = append(sess.ToolTrace, trace)
			sess.Messages = append(sess.Messages, toolMessage)
			requestMessages = append(requestMessages, toolMessage)
		}
		sess.UpdatedAt = time.Now().UTC()
		if err := e.deps.SessionStore.Save(ctx, sess); err != nil {
			return err
		}
	}

	return fmt.Errorf("runtime exceeded max turns (%d)", maxTurns)
}

func (e *engine) resolveProvider() (provider.Provider, string, error) {
	resolvedModel := e.deps.Config.Provider.DefaultModel
	client, err := provider.FactoryProvider(
		e.deps.ProviderFactory,
		e.deps.Config.Provider.DefaultProvider,
		e.deps.Config.Provider.DefaultProvider,
		resolvedModel,
	)
	if err != nil {
		return nil, "", err
	}
	return client, client.NormalizeModel(resolvedModel), nil
}

func (e *engine) bootstrapMessage(invocation Invocation, now time.Time) types.Message {
	return types.Message{
		Role:      types.RoleUser,
		Content:   fmt.Sprintf("bootstrap:%v", invocation.Args),
		CreatedAt: now,
	}
}

func (e *engine) runTurn(ctx context.Context, providerClient provider.Provider, model string, messages []types.Message, toolSpecs []types.ToolSpec) (turnResult, error) {
	reader, err := providerClient.Stream(ctx, &types.MessageRequest{
		Model:    model,
		Messages: append([]types.Message(nil), messages...),
		Tools:    append([]types.ToolSpec(nil), toolSpecs...),
	})
	if err != nil {
		return turnResult{}, err
	}
	defer reader.Close()

	assistant := types.Message{
		Role:      types.RoleAssistant,
		CreatedAt: time.Now().UTC(),
	}
	toolCalls := make([]*types.ToolCall, 0)
	stopReason := ""
	usage := types.Usage{}

	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return turnResult{}, err
		}
		switch event.Type {
		case provider.StreamEventMessageDelta:
			assistant.Content += event.Text
		case provider.StreamEventToolCall:
			toolCalls = append(toolCalls, cloneToolCall(event.ToolCall))
		case provider.StreamEventUsage:
			if event.Usage != nil {
				usage = *event.Usage
			}
		case provider.StreamEventStop:
			stopReason = "stop"
		case provider.StreamEventError:
			if event.Error != nil {
				return turnResult{}, event.Error
			}
			return turnResult{}, fmt.Errorf("provider stream error")
		}
	}

	assistant.StopReason = stopReason
	assistant.Usage = usage
	assistant.ToolCalls = flattenToolCalls(toolCalls)
	assistant.Metadata = messageMetadata(stopReason, usage, len(toolCalls), nil)
	return turnResult{assistant: assistant, toolCalls: toolCalls}, nil
}

func (e *engine) executeTool(ctx context.Context, call *types.ToolCall) (types.Message, types.ToolTraceEntry) {
	startedAt := time.Now().UTC()
	trace := types.ToolTraceEntry{
		ID:        call.ID,
		Name:      call.Name,
		Input:     stringifyJSON(call.Input),
		StartedAt: startedAt,
		EndedAt:   startedAt,
	}
	message := types.Message{
		Role:      types.RoleTool,
		Name:      call.Name,
		CreatedAt: startedAt,
		Metadata: map[string]string{
			"tool_call_id": call.ID,
		},
		ToolCalls: flattenToolCalls([]*types.ToolCall{call}),
		ToolResult: &types.ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
		},
	}

	tool, ok := e.deps.ToolRegistry.Get(call.Name)
	if !ok {
		message.Content = fmt.Sprintf("tool %q not found", call.Name)
		message.Metadata["error"] = "tool_not_found"
		message.ToolResult.Error = message.Content
		trace.Error = message.Content
		trace.EndedAt = time.Now().UTC()
		return message, trace
	}

	decision, err := e.deps.Permission.Decide(ctx, permissions.PermissionRequest{
		ToolName:    call.Name,
		CurrentMode: e.deps.Config.Permission.Mode,
		Required:    e.deps.Config.Permission.Mode,
	})
	if err != nil {
		message.Content = err.Error()
		message.Metadata["error"] = "permission_error"
		message.ToolResult.Error = err.Error()
		trace.Error = err.Error()
		trace.EndedAt = time.Now().UTC()
		return message, trace
	}
	if decision != nil {
		trace.Permission = string(decision.Decision)
	}
	if decision != nil && decision.Decision == permissions.DecisionDeny {
		message.Content = decision.Reason
		message.Metadata["error"] = "permission_denied"
		message.ToolResult.Error = decision.Reason
		trace.Error = decision.Reason
		trace.EndedAt = time.Now().UTC()
		return message, trace
	}

	result, execErr := tool.Execute(ctx, call.Input, tools.ToolEnv{
		WorkingDir: e.deps.Config.WorkingDir,
		Mode:       e.deps.Config.Permission.Mode,
	})
	trace.EndedAt = time.Now().UTC()
	if execErr != nil {
		message.Content = execErr.Error()
		message.Metadata["error"] = "tool_execution_failed"
		message.ToolResult.Error = execErr.Error()
		trace.Error = execErr.Error()
		return message, trace
	}
	trace.Success = true
	message.Content = renderToolResult(result)
	message.ToolResult = cloneToolResult(result)
	trace.Result = cloneToolResult(result)
	trace.Output = message.Content
	if result != nil {
		if result.ToolCallID != "" {
			message.Metadata["tool_call_id"] = result.ToolCallID
		}
		if result.Error != "" {
			message.Metadata["tool_error"] = result.Error
			trace.Error = result.Error
		}
		if result.Name != "" {
			message.Name = result.Name
		}
	}
	return message, trace
}

func shouldPersistAssistant(message types.Message) bool {
	return strings.TrimSpace(message.Content) != "" || len(message.Metadata) > 0 || len(message.ToolCalls) > 0 || hasUsage(message.Usage)
}

func hasUsage(usage types.Usage) bool {
	return usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.TotalTokens > 0
}

func flattenToolCalls(calls []*types.ToolCall) []types.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	flattened := make([]types.ToolCall, 0, len(calls))
	for _, call := range calls {
		if call == nil {
			continue
		}
		flattened = append(flattened, *cloneToolCall(call))
	}
	if len(flattened) == 0 {
		return nil
	}
	return flattened
}

func messageMetadata(stopReason string, usage types.Usage, toolCallCount int, base map[string]string) map[string]string {
	metadata := make(map[string]string)
	for k, v := range base {
		metadata[k] = v
	}
	if stopReason != "" {
		metadata["stop_reason"] = stopReason
	}
	if usage.InputTokens > 0 {
		metadata["usage_input_tokens"] = fmt.Sprintf("%d", usage.InputTokens)
	}
	if usage.OutputTokens > 0 {
		metadata["usage_output_tokens"] = fmt.Sprintf("%d", usage.OutputTokens)
	}
	if usage.TotalTokens > 0 {
		metadata["usage_total_tokens"] = fmt.Sprintf("%d", usage.TotalTokens)
	}
	if toolCallCount > 0 {
		metadata["tool_call_count"] = fmt.Sprintf("%d", toolCallCount)
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func renderToolResult(result *types.ToolResult) string {
	if result == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if len(result.Output) > 0 {
		parts = append(parts, stringifyJSON(result.Output))
	}
	if result.Error != "" {
		parts = append(parts, result.Error)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func stringifyJSON(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
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

func cloneToolResult(result *types.ToolResult) *types.ToolResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.Output = append(json.RawMessage(nil), result.Output...)
	return &cloned
}

func cloneToolCall(call *types.ToolCall) *types.ToolCall {
	if call == nil {
		return nil
	}
	cloned := *call
	cloned.Input = append(json.RawMessage(nil), call.Input...)
	return &cloned
}
