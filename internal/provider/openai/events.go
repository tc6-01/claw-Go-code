package openai

import "claude-go-code/pkg/types"

type EventType string

const (
	EventTypeResponseDelta    EventType = "response.delta"
	EventTypeResponseToolCall EventType = "response.tool_call"
	EventTypeResponseUsage    EventType = "response.usage"
	EventTypeResponseComplete EventType = "response.complete"
	EventTypeResponseError    EventType = "response.error"
)

type Event struct {
	Type      EventType
	DeltaText string
	ToolCall  *types.ToolCall
	Usage     *types.Usage
	Err       error
}
