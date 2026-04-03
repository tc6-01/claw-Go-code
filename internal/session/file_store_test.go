package session

import (
	"context"
	"testing"
	"time"

	"claude-go-code/pkg/types"
)

func TestFileStoreCreateAndLoad(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	sess := &types.Session{
		ID:        "test-session-1",
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
		CWD:       "/tmp",
		Model:     "claude-sonnet-4-5",
	}

	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("create: %v", err)
	}

	loaded, err := store.Load(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ID != "test-session-1" {
		t.Fatalf("id = %s, want test-session-1", loaded.ID)
	}
	if loaded.Model != "claude-sonnet-4-5" {
		t.Fatalf("model = %s, want claude-sonnet-4-5", loaded.Model)
	}
}

func TestFileStoreSaveAndReload(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	sess := &types.Session{
		ID:        "test-session-2",
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
		Model:     "claude-sonnet-4-5",
	}

	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("create: %v", err)
	}

	sess.Messages = append(sess.Messages, types.Message{
		Role:    types.RoleUser,
		Content: "hello",
	})
	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load(ctx, "test-session-2")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(loaded.Messages))
	}
}

func TestFileStoreList(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	for _, id := range []string{"s1", "s2", "s3"} {
		if err := store.Create(ctx, &types.Session{ID: id, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("list count = %d, want 3", len(list))
	}
}

func TestFileStoreDelete(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.Create(ctx, &types.Session{ID: "to-delete", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Load(ctx, "to-delete"); err == nil {
		t.Fatal("expected error loading deleted session")
	}
}

func TestFileStoreLoadNotFound(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	if _, err := store.Load(context.Background(), "nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestInMemoryStoreDelete(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := store.Create(ctx, &types.Session{ID: "mem-del", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.Delete(ctx, "mem-del"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Load(ctx, "mem-del"); err == nil {
		t.Fatal("expected error loading deleted session")
	}
	if err := store.Delete(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error deleting nonexistent session")
	}
}
