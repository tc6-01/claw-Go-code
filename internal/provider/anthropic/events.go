package anthropic

import "claude-go-code/pkg/types"

type EventType string

const (
	EventTypeMessageDelta EventType = "message_delta"
	EventTypeToolUse      EventType = "tool_use"
	EventTypeMessageStop  EventType = "message_stop"
)

type Event struct {
	Type      EventType
	DeltaText string
	ToolCall  *types.ToolCall
	Usage     *types.Usage
}
