package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Config struct {
	Enabled     bool
	RootDir     string
	AllowedDirs []string
	DenyExec    bool
}

type Sandbox struct {
	config Config
}

func New(cfg Config) *Sandbox {
	return &Sandbox{config: cfg}
}

func (s *Sandbox) IsEnabled() bool {
	return s.config.Enabled
}

func (s *Sandbox) DenyExec() bool {
	return s.config.Enabled && s.config.DenyExec
}

func (s *Sandbox) ValidatePath(path string) error {
	if !s.config.Enabled {
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if s.config.RootDir != "" {
		absRoot, _ := filepath.Abs(s.config.RootDir)
		if isUnder(absPath, absRoot) {
			return nil
		}
	}

	for _, dir := range s.config.AllowedDirs {
		absDir, _ := filepath.Abs(dir)
		if isUnder(absPath, absDir) {
			return nil
		}
	}

	return fmt.Errorf("path %s is outside sandbox boundaries", path)
}

func (s *Sandbox) ValidateCommand(command string) error {
	if !s.config.Enabled {
		return nil
	}
	if s.config.DenyExec {
		return fmt.Errorf("command execution is disabled in sandbox mode")
	}
	return nil
}

func isUnder(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}
