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

	if cfg.CLI.Interactive && isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		application, err := app.NewWithOptions(cfg, app.Options{
			PermissionConfirmer: newTerminalConfirmer(os.Stdin, os.Stdout),
		})
		if err != nil {
			log.Fatalf("build app: %v", err)
		}
		if err := application.Run(ctx, os.Args[1:]); err != nil {
			log.Fatalf("run app: %v", err)
		}
	} else {
		application, err := app.New(cfg)
		if err != nil {
			log.Fatalf("build app: %v", err)
		}
		if err := application.Run(ctx, os.Args[1:]); err != nil {
			log.Fatalf("run app: %v", err)
		}
	}
}

func currentWorkingDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
