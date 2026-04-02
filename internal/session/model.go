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
	copyValue.Messages = cloneMessages(in.Messages)
	copyValue.ToolTrace = cloneToolTrace(in.ToolTrace)
	copyValue.Usage = append([]types.Usage(nil), in.Usage...)
	copyValue.Todos = append([]types.TodoItem(nil), in.Todos...)
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

func cloneToolTrace(in []types.ToolTraceEntry) []types.ToolTraceEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.ToolTraceEntry, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Result = cloneToolResult(in[i].Result)
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
