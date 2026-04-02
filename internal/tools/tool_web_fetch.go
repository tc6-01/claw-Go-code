package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type webFetchInput struct {
	URL       string `json:"url"`
	Prompt    string `json:"prompt,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type webFetchOutput struct {
	URL         string `json:"url"`
	Code        int    `json:"code"`
	CodeText    string `json:"code_text,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Bytes       int    `json:"bytes"`
	DurationMS  int64  `json:"duration_ms"`
	Result      string `json:"result"`
}

func newWebFetchTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeDangerFull,
		spec: types.ToolSpec{
			Name:        "web_fetch",
			Description: "Fetch a web page and return a prompt-aware text summary",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"},"prompt":{"type":"string"},"timeout_ms":{"type":"integer","minimum":1}},"required":["url"]}`),
		},
		exec: executeWebFetch,
	}
}

func executeWebFetch(ctx context.Context, input json.RawMessage, _ ToolEnv) (*types.ToolResult, error) {
	var req webFetchInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("web_fetch: invalid input: %w", err)
	}
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("web_fetch: url is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		req.Prompt = "Summarize this page"
	}

	requestURL, err := normalizeFetchURL(req.URL)
	if err != nil {
		return nil, fmt.Errorf("web_fetch: %w", err)
	}
	started := time.Now()
	client := buildHTTPClient(time.Duration(req.TimeoutMS) * time.Millisecond)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("web_fetch: build request: %w", err)
	}
	httpReq.Header.Set("User-Agent", defaultWebUA)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("web_fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := readLimitedBody(resp, defaultWebMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("web_fetch: read body: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")
	normalized := normalizeFetchedContent(body, contentType)
	result := summarizeWebFetch(resp.Request.URL.String(), req.Prompt, normalized, body, contentType)

	return &types.ToolResult{Output: rawJSON(webFetchOutput{
		URL:         resp.Request.URL.String(),
		Code:        resp.StatusCode,
		CodeText:    http.StatusText(resp.StatusCode),
		ContentType: contentType,
		Bytes:       len(body),
		DurationMS:  time.Since(started).Milliseconds(),
		Result:      result,
	})}, nil
}
