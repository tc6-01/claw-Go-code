package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type stubTool struct {
	spec   types.ToolSpec
	result *types.ToolResult
	err    error
}

func (t stubTool) Spec() types.ToolSpec { return t.spec }

func (t stubTool) Execute(_ context.Context, _ json.RawMessage, _ ToolEnv) (*types.ToolResult, error) {
	return t.result, t.err
}

func TestRegistryRejectsNilTool(t *testing.T) {
	registry := NewRegistry(nil)
	if err := registry.Register(nil); err == nil {
		t.Fatal("expected nil tool error")
	}
}

func TestRegistrySpecsSortedByHelper(t *testing.T) {
	registry := NewRegistry([]Tool{
		stubTool{spec: types.ToolSpec{Name: "write_file"}},
		stubTool{spec: types.ToolSpec{Name: "read_file"}},
	})

	got := SpecNames(registry.Specs())
	want := []string{"read_file", "write_file"}
	if len(got) != len(want) {
		t.Fatalf("len(spec names) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("spec[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExecutorRunsToolAndBuildsToolMessage(t *testing.T) {
	registry := NewRegistry([]Tool{stubTool{
		spec: types.ToolSpec{Name: "read_file"},
		result: &types.ToolResult{
			Output: json.RawMessage(`{"contents":"ok"}`),
		},
	}})
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeWorkspaceWrite))

	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-1", Name: "read_file", Input: json.RawMessage(`{"path":"a.txt"}`)},
		Env:  ToolEnv{WorkingDir: t.TempDir(), Mode: permissions.ModeWorkspaceWrite},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Message.Role != types.RoleTool {
		t.Fatalf("message role = %q, want %q", result.Message.Role, types.RoleTool)
	}
	if result.Message.Name != "read_file" {
		t.Fatalf("message name = %q", result.Message.Name)
	}
	if !strings.Contains(result.Message.Content, `"tool_call_id":"tool-1"`) {
		t.Fatalf("message content missing tool call id: %s", result.Message.Content)
	}
	if !strings.Contains(result.Message.Content, `"contents":"ok"`) {
		t.Fatalf("message content missing output: %s", result.Message.Content)
	}
	if result.Message.ToolResult == nil || result.Message.ToolResult.ToolCallID != "tool-1" {
		t.Fatalf("unexpected tool result trace: %#v", result.Message.ToolResult)
	}
	if !result.Trace.Success {
		t.Fatal("expected successful trace")
	}
	if result.Trace.Name != "read_file" {
		t.Fatalf("trace name = %q", result.Trace.Name)
	}
	if result.Trace.Result == nil || result.Trace.Result.ToolCallID != "tool-1" {
		t.Fatalf("unexpected trace result: %#v", result.Trace.Result)
	}
}

func TestExecutorReturnsPermissionFailureAsToolMessage(t *testing.T) {
	registry := NewRegistry([]Tool{stubTool{spec: types.ToolSpec{Name: "write_file"}}})
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeReadOnly))

	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-2", Name: "write_file"},
		Env:  ToolEnv{Mode: permissions.ModeReadOnly},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Trace.Success {
		t.Fatal("expected failed trace")
	}
	if !strings.Contains(result.Message.Content, "requires workspace-write") {
		t.Fatalf("unexpected permission content: %s", result.Message.Content)
	}
}

func TestExecutorUsesBuiltinPermissionModes(t *testing.T) {
	registry := NewRegistry(BuiltinTools())
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeWorkspaceWrite))

	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-bash", Name: "bash", Input: json.RawMessage(`{"command":"pwd"}`)},
		Env:  ToolEnv{WorkingDir: t.TempDir(), Mode: permissions.ModeWorkspaceWrite},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Trace.Success {
		t.Fatal("expected bash to be denied in workspace-write mode")
	}
	if !strings.Contains(result.Message.Content, "danger-full-access") {
		t.Fatalf("unexpected permission content: %s", result.Message.Content)
	}
}

