package config

import (
	"os"
	"path/filepath"

	"claude-go-code/internal/permissions"
)

func DefaultConfig(workingDir string) Config {
	homeDir, _ := os.UserHomeDir()
	storageDir := filepath.Join(homeDir, ".claude-go-code", "sessions")
	rulesPath := filepath.Join(homeDir, ".claude-go-code", "permissions", "rules.json")

	return Config{
		WorkingDir: workingDir,
		Provider: ProviderConfig{
			DefaultProvider: "anthropic",
			DefaultModel:    "claude-sonnet-4-5",
			Anthropic:       EndpointConfig{BaseURL: "https://api.anthropic.com"},
			OpenAI:          EndpointConfig{BaseURL: "https://api.openai.com/v1"},
		},
		Session: SessionConfig{StorageDir: storageDir},
		Permission: PermissionConfig{
			Mode:             permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationDeny,
			RulesPath:        rulesPath,
		},
		Compact: CompactConfig{Enabled: true},
		CLI:     CLIConfig{Interactive: true},
	}
}
