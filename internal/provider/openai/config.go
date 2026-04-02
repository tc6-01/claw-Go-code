package openai

import (
	"fmt"
	"strings"

	appconfig "claude-go-code/internal/config"
)

const defaultBaseURL = "https://api.openai.com/v1"

type Config struct {
	BaseURL string
	APIKey  string
}

func NewConfig(cfg appconfig.EndpointConfig) Config {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return Config{
		BaseURL: baseURL,
		APIKey:  strings.TrimSpace(cfg.APIKey),
	}
}

func (c Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("openai base url is required")
	}
	return nil
}
