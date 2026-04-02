package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"claude-go-code/internal/permissions"
	"claude-go-code/pkg/types"
)

type globSearchInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

type globSearchOutput struct {
	Matches []string `json:"matches"`
	Count   int      `json:"count"`
}

func newGlobSearchTool() Tool {
	return builtinTool{
		requiredMode: permissions.ModeReadOnly,
		spec: types.ToolSpec{
			Name:        "glob_search",
			Description: "Find workspace files by glob pattern",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"}},"required":["pattern"]}`),
		},
		exec: executeGlobSearch,
	}
}

func executeGlobSearch(_ context.Context, input json.RawMessage, env ToolEnv) (*types.ToolResult, error) {
	var req globSearchInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("glob_search: invalid input: %w", err)
	}
	if req.Pattern == "" {
		return nil, fmt.Errorf("glob_search: pattern is required")
	}

	basePath, _, err := resolveWorkspacePath(env.WorkingDir, req.Path)
	if err != nil {
		return &types.ToolResult{Error: err.Error()}, nil
	}
	matcher, err := compileGlob(req.Pattern)
	if err != nil {
		return nil, fmt.Errorf("glob_search: invalid pattern: %w", err)
	}

	matches := make([]string, 0)
	walkErr := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		relPath, _, err := workspaceMatchPath(env.WorkingDir, path)
		if err != nil {
			return err
		}
		if matcher.MatchString(relPath) {
			matches = append(matches, relPath)
		}
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return &types.ToolResult{Error: walkErr.Error()}, nil
	}
	sort.Strings(matches)
	return &types.ToolResult{Output: rawJSON(globSearchOutput{Matches: matches, Count: len(matches)})}, nil
}

func compileGlob(pattern string) (*regexp.Regexp, error) {
	normalized := filepath.ToSlash(strings.TrimSpace(pattern))
	if normalized == "" {
		return nil, fmt.Errorf("empty pattern")
	}
	var builder strings.Builder
	builder.WriteString("^")
	for i := 0; i < len(normalized); i++ {
		ch := normalized[i]
		switch ch {
		case '*':
			if i+1 < len(normalized) && normalized[i+1] == '*' {
				builder.WriteString(".*")
				i++
			} else {
				builder.WriteString("[^/]*")
			}
		case '?':
			builder.WriteString("[^/]")
		case '.':
			builder.WriteString(`\.`)
		case '/', '\\':
			builder.WriteString("/")
		default:
			builder.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	builder.WriteString("$")
	return regexp.Compile(builder.String())
}

func workspaceMatchPath(root string, fullPath string) (string, string, error) {
	absPath, relPath, err := resolveWorkspacePath(root, fullPath)
	if err != nil {
		return "", "", err
	}
	return relPath, absPath, nil
}
