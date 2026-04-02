package config

import "claude-go-code/internal/permissions"

type Config struct {
	WorkingDir string
	Provider   ProviderConfig
	Session    SessionConfig
	Permission PermissionConfig
	Compact    CompactConfig
	CLI        CLIConfig
}

type ProviderConfig struct {
	DefaultProvider string
	DefaultModel    string
	Anthropic       EndpointConfig
	OpenAI          EndpointConfig
}

type EndpointConfig struct {
	BaseURL string
	APIKey  string
}

type SessionConfig struct {
	StorageDir string
}

type PermissionConfig struct {
	Mode             permissions.Mode
	EscalationPolicy permissions.EscalationPolicy
}

type CompactConfig struct {
	Enabled bool
}

type CLIConfig struct {
	Interactive bool
}
