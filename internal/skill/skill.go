package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"claude-go-code/pkg/types"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	SystemPrompt string   `json:"system_prompt" yaml:"system_prompt"`
	Tools        []string `json:"tools" yaml:"tools"`
	Triggers     []string `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	MaxUses      int      `json:"max_uses,omitempty" yaml:"max_uses,omitempty"`
}

func (s *Skill) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if s.SystemPrompt == "" {
		return fmt.Errorf("skill %s: system_prompt is required", s.Name)
	}
	return nil
}

func (s *Skill) MatchesTrigger(text string) bool {
	lower := strings.ToLower(text)
	for _, trigger := range s.Triggers {
		if strings.Contains(lower, strings.ToLower(trigger)) {
			return true
		}
	}
	return false
}

type ToolSpecProvider interface {
	Specs() []types.ToolSpec
}

type Manager struct {
	mu     sync.RWMutex
	skills map[string]*Skill
	dir    string
}

func NewManager(skillDir string) *Manager {
	return &Manager{
		skills: make(map[string]*Skill),
		dir:    skillDir,
	}
}

func (m *Manager) LoadDir() error {
	if m.dir == "" {
		return nil
	}
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read skill dir: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, name))
		if err != nil {
			return fmt.Errorf("read skill file %s: %w", name, err)
		}
		var sk Skill
		switch ext {
		case ".json":
			if err := json.Unmarshal(data, &sk); err != nil {
				return fmt.Errorf("parse skill %s: %w", name, err)
			}
		default:
			if err := yaml.Unmarshal(data, &sk); err != nil {
				return fmt.Errorf("parse skill %s: %w", name, err)
			}
		}
		if err := sk.Validate(); err != nil {
			return err
		}
		m.skills[sk.Name] = &sk
	}
	return nil
}

func (m *Manager) Register(sk *Skill) error {
	if err := sk.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skills[sk.Name] = sk
	return nil
}

func (m *Manager) Get(name string) (*Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sk, ok := m.skills[name]
	return sk, ok
}

func (m *Manager) List() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Skill, 0, len(m.skills))
	for _, sk := range m.skills {
		out = append(out, sk)
	}
	return out
}

func (m *Manager) Delete(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.skills[name]; !ok {
		return false
	}
	delete(m.skills, name)
	return true
}

func (m *Manager) FindByTrigger(text string) []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var matches []*Skill
	for _, sk := range m.skills {
		if sk.MatchesTrigger(text) {
			matches = append(matches, sk)
		}
	}
	return matches
}

type SessionSkill struct {
	Skill         *Skill
	RemainingUses int
}

type SessionSkills struct {
	mu     sync.RWMutex
	active map[string]*SessionSkill
}

func NewSessionSkills() *SessionSkills {
	return &SessionSkills{active: make(map[string]*SessionSkill)}
}

func (s *SessionSkills) Activate(sk *Skill) *SessionSkill {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := &SessionSkill{Skill: sk, RemainingUses: sk.MaxUses}
	s.active[sk.Name] = ss
	return ss
}

func (s *SessionSkills) Deactivate(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.active[name]; !ok {
		return false
	}
	delete(s.active, name)
	return true
}

func (s *SessionSkills) Active() []*SessionSkill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*SessionSkill, 0, len(s.active))
	for _, ss := range s.active {
		out = append(out, ss)
	}
	return out
}

func (s *SessionSkills) SystemPromptFragment() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.active) == 0 {
		return ""
	}
	var parts []string
	for _, ss := range s.active {
		parts = append(parts, fmt.Sprintf("<skill name=%q>\n%s\n</skill>", ss.Skill.Name, ss.Skill.SystemPrompt))
	}
	return strings.Join(parts, "\n\n")
}

func (s *SessionSkills) ToolNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	var names []string
	for _, ss := range s.active {
		for _, t := range ss.Skill.Tools {
			if !seen[t] {
				seen[t] = true
				names = append(names, t)
			}
		}
	}
	return names
}

func (s *SessionSkills) RecordUse(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok := s.active[name]
	if !ok {
		return false
	}
	if ss.RemainingUses <= 0 && ss.Skill.MaxUses > 0 {
		delete(s.active, name)
		return false
	}
	if ss.Skill.MaxUses > 0 {
		ss.RemainingUses--
	}
	return true
}
