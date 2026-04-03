package config

import (
	"time"

	"claude-go-code/internal/permissions"
)

type Config struct {
	WorkingDir string
	Provider   ProviderConfig
	Session    SessionConfig
	Permission PermissionConfig
	Compact    CompactConfig
	CLI        CLIConfig
	Server     ServerConfig
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
	StorageDir  string
	TTL         time.Duration
	IdleTimeout time.Duration
}

type PermissionConfig struct {
	Mode             permissions.Mode
	EscalationPolicy permissions.EscalationPolicy
	RulesPath        string
}

type CompactConfig struct {
	Enabled bool
}

type CLIConfig struct {
	Interactive bool
}

type ServerConfig struct {
	Host            string
	Port            int
	APIKeys         []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxConcurrent   int
	ShutdownTimeout time.Duration
}
