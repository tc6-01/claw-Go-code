package app

import (
	"context"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
	"claude-go-code/internal/provider"
	anthropicprovider "claude-go-code/internal/provider/anthropic"
	openaiprovider "claude-go-code/internal/provider/openai"
	"claude-go-code/internal/runtime"
	"claude-go-code/internal/session"
	"claude-go-code/internal/tools"
)

// App wires the top-level dependencies used by the CLI entrypoint.
type App struct {
	Config          config.Config
	SessionStore    session.Store
	ProviderFactory provider.Factory
	ToolRegistry    tools.Registry
	Permission      permissions.Engine
	Runtime         runtime.Engine
}

type Options struct {
	PermissionConfirmer permissions.Confirmer
}

func New(cfg config.Config) (*App, error) {
	return NewWithOptions(cfg, Options{})
}

func NewWithOptions(cfg config.Config, opts Options) (*App, error) {
	sessionStore := session.NewInMemoryStore()
	providerFactory := provider.NewFactory(cfg.Provider.DefaultProvider, map[string]provider.Provider{
		"anthropic": anthropicprovider.New(anthropicprovider.NewConfig(cfg.Provider.Anthropic), nil),
		"openai":    openaiprovider.New(openaiprovider.NewConfig(cfg.Provider.OpenAI), nil),
		"noop":      provider.NoopProvider{},
	})
	toolRegistry := tools.NewRegistry(tools.BuiltinTools())
	permissionEngine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      cfg.Permission.Mode,
		EscalationPolicy: cfg.Permission.EscalationPolicy,
		Confirmer:        opts.PermissionConfirmer,
	})
	runtimeEngine := runtime.NewEngine(runtime.Dependencies{
		Config:          cfg,
		SessionStore:    sessionStore,
		ProviderFactory: providerFactory,
		ToolRegistry:    toolRegistry,
		Permission:      permissionEngine,
	})

	return &App{
		Config:          cfg,
		SessionStore:    sessionStore,
		ProviderFactory: providerFactory,
		ToolRegistry:    toolRegistry,
		Permission:      permissionEngine,
		Runtime:         runtimeEngine,
	}, nil
}

func (a *App) Run(ctx context.Context, args []string) error {
	return a.Runtime.Run(ctx, runtime.Invocation{Args: args})
}
