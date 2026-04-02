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
			toolMessage, trace, toolResult := e.executeTool(ctx, call)
			sess.ToolTrace = append(sess.ToolTrace, trace)
			sess.Messages = append(sess.Messages, toolMessage)
			requestMessages = append(requestMessages, toolMessage)
			e.applyToolSideEffects(sess, call.Name, toolResult)
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

func (e *engine) executeTool(ctx context.Context, call *types.ToolCall) (types.Message, types.ToolTraceEntry, *types.ToolResult) {
	executor := tools.NewExecutor(e.deps.ToolRegistry, e.deps.Permission)
	result, err := executor.Execute(ctx, tools.ExecuteRequest{
		Call: *call,
		Env: tools.ToolEnv{
			WorkingDir: e.deps.Config.WorkingDir,
			Mode:       e.deps.Config.Permission.Mode,
		},
	})
	if err != nil {
		now := time.Now().UTC()
		toolResult := &types.ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Error:      err.Error(),
		}
		return types.Message{
				Role:       types.RoleTool,
				Name:       call.Name,
				Content:    mustJSON(toolResult),
				CreatedAt:  now,
				Metadata:   map[string]string{"tool_call_id": call.ID, "error": "tool_execution_failed"},
				ToolCalls:  flattenToolCalls([]*types.ToolCall{call}),
				ToolResult: cloneToolResult(toolResult),
			}, types.ToolTraceEntry{
				ID:        call.ID,
				Name:      call.Name,
				Input:     stringifyJSON(call.Input),
				Error:     err.Error(),
				StartedAt: now,
				EndedAt:   now,
				Success:   false,
			}, toolResult
	}
	return result.Message, result.Trace, result.Result
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

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func (e *engine) applyToolSideEffects(sess *types.Session, toolName string, result *types.ToolResult) {
	if sess == nil || result == nil || result.Error != "" {
		return
	}
	switch toolName {
	case "todo_write":
		var payload struct {
			Todos []types.TodoItem `json:"todos"`
		}
		if err := json.Unmarshal(result.Output, &payload); err != nil {
			return
		}
		sess.Todos = append([]types.TodoItem(nil), payload.Todos...)
	}
}
