package session

import (
	"context"
	"fmt"
	"sync"

	"claude-go-code/pkg/types"
)

type Store interface {
	Create(ctx context.Context, session *types.Session) error
	Save(ctx context.Context, session *types.Session) error
	Load(ctx context.Context, id string) (*types.Session, error)
	List(ctx context.Context) ([]types.SessionSummary, error)
}

type InMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*types.Session
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{sessions: make(map[string]*types.Session)}
}

func (s *InMemoryStore) Create(_ context.Context, session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = cloneSession(session)
	return nil
}

func (s *InMemoryStore) Save(_ context.Context, session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = cloneSession(session)
	return nil
}

func (s *InMemoryStore) Load(_ context.Context, id string) (*types.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return cloneSession(session), nil
}

func (s *InMemoryStore) List(_ context.Context) ([]types.SessionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.SessionSummary, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, types.SessionSummary{
			ID:        session.ID,
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
			Model:     session.Model,
			CWD:       session.CWD,
		})
	}
	return out, nil
}

func cloneSession(in *types.Session) *types.Session {
	if in == nil {
		return nil
	}
	copyValue := *in
	copyValue.Messages = append([]types.Message(nil), in.Messages...)
	copyValue.ToolTrace = append([]types.ToolTraceEntry(nil), in.ToolTrace...)
	copyValue.Todos = append([]types.TodoItem(nil), in.Todos...)
	return &copyValue
}
