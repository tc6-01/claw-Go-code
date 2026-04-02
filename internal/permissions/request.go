package permissions

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
)

type bashPermissionInput struct {
	Command string `json:"command"`
}

type webFetchPermissionInput struct {
	URL string `json:"url"`
}

type pathPermissionInput struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
}

func RequestForToolCall(toolName string, current, required Mode, input json.RawMessage) PermissionRequest {
	req := PermissionRequest{
		ToolName:    toolName,
		CurrentMode: current,
		Required:    required,
	}
	req.TargetKind, req.TargetPattern = deriveRuleTarget(toolName, input)
	return req
}

func deriveRuleTarget(toolName string, input json.RawMessage) (RuleTargetKind, string) {
	switch toolName {
	case "bash":
		var req bashPermissionInput
		if json.Unmarshal(input, &req) == nil {
			prefix := normalizeCommandPrefix(req.Command)
			if prefix != "" {
				return RuleTargetCommandPrefix, prefix
			}
		}
	case "web_fetch":
		var req webFetchPermissionInput
		if json.Unmarshal(input, &req) == nil {
			host := normalizeHost(req.URL)
			if host != "" {
				return RuleTargetHost, host
			}
		}
	case "read_file", "write_file", "edit_file", "grep_search", "glob_search":
		var req pathPermissionInput
		if json.Unmarshal(input, &req) == nil {
			target := normalizePathPattern(req.Path)
			if target == "" && toolName == "glob_search" {
				target = normalizePathPattern(req.Pattern)
			}
			if target != "" {
				return RuleTargetPathPattern, target
			}
		}
	}
	return RuleTargetAny, ""
}

func normalizeCommandPrefix(command string) string {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func normalizeHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ""
	}
	return host
}

func normalizePathPattern(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.ToSlash(filepath.Clean(trimmed))
	if cleaned == "." {
		return ""
	}
	return cleaned
}
