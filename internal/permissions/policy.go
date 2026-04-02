package permissions

type EscalationPolicy string

const (
	EscalationDeny   EscalationPolicy = "deny"
	EscalationPrompt EscalationPolicy = "prompt"
)

func normalizeEscalationPolicy(policy EscalationPolicy) EscalationPolicy {
	switch policy {
	case EscalationPrompt:
		return EscalationPrompt
	default:
		return EscalationDeny
	}
}
