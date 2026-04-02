package provider

import (
	"errors"
	"fmt"
	"io"

	"claude-go-code/pkg/types"
)

type StreamEventType string

const (
	StreamEventMessageStart StreamEventType = "message_start"
	StreamEventMessageDelta StreamEventType = "message_delta"
	StreamEventToolCall     StreamEventType = "tool_call"
	StreamEventUsage        StreamEventType = "usage"
	StreamEventStop         StreamEventType = "stop"
	StreamEventError        StreamEventType = "error"
)

type StreamEvent struct {
	Type     StreamEventType
	Text     string
	ToolCall *types.ToolCall
	Usage    *types.Usage
	Error    error
}

func MessageStartEvent() *StreamEvent {
	return &StreamEvent{Type: StreamEventMessageStart}
}

func MessageDeltaEvent(text string) *StreamEvent {
	return &StreamEvent{Type: StreamEventMessageDelta, Text: text}
}

func ToolCallEvent(call *types.ToolCall) *StreamEvent {
	return &StreamEvent{Type: StreamEventToolCall, ToolCall: call}
}

func UsageEvent(usage types.Usage) *StreamEvent {
	return &StreamEvent{Type: StreamEventUsage, Usage: &usage}
}

func StopEvent() *StreamEvent {
	return &StreamEvent{Type: StreamEventStop}
}

func ErrorEvent(err error) *StreamEvent {
	return &StreamEvent{Type: StreamEventError, Error: err}
}

func (e *StreamEvent) Validate() error {
	if e == nil {
		return errors.New("nil stream event")
	}

	switch e.Type {
	case StreamEventMessageStart, StreamEventStop:
		return nil
	case StreamEventMessageDelta:
		if e.Text == "" {
			return errors.New("message delta event requires text")
		}
		return nil
	case StreamEventToolCall:
		if e.ToolCall == nil {
			return errors.New("tool call event requires tool call payload")
		}
		return nil
	case StreamEventUsage:
		if e.Usage == nil {
			return errors.New("usage event requires usage payload")
		}
		return nil
	case StreamEventError:
		if e.Error == nil {
			return errors.New("error event requires error payload")
		}
		return nil
	default:
		return fmt.Errorf("unknown stream event type %q", e.Type)
	}
}

type StreamReader interface {
	Next() (*StreamEvent, error)
	Close() error
}

type SliceStreamReader struct {
	Events []*StreamEvent
	Index  int
}

func (r *SliceStreamReader) Next() (*StreamEvent, error) {
	if r.Index >= len(r.Events) {
		return nil, io.EOF
	}
	event := r.Events[r.Index]
	r.Index++
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return event, nil
}

func (r *SliceStreamReader) Close() error { return nil }
