package permissions

import (
	"context"
	"fmt"
)

type Engine interface {
	Decide(ctx context.Context, req PermissionRequest) (*PermissionDecision, error)
}

type Options struct {
	DefaultMode      Mode
	EscalationPolicy EscalationPolicy
	Confirmer        Confirmer
}

type StaticEngine struct {
	defaultMode      Mode
	escalationPolicy EscalationPolicy
	confirmer        Confirmer
}

func NewStaticEngine(defaultMode Mode) *StaticEngine {
	return NewStaticEngineWithOptions(Options{DefaultMode: defaultMode})
}

func NewStaticEngineWithOptions(opts Options) *StaticEngine {
	return &StaticEngine{
		defaultMode:      opts.DefaultMode,
		escalationPolicy: normalizeEscalationPolicy(opts.EscalationPolicy),
		confirmer:        opts.Confirmer,
	}
}

func (e *StaticEngine) Decide(ctx context.Context, req PermissionRequest) (*PermissionDecision, error) {
	current := req.CurrentMode
	if current == "" {
		current = e.defaultMode
	}
	if current == "" {
		current = ModeWorkspaceWrite
	}
	if rank(current) >= rank(req.Required) {
		return &PermissionDecision{Decision: DecisionAllow}, nil
	}
	if e.escalationPolicy == EscalationPrompt {
		if e.confirmer != nil {
			allowed, err := e.confirmer.Confirm(ctx, req)
			if err != nil {
				return nil, err
			}
			if allowed {
				return &PermissionDecision{
					Decision: DecisionAllow,
					Reason:   fmt.Sprintf("tool %s confirmed for %s from %s mode", req.ToolName, req.Required, current),
				}, nil
			}
			return &PermissionDecision{
				Decision: DecisionDeny,
				Reason:   fmt.Sprintf("tool %s was denied during confirmation from %s mode", req.ToolName, current),
			}, nil
		}
		return &PermissionDecision{
			Decision: DecisionPrompt,
			Reason:   fmt.Sprintf("tool %s requires %s and needs confirmation from %s mode", req.ToolName, req.Required, current),
		}, nil
	}
	return &PermissionDecision{
		Decision: DecisionDeny,
		Reason:   fmt.Sprintf("tool %s requires %s but current mode is %s", req.ToolName, req.Required, current),
	}, nil
}

func rank(mode Mode) int {
	switch mode {
	case ModeDangerFull:
		return 3
	case ModeWorkspaceWrite:
		return 2
	case ModeReadOnly:
		return 1
	default:
		return 0
	}
}
