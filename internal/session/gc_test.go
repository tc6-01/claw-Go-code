package session

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"claude-go-code/pkg/types"
)

func TestGarbageCollectorRunOnce(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()

	store.Create(ctx, &types.Session{ID: "old-session", CreatedAt: old, UpdatedAt: old})
	store.Create(ctx, &types.Session{ID: "recent-session", CreatedAt: recent, UpdatedAt: recent})

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	gc := NewGarbageCollector(store, GCConfig{
		IdleTimeout: 1 * time.Hour,
	}, logger)

	cleaned, err := gc.RunOnce(ctx)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}

	if _, err := store.Load(ctx, "recent-session"); err != nil {
		t.Fatal("recent session should survive gc")
	}
	if _, err := store.Load(ctx, "old-session"); err == nil {
		t.Fatal("old session should be cleaned")
	}
}

func TestGarbageCollectorTTL(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	old := time.Now().Add(-25 * time.Hour)
	store.Create(ctx, &types.Session{ID: "expired-ttl", CreatedAt: old, UpdatedAt: time.Now()})

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	gc := NewGarbageCollector(store, GCConfig{
		TTL: 24 * time.Hour,
	}, logger)

	cleaned, err := gc.RunOnce(ctx)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}
}

func TestGCFileStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}

	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()

	store.Create(ctx, &types.Session{ID: "file-old", CreatedAt: old, UpdatedAt: old})
	store.Create(ctx, &types.Session{ID: "file-recent", CreatedAt: recent, UpdatedAt: recent})

	cleaned, err := GCFileStore(dir, 0, 1*time.Hour)
	if err != nil {
		t.Fatalf("gc file store: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}
}
