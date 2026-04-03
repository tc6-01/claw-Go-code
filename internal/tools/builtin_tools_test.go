package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

func TestBuiltinToolsRegisterExpectedNames(t *testing.T) {
	registry := NewRegistry(BuiltinTools())
	got := SpecNames(registry.Specs())
	want := []string{"bash", "edit_file", "glob_search", "grep_search", "read_file", "write_file"}
	if len(got) != len(want) {
		t.Fatalf("len(specs) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("spec[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadFileSupportsOffsetAndLimit(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, root, "notes.txt", "hello world")

	result := executeToolForTest(t, newReadFileTool(), root, permissions.ModeReadOnly, `{"path":"notes.txt","offset":6,"limit":5}`)
	var payload readFileOutput
	decodeToolOutput(t, result, &payload)
	if payload.Content != "world" {
		t.Fatalf("content = %q, want %q", payload.Content, "world")
	}
	if !payload.Truncated && payload.BytesRead != 5 {
		t.Fatalf("unexpected read payload: %#v", payload)
	}
}

func TestWriteFileRejectsPathOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	result := executeToolForTest(t, newWriteFileTool(), root, permissions.ModeWorkspaceWrite, `{"path":"../escape.txt","content":"boom"}`)
	if !strings.Contains(result.Error, "outside workspace") {
		t.Fatalf("unexpected error: %q", result.Error)
	}
}

func TestEditFileReplaceAll(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, root, "todo.txt", "fix\nfix\n")

	result := executeToolForTest(t, newEditFileTool(), root, permissions.ModeWorkspaceWrite, `{"path":"todo.txt","old_string":"fix","new_string":"done","replace_all":true}`)
	var payload editFileOutput
	decodeToolOutput(t, result, &payload)
	if payload.Replacements != 2 {
		t.Fatalf("replacements = %d, want 2", payload.Replacements)
	}
	got, err := os.ReadFile(filepath.Join(root, "todo.txt"))
	if err != nil {
		t.Fatalf("read edited file: %v", err)
	}
	if string(got) != "done\ndone\n" {
		t.Fatalf("edited file = %q", string(got))
	}
}

func TestGlobSearchFindsMatchesWithinSubtree(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, root, "src/main.go", "package main\n")
	mustWriteFile(t, root, "src/lib/helper.go", "package lib\n")
	mustWriteFile(t, root, "README.md", "hi\n")

	result := executeToolForTest(t, newGlobSearchTool(), root, permissions.ModeReadOnly, `{"pattern":"src/**/*.go"}`)
	var payload globSearchOutput
	decodeToolOutput(t, result, &payload)
	if payload.Count != 1 || payload.Matches[0] != "src/lib/helper.go" {
		t.Fatalf("unexpected matches: %#v", payload)
	}
}

func TestGrepSearchReturnsContext(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, root, "app.log", "alpha\nneedle here\nomega\n")

	result := executeToolForTest(t, newGrepSearchTool(), root, permissions.ModeReadOnly, `{"pattern":"needle","context_lines":1}`)
	var payload grepSearchOutput
	decodeToolOutput(t, result, &payload)
	if payload.Count != 1 {
		t.Fatalf("count = %d, want 1", payload.Count)
	}
	match := payload.Matches[0]
	if match.Line != 2 || match.Column != 1 {
		t.Fatalf("unexpected match position: %#v", match)
	}
	if len(match.Before) != 1 || match.Before[0] != "alpha" {
		t.Fatalf("unexpected before context: %#v", match.Before)
	}
	if len(match.After) != 1 || match.After[0] != "omega" {
		t.Fatalf("unexpected after context: %#v", match.After)
	}
}

func TestBashReturnsExitCodeAndOutput(t *testing.T) {
	root := t.TempDir()
	result := executeToolForTest(t, newBashTool(), root, permissions.ModeDangerFull, `{"command":"printf 'hi' && printf 'warn' >&2 && exit 7"}`)
	var payload bashOutput
	decodeToolOutput(t, result, &payload)
	if result.Error == "" || payload.ExitCode != 7 {
		t.Fatalf("unexpected bash failure result: error=%q payload=%#v", result.Error, payload)
	}
	if payload.Stdout != "hi" || payload.Stderr != "warn" {
		t.Fatalf("unexpected streams: %#v", payload)
	}
}

func executeToolForTest(t *testing.T, tool Tool, root string, mode permissions.Mode, input string) *types.ToolResult {
	t.Helper()
	result, err := tool.Execute(context.Background(), json.RawMessage(input), ToolEnv{WorkingDir: root, Mode: mode})
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if result == nil {
		t.Fatal("expected tool result")
	}
	return result
}

func decodeToolOutput(t *testing.T, result *types.ToolResult, out any) {
	t.Helper()
	if len(result.Output) == 0 {
		t.Fatalf("expected output payload, got error=%q", result.Error)
	}
	if err := json.Unmarshal(result.Output, out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
}

func mustWriteFile(t *testing.T, root string, relative string, contents string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
