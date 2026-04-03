package sdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

type Session struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	WorkDir string `json:"work_dir,omitempty"`
}

type CreateSessionOpts struct {
	Model   string `json:"model,omitempty"`
	RepoURL string `json:"repo_url,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

type ChatResult struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamEvent struct {
	Type       string          `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolCall   json.RawMessage `json:"tool_call,omitempty"`
	ToolResult json.RawMessage `json:"tool_result,omitempty"`
	Error      string          `json:"error,omitempty"`
	Usage      json.RawMessage `json:"usage,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SessionSummary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Model     string    `json:"model"`
	CWD       string    `json:"cwd"`
}

func (c *Client) CreateSession(ctx context.Context, opts *CreateSessionOpts) (*Session, error) {
	var body io.Reader
	if opts != nil {
		data, _ := json.Marshal(opts)
		body = bytes.NewReader(data)
	}

	resp, err := c.do(ctx, http.MethodPost, "/v1/sessions", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sess Session
	if err := json.NewDecoder(resp.Body).Decode(&sess); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &sess, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/sessions/"+sessionID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sess Session
	if err := json.NewDecoder(resp.Body).Decode(&sess); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &sess, nil
}

func (c *Client) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/sessions", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Sessions []SessionSummary `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sessions: %w", err)
	}
	return result.Sessions, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) Chat(ctx context.Context, sessionID, message string) (*ChatResult, error) {
	f := false
	data, _ := json.Marshal(map[string]interface{}{
		"content": message,
		"stream":  &f,
	})

	resp, err := c.do(ctx, http.MethodPost, "/v1/sessions/"+sessionID+"/messages", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChatResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode chat result: %w", err)
	}
	return &result, nil
}

type StreamHandler func(event *StreamEvent) bool

func (c *Client) ChatStream(ctx context.Context, sessionID, message string, handler StreamHandler) error {
	data, _ := json.Marshal(map[string]interface{}{
		"content": message,
		"stream":  true,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/sessions/"+sessionID+"/messages", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		eventData := strings.TrimPrefix(line, "data: ")
		if eventData == "" {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(eventData), &event); err != nil {
			continue
		}

		if !handler(&event) {
			return nil
		}
	}

	return scanner.Err()
}

func (c *Client) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/sessions/"+sessionID+"/messages", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Messages []Message `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}
	return result.Messages, nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, errBody)
	}
	return resp, nil
}
