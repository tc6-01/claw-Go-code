package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
	sharedprovider "claude-go-code/internal/provider"
	"claude-go-code/internal/session"
	"claude-go-code/internal/tools"
	"claude-go-code/pkg/types"
)

func TestEngineRunCreatesBootstrapSession(t *testing.T) {
	store := session.NewInMemoryStore()
	engine := NewEngine(Dependencies{
		Config:          config.DefaultConfig(t.TempDir()),
		SessionStore:    store,
		ProviderFactory: sharedprovider.NewStaticFactory(),
		ToolRegistry:    tools.NewRegistry(nil),
		Permission:      permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if sess.Model != "claude-sonnet-4-5" {
		t.Fatalf("unexpected session model: %s", sess.Model)
	}
	if len(sess.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(sess.Messages))
	}
	if sess.Messages[0].Role != types.RoleUser {
		t.Fatalf("unexpected bootstrap role: %s", sess.Messages[0].Role)
	}
	if sess.Messages[0].Content != "bootstrap:[status]" {
		t.Fatalf("unexpected bootstrap message: %q", sess.Messages[0].Content)
	}
	if sess.Messages[1].Role != types.RoleAssistant {
		t.Fatalf("unexpected assistant role: %s", sess.Messages[1].Role)
	}
	if sess.Messages[1].Usage.TotalTokens == 0 {
		t.Fatalf("expected assistant usage to persist")
	}
	if sess.Messages[1].StopReason != "stop" {
		t.Fatalf("unexpected stop reason: %q", sess.Messages[1].StopReason)
	}
	if len(sess.Usage) != 1 || sess.Usage[0].TotalTokens == 0 {
		t.Fatalf("expected session usage aggregate, got %#v", sess.Usage)
	}
}

func TestEngineRunExecutesToolCallsAndPersistsTrace(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	store := session.NewInMemoryStore()
	registry := tools.NewRegistry([]tools.Tool{stubTool{name: "echo"}})
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("calling tool"),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-1", Name: "echo", Input: json.RawMessage(`{"value":"hi"}`)}),
			sharedprovider.UsageEvent(types.Usage{InputTokens: 2, OutputTokens: 1, TotalTokens: 3}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.UsageEvent(types.Usage{InputTokens: 3, OutputTokens: 1, TotalTokens: 4}),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: registry,
		Permission:   permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.Messages) != 4 {
		t.Fatalf("message count = %d, want 4", len(sess.Messages))
	}
	if sess.Messages[1].Content != "calling tool" {
		t.Fatalf("unexpected assistant content: %q", sess.Messages[1].Content)
	}
	if len(sess.Messages[1].ToolCalls) != 1 || sess.Messages[1].ToolCalls[0].ID != "call-1" {
		t.Fatalf("assistant tool call trace mismatch: %#v", sess.Messages[1].ToolCalls)
	}
	if sess.Messages[1].Usage.TotalTokens != 3 {
		t.Fatalf("unexpected assistant usage: %#v", sess.Messages[1].Usage)
	}
	if sess.Messages[2].Role != types.RoleTool {
		t.Fatalf("expected tool role, got %s", sess.Messages[2].Role)
	}
	if !strings.Contains(sess.Messages[2].Content, `"tool_call_id":"call-1"`) || !strings.Contains(sess.Messages[2].Content, `"value":"hi"`) {
		t.Fatalf("unexpected tool output: %q", sess.Messages[2].Content)
	}
	if sess.Messages[2].ToolResult == nil || sess.Messages[2].ToolResult.ToolCallID != "call-1" {
		t.Fatalf("unexpected tool result trace: %#v", sess.Messages[2].ToolResult)
	}
	if sess.Messages[3].Content != "done" {
		t.Fatalf("unexpected final assistant content: %q", sess.Messages[3].Content)
	}
	if len(sess.Usage) != 2 || sess.Usage[0].TotalTokens != 3 || sess.Usage[1].TotalTokens != 4 {
		t.Fatalf("unexpected session usage trace: %#v", sess.Usage)
	}
	if len(sess.ToolTrace) != 1 {
		t.Fatalf("tool trace count = %d, want 1", len(sess.ToolTrace))
	}
	if !sess.ToolTrace[0].Success {
		t.Fatal("expected successful tool trace")
	}
	if sess.ToolTrace[0].Name != "echo" {
		t.Fatalf("unexpected tool trace name: %s", sess.ToolTrace[0].Name)
	}
	if sess.ToolTrace[0].ID != "call-1" {
		t.Fatalf("unexpected tool trace id: %s", sess.ToolTrace[0].ID)
	}
	if sess.ToolTrace[0].Result == nil || sess.ToolTrace[0].Result.ToolCallID != "call-1" {
		t.Fatalf("unexpected tool trace result: %#v", sess.ToolTrace[0].Result)
	}
}

func TestEngineRunChainsReadAndGrepBuiltinTools(t *testing.T) {
	root := t.TempDir()
	writeRuntimeFile(t, root, "notes.txt", "alpha\nneedle\nomega\n")

	cfg := config.DefaultConfig(root)
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-read", Name: "read_file", Input: json.RawMessage(`{"path":"notes.txt"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-grep", Name: "grep_search", Input: json.RawMessage(`{"pattern":"needle","path":"notes.txt","context_lines":1}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission:   permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.ToolTrace) != 2 {
		t.Fatalf("tool trace count = %d, want 2", len(sess.ToolTrace))
	}
	if !strings.Contains(sess.Messages[2].Content, `"name":"read_file"`) || !strings.Contains(sess.Messages[2].Content, `"bytes_read":19`) || !strings.Contains(sess.Messages[2].Content, `needle`) {
		t.Fatalf("unexpected read_file message: %s", sess.Messages[2].Content)
	}
	if !strings.Contains(sess.Messages[4].Content, `"line":2`) || !strings.Contains(sess.Messages[4].Content, `"text":"needle"`) {
		t.Fatalf("unexpected grep_search message: %s", sess.Messages[4].Content)
	}
}

func TestEngineRunPersistsTodoWriteSideEffect(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-todo", Name: "todo_write", Input: json.RawMessage(`{"todos":[{"content":"ship m4"},{"content":"run tests","done":true}]}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission:   permissions.NewStaticEngine(permissions.ModeWorkspaceWrite),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.Todos) != 2 || sess.Todos[1].Content != "run tests" || !sess.Todos[1].Done {
		t.Fatalf("unexpected persisted todos: %#v", sess.Todos)
	}
}

func TestEngineRunEditAndBashLoop(t *testing.T) {
	root := t.TempDir()
	writeRuntimeFile(t, root, "note.txt", "before\n")

	cfg := config.DefaultConfig(root)
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeDangerFull
	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-edit", Name: "edit_file", Input: json.RawMessage(`{"path":"note.txt","old_string":"before","new_string":"after"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash", Name: "bash", Input: json.RawMessage(`{"command":"cat note.txt"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission:   permissions.NewStaticEngine(permissions.ModeDangerFull),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "note.txt"))
	if err != nil {
		t.Fatalf("read edited file: %v", err)
	}
	if string(got) != "after\n" {
		t.Fatalf("edited file = %q", string(got))
	}
	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if !strings.Contains(sess.Messages[4].Content, `"command":"cat note.txt"`) || !strings.Contains(sess.Messages[4].Content, `after`) {
		t.Fatalf("unexpected bash message: %s", sess.Messages[4].Content)
	}
}

func TestEngineRunWebSearchThenFetch(t *testing.T) {
	var pageServer *httptest.Server
	pageServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body><a class="result__a" href="` + pageServer.URL + `/page">Example Page</a></body></html>`))
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><title>Example Page</title></head><body><p>Hello from fetched page.</p></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer pageServer.Close()
	t.Setenv("CLAW_WEB_SEARCH_BASE_URL", pageServer.URL+"/search")

	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeDangerFull
	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-search", Name: "web_search", Input: json.RawMessage(`{"query":"example page","allowed_domains":["` + pageServer.URL + `"]}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-fetch", Name: "web_fetch", Input: json.RawMessage(`{"url":"` + pageServer.URL + `/page","prompt":"Summarize this page"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission:   permissions.NewStaticEngine(permissions.ModeDangerFull),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.ToolTrace) != 2 {
		t.Fatalf("tool trace count = %d, want 2", len(sess.ToolTrace))
	}
	if !strings.Contains(sess.Messages[2].Content, `"tool_use_id":"web_search_1"`) || !strings.Contains(sess.Messages[2].Content, `Example Page`) {
		t.Fatalf("unexpected web_search message: %s", sess.Messages[2].Content)
	}
	if !strings.Contains(sess.Messages[4].Content, `Fetched`) || !strings.Contains(sess.Messages[4].Content, `Hello from fetched page`) {
		t.Fatalf("unexpected web_fetch message: %s", sess.Messages[4].Content)
	}
}

func TestEngineRunPersistsPromptPermissionDecision(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeWorkspaceWrite
	cfg.Permission.EscalationPolicy = permissions.EscalationPrompt

	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash", Name: "bash", Input: json.RawMessage(`{"command":"pwd"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
		}),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.ToolTrace) != 1 {
		t.Fatalf("tool trace count = %d, want 1", len(sess.ToolTrace))
	}
	if sess.ToolTrace[0].Permission != string(permissions.DecisionPrompt) {
		t.Fatalf("unexpected permission decision: %#v", sess.ToolTrace[0])
	}
	if !strings.Contains(sess.Messages[2].Content, "needs confirmation") {
		t.Fatalf("unexpected tool message: %s", sess.Messages[2].Content)
	}
}

