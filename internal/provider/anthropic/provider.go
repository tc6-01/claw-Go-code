package anthropic

import (
	"context"
	"strings"

	shared "claude-go-code/internal/provider"
	"claude-go-code/pkg/types"
)

type Provider struct {
	cfg    Config
	client Client
}

func New(cfg Config, client Client) Provider {
	if client == nil {
		client = NewStubClient()
	}
	return Provider{cfg: cfg, client: client}
}

func (p Provider) Send(ctx context.Context, req *types.MessageRequest) (*types.MessageResponse, error) {
	return p.client.CreateMessage(ctx, req)
}

func (p Provider) Stream(ctx context.Context, req *types.MessageRequest) (shared.StreamReader, error) {
	events, err := p.client.StreamMessages(ctx, req)
	if err != nil {
		return nil, err
	}
	mapped := make([]*shared.StreamEvent, 0, len(events)+2)
	mapped = append(mapped, shared.MessageStartEvent())
	for _, event := range events {
		mapped = append(mapped, expandEvent(event)...)
	}
	return &shared.SliceStreamReader{Events: mapped}, nil
}

func (p Provider) NormalizeModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "claude-sonnet-4-5"
	}
	if !strings.HasPrefix(trimmed, "claude-") {
		return "claude-" + trimmed
	}
	return trimmed
}

func (p Provider) Capabilities() shared.ProviderCapabilities {
	return shared.ProviderCapabilities{Streaming: true, ToolCalls: true}
}

func expandEvent(event Event) []*shared.StreamEvent {
	switch event.Type {
	case EventTypeToolUse:
		return []*shared.StreamEvent{shared.ToolCallEvent(event.ToolCall)}
	case EventTypeMessageStop:
		events := make([]*shared.StreamEvent, 0, 2)
		if event.Usage != nil {
			events = append(events, shared.UsageEvent(*event.Usage))
		}
		events = append(events, shared.StopEvent())
		return events
	default:
		return []*shared.StreamEvent{shared.MessageDeltaEvent(event.DeltaText)}
	}
}
