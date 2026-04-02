package permissions

import "context"

type Confirmer interface {
	Confirm(ctx context.Context, req PermissionRequest) (bool, error)
}

type ConfirmFunc func(ctx context.Context, req PermissionRequest) (bool, error)

func (f ConfirmFunc) Confirm(ctx context.Context, req PermissionRequest) (bool, error) {
	return f(ctx, req)
}
