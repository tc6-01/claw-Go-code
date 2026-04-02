package permissions

import "context"

type ConfirmationScope string

const (
	ConfirmationScopeOnce    ConfirmationScope = "once"
	ConfirmationScopeSession ConfirmationScope = "session"
	ConfirmationScopeRule    ConfirmationScope = "rule"
)

type ConfirmationOutcome struct {
	Decision Decision
	Scope    ConfirmationScope
}

type Confirmer interface {
	Confirm(ctx context.Context, req PermissionRequest) (ConfirmationOutcome, error)
}

type ConfirmFunc func(ctx context.Context, req PermissionRequest) (ConfirmationOutcome, error)

func (f ConfirmFunc) Confirm(ctx context.Context, req PermissionRequest) (ConfirmationOutcome, error) {
	return f(ctx, req)
}

func normalizeConfirmationOutcome(outcome ConfirmationOutcome) ConfirmationOutcome {
	switch outcome.Decision {
	case DecisionAllow, DecisionDeny:
	default:
		outcome.Decision = DecisionDeny
	}
	switch outcome.Scope {
	case ConfirmationScopeSession, ConfirmationScopeRule:
	default:
		outcome.Scope = ConfirmationScopeOnce
	}
	return outcome
}
