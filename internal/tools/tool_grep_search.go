package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type grepSearchInput struct {
	Pattern      string `json:"pattern"`
	Path         string `json:"path,omitempty"`
	ContextLines int    `json:"context_lines,omitempty"`
	MaxResults   int    `json:"max_results,omitempty"`
}

type grepMatch struct {
	Path   string   `json:"path"`
	Line   int      `json:"line"`
	Column int      `json:"column"`
	Text   string   `json:"text"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
}

type grepSearchOutput struct {
	Matches []grepMatch `json:"matches"`
	Count   int         `json:"count"`
}

func newGrepSearchTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeReadOnly,
		spec: types.ToolSpec{
			Name:        "grep_search",
			Description: "Search UTF-8 text files in the workspace for literal text",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"context_lines":{"type":"integer","minimum":0},"max_results":{"type":"integer","minimum":1}},"required":["pattern"]}`),
		},
		exec: executeGrepSearch,
	}
}

func executeGrepSearch(_ context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error) {
	var req grepSearchInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("grep_search: invalid input: %w", err)
	}
	if req.Pattern == "" {
		return nil, fmt.Errorf("grep_search: pattern is required")
	}
	if req.ContextLines < 0 {
		return nil, fmt.Errorf("grep_search: context_lines must be >= 0")
	}
	if req.MaxResults < 0 {
		return nil, fmt.Errorf("grep_search: max_results must be >= 0")
	}
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}

	basePath, _, err := resolveWorkspacePath(env.WorkingDir, req.Path)
	if err != nil {
		return &types.ToolResult{Error: err.Error()}, nil
	}

	matches := make([]grepMatch, 0)
	walkErr := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || len(matches) >= req.MaxResults {
			return nil
		}
		data, relPath, err := readWorkspaceTextFile(env.WorkingDir, path)
		if err != nil {
			if strings.Contains(err.Error(), "binary files are not supported") || os.IsNotExist(err) {
				return nil
			}
			return err
		}
		fileMatches := grepFile(relPath, string(data), req.Pattern, req.ContextLines, req.MaxResults-len(matches))
		matches = append(matches, fileMatches...)
		return nil
	})
	if walkErr != nil {
		return &types.ToolResult{Error: walkErr.Error()}, nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path == matches[j].Path {
			return matches[i].Line < matches[j].Line
		}
		return matches[i].Path < matches[j].Path
	})
	return &types.ToolResult{Output: rawJSON(grepSearchOutput{Matches: matches, Count: len(matches)})}, nil
}

func grepFile(path string, contents string, pattern string, contextLines int, limit int) []grepMatch {
	if limit <= 0 {
		return nil
	}
	lines := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	matches := make([]grepMatch, 0)
	for i, line := range lines {
		column := strings.Index(line, pattern)
		if column < 0 {
			continue
		}
		start := max(0, i-contextLines)
		end := min(len(lines), i+contextLines+1)
		match := grepMatch{Path: path, Line: i + 1, Column: column + 1, Text: line}
		if start < i {
			match.Before = append([]string(nil), lines[start:i]...)
		}
		if i+1 < end {
			match.After = append([]string(nil), lines[i+1:end]...)
		}
		matches = append(matches, match)
		if len(matches) >= limit {
			break
		}
	}
	return matches
}
