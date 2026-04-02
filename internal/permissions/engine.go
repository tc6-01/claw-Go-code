package permissions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Engine interface {
	Decide(ctx context.Context, req PermissionRequest) (*PermissionDecision, error)
}

type Options struct {
	DefaultMode      Mode
	EscalationPolicy EscalationPolicy
	Confirmer        Confirmer
	RuleCachePath    string
}

type StaticEngine struct {
	defaultMode      Mode
	escalationPolicy EscalationPolicy
	confirmer        Confirmer
	ruleCachePath    string
	ruleLoadOnce     sync.Once
	ruleLoadErr      error
	mu               sync.RWMutex
	ruleDecisions    map[permissionCacheKey]Decision
	sessionDecisions map[permissionCacheKey]Decision
}

type permissionCacheKey struct {
	ToolName      string
	CurrentMode   Mode
	Required      Mode
	TargetKind    RuleTargetKind
	TargetPattern string
}

type persistedRuleCache struct {
	Version int                  `json:"version"`
	Entries []persistedRuleEntry `json:"entries"`
}

type persistedRuleEntry struct {
	ToolName      string         `json:"tool_name"`
	CurrentMode   Mode           `json:"current_mode"`
	Required      Mode           `json:"required"`
	TargetKind    RuleTargetKind `json:"target_kind,omitempty"`
	TargetPattern string         `json:"target_pattern,omitempty"`
	Decision      Decision       `json:"decision"`
}

func NewStaticEngine(defaultMode Mode) *StaticEngine {
	return NewStaticEngineWithOptions(Options{DefaultMode: defaultMode})
}

func NewStaticEngineWithOptions(opts Options) *StaticEngine {
	return &StaticEngine{
		defaultMode:      opts.DefaultMode,
		escalationPolicy: normalizeEscalationPolicy(opts.EscalationPolicy),
		confirmer:        opts.Confirmer,
		ruleCachePath:    opts.RuleCachePath,
		ruleDecisions:    make(map[permissionCacheKey]Decision),
		sessionDecisions: make(map[permissionCacheKey]Decision),
	}
}

func (e *StaticEngine) Decide(ctx context.Context, req PermissionRequest) (*PermissionDecision, error) {
	current := req.CurrentMode
	if current == "" {
		current = e.defaultMode
	}
	if current == "" {
		current = ModeWorkspaceWrite
	}
	req.CurrentMode = current
	if rank(current) >= rank(req.Required) {
		return &PermissionDecision{Decision: DecisionAllow}, nil
	}
	if cached, ok, err := e.lookupRuleDecision(req); err != nil {
		return nil, err
	} else if ok {
		return &PermissionDecision{
			Decision: cached,
			Reason:   fmt.Sprintf("tool %s reuses %s rule cached on disk", req.ToolName, cached),
		}, nil
	}
	if cached, ok := e.lookupSessionDecision(req); ok {
		return &PermissionDecision{
			Decision: cached,
			Reason:   fmt.Sprintf("tool %s reuses %s decision cached for this session", req.ToolName, cached),
		}, nil
	}
	if e.escalationPolicy == EscalationPrompt {
		if e.confirmer != nil {
			outcome, err := e.confirmer.Confirm(ctx, req)
			if err != nil {
				return nil, err
			}
			outcome = normalizeConfirmationOutcome(outcome)
			switch outcome.Scope {
			case ConfirmationScopeSession:
				e.storeSessionDecision(req, outcome.Decision)
			case ConfirmationScopeRule:
				if err := e.storeRuleDecision(req, outcome.Decision); err != nil {
					return nil, err
				}
			}
			if outcome.Decision == DecisionAllow {
				return &PermissionDecision{
					Decision: DecisionAllow,
					Reason:   fmt.Sprintf("tool %s confirmed for %s from %s mode (%s)", req.ToolName, req.Required, current, outcome.Scope),
				}, nil
			}
			return &PermissionDecision{
				Decision: DecisionDeny,
				Reason:   fmt.Sprintf("tool %s was denied during confirmation from %s mode (%s)", req.ToolName, current, outcome.Scope),
			}, nil
		}
		return &PermissionDecision{
			Decision: DecisionPrompt,
			Reason:   fmt.Sprintf("tool %s requires %s and needs confirmation from %s mode", req.ToolName, req.Required, current),
		}, nil
	}
	return &PermissionDecision{
		Decision: DecisionDeny,
		Reason:   fmt.Sprintf("tool %s requires %s but current mode is %s", req.ToolName, req.Required, current),
	}, nil
}

func rank(mode Mode) int {
	switch mode {
	case ModeDangerFull:
		return 3
	case ModeWorkspaceWrite:
		return 2
	case ModeReadOnly:
		return 1
	default:
		return 0
	}
}

func (e *StaticEngine) lookupSessionDecision(req PermissionRequest) (Decision, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	decision, ok := e.sessionDecisions[cacheKey(req)]
	return decision, ok
}

func (e *StaticEngine) storeSessionDecision(req PermissionRequest, decision Decision) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionDecisions[cacheKey(req)] = decision
}

