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
	"claude-go-code/pkg/types"
)

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

	anthropicCfg := anthropicprovider.NewConfig(cfg.Provider.Anthropic)
	var anthropicClient anthropicprovider.Client
	if anthropicCfg.APIKey != "" {
		anthropicClient = anthropicprovider.NewHTTPClient(anthropicCfg)
	}

	openaiCfg := openaiprovider.NewConfig(cfg.Provider.OpenAI)
	var openaiClient openaiprovider.Client
	if openaiCfg.APIKey != "" {
		openaiClient = openaiprovider.NewHTTPClient(openaiCfg)
	}

	providerFactory := provider.NewFactory(cfg.Provider.DefaultProvider, map[string]provider.Provider{
		"anthropic": anthropicprovider.New(anthropicCfg, anthropicClient),
		"openai":    openaiprovider.New(openaiCfg, openaiClient),
		"noop":      provider.NoopProvider{},
	})
	toolRegistry := tools.NewRegistry(tools.BuiltinTools())
	permissionEngine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      cfg.Permission.Mode,
		EscalationPolicy: cfg.Permission.EscalationPolicy,
		Confirmer:        opts.PermissionConfirmer,
		RuleCachePath:    cfg.Permission.RulesPath,
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

func (a *App) CreateSession(ctx context.Context) (*types.Session, error) {
	return a.Runtime.CreateSession(ctx)
}

func (a *App) RunPrompt(ctx context.Context, sessionID string, prompt string) (*runtime.PromptResult, error) {
	return a.Runtime.RunPrompt(ctx, sessionID, prompt)
}

func NewForServer(cfg config.Config) (*App, error) {
	sessionStore, err := session.NewFileStore(cfg.Session.StorageDir)
	if err != nil {
		return nil, err
	}

	anthropicCfg := anthropicprovider.NewConfig(cfg.Provider.Anthropic)
	var anthropicClient anthropicprovider.Client
	if anthropicCfg.APIKey != "" {
		anthropicClient = anthropicprovider.NewHTTPClient(anthropicCfg)
	}

	openaiCfg := openaiprovider.NewConfig(cfg.Provider.OpenAI)
	var openaiClient openaiprovider.Client
	if openaiCfg.APIKey != "" {
		openaiClient = openaiprovider.NewHTTPClient(openaiCfg)
	}

	providerFactory := provider.NewFactory(cfg.Provider.DefaultProvider, map[string]provider.Provider{
		"anthropic": anthropicprovider.New(anthropicCfg, anthropicClient),
		"openai":    openaiprovider.New(openaiCfg, openaiClient),
		"noop":      provider.NoopProvider{},
	})
	toolRegistry := tools.NewRegistry(tools.BuiltinTools())
	permissionEngine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      cfg.Permission.Mode,
		EscalationPolicy: cfg.Permission.EscalationPolicy,
		RuleCachePath:    cfg.Permission.RulesPath,
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
