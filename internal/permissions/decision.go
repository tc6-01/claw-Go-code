package permissions

type PermissionRequest struct {
	ToolName    string
	CurrentMode Mode
	Required    Mode
}

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionDeny   Decision = "deny"
	DecisionPrompt Decision = "prompt"
)

type PermissionDecision struct {
	Decision Decision
	Reason   string
}
