package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"claude-go-code/pkg/types"
)

const (
	chatCompletionsEndpoint = "/chat/completions"
	defaultMaxTokens        = 8096
	maxSSELineBytes         = 1 << 20
)

type httpClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewHTTPClient(cfg Config) Client {
	return &httpClient{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		http:    &http.Client{},
	}
}

func (c *httpClient) CreateResponse(ctx context.Context, req *types.MessageRequest) (*types.MessageResponse, error) {
	apiReq := buildAPIRequest(req, false)
	resp, err := c.doRequest(ctx, apiReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return toMessageResponse(&apiResp), nil
}

func (c *httpClient) StreamResponses(ctx context.Context, req *types.MessageRequest) ([]Event, error) {
	apiReq := buildAPIRequest(req, true)
	resp, err := c.doRequest(ctx, apiReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return parseSSEStream(resp.Body)
}

func (c *httpClient) doRequest(ctx context.Context, apiReq chatCompletionRequest) (*http.Response, error) {
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatCompletionsEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai api error (status %d): %s", resp.StatusCode, errBody)
	}
	return resp, nil
}

type chatCompletionRequest struct {
	Model       string             `json:"model"`
	Messages    []chatMessage      `json:"messages"`
	Tools       []chatTool         `json:"tools,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	StreamOpts  *streamOptions     `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role       string          `json:"role"`
	Content    interface{}     `json:"content,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolCalls  []chatToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatFunctionCall `json:"function"`
}

type chatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionResponse struct {
	ID      string         `json:"id"`
	Choices []chatChoice   `json:"choices"`
	Usage   chatUsage      `json:"usage"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMsg     `json:"message"`
	Delta        chatMsg     `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type chatMsg struct {
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func buildAPIRequest(req *types.MessageRequest, stream bool) chatCompletionRequest {
	r := chatCompletionRequest{
		Model:     req.Model,
		MaxTokens: defaultMaxTokens,
		Stream:    stream,
	}

	if stream {
		r.StreamOpts = &streamOptions{IncludeUsage: true}
	}

	if req.System != "" {
		r.Messages = append(r.Messages, chatMessage{Role: "system", Content: req.System})
	}

	for _, msg := range req.Messages {
		r.Messages = append(r.Messages, toChatMessage(msg))
	}

	for _, t := range req.Tools {
		r.Tools = append(r.Tools, chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	return r
}

func toChatMessage(msg types.Message) chatMessage {
	if msg.Role == types.RoleTool && msg.ToolResult != nil {
		content := ""
		if msg.ToolResult.Error != "" {
			content = msg.ToolResult.Error
		} else if msg.ToolResult.Output != nil {
			content = string(msg.ToolResult.Output)
		}
		return chatMessage{
			Role:       "tool",
			Content:    content,
			ToolCallID: msg.ToolResult.ToolCallID,
		}
	}

	if msg.Role == types.RoleAssistant && len(msg.ToolCalls) > 0 {
		var toolCalls []chatToolCall
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, chatToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: chatFunctionCall{
					Name:      tc.Name,
					Arguments: string(tc.Input),
				},
			})
		}
		return chatMessage{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: toolCalls,
		}
	}

	return chatMessage{Role: string(msg.Role), Content: msg.Content}
}

func toMessageResponse(resp *chatCompletionResponse) *types.MessageResponse {
	if len(resp.Choices) == 0 {
		return &types.MessageResponse{}
	}

	choice := resp.Choices[0]
	var toolCalls []types.ToolCall
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, types.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return &types.MessageResponse{
		Message: types.Message{
			ID:        resp.ID,
			Role:      types.RoleAssistant,
			Content:   choice.Message.Content,
			ToolCalls: toolCalls,
		},
		Usage: types.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
		StopReason: choice.FinishReason,
	}
}

func parseSSEStream(r io.Reader) ([]Event, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, maxSSELineBytes), maxSSELineBytes)

	var events []Event
	type pendingToolCall struct {
		id, name string
		args     strings.Builder
	}
	pending := map[int]*pendingToolCall{}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			events = append(events, Event{Type: EventTypeResponseComplete})
			break
		}

		var chunk chatCompletionResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage.TotalTokens > 0 {
			usage := types.Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
				TotalTokens:  chunk.Usage.TotalTokens,
			}
			events = append(events, Event{Type: EventTypeResponseUsage, Usage: &usage})
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		finishReason := chunk.Choices[0].FinishReason

		if delta.Content != "" {
			events = append(events, Event{Type: EventTypeResponseDelta, DeltaText: delta.Content})
		}

		for i, tc := range delta.ToolCalls {
			idx := tc.ID
			if idx == "" {
				idx = fmt.Sprintf("%d", i)
			}
			idxInt := i

			if tc.Function.Name != "" {
				pending[idxInt] = &pendingToolCall{id: tc.ID, name: tc.Function.Name}
			}
			if pt, ok := pending[idxInt]; ok {
				pt.args.WriteString(tc.Function.Arguments)
			}
		}

		if finishReason == "tool_calls" || finishReason == "stop" {
			for idx, pt := range pending {
				raw := json.RawMessage(pt.args.String())
				if pt.args.Len() == 0 {
					raw = json.RawMessage("{}")
				}
				events = append(events, Event{
					Type:     EventTypeResponseToolCall,
					ToolCall: &types.ToolCall{ID: pt.id, Name: pt.name, Input: raw},
				})
				delete(pending, idx)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read sse stream: %w", err)
	}
	return events, nil
}
