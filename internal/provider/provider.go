package provider

import (
	"context"
	"fmt"
	"strings"

	"claude-go-code/pkg/types"
)

type Provider interface {
	Send(ctx context.Context, req *types.MessageRequest) (*types.MessageResponse, error)
	Stream(ctx context.Context, req *types.MessageRequest) (StreamReader, error)
	NormalizeModel(model string) string
	Capabilities() ProviderCapabilities
}

func (NoopProvider) Stream(ctx context.Context, req *types.MessageRequest) (StreamReader, error) {
	resp, err := NoopProvider{}.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	events := []*StreamEvent{MessageStartEvent()}
	if resp.Message.Content != "" {
		events = append(events, MessageDeltaEvent(resp.Message.Content))
	}
	if resp.Usage.TotalTokens > 0 || resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		events = append(events, UsageEvent(resp.Usage))
	}
	events = append(events, StopEvent())
	return &SliceStreamReader{Events: events}, nil
}

func (NoopProvider) NormalizeModel(model string) string {
	return strings.TrimSpace(model)
}

func (NoopProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{Streaming: true, ToolCalls: true}
}

func ResolveProviderName(requested string, fallback string) string {
	name := strings.ToLower(strings.TrimSpace(requested))
	if name != "" {
		return name
	}
	return strings.ToLower(strings.TrimSpace(fallback))
}

func ProviderNameFromModel(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case normalized == "":
		return ""
	case strings.HasPrefix(normalized, "claude"):
		return "anthropic"
	case strings.HasPrefix(normalized, "gpt"), strings.HasPrefix(normalized, "o1"), strings.HasPrefix(normalized, "o3"):
		return "openai"
	default:
		return ""
	}
}

func ResolveProviderNameFromModel(requested string, fallback string, model string) string {
	if name := ResolveProviderName(requested, fallback); name != "" {
		return name
	}
	if inferred := ProviderNameFromModel(model); inferred != "" {
		return inferred
	}
	return "noop"
}

func FactoryProvider(f Factory, requested string, fallback string, model string) (Provider, error) {
	name := ResolveProviderNameFromModel(requested, fallback, model)
	if name == "" {
		return nil, fmt.Errorf("provider resolution failed")
	}
	return f.Get(name)
}
