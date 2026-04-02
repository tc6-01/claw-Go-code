package types

import (
	"context"
	"encoding/json"
	"time"
)

type Context = context.Context

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	ID        string            `json:"id,omitempty"`
	Role      Role              `json:"role"`
	Name      string            `json:"name,omitempty"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
}

type MessageRequest struct {
	Model    string            `json:"model"`
	System   string            `json:"system,omitempty"`
	Messages []Message         `json:"messages"`
	Tools    []ToolSpec        `json:"tools,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type MessageResponse struct {
	Message    Message `json:"message"`
	Usage      Usage   `json:"usage"`
	StopReason string  `json:"stop_reason,omitempty"`
}

type PromptBundle struct {
	System       string          `json:"system"`
	Instructions []string        `json:"instructions,omitempty"`
	Messages     []Message       `json:"messages,omitempty"`
	Tools        []ToolSpec      `json:"tools,omitempty"`
	Context      json.RawMessage `json:"context,omitempty"`
}
