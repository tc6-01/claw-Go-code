package types

import "time"

const CurrentSessionVersion = 1

type Session struct {
	ID             string           `json:"id"`
	Version        int              `json:"version"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	CWD            string           `json:"cwd"`
	Model          string           `json:"model"`
	PermissionMode string           `json:"permission_mode"`
	Messages       []Message        `json:"messages,omitempty"`
	ToolTrace      []ToolTraceEntry `json:"tool_trace,omitempty"`
	Usage          []Usage          `json:"usage,omitempty"`
	Todos          []TodoItem       `json:"todos,omitempty"`
}

type SessionSummary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Model     string    `json:"model"`
	CWD       string    `json:"cwd"`
}

type ToolTraceEntry struct {
	ID         string      `json:"id,omitempty"`
	Name       string      `json:"name"`
	Input      string      `json:"input,omitempty"`
	Output     string      `json:"output,omitempty"`
	Error      string      `json:"error,omitempty"`
	StartedAt  time.Time   `json:"started_at"`
	EndedAt    time.Time   `json:"ended_at"`
	Success    bool        `json:"success"`
	Permission string      `json:"permission,omitempty"`
	Result     *ToolResult `json:"result,omitempty"`
}

type TodoItem struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}