func TestEngineRunExecutesToolAfterConfirmation(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeWorkspaceWrite
	cfg.Permission.EscalationPolicy = permissions.EscalationPrompt

	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash", Name: "bash", Input: json.RawMessage(`{"command":"printf approved"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
			Confirmer: permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
				if req.ToolName != "bash" {
					t.Fatalf("unexpected permission request: %#v", req)
				}
				return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeOnce}, nil
			}),
		}),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.ToolTrace) != 1 || !sess.ToolTrace[0].Success {
		t.Fatalf("unexpected tool trace: %#v", sess.ToolTrace)
	}
	if !strings.Contains(sess.Messages[2].Content, "approved") {
		t.Fatalf("unexpected bash tool message: %s", sess.Messages[2].Content)
	}
}

func TestEngineRunReusesSessionApprovalForRepeatedTool(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeWorkspaceWrite
	cfg.Permission.EscalationPolicy = permissions.EscalationPrompt

	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-1", Name: "bash", Input: json.RawMessage(`{"command":"printf first"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-2", Name: "bash", Input: json.RawMessage(`{"command":"printf second"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	confirmCalls := 0
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
			Confirmer: permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
				confirmCalls++
				if req.ToolName != "bash" {
					t.Fatalf("unexpected permission request: %#v", req)
				}
				return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeSession}, nil
			}),
		}),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.ToolTrace) != 2 || !sess.ToolTrace[0].Success || !sess.ToolTrace[1].Success {
		t.Fatalf("unexpected tool traces: %#v", sess.ToolTrace)
	}
	if confirmCalls != 1 {
		t.Fatalf("confirmCalls = %d, want 1", confirmCalls)
	}
}

func TestEngineRunReusesSessionDenialForRepeatedTool(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeWorkspaceWrite
	cfg.Permission.EscalationPolicy = permissions.EscalationPrompt

	store := session.NewInMemoryStore()
	scripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-deny-1", Name: "bash", Input: json.RawMessage(`{"command":"printf first"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-deny-2", Name: "bash", Input: json.RawMessage(`{"command":"printf second"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	confirmCalls := 0
	engine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: store,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": scripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
			Confirmer: permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
				confirmCalls++
				if req.ToolName != "bash" {
					t.Fatalf("unexpected permission request: %#v", req)
				}
				return permissions.ConfirmationOutcome{Decision: permissions.DecisionDeny, Scope: permissions.ConfirmationScopeSession}, nil
			}),
		}),
	})

	if err := engine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	sess, err := store.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.ToolTrace) != 2 || sess.ToolTrace[0].Success || sess.ToolTrace[1].Success {
		t.Fatalf("unexpected tool traces: %#v", sess.ToolTrace)
	}
	if !strings.Contains(sess.Messages[4].Content, "reuses deny decision cached") {
		t.Fatalf("expected cached deny message, got %s", sess.Messages[4].Content)
	}
	if confirmCalls != 1 {
		t.Fatalf("confirmCalls = %d, want 1", confirmCalls)
	}
}

