package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"claude-go-code/internal/permissions"
)

type LoadOptions struct {
	WorkingDir string
}

func Load(_ context.Context, opts LoadOptions) (Config, error) {
	cfg := DefaultConfig(opts.WorkingDir)

	if v := os.Getenv("CLAW_PROVIDER"); v != "" {
		cfg.Provider.DefaultProvider = v
	}
	if v := os.Getenv("CLAW_MODEL"); v != "" {
		cfg.Provider.DefaultModel = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.Provider.Anthropic.APIKey = v
	}
	if v := os.Getenv("ANTHROPIC_BASE_URL"); v != "" {
		cfg.Provider.Anthropic.BaseURL = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.Provider.OpenAI.APIKey = v
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		cfg.Provider.OpenAI.BaseURL = v
	}
	if v := os.Getenv("CLAW_PERMISSION_MODE"); v != "" {
		mode, err := permissions.ParseMode(v)
		if err != nil {
			return Config{}, err
		}
		cfg.Permission.Mode = mode
	}
	if v := os.Getenv("CLAW_PERMISSION_ESCALATION_POLICY"); v != "" {
		policy, err := parseEscalationPolicy(v)
		if err != nil {
			return Config{}, err
		}
		cfg.Permission.EscalationPolicy = policy
	}
	if v := os.Getenv("CLAW_PERMISSION_RULES_PATH"); v != "" {
		cfg.Permission.RulesPath = v
	}

	if v := os.Getenv("CLAW_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("CLAW_SERVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLAW_SERVER_PORT: %w", err)
		}
		cfg.Server.Port = port
	}
	if v := os.Getenv("CLAW_API_KEYS"); v != "" {
		cfg.Server.APIKeys = splitCSV(v)
	}
	if v := os.Getenv("CLAW_SERVER_READ_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLAW_SERVER_READ_TIMEOUT: %w", err)
		}
		cfg.Server.ReadTimeout = d
	}
	if v := os.Getenv("CLAW_SERVER_WRITE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLAW_SERVER_WRITE_TIMEOUT: %w", err)
		}
		cfg.Server.WriteTimeout = d
	}
	if v := os.Getenv("CLAW_SESSION_STORAGE_DIR"); v != "" {
		cfg.Session.StorageDir = v
	}
	if v := os.Getenv("CLAW_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("CLAW_SERVER_RATE_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLAW_SERVER_RATE_LIMIT: %w", err)
		}
		cfg.Server.RateLimit = n
	}
	if v := os.Getenv("CLAW_SANDBOX_ENABLED"); v == "true" || v == "1" {
		cfg.Sandbox.Enabled = true
	}
	if v := os.Getenv("CLAW_SANDBOX_ROOT_DIR"); v != "" {
		cfg.Sandbox.RootDir = v
	}
	if v := os.Getenv("CLAW_SANDBOX_DENY_EXEC"); v == "true" || v == "1" {
		cfg.Sandbox.DenyExec = true
	}
	if v := os.Getenv("CLAW_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("CLAW_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}

	return cfg, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseEscalationPolicy(v string) (permissions.EscalationPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", string(permissions.EscalationDeny):
		return permissions.EscalationDeny, nil
	case string(permissions.EscalationPrompt):
		return permissions.EscalationPrompt, nil
	default:
		return "", fmt.Errorf("unknown escalation policy: %s", v)
	}
}
