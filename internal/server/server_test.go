package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
	sharedprovider "claude-go-code/internal/provider"
	"claude-go-code/internal/runtime"
	"claude-go-code/internal/session"
	"claude-go-code/internal/tools"
	"claude-go-code/pkg/types"
)

func newScriptedProvider() *scriptedProvider {
	return &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("hello from assistant"),
			sharedprovider.UsageEvent(types.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}),
			sharedprovider.StopEvent(),
		},
	}}
}

func testEngine(t *testing.T) (runtime.Engine, session.Store) {
	t.Helper()
	store := session.NewInMemoryStore()
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "test-model"
	eng := runtime.NewEngine(runtime.Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": newScriptedProvider(),
		}),
		ToolRegistry: tools.NewRegistry(nil),
		Permission:   permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})
	return eng, store
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	eng, store := testEngine(t)
	srv := New(eng, store, config.ServerConfig{
		Host:            "127.0.0.1",
		Port:            0,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	})
	return httptest.NewServer(srv.Handler())
}

func testServerWithAuth(t *testing.T) *httptest.Server {
	t.Helper()
	eng, store := testEngine(t)
	srv := New(eng, store, config.ServerConfig{
		Host:            "127.0.0.1",
		Port:            0,
		APIKeys:         []string{"test-key-123"},
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	})
	return httptest.NewServer(srv.Handler())
}

func TestHealthEndpoint(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("get health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("status = %v", body["status"])
	}
}

func TestCreateAndGetSession(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/v1/sessions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var created struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	if created.ID == "" {
		t.Fatal("empty session id")
	}

	resp2, err := http.Get(ts.URL + "/v1/sessions/" + created.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("get status = %d", resp2.StatusCode)
	}
}

func TestListSessions(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	http.Post(ts.URL+"/v1/sessions", "application/json", strings.NewReader(`{}`))

	resp, err := http.Get(ts.URL + "/v1/sessions")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDeleteSession(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/v1/sessions", "application/json", strings.NewReader(`{}`))
	var created struct{ ID string `json:"id"` }
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	req, _ := http.NewRequest("DELETE", ts.URL+"/v1/sessions/"+created.ID, nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete session: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("status = %d", resp2.StatusCode)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/sessions/nonexistent")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSendMessageNonStream(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/v1/sessions", "application/json", strings.NewReader(`{}`))
	var created struct{ ID string `json:"id"` }
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	stream := false
	body, _ := json.Marshal(map[string]interface{}{
		"content": "hello",
		"stream":  stream,
	})
	resp2, err := http.Post(ts.URL+"/v1/sessions/"+created.ID+"/messages", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("status = %d, body = %s", resp2.StatusCode, b)
	}

	var msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	json.NewDecoder(resp2.Body).Decode(&msg)
	if msg.Content == "" {
		t.Fatal("empty assistant content")
	}
}

func TestSendMessageStream(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/v1/sessions", "application/json", strings.NewReader(`{}`))
	var created struct{ ID string `json:"id"` }
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"content": "hello",
		"stream":  true,
	})
	resp2, err := http.Post(ts.URL+"/v1/sessions/"+created.ID+"/messages", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("status = %d, body = %s", resp2.StatusCode, b)
	}
	if !strings.Contains(resp2.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type = %s, want text/event-stream", resp2.Header.Get("Content-Type"))
	}

	sseData, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(sseData), "text_delta") {
		t.Fatalf("SSE output missing text_delta: %s", sseData)
	}
	if !strings.Contains(string(sseData), "done") {
		t.Fatalf("SSE output missing done: %s", sseData)
	}
}

func TestAuthMiddleware(t *testing.T) {
	ts := testServerWithAuth(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/sessions")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get with key: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp2.StatusCode)
	}

	resp3, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("health status = %d, want 200 (should bypass auth)", resp3.StatusCode)
	}
}

func TestGetMessages(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/v1/sessions", "application/json", strings.NewReader(`{}`))
	var created struct{ ID string `json:"id"` }
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	body, _ := json.Marshal(map[string]interface{}{"content": "hi", "stream": false})
	resp2, _ := http.Post(ts.URL+"/v1/sessions/"+created.ID+"/messages", "application/json", strings.NewReader(string(body)))
	resp2.Body.Close()

	resp3, err := http.Get(ts.URL + "/v1/sessions/" + created.ID + "/messages")
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("status = %d", resp3.StatusCode)
	}
	var result struct {
		Messages []types.Message `json:"messages"`
	}
	json.NewDecoder(resp3.Body).Decode(&result)
	if len(result.Messages) == 0 {
		t.Fatal("expected messages after sending one")
	}
}

func TestListModels(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

type scriptedProvider struct {
	events [][]*sharedprovider.StreamEvent
	index  int
}

func (p *scriptedProvider) Send(context.Context, *types.MessageRequest) (*types.MessageResponse, error) {
	return &types.MessageResponse{}, nil
}

func (p *scriptedProvider) Stream(_ context.Context, _ *types.MessageRequest) (sharedprovider.StreamReader, error) {
	if p.index >= len(p.events) {
		return &sharedprovider.SliceStreamReader{Events: []*sharedprovider.StreamEvent{
			sharedprovider.MessageStartEvent(), sharedprovider.StopEvent(),
		}}, nil
	}
	reader := &sharedprovider.SliceStreamReader{Events: p.events[p.index]}
	p.index++
	return reader, nil
}

func (p *scriptedProvider) NormalizeModel(model string) string {
	return model
}

func (p *scriptedProvider) Capabilities() sharedprovider.ProviderCapabilities {
	return sharedprovider.ProviderCapabilities{Streaming: true, ToolCalls: true}
}
