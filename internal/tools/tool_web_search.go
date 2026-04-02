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

type webSearchInput struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	TimeoutMS      int      `json:"timeout_ms,omitempty"`
}

type webSearchResultItem struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   []webSearchHit `json:"content,omitempty"`
}

type webSearchOutput struct {
	Query           string                `json:"query"`
	Results         []webSearchResultItem `json:"results"`
	DurationSeconds float64               `json:"duration_seconds"`
}

func newWebSearchTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeDangerFull,
		spec: types.ToolSpec{
			Name:        "web_search",
			Description: "Run a lightweight HTML web search and return filtered results",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"allowed_domains":{"type":"array","items":{"type":"string"}},"blocked_domains":{"type":"array","items":{"type":"string"}},"limit":{"type":"integer","minimum":1},"timeout_ms":{"type":"integer","minimum":1}},"required":["query"]}`),
		},
		exec: executeWebSearch,
	}
}

func executeWebSearch(ctx context.Context, input json.RawMessage, _ ToolEnv) (*types.ToolResult, error) {
	var req webSearchInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("web_search: invalid input: %w", err)
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("web_search: query is required")
	}
	if req.Limit <= 0 {
		req.Limit = 8
	}
	searchURL, err := buildSearchURL(req.Query)
	if err != nil {
		return nil, fmt.Errorf("web_search: %w", err)
	}

	started := time.Now()
	client := buildHTTPClient(time.Duration(req.TimeoutMS) * time.Millisecond)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("web_search: build request: %w", err)
	}
	httpReq.Header.Set("User-Agent", defaultWebUA)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("web_search: %w", err)
	}
	defer resp.Body.Close()

	htmlBody, err := readLimitedBody(resp, defaultWebMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("web_search: read body: %w", err)
	}

	hits := extractSearchHits(htmlBody)
	if len(hits) == 0 && resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.Host != "" {
		hits = extractSearchHitsFromGenericLinks(htmlBody)
	}
	if len(req.AllowedDomains) > 0 {
		filtered := hits[:0]
		for _, hit := range hits {
			if hostMatchesList(hit.URL, req.AllowedDomains) {
				filtered = append(filtered, hit)
			}
		}
		hits = filtered
	}
	if len(req.BlockedDomains) > 0 {
		filtered := hits[:0]
		for _, hit := range hits {
			if !hostMatchesList(hit.URL, req.BlockedDomains) {
				filtered = append(filtered, hit)
			}
		}
		hits = filtered
	}
	hits = dedupeSearchHits(hits)
	if len(hits) > req.Limit {
		hits = hits[:req.Limit]
	}

	summary := fmt.Sprintf("No web search results matched the query %q.", req.Query)
	if len(hits) > 0 {
		lines := make([]string, 0, len(hits))
		for _, hit := range hits {
			lines = append(lines, fmt.Sprintf("- [%s](%s)", hit.Title, hit.URL))
		}
		summary = fmt.Sprintf("Search results for %q. Include a Sources section in the final answer.\n%s", req.Query, strings.Join(lines, "\n"))
	}

	return &types.ToolResult{Output: rawJSON(webSearchOutput{
		Query: req.Query,
		Results: []webSearchResultItem{
			{Type: "commentary", Text: summary},
			{Type: "search_result", ToolUseID: "web_search_1", Content: hits},
		},
		DurationSeconds: time.Since(started).Seconds(),
	})}, nil
}
