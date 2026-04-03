package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"claude-go-code/internal/runtime"
	"claude-go-code/internal/skill"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WSClientMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Message string          `json:"message,omitempty"`
	Skills  []string        `json:"skills,omitempty"`
	Skill   string          `json:"skill,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
}

type WSServerMessage struct {
	Type          string          `json:"type"`
	ReqID         string          `json:"req_id,omitempty"`
	SessionID     string          `json:"session_id,omitempty"`
	MessageID     string          `json:"message_id,omitempty"`
	Delta         string          `json:"delta,omitempty"`
	Tool          string          `json:"tool,omitempty"`
	ToolUseID     string          `json:"tool_use_id,omitempty"`
	Content       string          `json:"content,omitempty"`
	Input         json.RawMessage `json:"input,omitempty"`
	StopReason    string          `json:"stop_reason,omitempty"`
	Usage         *wsUsage        `json:"usage,omitempty"`
	Skill         string          `json:"skill,omitempty"`
	RemainingUses int             `json:"remaining_uses,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	Error         string          `json:"error,omitempty"`
}

type wsUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type WSSession struct {
	conn      *websocket.Conn
	sessionID string
	server    *Server
	skills    *skill.SessionSkills
	send      chan []byte
	logger    *slog.Logger
	cancel    context.CancelFunc
}

type WSManager struct {
	mu       sync.RWMutex
	sessions map[string]map[*WSSession]bool
}

func NewWSManager() *WSManager {
	return &WSManager{
		sessions: make(map[string]map[*WSSession]bool),
	}
}

func (m *WSManager) Add(sessionID string, ws *WSSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions[sessionID] == nil {
		m.sessions[sessionID] = make(map[*WSSession]bool)
	}
	m.sessions[sessionID][ws] = true
}

func (m *WSManager) Remove(sessionID string, ws *WSSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conns, ok := m.sessions[sessionID]; ok {
		delete(conns, ws)
		if len(conns) == 0 {
			delete(m.sessions, sessionID)
		}
	}
}

func (s *Server) handleWebSocket(c *gin.Context) {
	sessionID := c.Param("id")

	_, err := s.sessions.Load(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, http.StatusNotFound, ErrNotFound, "session not found")
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	ws := &WSSession{
		conn:      conn,
		sessionID: sessionID,
		server:    s,
		skills:    skill.NewSessionSkills(),
		send:      make(chan []byte, 256),
		logger:    s.logger.With("session_id", sessionID),
		cancel:    cancel,
	}

	if s.wsMgr != nil {
		s.wsMgr.Add(sessionID, ws)
	}

	ws.sendJSON(WSServerMessage{Type: "connected", SessionID: sessionID})

	go ws.writePump()
	ws.readPump(ctx)
}

func (ws *WSSession) readPump(ctx context.Context) {
	defer func() {
		ws.cancel()
		if ws.server.wsMgr != nil {
			ws.server.wsMgr.Remove(ws.sessionID, ws)
		}
		ws.conn.Close()
		close(ws.send)
	}()

	ws.conn.SetReadLimit(maxMessageSize)
	ws.conn.SetReadDeadline(time.Now().Add(pongWait))
	ws.conn.SetPongHandler(func(string) error {
		ws.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				ws.logger.Error("websocket read error", "error", err)
			}
			break
		}

		var msg WSClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			ws.sendJSON(WSServerMessage{Type: "error", Error: "invalid message format"})
			continue
		}

		ws.handleMessage(ctx, &msg)
	}
}

