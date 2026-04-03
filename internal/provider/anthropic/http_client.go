package anthropic

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
	apiVersion       = "2023-06-01"
	messagesEndpoint = "/v1/messages"
	defaultMaxTokens = 8096
	maxSSELineBytes  = 1 << 20 // 1MB
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

func (c *httpClient) CreateMessage(ctx context.Context, req *types.MessageRequest) (*types.MessageResponse, error) {
	resp, err := c.doRequest(ctx, buildAPIRequest(req, false))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return toMessageResponse(&apiResp), nil
}

func (c *httpClient) StreamMessages(ctx context.Context, req *types.MessageRequest) ([]Event, error) {
	resp, err := c.doRequest(ctx, buildAPIRequest(req, true))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return parseSSEStream(resp.Body)
}

func (c *httpClient) doRequest(ctx context.Context, apiReq apiRequest) (*http.Response, error) {
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+messagesEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic api error (status %d): %s", resp.StatusCode, errBody)
	}
	return resp, nil
}

// --- Anthropic API types ---

type apiRequest struct {
	Model     string       `json:"model"`
	Messages  []apiMessage `json:"messages"`
	System    string       `json:"system,omitempty"`
	MaxTokens int          `json:"max_tokens"`
	Tools     []apiTool    `json:"tools,omitempty"`
	Stream    bool         `json:"stream,omitempty"`
}

type apiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type apiTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type apiResponse struct {
	ID         string             `json:"id"`
	Role       string             `json:"role"`
	Content    []apiResponseBlock `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      apiUsage           `json:"usage"`
}

type apiResponseBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// --- Request building ---

func buildAPIRequest(req *types.MessageRequest, stream bool) apiRequest {
	ar := apiRequest{
		Model:     req.Model,
		System:    req.System,
		MaxTokens: defaultMaxTokens,
		Stream:    stream,
	}
	for _, msg := range req.Messages {
		ar.Messages = append(ar.Messages, toAPIMessage(msg))
	}
	for _, t := range req.Tools {
		ar.Tools = append(ar.Tools, apiTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return ar
}

func toAPIMessage(msg types.Message) apiMessage {
	if msg.Role == types.RoleTool && msg.ToolResult != nil {
		block := map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": msg.ToolResult.ToolCallID,
		}
		if msg.ToolResult.Error != "" {
			block["content"] = msg.ToolResult.Error
			block["is_error"] = true
		} else if msg.ToolResult.Output != nil {
			block["content"] = string(msg.ToolResult.Output)
		}
		return apiMessage{Role: "user", Content: []interface{}{block}}
	}

	if msg.Role == types.RoleAssistant && len(msg.ToolCalls) > 0 {
		var blocks []interface{}
		if msg.Content != "" {
			blocks = append(blocks, map[string]interface{}{"type": "text", "text": msg.Content})
		}
		for _, tc := range msg.ToolCalls {
			blocks = append(blocks, map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Name,
				"input": tc.Input,
			})
		}
		return apiMessage{Role: "assistant", Content: blocks}
	}

	return apiMessage{Role: string(msg.Role), Content: msg.Content}
}

// --- Response conversion ---

func toMessageResponse(resp *apiResponse) *types.MessageResponse {
	var textParts []string
	var toolCalls []types.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, types.ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return &types.MessageResponse{
		Message: types.Message{
			ID:        resp.ID,
			Role:      types.RoleAssistant,
			Content:   strings.Join(textParts, ""),
			ToolCalls: toolCalls,
		},
		Usage: types.Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
		StopReason: resp.StopReason,
	}
}

// --- SSE stream parsing ---

type sseData struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
	Usage   *apiUsage    `json:"usage"`
	Message *apiResponse `json:"message"`
}

func parseSSEStream(r io.Reader) ([]Event, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, maxSSELineBytes), maxSSELineBytes)

	var events []Event
	var inputTokens int
	var lastUsage *types.Usage

	type pendingTool struct {
		id, name string
		input    strings.Builder
	}
	pending := map[int]*pendingTool{}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		var d sseData
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &d); err != nil {
			continue
		}

		switch d.Type {
		case "message_start":
			if d.Message != nil {
				inputTokens = d.Message.Usage.InputTokens
			}

		case "content_block_start":
			if d.ContentBlock.Type == "tool_use" {
				pending[d.Index] = &pendingTool{id: d.ContentBlock.ID, name: d.ContentBlock.Name}
			}

		case "content_block_delta":
			if d.Delta.Type == "text_delta" {
				events = append(events, Event{Type: EventTypeMessageDelta, DeltaText: d.Delta.Text})
			} else if d.Delta.Type == "input_json_delta" {
				if pt, ok := pending[d.Index]; ok {
					pt.input.WriteString(d.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if pt, ok := pending[d.Index]; ok {
				raw := json.RawMessage(pt.input.String())
				if pt.input.Len() == 0 {
					raw = json.RawMessage("{}")
				}
				events = append(events, Event{
					Type:     EventTypeToolUse,
					ToolCall: &types.ToolCall{ID: pt.id, Name: pt.name, Input: raw},
				})
				delete(pending, d.Index)
			}

		case "message_delta":
			if d.Usage != nil {
				lastUsage = &types.Usage{
					InputTokens:  inputTokens,
					OutputTokens: d.Usage.OutputTokens,
					TotalTokens:  inputTokens + d.Usage.OutputTokens,
				}
			}

		case "message_stop":
			events = append(events, Event{Type: EventTypeMessageStop, Usage: lastUsage})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read sse stream: %w", err)
	}
	return events, nil
}
