package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
)

func TestHandlePermissionRuleCommandList(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	engine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      permissions.ModeWorkspaceWrite,
		EscalationPolicy: permissions.EscalationPrompt,
		RuleCachePath:    cfg.Permission.RulesPath,
		Confirmer: permissions.ConfirmFunc(func(_ context.Context, _ permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), permissions.RequestForToolCall(
		"bash",
		permissions.ModeWorkspaceWrite,
		permissions.ModeDangerFull,
		[]byte(`{"command":"git status"}`),
	)); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{"permissions", "rules", "list"}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if !strings.Contains(out.String(), "command_prefix=git") {
		t.Fatalf("unexpected list output: %s", out.String())
	}
}

func TestHandlePermissionRuleCommandListJSON(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	engine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      permissions.ModeWorkspaceWrite,
		EscalationPolicy: permissions.EscalationPrompt,
		RuleCachePath:    cfg.Permission.RulesPath,
		Confirmer: permissions.ConfirmFunc(func(_ context.Context, _ permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), permissions.RequestForToolCall(
		"web_fetch",
		permissions.ModeWorkspaceWrite,
		permissions.ModeDangerFull,
		[]byte(`{"url":"https://example.com/page"}`),
	)); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{"permissions", "rules", "list", "--json"}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	var payload struct {
		Path  string                   `json:"path"`
		Rules []permissions.StoredRule `json:"rules"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%s", err, out.String())
	}
	if payload.Path != cfg.Permission.RulesPath {
		t.Fatalf("path = %q, want %q", payload.Path, cfg.Permission.RulesPath)
	}
	if len(payload.Rules) != 1 || payload.Rules[0].Matcher.TargetKind != permissions.RuleTargetHost {
		t.Fatalf("unexpected rules payload: %#v", payload.Rules)
	}
}

func TestHandlePermissionRuleCommandClear(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	engine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      permissions.ModeWorkspaceWrite,
		EscalationPolicy: permissions.EscalationPrompt,
		RuleCachePath:    cfg.Permission.RulesPath,
		Confirmer: permissions.ConfirmFunc(func(_ context.Context, _ permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionDeny, Scope: permissions.ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), permissions.RequestForToolCall(
		"web_fetch",
		permissions.ModeWorkspaceWrite,
		permissions.ModeDangerFull,
		[]byte(`{"url":"https://example.com/page"}`),
	)); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{"permissions", "rules", "clear"}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected rules to be cleared, got %#v", rules)
	}
}

func TestHandlePermissionRuleCommandRemove(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	engine := permissions.NewStaticEngineWithOptions(permissions.Options{
		DefaultMode:      permissions.ModeWorkspaceWrite,
		EscalationPolicy: permissions.EscalationPrompt,
		RuleCachePath:    cfg.Permission.RulesPath,
		Confirmer: permissions.ConfirmFunc(func(_ context.Context, req permissions.PermissionRequest) (permissions.ConfirmationOutcome, error) {
			if req.ToolName == "bash" {
				return permissions.ConfirmationOutcome{Decision: permissions.DecisionAllow, Scope: permissions.ConfirmationScopeRule}, nil
			}
			return permissions.ConfirmationOutcome{Decision: permissions.DecisionDeny, Scope: permissions.ConfirmationScopeRule}, nil
		}),
	})
	if _, err := engine.Decide(context.Background(), permissions.RequestForToolCall(
		"bash",
		permissions.ModeWorkspaceWrite,
		permissions.ModeDangerFull,
		[]byte(`{"command":"git status"}`),
	)); err != nil {
		t.Fatalf("Decide() bash error = %v", err)
	}
	if _, err := engine.Decide(context.Background(), permissions.RequestForToolCall(
		"web_fetch",
		permissions.ModeWorkspaceWrite,
		permissions.ModeDangerFull,
		[]byte(`{"url":"https://example.com/page"}`),
	)); err != nil {
		t.Fatalf("Decide() web_fetch error = %v", err)
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{"permissions", "rules", "remove", "1"}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected one remaining rule, got %#v", rules)
	}
	if rules[0].Matcher.ToolName != "web_fetch" {
		t.Fatalf("unexpected remaining rule: %#v", rules[0])
	}
	if !strings.Contains(out.String(), "Removed rule 1:") {
		t.Fatalf("unexpected remove output: %s", out.String())
	}
}

func TestHandlePermissionRuleCommandAdd(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{
		"permissions", "rules", "add",
		"--tool", "bash",
		"--current", string(permissions.ModeWorkspaceWrite),
		"--required", string(permissions.ModeDangerFull),
		"--decision", string(permissions.DecisionAllow),
		"--command-prefix", "git",
	}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected one added rule, got %#v", rules)
	}
	if rules[0].Matcher.TargetKind != permissions.RuleTargetCommandPrefix || rules[0].Matcher.TargetPattern != "git" {
		t.Fatalf("unexpected stored rule: %#v", rules[0])
	}
	if !strings.Contains(out.String(), "Added rule:") {
		t.Fatalf("unexpected add output: %s", out.String())
	}
}

func TestHandlePermissionRuleCommandAddJSON(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{
		"permissions", "rules", "add",
		"--tool", "web_fetch",
		"--current", string(permissions.ModeWorkspaceWrite),
		"--required", string(permissions.ModeDangerFull),
		"--decision", string(permissions.DecisionDeny),
		"--host", "example.com",
		"--json",
	}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	var payload struct {
		Path           string                  `json:"path"`
		Action         string                  `json:"action"`
		Rule           *permissions.StoredRule `json:"rule"`
		RemainingCount int                     `json:"remaining_count"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%s", err, out.String())
	}
	if payload.Action != "added" {
		t.Fatalf("unexpected action: %#v", payload)
	}
	if payload.Rule == nil || payload.Rule.Matcher.TargetKind != permissions.RuleTargetHost {
		t.Fatalf("unexpected rule payload: %#v", payload)
	}
	if payload.RemainingCount != 1 {
		t.Fatalf("unexpected remaining count: %#v", payload)
	}
}

