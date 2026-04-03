package main

import (
	"context"
	"log"
	"os"

	"claude-go-code/internal/app"
	"claude-go-code/internal/config"
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

	if cfg.CLI.Interactive && isTerminal(os.Stdin) && isTerminal(os.Stdout) && shouldStartInteractiveCLI(os.Args[1:]) {
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
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "chat" {
		args = args[1:]
	}
	if err := application.Run(ctx, args); err != nil {
		log.Fatalf("run app: %v", err)
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
