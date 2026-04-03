package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"claude-go-code/internal/app"
	"claude-go-code/internal/config"
	"claude-go-code/internal/runtime"
	"claude-go-code/pkg/types"
)

type interactiveRunner interface {
	CreateSession(ctx context.Context) (*types.Session, error)
	RunPrompt(ctx context.Context, sessionID string, prompt string) (*runtime.PromptResult, error)
}

func runInteractiveCLI(ctx context.Context, application interactiveRunner, cfg config.Config, in io.Reader, out io.Writer) error {
	session, err := application.CreateSession(ctx)
	if err != nil {
		return err
	}

	if err := printREPLBanner(out, session, cfg); err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	turns := 0

	for {
		if _, err := fmt.Fprint(out, "› "); err != nil {
			return err
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			nextSession, nextTurns, shouldExit, err := handleREPLCommand(ctx, application, cfg, session, turns, input, out)
			if err != nil {
				return err
			}
			session = nextSession
			turns = nextTurns
			if shouldExit {
				return nil
			}
			continue
		}

		result, err := application.RunPrompt(ctx, session.ID, input)
		if err != nil {
			if _, writeErr := fmt.Fprintf(out, "error: %v\n", err); writeErr != nil {
				return writeErr
			}
			continue
		}
		session = result.Session
		turns++
		if err := renderAssistant(out, result.Assistant); err != nil {
			return err
		}
	}
}

func buildInteractiveApp(ctx context.Context, cfg config.Config, in io.Reader, out io.Writer) (*app.App, error) {
	return app.NewWithOptions(cfg, app.Options{
		PermissionConfirmer: newTerminalConfirmer(in, out),
	})
}

func printREPLBanner(out io.Writer, session *types.Session, cfg config.Config) error {
	_, err := fmt.Fprintf(
		out,
		"Claw interactive mode\nsession=%s model=%s permission=%s\nType /help for commands.\n",
		session.ID,
		session.Model,
		cfg.Permission.Mode,
	)
	return err
}

func handleREPLCommand(ctx context.Context, application interactiveRunner, cfg config.Config, session *types.Session, turns int, input string, out io.Writer) (*types.Session, int, bool, error) {
	name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(input, "/")))
	switch name {
	case "help":
		_, err := fmt.Fprintln(out, "/help  show commands\n/status show current session state\n/new    start a fresh session\n/exit   quit interactive mode")
		return session, turns, false, err
	case "status":
		_, err := fmt.Fprintf(
			out,
			"session=%s model=%s permission=%s turns=%d messages=%d\n",
			session.ID,
			session.Model,
			cfg.Permission.Mode,
			turns,
			len(session.Messages),
		)
		return session, turns, false, err
	case "new", "clear", "compact":
		nextSession, err := application.CreateSession(ctx)
		if err != nil {
			return session, turns, false, err
		}
		_, err = fmt.Fprintf(out, "started new session %s\n", nextSession.ID)
		return nextSession, 0, false, err
	case "exit", "quit":
		_, err := fmt.Fprintln(out, "bye")
		return session, turns, true, err
	default:
		_, err := fmt.Fprintf(out, "unknown command: /%s\n", name)
		return session, turns, false, err
	}
}

func renderAssistant(out io.Writer, message types.Message) error {
	content := strings.TrimSpace(message.Content)
	if content == "" {
		content = "(no assistant text)"
	}
	_, err := fmt.Fprintln(out, content)
	return err
}
