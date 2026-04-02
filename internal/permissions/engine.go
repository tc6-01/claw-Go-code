package permissions

import (
	"context"
	"fmt"
	"sync"
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
	mu               sync.RWMutex
	sessionDecisions map[permissionCacheKey]Decision
}

type permissionCacheKey struct {
	ToolName    string
	CurrentMode Mode
	Required    Mode
}

func NewStaticEngine(defaultMode Mode) *StaticEngine {
	return NewStaticEngineWithOptions(Options{DefaultMode: defaultMode})
}

func NewStaticEngineWithOptions(opts Options) *StaticEngine {
	return &StaticEngine{
		defaultMode:      opts.DefaultMode,
		escalationPolicy: normalizeEscalationPolicy(opts.EscalationPolicy),
		confirmer:        opts.Confirmer,
		sessionDecisions: make(map[permissionCacheKey]Decision),
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
	req.CurrentMode = current
	if rank(current) >= rank(req.Required) {
		return &PermissionDecision{Decision: DecisionAllow}, nil
	}
	if cached, ok := e.lookupSessionDecision(req); ok {
		return &PermissionDecision{
			Decision: cached,
			Reason:   fmt.Sprintf("tool %s reuses %s decision cached for this session", req.ToolName, cached),
		}, nil
	}
	if e.escalationPolicy == EscalationPrompt {
		if e.confirmer != nil {
			outcome, err := e.confirmer.Confirm(ctx, req)
			if err != nil {
				return nil, err
			}
			outcome = normalizeConfirmationOutcome(outcome)
			if outcome.Scope == ConfirmationScopeSession {
				e.storeSessionDecision(req, outcome.Decision)
			}
			if outcome.Decision == DecisionAllow {
				return &PermissionDecision{
					Decision: DecisionAllow,
					Reason:   fmt.Sprintf("tool %s confirmed for %s from %s mode (%s)", req.ToolName, req.Required, current, outcome.Scope),
				}, nil
			}
			return &PermissionDecision{
				Decision: DecisionDeny,
				Reason:   fmt.Sprintf("tool %s was denied during confirmation from %s mode (%s)", req.ToolName, current, outcome.Scope),
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

func (e *StaticEngine) lookupSessionDecision(req PermissionRequest) (Decision, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	decision, ok := e.sessionDecisions[permissionCacheKey{
		ToolName:    req.ToolName,
		CurrentMode: req.CurrentMode,
		Required:    req.Required,
	}]
	return decision, ok
}

func (e *StaticEngine) storeSessionDecision(req PermissionRequest, decision Decision) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionDecisions[permissionCacheKey{
		ToolName:    req.ToolName,
		CurrentMode: req.CurrentMode,
		Required:    req.Required,
	}] = decision
}
