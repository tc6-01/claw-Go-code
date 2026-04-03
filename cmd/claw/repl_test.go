package main

import (
	"bytes"
	"context"
	"testing"

	"claude-go-code/internal/config"
	"claude-go-code/internal/runtime"
	"claude-go-code/pkg/types"
)

func TestRunInteractiveCLIHandlesPromptAndCommands(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	runner := &stubInteractiveRunner{
		session: &types.Session{ID: "session-1", Model: "noop-model"},
		results: []*runtime.PromptResult{
			{
				Session: &types.Session{
					ID:       "session-1",
					Model:    "noop-model",
					Messages: []types.Message{{Role: types.RoleUser, Content: "hello"}, {Role: types.RoleAssistant, Content: "assistant reply"}},
				},
				Assistant: types.Message{Role: types.RoleAssistant, Content: "assistant reply"},
			},
		},
	}
	input := bytes.NewBufferString("/status\nhello\n/new\n/exit\n")
	var output bytes.Buffer

	if err := runInteractiveCLI(context.Background(), runner, cfg, input, &output); err != nil {
		t.Fatalf("run interactive cli: %v", err)
	}

	if runner.promptCount != 1 {
		t.Fatalf("promptCount = %d, want 1", runner.promptCount)
	}
	got := output.String()
	if !containsAll(got,
		"Claw interactive mode",
		"session=session-1",
		"assistant reply",
		"started new session session-2",
		"bye",
	) {
		t.Fatalf("unexpected repl output:\n%s", got)
	}
}

func TestShouldStartInteractiveCLI(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{name: "no args", args: nil, want: true},
		{name: "chat", args: []string{"chat"}, want: true},
		{name: "status", args: []string{"status"}, want: false},
		{name: "chat with extra args", args: []string{"chat", "now"}, want: false},
	}

	for _, tc := range cases {
		if got := shouldStartInteractiveCLI(tc.args); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

type stubInteractiveRunner struct {
	session     *types.Session
	nextSession int
	results     []*runtime.PromptResult
	promptCount int
}

func (s *stubInteractiveRunner) CreateSession(context.Context) (*types.Session, error) {
	s.nextSession++
	if s.session != nil && s.nextSession == 1 {
		current := *s.session
		return &current, nil
	}
	s.session = &types.Session{
		ID:    "session-" + string(rune('0'+s.nextSession)),
		Model: "noop-model",
	}
	current := *s.session
	return &current, nil
}

func (s *stubInteractiveRunner) RunPrompt(_ context.Context, sessionID string, prompt string) (*runtime.PromptResult, error) {
	s.promptCount++
	if len(s.results) == 0 {
		return &runtime.PromptResult{
			Session:   &types.Session{ID: sessionID, Model: "noop-model"},
			Assistant: types.Message{Role: types.RoleAssistant, Content: prompt},
		}, nil
	}
	result := s.results[0]
	s.results = s.results[1:]
	s.session = result.Session
	return result, nil
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !bytes.Contains([]byte(haystack), []byte(needle)) {
			return false
		}
	}
	return true
}
