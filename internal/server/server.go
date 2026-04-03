package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"claude-go-code/internal/config"
	"claude-go-code/internal/runtime"
	"claude-go-code/internal/session"
	"claude-go-code/internal/skill"
	"claude-go-code/internal/workdir"
)

type Server struct {
	engine     runtime.Engine
	sessions   session.Store
	config     config.ServerConfig
	router     *gin.Engine
	server     *http.Server
	logger     *slog.Logger
	workdirMgr *workdir.Manager
	gc         *session.GarbageCollector
	skillMgr   *skill.Manager
	wsMgr      *WSManager
}

type Option func(*Server)

func WithWorkdirManager(mgr *workdir.Manager) Option {
	return func(s *Server) { s.workdirMgr = mgr }
}

func WithGC(gc *session.GarbageCollector) Option {
	return func(s *Server) { s.gc = gc }
}

func WithSkillManager(mgr *skill.Manager) Option {
	return func(s *Server) { s.skillMgr = mgr }
}

func WithWSManager(mgr *WSManager) Option {
	return func(s *Server) { s.wsMgr = mgr }
}

func New(eng runtime.Engine, store session.Store, cfg config.ServerConfig, opts ...Option) *Server {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	s := &Server{
		engine:   eng,
		sessions: store,
		config:   cfg,
		router:   r,
		logger:   logger,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.setupMiddleware()
	s.setupRoutes()
	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(gin.Recovery())
	s.router.Use(requestIDMiddleware())
	s.router.Use(loggerMiddleware(s.logger))
	s.router.Use(metricsMiddleware())
	s.router.Use(corsMiddleware())

	if len(s.config.APIKeys) > 0 {
		s.router.Use(authMiddleware(s.config.APIKeys))
	}

	if s.config.RateLimit > 0 {
		s.router.Use(rateLimitMiddleware(s.config.RateLimit))
	}
}

func (s *Server) setupRoutes() {
	v1 := s.router.Group("/v1")
	{
		v1.POST("/sessions", s.handleCreateSession)
		v1.GET("/sessions", s.handleListSessions)
		v1.GET("/sessions/:id", s.handleGetSession)
		v1.DELETE("/sessions/:id", s.handleDeleteSession)
		v1.POST("/sessions/:id/messages", s.handleSendMessage)
		v1.GET("/sessions/:id/messages", s.handleGetMessages)
		v1.GET("/models", s.handleListModels)

		if s.skillMgr != nil {
			v1.GET("/skills", s.handleListSkills)
			v1.GET("/skills/:name", s.handleGetSkill)
			v1.POST("/skills", s.handleCreateSkill)
			v1.DELETE("/skills/:name", s.handleDeleteSkill)
		}
	}

	if s.wsMgr != nil {
		s.router.GET("/ws/:id", s.handleWebSocket)
	}

	s.router.GET("/health", s.handleHealth)
	s.router.GET("/metrics", metricsHandler())
}

func (s *Server) ListenAndServe() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	if s.gc != nil {
		s.gc.Start(context.Background())
		defer s.gc.Stop()
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("server starting", "addr", s.server.Addr)
		errCh <- s.server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		s.logger.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
