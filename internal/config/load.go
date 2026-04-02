package config

import (
	"context"
	"os"

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
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.Provider.OpenAI.APIKey = v
	}
	if v := os.Getenv("CLAW_PERMISSION_MODE"); v != "" {
		mode, err := permissions.ParseMode(v)
		if err != nil {
			return Config{}, err
		}
		cfg.Permission.Mode = mode
	}

	return cfg, nil
}
