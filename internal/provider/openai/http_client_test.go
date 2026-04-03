package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"claude-go-code/pkg/types"
)

func TestHTTPClientCreateResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatCompletionResponse{
			ID: "resp-1",
			Choices: []chatChoice{{
				Message:      chatMsg{Role: "assistant", Content: "hello"},
				FinishReason: "stop",
			}},
			Usage: chatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer srv.Close()

	client := NewHTTPClient(Config{BaseURL: srv.URL, APIKey: "test-key"})
	resp, err := client.CreateResponse(t.Context(), &types.MessageRequest{
		Model:    "gpt-4o-mini",
		Messages: []types.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("create response: %v", err)
	}
	if resp.Message.Content != "hello" {
		t.Fatalf("unexpected content: %s", resp.Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected tokens: %d", resp.Usage.TotalTokens)
	}
}

func TestHTTPClientCreateResponseWithToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatCompletionResponse{
			ID: "resp-2",
			Choices: []chatChoice{{
				Message: chatMsg{
					Role: "assistant",
					ToolCalls: []chatToolCall{{
						ID:       "call-1",
						Type:     "function",
						Function: chatFunctionCall{Name: "read_file", Arguments: `{"path":"/tmp/x"}`},
					}},
				},
				FinishReason: "tool_calls",
			}},
			Usage: chatUsage{PromptTokens: 8, CompletionTokens: 12, TotalTokens: 20},
		})
	}))
	defer srv.Close()

	client := NewHTTPClient(Config{BaseURL: srv.URL, APIKey: "key"})
	resp, err := client.CreateResponse(t.Context(), &types.MessageRequest{
		Model:    "gpt-4o-mini",
		Messages: []types.Message{{Role: "user", Content: "read a file"}},
	})
	if err != nil {
		t.Fatalf("create response: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	tc := resp.Message.ToolCalls[0]
	if tc.Name != "read_file" || tc.ID != "call-1" {
		t.Fatalf("unexpected tool call: %+v", tc)
	}
}

func TestHTTPClientStreamResponses(t *testing.T) {
	ssePayload := strings.Join([]string{
		`data: {"id":"resp-1","choices":[{"delta":{"content":"hel"},"finish_reason":null}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		`data: {"id":"resp-1","choices":[{"delta":{"content":"lo"},"finish_reason":null}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		`data: {"id":"resp-1","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		`data: {"id":"resp-1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		`data: [DONE]`,
		"",
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ssePayload)
	}))
	defer srv.Close()

	client := NewHTTPClient(Config{BaseURL: srv.URL, APIKey: "key"})
	events, err := client.StreamResponses(t.Context(), &types.MessageRequest{
		Model:    "gpt-4o-mini",
		Messages: []types.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var text strings.Builder
	var hasUsage, hasComplete bool
	for _, e := range events {
		switch e.Type {
		case EventTypeResponseDelta:
			text.WriteString(e.DeltaText)
		case EventTypeResponseUsage:
			hasUsage = true
			if e.Usage.TotalTokens != 15 {
				t.Fatalf("unexpected usage tokens: %d", e.Usage.TotalTokens)
			}
		case EventTypeResponseComplete:
			hasComplete = true
		}
	}
	if text.String() != "hello" {
		t.Fatalf("unexpected streamed text: %q", text.String())
	}
	if !hasUsage {
		t.Fatal("missing usage event")
	}
	if !hasComplete {
		t.Fatal("missing complete event")
	}
}

func TestHTTPClientStreamToolCalls(t *testing.T) {
	ssePayload := strings.Join([]string{
		`data: {"id":"resp-1","choices":[{"delta":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"bash","arguments":""}}]},"finish_reason":null}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		`data: {"id":"resp-1","choices":[{"delta":{"tool_calls":[{"id":"","type":"function","function":{"name":"","arguments":"{\"cmd\":"}}]},"finish_reason":null}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		`data: {"id":"resp-1","choices":[{"delta":{"tool_calls":[{"id":"","type":"function","function":{"name":"","arguments":"\"ls\"}"}}]},"finish_reason":null}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		`data: {"id":"resp-1","choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		`data: [DONE]`,
		"",
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ssePayload)
	}))
	defer srv.Close()

	client := NewHTTPClient(Config{BaseURL: srv.URL, APIKey: "key"})
	events, err := client.StreamResponses(t.Context(), &types.MessageRequest{
		Model:    "gpt-4o-mini",
		Messages: []types.Message{{Role: "user", Content: "run ls"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var foundTool bool
	for _, e := range events {
		if e.Type == EventTypeResponseToolCall && e.ToolCall != nil {
			foundTool = true
			if e.ToolCall.Name != "bash" {
				t.Fatalf("unexpected tool name: %s", e.ToolCall.Name)
			}
			if e.ToolCall.ID != "call-1" {
				t.Fatalf("unexpected tool id: %s", e.ToolCall.ID)
			}
		}
	}
	if !foundTool {
		t.Fatal("no tool call event found")
	}
}

func TestHTTPClientAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	client := NewHTTPClient(Config{BaseURL: srv.URL, APIKey: "bad"})
	_, err := client.CreateResponse(t.Context(), &types.MessageRequest{Model: "gpt-4o-mini"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 in error: %v", err)
	}
}

func TestBuildAPIRequestWithSystem(t *testing.T) {
	req := &types.MessageRequest{
		Model:  "gpt-4o-mini",
		System: "you are helpful",
		Messages: []types.Message{
			{Role: "user", Content: "hi"},
		},
	}
	apiReq := buildAPIRequest(req, false)
	if len(apiReq.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(apiReq.Messages))
	}
	if apiReq.Messages[0].Role != "system" {
		t.Fatalf("first message should be system, got %s", apiReq.Messages[0].Role)
	}
}

func TestBuildAPIRequestToolResult(t *testing.T) {
	req := &types.MessageRequest{
		Model: "gpt-4o-mini",
		Messages: []types.Message{
			{
				Role: types.RoleTool,
				ToolResult: &types.ToolResult{
					ToolCallID: "call-1",
					Output:     json.RawMessage(`"file content"`),
				},
			},
		},
	}
	apiReq := buildAPIRequest(req, false)
	msg := apiReq.Messages[0]
	if msg.Role != "tool" {
		t.Fatalf("expected tool role, got %s", msg.Role)
	}
	if msg.ToolCallID != "call-1" {
		t.Fatalf("expected tool_call_id call-1, got %s", msg.ToolCallID)
	}
}
