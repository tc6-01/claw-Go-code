package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"claude-go-code/internal/permissions"
)

func TestWebFetchReturnsPromptAwareSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/page" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Ignored</title></head><body><h1>Test Page</h1><p>Hello <b>world</b> from local server.</p></body></html>`))
	}))
	defer server.Close()

	result := executeToolForTest(t, newWebFetchTool(), t.TempDir(), permissions.ModeDangerFull, `{"url":"`+server.URL+`/page","prompt":"Summarize this page"}`)
	var payload webFetchOutput
	decodeToolOutput(t, result, &payload)
	if payload.Code != http.StatusOK {
		t.Fatalf("code = %d, want %d", payload.Code, http.StatusOK)
	}
	if !strings.Contains(payload.Result, "Fetched") || !strings.Contains(payload.Result, "Test Page") || !strings.Contains(payload.Result, "Hello world from local server") {
		t.Fatalf("unexpected fetch result: %q", payload.Result)
	}

	titled := executeToolForTest(t, newWebFetchTool(), t.TempDir(), permissions.ModeDangerFull, `{"url":"`+server.URL+`/page","prompt":"What is the page title?"}`)
	decodeToolOutput(t, titled, &payload)
	if !strings.Contains(payload.Result, "Title: Ignored") {
		t.Fatalf("unexpected title result: %q", payload.Result)
	}
}

func TestWebFetchSupportsPlainTextAndRejectsInvalidURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain text response"))
	}))
	defer server.Close()

	result := executeToolForTest(t, newWebFetchTool(), t.TempDir(), permissions.ModeDangerFull, `{"url":"`+server.URL+`","prompt":"Show me the content"}`)
	var payload webFetchOutput
	decodeToolOutput(t, result, &payload)
	if payload.URL != server.URL {
		t.Fatalf("url = %q, want %q", payload.URL, server.URL)
	}
	if !strings.Contains(payload.Result, "plain text response") {
		t.Fatalf("unexpected plain text result: %q", payload.Result)
	}

	_, err := newWebFetchTool().Execute(context.Background(), json.RawMessage(`{"url":"not a url","prompt":"Summarize"}`), ToolEnv{WorkingDir: t.TempDir(), Mode: permissions.ModeDangerFull})
	if err == nil || (!strings.Contains(err.Error(), "relative URL without a base") && !strings.Contains(err.Error(), "invalid")) {
		t.Fatalf("unexpected invalid URL error: %v", err)
	}
}

func TestWebSearchExtractsAndFiltersResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.URL.Query().Get("q") != "rust web search" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><a class="result__a" href="https://docs.rs/reqwest">Reqwest docs</a><a class="result__a" href="https://example.com/blocked">Blocked result</a></body></html>`))
	}))
	defer server.Close()

	t.Setenv("CLAW_WEB_SEARCH_BASE_URL", server.URL+"/search")
	result := executeToolForTest(t, newWebSearchTool(), t.TempDir(), permissions.ModeDangerFull, `{"query":"rust web search","allowed_domains":["https://DOCS.rs/"],"blocked_domains":["HTTPS://EXAMPLE.COM"]}`)
	var payload webSearchOutput
	decodeToolOutput(t, result, &payload)
	if payload.Query != "rust web search" {
		t.Fatalf("query = %q", payload.Query)
	}
	if len(payload.Results) != 2 || len(payload.Results[1].Content) != 1 {
		t.Fatalf("unexpected results payload: %#v", payload.Results)
	}
	if payload.Results[1].Content[0].Title != "Reqwest docs" || payload.Results[1].Content[0].URL != "https://docs.rs/reqwest" {
		t.Fatalf("unexpected search hit: %#v", payload.Results[1].Content[0])
	}
}

func TestWebSearchHandlesGenericLinksAndInvalidBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fallback" || r.URL.Query().Get("q") != "generic links" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><a href="https://example.com/one">Example One</a><a href="https://example.com/one">Duplicate Example One</a><a href="https://docs.rs/tokio">Tokio Docs</a></body></html>`))
	}))
	defer server.Close()

	t.Setenv("CLAW_WEB_SEARCH_BASE_URL", server.URL+"/fallback")
	result := executeToolForTest(t, newWebSearchTool(), t.TempDir(), permissions.ModeDangerFull, `{"query":"generic links"}`)
	var payload webSearchOutput
	decodeToolOutput(t, result, &payload)
	if len(payload.Results) != 2 || len(payload.Results[1].Content) != 2 {
		t.Fatalf("unexpected generic link results: %#v", payload.Results)
	}
	if payload.Results[1].Content[0].URL != "https://example.com/one" || payload.Results[1].Content[1].URL != "https://docs.rs/tokio" {
		t.Fatalf("unexpected generic hits: %#v", payload.Results[1].Content)
	}

	t.Setenv("CLAW_WEB_SEARCH_BASE_URL", "://bad-base-url")
	_, err := newWebSearchTool().Execute(context.Background(), json.RawMessage(`{"query":"generic links"}`), ToolEnv{WorkingDir: t.TempDir(), Mode: permissions.ModeDangerFull})
	if err == nil || (!strings.Contains(err.Error(), "relative URL without a base") && !strings.Contains(err.Error(), "missing protocol scheme") && !strings.Contains(err.Error(), "first path segment")) {
		t.Fatalf("unexpected invalid base error: %v", err)
	}
}
