package workdir

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Manager struct {
	dataDir string
	mu      sync.Mutex
}

func NewManager(dataDir string) (*Manager, error) {
	for _, sub := range []string{"repos", "worktrees"} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return &Manager{dataDir: dataDir}, nil
}

type SetupResult struct {
	WorkDir   string
	RepoHash  string
	IsGitRepo bool
}

func (m *Manager) Setup(ctx context.Context, sessionID string, repoURL string, branch string) (*SetupResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wtDir := filepath.Join(m.dataDir, "worktrees", sessionID)

	if repoURL == "" {
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			return nil, fmt.Errorf("create worktree dir: %w", err)
		}
		return &SetupResult{WorkDir: wtDir, IsGitRepo: false}, nil
	}

	repoHash := hashRepoURL(repoURL)
	bareDir := filepath.Join(m.dataDir, "repos", repoHash)

	if err := m.ensureBareRepo(ctx, repoURL, bareDir); err != nil {
		return nil, fmt.Errorf("ensure bare repo: %w", err)
	}

	if err := m.fetchLatest(ctx, bareDir); err != nil {
		return nil, fmt.Errorf("fetch latest: %w", err)
	}

	if branch == "" {
		branch = "main"
	}

	wtBranch := fmt.Sprintf("claw/%s", sessionID)
	if err := m.createWorktree(ctx, bareDir, wtDir, branch, wtBranch); err != nil {
		return nil, fmt.Errorf("create worktree: %w", err)
	}

	return &SetupResult{
		WorkDir:   wtDir,
		RepoHash:  repoHash,
		IsGitRepo: true,
	}, nil
}

func (m *Manager) Cleanup(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wtDir := filepath.Join(m.dataDir, "worktrees", sessionID)
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		return nil
	}

	gitDir := filepath.Join(wtDir, ".git")
	if data, err := os.ReadFile(gitDir); err == nil {
		content := string(data)
		if strings.Contains(content, "gitdir:") {
			parts := strings.SplitN(content, "gitdir: ", 2)
			if len(parts) == 2 {
				bareDir := filepath.Dir(filepath.Dir(strings.TrimSpace(parts[1])))
				cmd := exec.Command("git", "worktree", "remove", "--force", wtDir)
				cmd.Dir = bareDir
				_ = cmd.Run()
			}
		}
	}

	return os.RemoveAll(wtDir)
}

func (m *Manager) WorkDir(sessionID string) string {
	return filepath.Join(m.dataDir, "worktrees", sessionID)
}

func (m *Manager) ensureBareRepo(ctx context.Context, repoURL string, bareDir string) error {
	if _, err := os.Stat(filepath.Join(bareDir, "HEAD")); err == nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--bare", repoURL, bareDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) fetchLatest(ctx context.Context, bareDir string) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", "--all", "--prune")
	cmd.Dir = bareDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) createWorktree(ctx context.Context, bareDir string, wtDir string, sourceBranch string, newBranch string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", newBranch, wtDir, sourceBranch)
	cmd.Dir = bareDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hashRepoURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", h[:8])
}