func TestExecutorAllowsReadOnlyBuiltinInReadOnlyMode(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, root, "notes.txt", "hello")

	registry := NewRegistry(BuiltinTools())
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeReadOnly))
	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-read", Name: "read_file", Input: json.RawMessage(`{"path":"notes.txt"}`)},
		Env:  ToolEnv{WorkingDir: root, Mode: permissions.ModeReadOnly},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Trace.Success {
		t.Fatalf("expected success, got trace %#v", result.Trace)
	}
	if !strings.Contains(result.Message.Content, `"content":"hello"`) {
		t.Fatalf("unexpected read result: %s", result.Message.Content)
	}
}

func TestExecutorDeniesWriteBuiltinInReadOnlyMode(t *testing.T) {
	registry := NewRegistry(BuiltinTools())
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeReadOnly))
	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-write", Name: "write_file", Input: json.RawMessage(`{"path":"a.txt","content":"x"}`)},
		Env:  ToolEnv{WorkingDir: t.TempDir(), Mode: permissions.ModeReadOnly},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Trace.Success {
		t.Fatal("expected denied write_file trace")
	}
	if !strings.Contains(result.Message.Content, "requires workspace-write") {
		t.Fatalf("unexpected permission content: %s", result.Message.Content)
	}
}

func TestExecutorAllowsWebToolInDangerFullMode(t *testing.T) {
	registry := NewRegistry([]Tool{stubTool{
		spec:   types.ToolSpec{Name: "web_fetch", Permission: string(permissions.ModeDangerFull)},
		result: &types.ToolResult{Output: json.RawMessage(`{"result":"ok"}`)},
	}})
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeDangerFull))
	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-web", Name: "web_fetch", Input: json.RawMessage(`{"url":"https://example.com"}`)},
		Env:  ToolEnv{WorkingDir: t.TempDir(), Mode: permissions.ModeDangerFull},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Trace.Success {
		t.Fatalf("expected web_fetch success, got %#v", result.Trace)
	}
}

func TestExecutorReturnsPromptDecisionAsToolMessage(t *testing.T) {
	registry := NewRegistry(BuiltinTools())
	executor := NewExecutor(registry, permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      permissions.ModeWorkspaceWrite,
		EscalationPolicy: permissions.EscalationPrompt,
	}))
	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-bash-prompt", Name: "bash", Input: json.RawMessage(`{"command":"pwd"}`)},
		Env:  ToolEnv{WorkingDir: t.TempDir(), Mode: permissions.ModeWorkspaceWrite},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Trace.Permission != string(permissions.DecisionPrompt) {
		t.Fatalf("unexpected permission decision: %#v", result.Trace)
	}
	if !strings.Contains(result.Message.Content, "needs confirmation") {
		t.Fatalf("unexpected prompt content: %s", result.Message.Content)
	}
}

func TestExecutorReturnsNotFoundError(t *testing.T) {
	executor := NewExecutor(NewRegistry(nil), permissions.NewStaticEngine(permissions.ModeWorkspaceWrite))

	if _, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-missing", Name: "missing_tool"},
		Env:  ToolEnv{Mode: permissions.ModeWorkspaceWrite},
	}); err == nil {
		t.Fatal("expected missing tool error")
	}
}

func TestExecutorReturnsNilResultError(t *testing.T) {
	registry := NewRegistry([]Tool{stubTool{spec: types.ToolSpec{Name: "read_file"}}})
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeWorkspaceWrite))

	if _, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-nil", Name: "read_file"},
		Env:  ToolEnv{Mode: permissions.ModeWorkspaceWrite},
	}); err == nil {
		t.Fatal("expected nil result error")
	}
}

func TestExecutorReturnsToolError(t *testing.T) {
	registry := NewRegistry([]Tool{stubTool{
		spec:   types.ToolSpec{Name: "read_file"},
		result: &types.ToolResult{Error: "boom"},
	}})
	executor := NewExecutor(registry, permissions.NewStaticEngine(permissions.ModeWorkspaceWrite))

	result, err := executor.Execute(context.Background(), ExecuteRequest{
		Call: types.ToolCall{ID: "tool-3", Name: "read_file"},
		Env:  ToolEnv{Mode: permissions.ModeWorkspaceWrite},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Trace.Success {
		t.Fatal("expected failed trace for tool error")
	}
	if !strings.Contains(result.Message.Content, `"error":"boom"`) {
		t.Fatalf("missing tool error in content: %s", result.Message.Content)
	}
}
