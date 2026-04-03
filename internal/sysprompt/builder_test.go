package sysprompt

import (
	"strings"
	"testing"

	"claude-go-code/pkg/types"
)

func TestBuildDefault(t *testing.T) {
	result := Build(Context{
		CWD:   "/workspace",
		Model: "claude-sonnet-4-5",
	})

	if !strings.Contains(result, "AI coding assistant") {
		t.Fatal("missing identity")
	}
	if !strings.Contains(result, "/workspace") {
		t.Fatal("missing CWD")
	}
	if !strings.Contains(result, "claude-sonnet-4-5") {
		t.Fatal("missing model")
	}
}

func TestBuildWithTools(t *testing.T) {
	result := Build(Context{
		ToolSpecs: []types.ToolSpec{
			{Name: "read_file", Description: "Read a file"},
			{Name: "bash", Description: "Run a command"},
		},
	})

	if !strings.Contains(result, "read_file: Read a file") {
		t.Fatal("missing read_file tool")
	}
	if !strings.Contains(result, "bash: Run a command") {
		t.Fatal("missing bash tool")
	}
}

func TestBuildCustomOverride(t *testing.T) {
	result := Build(Context{
		CustomSystem: "You are a custom bot.",
	})

	if result != "You are a custom bot." {
		t.Fatalf("expected custom override, got: %s", result)
	}
}

func TestBuildCustomWithAppend(t *testing.T) {
	result := Build(Context{
		CustomSystem: "Custom.",
		SystemAppend: "Extra.",
	})

	if result != "Custom.\n\nExtra." {
		t.Fatalf("unexpected: %s", result)
	}
}

func TestBuildDefaultWithAppend(t *testing.T) {
	result := Build(Context{
		SystemAppend: "Always respond in JSON.",
	})

	if !strings.HasSuffix(result, "Always respond in JSON.") {
		t.Fatal("missing append")
	}
	if !strings.Contains(result, "AI coding assistant") {
		t.Fatal("missing identity")
	}
}