func TestEngineRunReusesPersistedRuleApprovalAcrossEngines(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeWorkspaceWrite
	cfg.Permission.EscalationPolicy = permissions.EscalationPrompt
	cfg.Permission.RulesPath = rulesPath

	firstStore := session.NewInMemoryStore()
	firstScripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-first", Name: "bash", Input: json.RawMessage(`{"command":"printf persisted"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	confirmCalls := 0
	firstEngine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: firstStore,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": firstScripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
			RuleCachePath:    rulesPath,
			Confirmer: permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
				confirmCalls++
				if req.ToolName != "bash" {
					t.Fatalf("unexpected permission request: %#v", req)
				}
				return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeRule}, nil
			}),
		}),
	})
	if err := firstEngine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("first run engine: %v", err)
	}

	secondStore := session.NewInMemoryStore()
	secondScripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-second", Name: "bash", Input: json.RawMessage(`{"command":"printf persisted"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	secondEngine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: secondStore,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": secondScripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
			RuleCachePath:    rulesPath,
		}),
	})
	if err := secondEngine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("second run engine: %v", err)
	}

	sess, err := secondStore.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load second session: %v", err)
	}
	if len(sess.ToolTrace) != 1 || !sess.ToolTrace[0].Success {
		t.Fatalf("unexpected second tool trace: %#v", sess.ToolTrace)
	}
	if !strings.Contains(sess.Messages[2].Content, "persisted") {
		t.Fatalf("unexpected persisted approval tool message: %s", sess.Messages[2].Content)
	}
	if confirmCalls != 1 {
		t.Fatalf("confirmCalls = %d, want 1", confirmCalls)
	}
}

