package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillValidation(t *testing.T) {
	tests := []struct {
		name    string
		skill   Skill
		wantErr bool
	}{
		{"valid", Skill{Name: "test", SystemPrompt: "hello"}, false},
		{"no name", Skill{SystemPrompt: "hello"}, true},
		{"no prompt", Skill{Name: "test"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.skill.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate: got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestSkillTrigger(t *testing.T) {
	sk := &Skill{Name: "review", Triggers: []string{"review", "审查"}}
	if !sk.MatchesTrigger("please review my code") {
		t.Fatal("expected trigger match")
	}
	if !sk.MatchesTrigger("帮我审查代码") {
		t.Fatal("expected trigger match for Chinese")
	}
	if sk.MatchesTrigger("write some code") {
		t.Fatal("unexpected trigger match")
	}
}

func TestManagerRegisterAndGet(t *testing.T) {
	mgr := NewManager("")
	sk := &Skill{Name: "test-skill", SystemPrompt: "hello"}
	if err := mgr.Register(sk); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, ok := mgr.Get("test-skill")
	if !ok {
		t.Fatal("skill not found")
	}
	if got.Name != "test-skill" {
		t.Fatalf("unexpected name: %s", got.Name)
	}
}

func TestManagerLoadDir(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `name: code-review
description: Code review expert
system_prompt: |
  You are a code review expert.
tools:
  - read_file
  - grep_search
triggers:
  - review
max_uses: 10
`
	if err := os.WriteFile(filepath.Join(dir, "code-review.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	mgr := NewManager(dir)
	if err := mgr.LoadDir(); err != nil {
		t.Fatalf("load dir: %v", err)
	}

	sk, ok := mgr.Get("code-review")
	if !ok {
		t.Fatal("skill not found after load")
	}
	if len(sk.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(sk.Tools))
	}
	if sk.MaxUses != 10 {
		t.Fatalf("expected max_uses=10, got %d", sk.MaxUses)
	}
}

func TestManagerLoadDirNotExist(t *testing.T) {
	mgr := NewManager("/nonexistent/path")
	if err := mgr.LoadDir(); err != nil {
		t.Fatalf("load nonexistent dir should not error: %v", err)
	}
}

func TestManagerList(t *testing.T) {
	mgr := NewManager("")
	mgr.Register(&Skill{Name: "a", SystemPrompt: "x"})
	mgr.Register(&Skill{Name: "b", SystemPrompt: "y"})
	if len(mgr.List()) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(mgr.List()))
	}
}

func TestManagerDelete(t *testing.T) {
	mgr := NewManager("")
	mgr.Register(&Skill{Name: "a", SystemPrompt: "x"})
	if !mgr.Delete("a") {
		t.Fatal("delete should return true")
	}
	if mgr.Delete("a") {
		t.Fatal("second delete should return false")
	}
	if _, ok := mgr.Get("a"); ok {
		t.Fatal("skill should be gone")
	}
}

func TestManagerFindByTrigger(t *testing.T) {
	mgr := NewManager("")
	mgr.Register(&Skill{Name: "review", SystemPrompt: "x", Triggers: []string{"review"}})
	mgr.Register(&Skill{Name: "debug", SystemPrompt: "x", Triggers: []string{"debug"}})
	matches := mgr.FindByTrigger("please review")
	if len(matches) != 1 || matches[0].Name != "review" {
		t.Fatalf("unexpected matches: %+v", matches)
	}
}

func TestSessionSkills(t *testing.T) {
	ss := NewSessionSkills()
	sk := &Skill{Name: "review", SystemPrompt: "review prompt", Tools: []string{"read_file"}, MaxUses: 2}

	activated := ss.Activate(sk)
	if activated.RemainingUses != 2 {
		t.Fatalf("expected 2 remaining uses, got %d", activated.RemainingUses)
	}

	active := ss.Active()
	if len(active) != 1 {
		t.Fatalf("expected 1 active skill")
	}

	fragment := ss.SystemPromptFragment()
	if fragment == "" {
		t.Fatal("expected non-empty system prompt fragment")
	}

	names := ss.ToolNames()
	if len(names) != 1 || names[0] != "read_file" {
		t.Fatalf("unexpected tool names: %v", names)
	}

	if !ss.RecordUse("review") {
		t.Fatal("first use should succeed")
	}
	if !ss.RecordUse("review") {
		t.Fatal("second use should succeed")
	}
	if ss.RecordUse("review") {
		t.Fatal("third use should fail (max_uses=2)")
	}

	ss.Activate(&Skill{Name: "debug", SystemPrompt: "debug"})
	if !ss.Deactivate("debug") {
		t.Fatal("deactivate should return true")
	}
	if ss.Deactivate("debug") {
		t.Fatal("second deactivate should return false")
	}
}

func TestSessionSkillsUnlimitedUses(t *testing.T) {
	ss := NewSessionSkills()
	sk := &Skill{Name: "unlimited", SystemPrompt: "x", MaxUses: 0}
	ss.Activate(sk)

	for i := 0; i < 100; i++ {
		if !ss.RecordUse("unlimited") {
			t.Fatalf("use %d should succeed for unlimited skill", i)
		}
	}
}
