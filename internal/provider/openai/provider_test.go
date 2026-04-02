package openai

import (
	"context"
	"errors"
	"io"
	"testing"

	shared "claude-go-code/internal/provider"
	"claude-go-code/pkg/types"
)

func TestNewConfigNormalizesDefaults(t *testing.T) {
	cfg := NewConfig(struct{ BaseURL, APIKey string }{BaseURL: "https://api.openai.com/v1/", APIKey: "  key  "})
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("unexpected base url: %s", cfg.BaseURL)
	}
	if cfg.APIKey != "key" {
		t.Fatalf("unexpected api key: %q", cfg.APIKey)
	}
}

func TestNewConfigUsesDefaultBaseURL(t *testing.T) {
	cfg := NewConfig(struct{ BaseURL, APIKey string }{})
	if cfg.BaseURL != defaultBaseURL {
		t.Fatalf("expected default base url, got %s", cfg.BaseURL)
	}
}

func TestProviderNormalizeModel(t *testing.T) {
	p := New(Config{}, nil)
	cases := map[string]string{
		"":          "gpt-4o-mini",
		"4o-mini":   "gpt-4o-mini",
		" gpt-4.1 ": "gpt-4.1",
		"o3-mini":   "o3-mini",
	}
	for input, want := range cases {
		if got := p.NormalizeModel(input); got != want {
			t.Fatalf("normalize %q: got %q want %q", input, got, want)
		}
	}
}

func TestProviderSendDelegatesToClient(t *testing.T) {
	p := New(Config{}, fakeClient{response: &types.MessageResponse{Message: types.Message{Role: types.RoleAssistant, Content: "ok"}, StopReason: "stop"}})
	resp, err := p.Send(context.Background(), &types.MessageRequest{})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if resp.Message.Content != "ok" {
		t.Fatalf("unexpected content: %s", resp.Message.Content)
	}
}

func TestProviderStreamMapsUnifiedEvents(t *testing.T) {
	p := New(Config{}, fakeClient{events: []Event{
		{Type: EventTypeResponseDelta, DeltaText: "hi"},
		{Type: EventTypeResponseToolCall, ToolCall: &types.ToolCall{ID: "tool-1", Name: "read_file"}},
		{Type: EventTypeResponseUsage, Usage: &types.Usage{TotalTokens: 9}},
		{Type: EventTypeResponseComplete},
	}})
	reader, err := p.Stream(context.Background(), &types.MessageRequest{})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()

	assertEvent(t, reader, shared.StreamEventMessageStart, "")
	assertEvent(t, reader, shared.StreamEventMessageDelta, "hi")

	event, err := reader.Next()
	if err != nil {
		t.Fatalf("next tool event: %v", err)
	}
	if event.Type != shared.StreamEventToolCall || event.ToolCall == nil || event.ToolCall.Name != "read_file" {
		t.Fatalf("unexpected tool event: %#v", event)
	}

	event, err = reader.Next()
	if err != nil {
		t.Fatalf("next usage event: %v", err)
	}
	if event.Type != shared.StreamEventUsage || event.Usage == nil || event.Usage.TotalTokens != 9 {
		t.Fatalf("unexpected usage event: %#v", event)
	}

	assertEvent(t, reader, shared.StreamEventStop, "")
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestProviderStreamConvertsMissingUsageToErrorEvent(t *testing.T) {
	p := New(Config{}, fakeClient{events: []Event{{Type: EventTypeResponseUsage}}})
	reader, err := p.Stream(context.Background(), &types.MessageRequest{})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()

	assertEvent(t, reader, shared.StreamEventMessageStart, "")
	event, err := reader.Next()
	if err != nil {
		t.Fatalf("next error event: %v", err)
	}
	if event.Type != shared.StreamEventError || event.Error == nil || event.Error.Error() != "openai usage event missing usage payload" {
		t.Fatalf("unexpected missing-usage event: %#v", event)
	}
}

func TestProviderStreamMapsErrorEvent(t *testing.T) {
	wantErr := errors.New("boom")
	p := New(Config{}, fakeClient{events: []Event{{Type: EventTypeResponseError, Err: wantErr}}})
	reader, err := p.Stream(context.Background(), &types.MessageRequest{})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()

	assertEvent(t, reader, shared.StreamEventMessageStart, "")
	event, err := reader.Next()
	if err != nil {
		t.Fatalf("next error event: %v", err)
	}
	if event.Type != shared.StreamEventError || !errors.Is(event.Error, wantErr) {
		t.Fatalf("unexpected error event: %#v", event)
	}
}

func TestProviderStreamReturnsClientError(t *testing.T) {
	p := New(Config{}, fakeClient{streamErr: errors.New("boom")})
	if _, err := p.Stream(context.Background(), &types.MessageRequest{}); err == nil {
		t.Fatal("expected stream error")
	}
}

func assertEvent(t *testing.T, reader shared.StreamReader, wantType shared.StreamEventType, wantText string) {
	t.Helper()
	event, err := reader.Next()
	if err != nil {
		t.Fatalf("next event: %v", err)
	}
	if event.Type != wantType || event.Text != wantText {
		t.Fatalf("unexpected event: %#v", event)
	}
}

type fakeClient struct {
	response  *types.MessageResponse
	events    []Event
	sendErr   error
	streamErr error
}

func (f fakeClient) CreateResponse(_ context.Context, _ *types.MessageRequest) (*types.MessageResponse, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	if f.response != nil {
		return f.response, nil
	}
	return &types.MessageResponse{Message: types.Message{Role: types.RoleAssistant, Content: "default"}}, nil
}

func (f fakeClient) StreamResponses(_ context.Context, _ *types.MessageRequest) ([]Event, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return f.events, nil
}
