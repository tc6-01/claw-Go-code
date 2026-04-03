package workdir

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupWithoutRepo(t *testing.T) {
	dataDir := t.TempDir()
	mgr, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	result, err := mgr.Setup(context.Background(), "test-session", "", "")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if result.IsGitRepo {
		t.Fatal("expected non-git repo")
	}
	if _, err := os.Stat(result.WorkDir); err != nil {
		t.Fatalf("workdir not created: %v", err)
	}
}

func TestCleanup(t *testing.T) {
	dataDir := t.TempDir()
	mgr, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	_, err = mgr.Setup(context.Background(), "cleanup-test", "", "")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := mgr.Cleanup(context.Background(), "cleanup-test"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	wtDir := filepath.Join(dataDir, "worktrees", "cleanup-test")
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Fatal("expected worktree dir to be removed")
	}
}

func TestCleanupNonexistent(t *testing.T) {
	dataDir := t.TempDir()
	mgr, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	if err := mgr.Cleanup(context.Background(), "nonexistent"); err != nil {
		t.Fatalf("cleanup nonexistent: %v", err)
	}
}

func TestWorkDir(t *testing.T) {
	dataDir := t.TempDir()
	mgr, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	expected := filepath.Join(dataDir, "worktrees", "sess-123")
	if mgr.WorkDir("sess-123") != expected {
		t.Fatalf("workdir = %s, want %s", mgr.WorkDir("sess-123"), expected)
	}
}
