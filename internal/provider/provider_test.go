package provider

import (
	"errors"
	"io"
	"testing"

	"claude-go-code/pkg/types"
)

type stubProvider struct {
	response *types.MessageResponse
}

func (p stubProvider) Send(_ types.Context, _ *types.MessageRequest) (*types.MessageResponse, error) {
	return p.response, nil
}

func (p stubProvider) Stream(_ types.Context, _ *types.MessageRequest) (StreamReader, error) {
	return &SliceStreamReader{}, nil
}

func (p stubProvider) NormalizeModel(model string) string {
	return model
}

func (p stubProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{Streaming: true, ToolCalls: true}
}

func TestStreamEventValidate(t *testing.T) {
	tests := []struct {
		name    string
		event   *StreamEvent
		wantErr bool
	}{
		{name: "message start", event: MessageStartEvent()},
		{name: "message delta", event: MessageDeltaEvent("hello")},
		{name: "tool call", event: ToolCallEvent(&types.ToolCall{ID: "call-1", Name: "read_file"})},
		{name: "usage", event: UsageEvent(types.Usage{TotalTokens: 3})},
		{name: "stop", event: StopEvent()},
		{name: "error", event: ErrorEvent(errors.New("boom"))},
		{name: "nil", event: nil, wantErr: true},
		{name: "empty delta", event: MessageDeltaEvent(""), wantErr: true},
		{name: "missing tool call", event: &StreamEvent{Type: StreamEventToolCall}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestSliceStreamReaderValidatesEvents(t *testing.T) {
	reader := &SliceStreamReader{Events: []*StreamEvent{MessageDeltaEvent("ok"), nil}}

	if _, err := reader.Next(); err != nil {
		t.Fatalf("first next failed: %v", err)
	}
	if _, err := reader.Next(); err == nil {
		t.Fatal("expected validation error for nil event")
	}
}

func TestResolveProviderNameFromModel(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		fallback  string
		model     string
		want      string
	}{
		{name: "requested wins", requested: "openai", fallback: "anthropic", model: "claude-3-7", want: "openai"},
		{name: "fallback used", fallback: "anthropic", model: "gpt-4o", want: "anthropic"},
		{name: "infer anthropic", model: "claude-3-7-sonnet", want: "anthropic"},
		{name: "infer openai", model: "gpt-4o-mini", want: "openai"},
		{name: "default noop", want: "noop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveProviderNameFromModel(tt.requested, tt.fallback, tt.model)
			if got != tt.want {
				t.Fatalf("ResolveProviderNameFromModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFactorySelectsNamedProvider(t *testing.T) {
	response := &types.MessageResponse{Message: types.Message{Role: types.RoleAssistant, Content: "from-openai"}}
	factory := NewFactory("anthropic", map[string]Provider{
		"anthropic": stubProvider{response: &types.MessageResponse{Message: types.Message{Role: types.RoleAssistant, Content: "from-anthropic"}}},
		"openai":    stubProvider{response: response},
	})

	providerClient, err := FactoryProvider(factory, "openai", "anthropic", "")
	if err != nil {
		t.Fatalf("FactoryProvider() error = %v", err)
	}

	result, err := providerClient.Send(types.Context(nil), &types.MessageRequest{})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result.Message.Content != "from-openai" {
		t.Fatalf("unexpected provider response %q", result.Message.Content)
	}
}

func TestFactoryRejectsUnknownProvider(t *testing.T) {
	factory := NewFactory("anthropic", map[string]Provider{"anthropic": NoopProvider{}})

	if _, err := FactoryProvider(factory, "missing", "anthropic", ""); err == nil {
		t.Fatal("expected unknown provider error")
	}
}

func TestNoopProviderStreamProducesUnifiedEvents(t *testing.T) {
	reader, err := NoopProvider{}.Stream(types.Context(nil), &types.MessageRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "ping"}}})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var got []StreamEventType
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		got = append(got, event.Type)
	}

	want := []StreamEventType{StreamEventMessageStart, StreamEventMessageDelta, StreamEventUsage, StreamEventStop}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