func TestEngineRunReusesPersistedRuleDenialAcrossEngines(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Provider.DefaultProvider = "noop"
	cfg.Provider.DefaultModel = "noop-model"
	cfg.Permission.Mode = permissions.ModeWorkspaceWrite
	cfg.Permission.EscalationPolicy = permissions.EscalationPrompt
	cfg.Permission.RulesPath = rulesPath

	firstStore := session.NewInMemoryStore()
	firstScripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-first-deny", Name: "bash", Input: json.RawMessage(`{"command":"printf blocked"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	confirmCalls := 0
	firstEngine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: firstStore,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": firstScripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
			RuleCachePath:    rulesPath,
			Confirmer: permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
				confirmCalls++
				if req.ToolName != "bash" {
					t.Fatalf("unexpected permission request: %#v", req)
				}
				return permissions.ConfirmationOutcome{Decision: permissions.DecisionDeny, Scope: permissions.ConfirmationScopeRule}, nil
			}),
		}),
	})
	if err := firstEngine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("first run engine: %v", err)
	}

	secondStore := session.NewInMemoryStore()
	secondScripted := &scriptedProvider{events: [][]*sharedprovider.StreamEvent{
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.ToolCallEvent(&types.ToolCall{ID: "call-bash-second-deny", Name: "bash", Input: json.RawMessage(`{"command":"printf blocked"}`)}),
			sharedprovider.StopEvent(),
		},
		{
			sharedprovider.MessageStartEvent(),
			sharedprovider.MessageDeltaEvent("done"),
			sharedprovider.StopEvent(),
		},
	}}
	secondEngine := NewEngine(Dependencies{
		Config:       cfg,
		SessionStore: secondStore,
		ProviderFactory: sharedprovider.NewFactory("noop", map[string]sharedprovider.Provider{
			"noop": secondScripted,
		}),
		ToolRegistry: tools.NewRegistry(tools.BuiltinTools()),
		Permission: permissions.NewStaticEngineWithOptions(permissions.Options{
			DefaultMode:      permissions.ModeWorkspaceWrite,
			EscalationPolicy: permissions.EscalationPrompt,
			RuleCachePath:    rulesPath,
		}),
	})
	if err := secondEngine.Run(context.Background(), Invocation{Args: []string{"status"}}); err != nil {
		t.Fatalf("second run engine: %v", err)
	}

	sess, err := secondStore.Load(context.Background(), "bootstrap-session")
	if err != nil {
		t.Fatalf("load second session: %v", err)
	}
	if len(sess.ToolTrace) != 1 || sess.ToolTrace[0].Success {
		t.Fatalf("unexpected second tool trace: %#v", sess.ToolTrace)
	}
	if !strings.Contains(sess.Messages[2].Content, "reuses deny rule cached on disk") {
		t.Fatalf("unexpected persisted deny tool message: %s", sess.Messages[2].Content)
	}
	if confirmCalls != 1 {
		t.Fatalf("confirmCalls = %d, want 1", confirmCalls)
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
		return &sharedprovider.SliceStreamReader{Events: []*sharedprovider.StreamEvent{sharedprovider.MessageStartEvent(), sharedprovider.StopEvent()}}, nil
	}
	reader := &sharedprovider.SliceStreamReader{Events: p.events[p.index]}
	p.index++
	return reader, nil
}

func (p *scriptedProvider) NormalizeModel(model string) string { return model }

func (p *scriptedProvider) Capabilities() sharedprovider.ProviderCapabilities {
	return sharedprovider.ProviderCapabilities{Streaming: true, ToolCalls: true}
}

type stubTool struct {
	name string
}

func (t stubTool) Spec() types.ToolSpec {
	return types.ToolSpec{Name: t.name}
}

func (t stubTool) Execute(_ context.Context, input json.RawMessage, _ tools.ToolEnv) (*types.ToolResult, error) {
	return &types.ToolResult{ToolCallID: "call-1", Name: t.name, Output: append(json.RawMessage(nil), input...)}, nil
}

func writeRuntimeFile(t *testing.T, root string, relative string, contents string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
