package sysprompt

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"claude-go-code/pkg/types"
)

type Context struct {
	CWD         string
	Model       string
	ToolSpecs   []types.ToolSpec
	CustomSystem string
	SystemAppend string
}

func Build(ctx Context) string {
	if ctx.CustomSystem != "" {
		if ctx.SystemAppend != "" {
			return ctx.CustomSystem + "\n\n" + ctx.SystemAppend
		}
		return ctx.CustomSystem
	}

	var parts []string

	parts = append(parts, identity())
	parts = append(parts, environment(ctx))

	if len(ctx.ToolSpecs) > 0 {
		parts = append(parts, toolSection(ctx.ToolSpecs))
	}

	result := strings.Join(parts, "\n\n")
	if ctx.SystemAppend != "" {
		result += "\n\n" + ctx.SystemAppend
	}
	return result
}

func identity() string {
	return `You are an AI coding assistant powered by Claw, an agent harness for coding tasks.
You help users with software engineering tasks including reading, writing, and editing code,
running commands, searching codebases, and answering technical questions.
You have access to tools that let you interact with the user's codebase and environment.
Always use the appropriate tool for the task rather than guessing or making assumptions.`
}

func environment(ctx Context) string {
	now := time.Now()
	lines := []string{
		"<environment>",
		fmt.Sprintf("OS: %s/%s", runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("Date: %s", now.Format("2006-01-02 15:04:05 MST")),
	}
	if ctx.CWD != "" {
		lines = append(lines, fmt.Sprintf("Working Directory: %s", ctx.CWD))
	}
	if ctx.Model != "" {
		lines = append(lines, fmt.Sprintf("Model: %s", ctx.Model))
	}
	lines = append(lines, "</environment>")
	return strings.Join(lines, "\n")
}

func toolSection(specs []types.ToolSpec) string {
	var lines []string
	lines = append(lines, "<available_tools>")
	for _, spec := range specs {
		desc := spec.Description
		if desc == "" {
			desc = "(no description)"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", spec.Name, desc))
	}
	lines = append(lines, "</available_tools>")
	return strings.Join(lines, "\n")
}
