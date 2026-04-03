package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"claude-go-code/internal/app"
	"claude-go-code/internal/config"
	"claude-go-code/internal/server"
	"claude-go-code/internal/session"
	"claude-go-code/internal/skill"
	"claude-go-code/internal/workdir"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load(ctx, config.LoadOptions{
		WorkingDir: currentWorkingDir(),
	})
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if handled, err := handlePermissionRuleCommand(cfg, os.Args[1:], os.Stdout); handled {
		if err != nil {
			log.Fatalf("permissions rules: %v", err)
		}
		return
	}

	args := os.Args[1:]

	if len(args) > 0 && args[0] == "serve" {
		runServe(ctx, cfg, args[1:])
		return
	}

	if cfg.CLI.Interactive && isTerminal(os.Stdin) && isTerminal(os.Stdout) && shouldStartInteractiveCLI(args) {
		application, err := buildInteractiveApp(ctx, cfg, os.Stdin, os.Stdout)
		if err != nil {
			log.Fatalf("build app: %v", err)
		}
		if err := runInteractiveCLI(ctx, application, cfg, os.Stdin, os.Stdout); err != nil {
			log.Fatalf("run interactive cli: %v", err)
		}
		return
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}
	if len(args) > 0 && args[0] == "chat" {
		args = args[1:]
	}
	if err := application.Run(ctx, args); err != nil {
		log.Fatalf("run app: %v", err)
	}
}

func runServe(_ context.Context, cfg config.Config, args []string) {
	parseServeFlags(cfg, args)

	application, err := app.NewForServer(cfg)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var opts []server.Option

	wdMgr, err := workdir.NewManager(cfg.DataDir)
	if err != nil {
		logger.Warn("workdir manager init failed, worktree isolation disabled", "error", err)
	} else {
		opts = append(opts, server.WithWorkdirManager(wdMgr))
	}

	gcConfig := session.GCConfig{
		TTL:         cfg.Session.TTL,
		IdleTimeout: cfg.Session.IdleTimeout,
		Interval:    cfg.Session.TTL / 24,
	}
	if gcConfig.Interval < 1*60*1e9 {
		gcConfig.Interval = 10 * 60 * 1e9
	}
	gc := session.NewGarbageCollector(application.SessionStore, gcConfig, logger)
	opts = append(opts, server.WithGC(gc))

	skillDir := cfg.DataDir + "/skills"
	skillMgr := skill.NewManager(skillDir)
	if err := skillMgr.LoadDir(); err != nil {
		logger.Warn("skill load failed", "error", err)
	}
	opts = append(opts, server.WithSkillManager(skillMgr))

	wsMgr := server.NewWSManager()
	opts = append(opts, server.WithWSManager(wsMgr))

	srv := server.New(application.Runtime, application.SessionStore, cfg.Server, opts...)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func parseServeFlags(cfg config.Config, args []string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 < len(args) {
				i++
			}
		case "--host":
			if i+1 < len(args) {
				i++
			}
		}
	}
}

func shouldStartInteractiveCLI(args []string) bool {
	if len(args) == 0 {
		return true
	}
	return len(args) == 1 && args[0] == "chat"
}

func currentWorkingDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
