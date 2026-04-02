package runtime

import (
	"context"
	"fmt"
	"time"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
	"claude-go-code/internal/provider"
	"claude-go-code/internal/session"
	"claude-go-code/internal/tools"
	"claude-go-code/pkg/types"
)

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

func NewEngine(deps Dependencies) Engine {
	return &engine{deps: deps}
}

func (e *engine) Run(ctx context.Context, invocation Invocation) error {
	providerClient := e.deps.ProviderFactory.Default()
	result, err := providerClient.Send(ctx, &types.MessageRequest{
		Model: e.deps.Config.Provider.DefaultModel,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: fmt.Sprintf("bootstrap:%v", invocation.Args),
		}},
	})
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	sess := &types.Session{
		ID:             "bootstrap-session",
		Version:        types.CurrentSessionVersion,
		CreatedAt:      now,
		UpdatedAt:      now,
		CWD:            e.deps.Config.WorkingDir,
		Model:          e.deps.Config.Provider.DefaultModel,
		PermissionMode: string(e.deps.Config.Permission.Mode),
		Messages:       []types.Message{result.Message},
	}
	if err := e.deps.SessionStore.Create(ctx, sess); err != nil {
		return err
	}
	return nil
}
