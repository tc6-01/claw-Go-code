package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"claude-go-code/internal/skill"
	"claude-go-code/pkg/types"
)

type createSessionRequest struct {
	Model   string `json:"model,omitempty"`
	RepoURL string `json:"repo_url,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

type createSessionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	WorkDir string `json:"work_dir,omitempty"`
}

type sendMessageRequest struct {
	Content string `json:"content"`
	Stream  *bool  `json:"stream,omitempty"`
}

type messageResponse struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sseEventPayload struct {
	Type       string            `json:"type"`
	Text       string            `json:"text,omitempty"`
	ToolCall   *types.ToolCall   `json:"tool_call,omitempty"`
	ToolResult *types.ToolResult `json:"tool_result,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Error      string            `json:"error,omitempty"`
	Usage      *types.Usage      `json:"usage,omitempty"`
	Message    *types.Message    `json:"message,omitempty"`
}

func (s *Server) handleCreateSession(c *gin.Context) {
	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		writeError(c, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}

	sess, err := s.engine.CreateSession(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, ErrInternalError, err.Error())
		return
	}

	resp := createSessionResponse{
		ID:    sess.ID,
		Model: sess.Model,
	}

	if s.workdirMgr != nil {
		result, err := s.workdirMgr.Setup(c.Request.Context(), sess.ID, req.RepoURL, req.Branch)
		if err != nil {
			s.logger.Warn("worktree setup failed", "session_id", sess.ID, "error", err)
		} else {
			resp.WorkDir = result.WorkDir
		}
	}

	activeSessions.Inc()
	c.JSON(http.StatusCreated, resp)
}

func (s *Server) handleListSessions(c *gin.Context) {
	sessions, err := s.sessions.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, ErrInternalError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (s *Server) handleGetSession(c *gin.Context) {
	id := c.Param("id")
	sess, err := s.sessions.Load(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusNotFound, ErrSessionNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(c *gin.Context) {
	id := c.Param("id")
	if err := s.sessions.Delete(c.Request.Context(), id); err != nil {
		writeError(c, http.StatusNotFound, ErrSessionNotFound, err.Error())
		return
	}

	if s.workdirMgr != nil {
		if err := s.workdirMgr.Cleanup(c.Request.Context(), id); err != nil {
			s.logger.Warn("worktree cleanup failed", "session_id", id, "error", err)
		}
	}

	activeSessions.Dec()
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": id})
}

func (s *Server) handleSendMessage(c *gin.Context) {
	id := c.Param("id")
	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}
	if req.Content == "" {
		writeError(c, http.StatusBadRequest, ErrInvalidRequest, "content is required")
		return
	}

	if _, err := s.sessions.Load(c.Request.Context(), id); err != nil {
		writeError(c, http.StatusNotFound, ErrSessionNotFound, err.Error())
		return
	}

	streaming := true
	if req.Stream != nil {
		streaming = *req.Stream
	}

	if streaming {
		s.handleStreamMessage(c, id, req.Content)
		return
	}

	result, err := s.engine.RunPrompt(c.Request.Context(), id, req.Content)
	if err != nil {
		writeError(c, http.StatusInternalServerError, ErrProviderError, err.Error())
		return
	}
	c.JSON(http.StatusOK, messageResponse{
		Role:    string(result.Assistant.Role),
		Content: result.Assistant.Content,
	})
}

func (s *Server) handleStreamMessage(c *gin.Context, sessionID string, content string) {
	ch, err := s.engine.RunPromptStream(c.Request.Context(), sessionID, content)
	if err != nil {
		writeError(c, http.StatusInternalServerError, ErrProviderError, err.Error())
		return
	}

	RecordStreamStart()
	defer RecordStreamEnd()

	sse := NewSSEWriter(c.Writer)
	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			payload := sseEventPayload{
				Type:       string(event.Type),
				Text:       event.Text,
				ToolCall:   event.ToolCall,
				ToolResult: event.ToolResult,
				ToolCallID: event.ToolCallID,
				Error:      event.ErrorText,
				Usage:      event.Usage,
				Message:    event.Message,
			}
			if err := sse.WriteEvent(string(event.Type), payload); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleGetMessages(c *gin.Context) {
	id := c.Param("id")
	sess, err := s.sessions.Load(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusNotFound, ErrSessionNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": sess.Messages})
}

func (s *Server) handleListModels(c *gin.Context) {
	models := []gin.H{
		{"id": "claude-sonnet-4-5", "provider": "anthropic"},
		{"id": "claude-haiku-4", "provider": "anthropic"},
		{"id": "claude-sonnet-4-5", "provider": "claude-code"},
		{"id": "claude-haiku-4", "provider": "claude-code"},
		{"id": "gpt-4o-mini", "provider": "openai"},
		{"id": "gpt-4.1", "provider": "openai"},
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (s *Server) handleListSkills(c *gin.Context) {
	skills := s.skillMgr.List()
	c.JSON(http.StatusOK, gin.H{"skills": skills})
}

func (s *Server) handleGetSkill(c *gin.Context) {
	name := c.Param("name")
	sk, ok := s.skillMgr.Get(name)
	if !ok {
		writeError(c, http.StatusNotFound, ErrNotFound, "skill not found: "+name)
		return
	}
	c.JSON(http.StatusOK, sk)
}

type createSkillRequest struct {
	Name         string   `json:"name" binding:"required"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"system_prompt" binding:"required"`
	Tools        []string `json:"tools"`
	Triggers     []string `json:"triggers"`
	MaxUses      int      `json:"max_uses"`
}

func (s *Server) handleCreateSkill(c *gin.Context) {
	var req createSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}
	sk := &skill.Skill{
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
		Triggers:     req.Triggers,
		MaxUses:      req.MaxUses,
	}
	if err := s.skillMgr.Register(sk); err != nil {
		writeError(c, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}
	c.JSON(http.StatusCreated, sk)
}

func (s *Server) handleDeleteSkill(c *gin.Context) {
	name := c.Param("name")
	if !s.skillMgr.Delete(name) {
		writeError(c, http.StatusNotFound, ErrNotFound, "skill not found: "+name)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true, "name": name})
}

