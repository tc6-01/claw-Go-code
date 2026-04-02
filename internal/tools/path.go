package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func resolveWorkspacePath(root string, requested string) (string, string, error) {
	if strings.TrimSpace(root) == "" {
		return "", "", fmt.Errorf("working directory is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}

	candidate := strings.TrimSpace(requested)
	if candidate == "" {
		candidate = "."
	}

	var target string
	if filepath.IsAbs(candidate) {
		target = filepath.Clean(candidate)
	} else {
		target = filepath.Join(rootAbs, candidate)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}

	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path %q is outside workspace", requested)
	}
	if rel == "." {
		return targetAbs, ".", nil
	}
	return targetAbs, filepath.ToSlash(rel), nil
}

func readWorkspaceTextFile(root string, requested string) ([]byte, string, error) {
	absPath, relPath, err := resolveWorkspacePath(root, requested)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, relPath, err
	}
	if !utf8.Valid(data) {
		return nil, relPath, fmt.Errorf("binary files are not supported")
	}
	return data, relPath, nil
}
