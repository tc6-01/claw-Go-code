package session

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	metadata map[string]sessionMetadata
}

type sessionMetadata struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Model     string
	CWD       string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sessions: make(map[string]*types.Session),
		metadata: make(map[string]sessionMetadata),
	}
}

func (s *InMemoryStore) Create(_ context.Context, session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = cloneSession(session)
	s.metadata[session.ID] = sessionMetadata{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Model:     "",
		CWD:       "",
	}
	return nil
}

func (s *InMemoryStore) Save(_ context.Context, session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = cloneSession(session)
	if meta, ok := s.metadata[session.ID]; ok {
		meta.UpdatedAt = time.Now()
		s.metadata[session.ID] = meta
	}
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
	for id := range s.sessions {
		meta := s.metadata[id]
		out = append(out, types.SessionSummary{
			ID:        id,
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
			Model:     meta.Model,
			CWD:       meta.CWD,
		})
	}
	return out, nil
}

func cloneSession(in *types.Session) *types.Session {
	if in == nil {
		return nil
	}
	copyValue := *in
	copyValue.Messages = cloneMessages(in.Messages)
	return &copyValue
}

func cloneMessages(in []types.Message) []types.Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.Message, len(in))
	for i := range in {
		out[i] = in[i]
		if in[i].Metadata != nil {
			out[i].Metadata = make(map[string]string, len(in[i].Metadata))
			for k, v := range in[i].Metadata {
				out[i].Metadata[k] = v
			}
		}
		out[i].ToolCalls = cloneToolCalls(in[i].ToolCalls)
		out[i].ToolResult = cloneToolResult(in[i].ToolResult)
	}
	return out
}

func cloneToolCalls(in []types.ToolCall) []types.ToolCall {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.ToolCall, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Input = append([]byte(nil), in[i].Input...)
	}
	return out
}

func cloneToolResult(in *types.ToolResult) *types.ToolResult {
	if in == nil {
		return nil
	}
	out := *in
	out.Output = append([]byte(nil), in.Output...)
	return &out
}