func (e *StaticEngine) lookupRuleDecision(req PermissionRequest) (Decision, bool, error) {
	if err := e.ensureRuleDecisionsLoaded(); err != nil {
		return "", false, err
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	decision, ok := e.ruleDecisions[cacheKey(req)]
	return decision, ok, nil
}

func (e *StaticEngine) storeRuleDecision(req PermissionRequest, decision Decision) error {
	if err := e.ensureRuleDecisionsLoaded(); err != nil {
		return err
	}

	key := cacheKey(req)
	e.mu.Lock()
	e.ruleDecisions[key] = decision
	snapshot := make(map[permissionCacheKey]Decision, len(e.ruleDecisions))
	for existingKey, existingDecision := range e.ruleDecisions {
		snapshot[existingKey] = existingDecision
	}
	e.mu.Unlock()

	return persistRuleDecisionFile(e.ruleCachePath, snapshot)
}

func (e *StaticEngine) ensureRuleDecisionsLoaded() error {
	e.ruleLoadOnce.Do(func() {
		if e.ruleCachePath == "" {
			return
		}
		loaded, err := loadRuleDecisionFile(e.ruleCachePath)
		if err != nil {
			e.ruleLoadErr = err
			return
		}
		e.mu.Lock()
		for key, decision := range loaded {
			e.ruleDecisions[key] = decision
		}
		e.mu.Unlock()
	})
	return e.ruleLoadErr
}

func cacheKey(req PermissionRequest) permissionCacheKey {
	matcher := req.Matcher()
	return permissionCacheKey{
		ToolName:      matcher.ToolName,
		CurrentMode:   matcher.CurrentMode,
		Required:      matcher.Required,
		TargetKind:    matcher.TargetKind,
		TargetPattern: matcher.TargetPattern,
	}
}

func loadRuleDecisionFile(path string) (map[permissionCacheKey]Decision, error) {
	decisions := make(map[permissionCacheKey]Decision)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return decisions, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return decisions, nil
	}

	var persisted persistedRuleCache
	if err := json.Unmarshal(data, &persisted); err != nil {
		return nil, err
	}
	for _, entry := range persisted.Entries {
		switch entry.Decision {
		case DecisionAllow, DecisionDeny:
			decisions[permissionCacheKey{
				ToolName:      entry.ToolName,
				CurrentMode:   entry.CurrentMode,
				Required:      entry.Required,
				TargetKind:    entry.TargetKind,
				TargetPattern: entry.TargetPattern,
			}] = entry.Decision
		}
	}
	return decisions, nil
}

func persistRuleDecisionFile(path string, decisions map[permissionCacheKey]Decision) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	entries := make([]persistedRuleEntry, 0, len(decisions))
	for key, decision := range decisions {
		entries = append(entries, persistedRuleEntry{
			ToolName:      key.ToolName,
			CurrentMode:   key.CurrentMode,
			Required:      key.Required,
			TargetKind:    key.TargetKind,
			TargetPattern: key.TargetPattern,
			Decision:      decision,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ToolName != entries[j].ToolName {
			return entries[i].ToolName < entries[j].ToolName
		}
		if entries[i].CurrentMode != entries[j].CurrentMode {
			return entries[i].CurrentMode < entries[j].CurrentMode
		}
		if entries[i].Required != entries[j].Required {
			return entries[i].Required < entries[j].Required
		}
		if entries[i].TargetKind != entries[j].TargetKind {
			return entries[i].TargetKind < entries[j].TargetKind
		}
		if entries[i].TargetPattern != entries[j].TargetPattern {
			return entries[i].TargetPattern < entries[j].TargetPattern
		}
		return entries[i].Decision < entries[j].Decision
	})

	payload, err := json.MarshalIndent(persistedRuleCache{
		Version: 1,
		Entries: entries,
	}, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func LoadRules(path string) ([]StoredRule, error) {
	decisions, err := loadRuleDecisionFile(path)
	if err != nil {
		return nil, err
	}
	rules := make([]StoredRule, 0, len(decisions))
	for key, decision := range decisions {
		rules = append(rules, StoredRule{
			Matcher: RuleMatcher{
				ToolName:      key.ToolName,
				CurrentMode:   key.CurrentMode,
				Required:      key.Required,
				TargetKind:    key.TargetKind,
				TargetPattern: key.TargetPattern,
			},
			Decision: decision,
		})
	}
	sort.Slice(rules, func(i, j int) bool {
		left := rules[i].Matcher
		right := rules[j].Matcher
		if left.ToolName != right.ToolName {
			return left.ToolName < right.ToolName
		}
		if left.CurrentMode != right.CurrentMode {
			return left.CurrentMode < right.CurrentMode
		}
		if left.Required != right.Required {
			return left.Required < right.Required
		}
		if left.TargetKind != right.TargetKind {
			return left.TargetKind < right.TargetKind
		}
		if left.TargetPattern != right.TargetPattern {
			return left.TargetPattern < right.TargetPattern
		}
		return rules[i].Decision < rules[j].Decision
	})
	return rules, nil
}

func ClearRules(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
