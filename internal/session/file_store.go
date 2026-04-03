package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"claude-go-code/pkg/types"
)

type FileStore struct {
	dir string
	mu  sync.RWMutex
}

func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) Create(_ context.Context, sess *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeFile(sess)
}

func (s *FileStore) Save(_ context.Context, sess *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeFile(sess)
}

func (s *FileStore) Load(_ context.Context, id string) (*types.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %s not found", id)
		}
		return nil, err
	}

	var sess types.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("decode session %s: %w", id, err)
	}
	return &sess, nil
}

func (s *FileStore) List(_ context.Context) ([]types.SessionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []types.SessionSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var sess types.Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		out = append(out, types.SessionSummary{
			ID:        sess.ID,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
			Model:     sess.Model,
			CWD:       sess.CWD,
		})
	}
	return out, nil
}

func (s *FileStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.path(id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("session %s not found", id)
	}
	return os.Remove(path)
}

func (s *FileStore) writeFile(sess *types.Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	return os.WriteFile(s.path(sess.ID), data, 0o644)
}

func (s *FileStore) path(id string) string {
	safe := strings.ReplaceAll(id, "/", "_")
	return filepath.Join(s.dir, safe+".json")
}
