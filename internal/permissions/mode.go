package permissions

import "fmt"

type Mode string

const (
	ModeReadOnly       Mode = "read-only"
	ModeWorkspaceWrite Mode = "workspace-write"
	ModeDangerFull     Mode = "danger-full-access"
)

func ParseMode(v string) (Mode, error) {
	mode := Mode(v)
	switch mode {
	case ModeReadOnly, ModeWorkspaceWrite, ModeDangerFull:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown permission mode: %s", v)
	}
}
