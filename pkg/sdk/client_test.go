package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientCreateSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sessions" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Session{ID: "sess_1", Model: "claude-sonnet-4-5"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	sess, err := client.CreateSession(context.Background(), nil)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.ID != "sess_1" {
		t.Fatalf("unexpected session id: %s", sess.ID)
	}
}

func TestClientChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ChatResult{Role: "assistant", Content: "hello"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key")
	result, err := client.Chat(context.Background(), "sess_1", "hi")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if result.Content != "hello" {
		t.Fatalf("unexpected content: %s", result.Content)
	}
}

func TestClientChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		events := []StreamEvent{
			{Type: "text_delta", Text: "hel"},
			{Type: "text_delta", Text: "lo"},
			{Type: "done"},
		}
		for _, e := range events {
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key")
	var text strings.Builder
	err := client.ChatStream(context.Background(), "sess_1", "hi", func(event *StreamEvent) bool {
		if event.Type == "text_delta" {
			text.WriteString(event.Text)
		}
		return event.Type != "done"
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if text.String() != "hello" {
		t.Fatalf("unexpected text: %q", text.String())
	}
}

func TestClientListSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": []SessionSummary{{ID: "s1"}, {ID: "s2"}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key")
	sessions, err := client.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestClientDeleteSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"deleted": true})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key")
	if err := client.DeleteSession(context.Background(), "sess_1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestClientAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"type":"auth_error","message":"invalid key"}}`)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "bad")
	_, err := client.CreateSession(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 in error: %v", err)
	}
}
