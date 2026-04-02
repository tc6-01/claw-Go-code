package provider

import (
	"context"
	"fmt"
	"strings"

	"claude-go-code/pkg/types"
)

type ProviderCapabilities struct {
	Streaming bool
	ToolCalls bool
}

type Factory interface {
	Default() Provider
	Get(name string) (Provider, error)
}

type StaticFactory struct {
	defaultName string
	providers   map[string]Provider
}

func NewStaticFactory() *StaticFactory {
	noop := NoopProvider{}
	return &StaticFactory{
		defaultName: "anthropic",
		providers: map[string]Provider{
			"anthropic": noop,
			"openai":    noop,
			"noop":      noop,
		},
	}
}

func NewFactory(defaultName string, providers map[string]Provider) *StaticFactory {
	cloned := make(map[string]Provider, len(providers))
	for name, provider := range providers {
		cloned[strings.ToLower(strings.TrimSpace(name))] = provider
	}
	if len(cloned) == 0 {
		cloned["noop"] = NoopProvider{}
	}
	resolvedDefault := strings.ToLower(strings.TrimSpace(defaultName))
	if resolvedDefault == "" {
		resolvedDefault = "noop"
	}
	if _, ok := cloned[resolvedDefault]; !ok {
		cloned[resolvedDefault] = NoopProvider{}
	}
	return &StaticFactory{defaultName: resolvedDefault, providers: cloned}
}

func (f *StaticFactory) Default() Provider {
	provider, err := f.Get(f.defaultName)
	if err != nil {
		return NoopProvider{}
	}
	return provider
}

func (f *StaticFactory) Get(name string) (Provider, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = f.defaultName
	}
	provider, ok := f.providers[key]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return provider, nil
}

type NoopProvider struct{}

func (NoopProvider) Send(_ context.Context, req *types.MessageRequest) (*types.MessageResponse, error) {
	return &types.MessageResponse{
		Message:    types.Message{Role: types.RoleAssistant, Content: "runtime skeleton ready"},
		Usage:      types.Usage{TotalTokens: len(req.Messages)},
		StopReason: "stub",
	}, nil
}
