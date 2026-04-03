package config

import (
	"os"
	"path/filepath"
	"time"

	"claude-go-code/internal/permissions"
)

func DefaultConfig(workingDir string) Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".claw", "data")
	storageDir := filepath.Join(homeDir, ".claw", "sessions")
	rulesPath := filepath.Join(homeDir, ".claw", "permissions", "rules.json")

	return Config{
		WorkingDir: workingDir,
		DataDir:    dataDir,
		Provider: ProviderConfig{
			DefaultProvider: "anthropic",
			DefaultModel:    "claude-sonnet-4-5",
			Anthropic:       EndpointConfig{BaseURL: "https://api.anthropic.com"},
			OpenAI:          EndpointConfig{BaseURL: "https://api.openai.com/v1"},
			ClaudeCode:      EndpointConfig{},
		},
		Session: SessionConfig{
			StorageDir:  storageDir,
			TTL:         24 * time.Hour,
			IdleTimeout: 1 * time.Hour,
		},
		Permission: PermissionConfig{
			Mode:             permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationDeny,
			RulesPath:        rulesPath,
		},
		Compact: CompactConfig{Enabled: true},
		CLI:     CLIConfig{Interactive: true},
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    300 * time.Second,
			MaxConcurrent:   100,
			ShutdownTimeout: 30 * time.Second,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}
