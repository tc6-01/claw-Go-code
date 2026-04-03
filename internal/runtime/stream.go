package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"claude-go-code/internal/provider"
	"claude-go-code/internal/sysprompt"
	"claude-go-code/internal/tools"
	"claude-go-code/pkg/types"
)

type StreamEventType string

const (
	EventTextDelta  StreamEventType = "text_delta"
	EventToolUse    StreamEventType = "tool_use"
	EventToolStart  StreamEventType = "tool_start"
	EventToolResult StreamEventType = "tool_result"
	EventToolError  StreamEventType = "tool_error"
	EventUsage      StreamEventType = "usage"
	EventMessageEnd StreamEventType = "message_end"
	EventError      StreamEventType = "error"
	EventDone       StreamEventType = "done"
)

type StreamEvent struct {
	Type       StreamEventType  `json:"type"`
	Text       string           `json:"text,omitempty"`
	ToolCall   *types.ToolCall  `json:"tool_call,omitempty"`
	ToolResult *types.ToolResult `json:"tool_result,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Error      error            `json:"-"`
	ErrorText  string           `json:"error,omitempty"`
	Usage      *types.Usage     `json:"usage,omitempty"`
	Message    *types.Message   `json:"message,omitempty"`
}

func (e *engine) RunPromptStream(ctx context.Context, sessionID string, prompt string) (<-chan StreamEvent, error) {
	sess, err := e.deps.SessionStore.Load(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	providerClient, model, err := e.resolveProvider()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sess.Model) == "" {
		sess.Model = model
	}

	now := time.Now().UTC()
	userMessage := types.Message{
		Role:      types.RoleUser,
		Content:   prompt,
		CreatedAt: now,
	}
	sess.Messages = append(sess.Messages, userMessage)
	sess.UpdatedAt = now
	if err := e.deps.SessionStore.Save(ctx, sess); err != nil {
		return nil, err
	}

	out := make(chan StreamEvent, 128)

	go func() {
		defer close(out)
		e.streamLoop(ctx, sess, providerClient, out)
	}()

	return out, nil
}

func (e *engine) streamLoop(ctx context.Context, sess *types.Session, providerClient provider.Provider, out chan<- StreamEvent) {
	requestMessages := append([]types.Message(nil), sess.Messages...)
	toolSpecs := e.deps.ToolRegistry.Specs()

	systemPrompt := sysprompt.Build(sysprompt.Context{
		CWD:       sess.CWD,
		Model:     sess.Model,
		ToolSpecs: toolSpecs,
	})

	for turn := 0; turn < maxTurns; turn++ {
		if ctx.Err() != nil {
			out <- StreamEvent{Type: EventError, Error: ctx.Err(), ErrorText: ctx.Err().Error()}
			return
		}

		reader, err := providerClient.Stream(ctx, &types.MessageRequest{
			Model:    sess.Model,
			System:   systemPrompt,
			Messages: append([]types.Message(nil), requestMessages...),
			Tools:    append([]types.ToolSpec(nil), toolSpecs...),
		})
		if err != nil {
			out <- StreamEvent{Type: EventError, Error: err, ErrorText: err.Error()}
			return
		}

		assistant := types.Message{
			Role:      types.RoleAssistant,
			CreatedAt: time.Now().UTC(),
		}
		var toolCalls []*types.ToolCall
		stopReason := ""
		usage := types.Usage{}

		for {
			event, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				reader.Close()
				out <- StreamEvent{Type: EventError, Error: err, ErrorText: err.Error()}
				return
			}

			switch event.Type {
			case provider.StreamEventMessageDelta:
				assistant.Content += event.Text
				out <- StreamEvent{Type: EventTextDelta, Text: event.Text}
			case provider.StreamEventToolCall:
				toolCalls = append(toolCalls, cloneToolCall(event.ToolCall))
				out <- StreamEvent{Type: EventToolUse, ToolCall: cloneToolCall(event.ToolCall)}
			case provider.StreamEventUsage:
				if event.Usage != nil {
					usage = *event.Usage
					out <- StreamEvent{Type: EventUsage, Usage: event.Usage}
				}
			case provider.StreamEventStop:
				stopReason = "stop"
			case provider.StreamEventError:
				reader.Close()
				errMsg := "provider stream error"
				if event.Error != nil {
					errMsg = event.Error.Error()
				}
				out <- StreamEvent{Type: EventError, Error: event.Error, ErrorText: errMsg}
				return
			}
		}
		reader.Close()

		assistant.StopReason = stopReason
		assistant.Usage = usage
		assistant.ToolCalls = flattenToolCalls(toolCalls)
		assistant.Metadata = messageMetadata(stopReason, usage, len(toolCalls), nil)

		if shouldPersistAssistant(assistant) {
			sess.Messages = append(sess.Messages, assistant)
			requestMessages = append(requestMessages, assistant)
		}

		if len(toolCalls) == 0 {
			out <- StreamEvent{Type: EventMessageEnd, Message: &assistant, Usage: &usage}
			sess.UpdatedAt = time.Now().UTC()
			_ = e.deps.SessionStore.Save(ctx, sess)
			out <- StreamEvent{Type: EventDone}
			return
		}

		for _, call := range toolCalls {
			out <- StreamEvent{Type: EventToolStart, ToolCall: call}

			executor := tools.NewExecutor(e.deps.ToolRegistry, e.deps.Permission)
			result, err := executor.Execute(ctx, tools.ExecuteRequest{
				Call: *call,
				Env: tools.ToolEnv{
					WorkingDir: e.deps.Config.WorkingDir,
					Mode:       e.deps.Config.Permission.Mode,
				},
			})
			if err != nil {
				toolResult := &types.ToolResult{
					ToolCallID: call.ID,
					Name:       call.Name,
					Error:      err.Error(),
				}
				errMsg := types.Message{
					Role:       types.RoleTool,
					Name:       call.Name,
					Content:    mustJSON(toolResult),
					CreatedAt:  time.Now().UTC(),
					Metadata:   map[string]string{"tool_call_id": call.ID, "error": "tool_execution_failed"},
					ToolCalls:  flattenToolCalls([]*types.ToolCall{call}),
					ToolResult: cloneToolResult(toolResult),
				}
				sess.Messages = append(sess.Messages, errMsg)
				requestMessages = append(requestMessages, errMsg)
				out <- StreamEvent{Type: EventToolError, ToolCallID: call.ID, ErrorText: err.Error(), Error: err}
			} else {
				sess.Messages = append(sess.Messages, result.Message)
				requestMessages = append(requestMessages, result.Message)
				out <- StreamEvent{Type: EventToolResult, ToolResult: result.Result, ToolCallID: call.ID}
			}
		}

		sess.UpdatedAt = time.Now().UTC()
		if err := e.deps.SessionStore.Save(ctx, sess); err != nil {
			out <- StreamEvent{Type: EventError, Error: err, ErrorText: err.Error()}
			return
		}
	}

	err := fmt.Errorf("runtime exceeded max turns (%d)", maxTurns)
	out <- StreamEvent{Type: EventError, Error: err, ErrorText: err.Error()}
}