func TestHandlePermissionRuleCommandAddUpdatesExistingRule(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	for _, decision := range []permissions.Decision{permissions.DecisionAllow, permissions.DecisionDeny} {
		var out bytes.Buffer
		handled, err := handlePermissionRuleCommand(cfg, []string{
			"permissions", "rules", "add",
			"--tool", "web_fetch",
			"--current", string(permissions.ModeWorkspaceWrite),
			"--required", string(permissions.ModeDangerFull),
			"--decision", string(decision),
			"--host", "example.com",
		}, &out)
		if err != nil {
			t.Fatalf("handlePermissionRuleCommand() error = %v", err)
		}
		if !handled {
			t.Fatal("expected command to be handled")
		}
	}

	rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected a single updated rule, got %#v", rules)
	}
	if rules[0].Decision != permissions.DecisionDeny {
		t.Fatalf("expected updated rule decision to be deny, got %#v", rules[0])
	}
}

func TestHandlePermissionRuleCommandRemoveByMatcher(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	for _, args := range [][]string{
		{
			"permissions", "rules", "add",
			"--tool", "bash",
			"--current", string(permissions.ModeWorkspaceWrite),
			"--required", string(permissions.ModeDangerFull),
			"--decision", string(permissions.DecisionAllow),
			"--command-prefix", "git",
		},
		{
			"permissions", "rules", "add",
			"--tool", "web_fetch",
			"--current", string(permissions.ModeWorkspaceWrite),
			"--required", string(permissions.ModeDangerFull),
			"--decision", string(permissions.DecisionDeny),
			"--host", "example.com",
		},
	} {
		var out bytes.Buffer
		if _, err := handlePermissionRuleCommand(cfg, args, &out); err != nil {
			t.Fatalf("seed rule command error = %v", err)
		}
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{
		"permissions", "rules", "remove",
		"--tool", "web_fetch",
		"--host", "example.com",
	}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if len(rules) != 1 || rules[0].Matcher.ToolName != "bash" {
		t.Fatalf("unexpected remaining rules: %#v", rules)
	}
	if !strings.Contains(out.String(), "Removed 1 rule(s) matching filter") {
		t.Fatalf("unexpected remove output: %s", out.String())
	}
}

func TestHandlePermissionRuleCommandRemoveJSON(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	cfg.Permission.RulesPath = t.TempDir() + "/rules.json"

	for _, args := range [][]string{
		{
			"permissions", "rules", "add",
			"--tool", "bash",
			"--current", string(permissions.ModeWorkspaceWrite),
			"--required", string(permissions.ModeDangerFull),
			"--decision", string(permissions.DecisionAllow),
			"--command-prefix", "git",
		},
		{
			"permissions", "rules", "add",
			"--tool", "web_fetch",
			"--current", string(permissions.ModeWorkspaceWrite),
			"--required", string(permissions.ModeDangerFull),
			"--decision", string(permissions.DecisionDeny),
			"--host", "example.com",
		},
	} {
		var out bytes.Buffer
		if _, err := handlePermissionRuleCommand(cfg, args, &out); err != nil {
			t.Fatalf("seed rule command error = %v", err)
		}
	}

	var out bytes.Buffer
	handled, err := handlePermissionRuleCommand(cfg, []string{
		"permissions", "rules", "remove",
		"--tool", "web_fetch",
		"--host", "example.com",
		"--json",
	}, &out)
	if err != nil {
		t.Fatalf("handlePermissionRuleCommand() error = %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	var payload struct {
		Path           string                   `json:"path"`
		Action         string                   `json:"action"`
		RemovedRules   []permissions.StoredRule `json:"removed_rules"`
		RemainingCount int                      `json:"remaining_count"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%s", err, out.String())
	}
	if payload.Action != "remove" {
		t.Fatalf("unexpected action payload: %#v", payload)
	}
	if len(payload.RemovedRules) != 1 || payload.RemovedRules[0].Matcher.ToolName != "web_fetch" {
		t.Fatalf("unexpected removed rules payload: %#v", payload)
	}
	if payload.RemainingCount != 1 {
		t.Fatalf("unexpected remaining count: %#v", payload)
	}
}