func (ws *WSSession) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		ws.conn.Close()
	}()

	for {
		select {
		case message, ok := <-ws.send:
			ws.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				ws.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := ws.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			ws.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (ws *WSSession) handleMessage(ctx context.Context, msg *WSClientMessage) {
	switch msg.Type {
	case "send_message":
		ws.handleSendMessage(ctx, msg)
	case "activate_skill":
		ws.handleActivateSkill(msg)
	case "deactivate_skill":
		ws.handleDeactivateSkill(msg)
	case "interrupt":
		ws.sendJSON(WSServerMessage{Type: "interrupted", ReqID: msg.ID, Reason: "user_requested"})
	case "ping":
		ws.sendJSON(WSServerMessage{Type: "pong"})
	default:
		ws.sendJSON(WSServerMessage{Type: "error", ReqID: msg.ID, Error: "unknown message type: " + msg.Type})
	}
}

func (ws *WSSession) handleSendMessage(ctx context.Context, msg *WSClientMessage) {
	if msg.Message == "" {
		ws.sendJSON(WSServerMessage{Type: "error", ReqID: msg.ID, Error: "message content is required"})
		return
	}

	ch, err := ws.server.engine.RunPromptStream(ctx, ws.sessionID, msg.Message)
	if err != nil {
		ws.sendJSON(WSServerMessage{Type: "error", ReqID: msg.ID, Error: err.Error()})
		return
	}

	ws.sendJSON(WSServerMessage{Type: "message_start", ReqID: msg.ID})

	for event := range ch {
		switch event.Type {
		case runtime.EventTextDelta:
			ws.sendJSON(WSServerMessage{Type: "text_delta", ReqID: msg.ID, Delta: event.Text})
		case runtime.EventToolUse:
			var toolName string
			var toolInput json.RawMessage
			if event.ToolCall != nil {
				toolName = event.ToolCall.Name
				toolInput = event.ToolCall.Input
			}
			ws.sendJSON(WSServerMessage{
				Type:  "tool_use",
				ReqID: msg.ID,
				Tool:  toolName,
				Input: toolInput,
			})
		case runtime.EventToolResult:
			ws.sendJSON(WSServerMessage{
				Type:      "tool_result",
				ReqID:     msg.ID,
				ToolUseID: event.ToolCallID,
				Content:   event.Text,
			})
		case runtime.EventToolError:
			ws.sendJSON(WSServerMessage{
				Type:      "tool_error",
				ReqID:     msg.ID,
				ToolUseID: event.ToolCallID,
				Error:     event.ErrorText,
			})
		case runtime.EventUsage:
			var usage *wsUsage
			if event.Usage != nil {
				usage = &wsUsage{InputTokens: event.Usage.InputTokens, OutputTokens: event.Usage.OutputTokens}
			}
			ws.sendJSON(WSServerMessage{
				Type:  "usage",
				ReqID: msg.ID,
				Usage: usage,
			})
		case runtime.EventMessageEnd:
			stopReason := ""
			if event.Message != nil {
				stopReason = "end_turn"
			}
			ws.sendJSON(WSServerMessage{
				Type:       "message_end",
				ReqID:      msg.ID,
				StopReason: stopReason,
			})
		case runtime.EventDone:
			ws.sendJSON(WSServerMessage{Type: "done", ReqID: msg.ID})
		case runtime.EventError:
			ws.sendJSON(WSServerMessage{Type: "error", ReqID: msg.ID, Error: event.ErrorText})
		}
	}
}

func (ws *WSSession) handleActivateSkill(msg *WSClientMessage) {
	if ws.server.skillMgr == nil {
		ws.sendJSON(WSServerMessage{Type: "error", ReqID: msg.ID, Error: "skill system not available"})
		return
	}
	sk, ok := ws.server.skillMgr.Get(msg.Skill)
	if !ok {
		ws.sendJSON(WSServerMessage{Type: "error", ReqID: msg.ID, Error: "skill not found: " + msg.Skill})
		return
	}
	ss := ws.skills.Activate(sk)
	ws.sendJSON(WSServerMessage{
		Type:          "skill_activated",
		ReqID:         msg.ID,
		Skill:         msg.Skill,
		RemainingUses: ss.RemainingUses,
	})
}

func (ws *WSSession) handleDeactivateSkill(msg *WSClientMessage) {
	if ws.skills.Deactivate(msg.Skill) {
		ws.sendJSON(WSServerMessage{Type: "skill_deactivated", ReqID: msg.ID, Skill: msg.Skill})
	} else {
		ws.sendJSON(WSServerMessage{Type: "error", ReqID: msg.ID, Error: "skill not active: " + msg.Skill})
	}
}

func (ws *WSSession) sendJSON(msg WSServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		ws.logger.Error("marshal ws message", "error", err)
		return
	}
	select {
	case ws.send <- data:
	default:
		ws.logger.Warn("websocket send buffer full, dropping message")
	}
}
