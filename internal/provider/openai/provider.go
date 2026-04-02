package openai

import (
	"context"
	"errors"
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
	return p.client.CreateResponse(ctx, req)
}

func (p Provider) Stream(ctx context.Context, req *types.MessageRequest) (shared.StreamReader, error) {
	events, err := p.client.StreamResponses(ctx, req)
	if err != nil {
		return nil, err
	}
	mapped := make([]*shared.StreamEvent, 0, len(events)+1)
	mapped = append(mapped, shared.MessageStartEvent())
	for _, event := range events {
		mapped = append(mapped, mapEvent(event))
	}
	return &shared.SliceStreamReader{Events: mapped}, nil
}

func (p Provider) NormalizeModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "gpt-4o-mini"
	}
	if strings.HasPrefix(trimmed, "gpt-") || strings.HasPrefix(trimmed, "o1") || strings.HasPrefix(trimmed, "o3") {
		return trimmed
	}
	return "gpt-" + trimmed
}

func (p Provider) Capabilities() shared.ProviderCapabilities {
	return shared.ProviderCapabilities{Streaming: true, ToolCalls: true}
}

func mapEvent(event Event) *shared.StreamEvent {
	switch event.Type {
	case EventTypeResponseToolCall:
		return shared.ToolCallEvent(event.ToolCall)
	case EventTypeResponseUsage:
		if event.Usage == nil {
			return shared.ErrorEvent(errors.New("openai usage event missing usage payload"))
		}
		return shared.UsageEvent(*event.Usage)
	case EventTypeResponseComplete:
		return shared.StopEvent()
	case EventTypeResponseError:
		return shared.ErrorEvent(event.Err)
	default:
		return shared.MessageDeltaEvent(event.DeltaText)
	}
}
