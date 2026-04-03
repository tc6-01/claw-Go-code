package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
)

type ruleFilter struct {
	toolName      string
	currentMode   permissions.Mode
	requiredMode  permissions.Mode
	targetKind    permissions.RuleTargetKind
	targetPattern string
}

type ruleListResponse struct {
	Path  string                   `json:"path"`
	Rules []permissions.StoredRule `json:"rules"`
}

type ruleMutationResponse struct {
	Path           string                   `json:"path"`
	Action         string                   `json:"action"`
	Rule           *permissions.StoredRule  `json:"rule,omitempty"`
	RemovedRules   []permissions.StoredRule `json:"removed_rules,omitempty"`
	RemainingCount int                      `json:"remaining_count"`
}

func handlePermissionRuleCommand(cfg config.Config, args []string, out io.Writer) (bool, error) {
	if len(args) < 3 || args[0] != "permissions" || args[1] != "rules" {
		return false, nil
	}

	switch args[2] {
	case "list":
		jsonOutput, subArgs := consumeJSONFlag(args[3:])
		rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
		if err != nil {
			return true, err
		}
		if jsonOutput {
			return true, json.NewEncoder(out).Encode(ruleListResponse{
				Path:  cfg.Permission.RulesPath,
				Rules: rules,
			})
		}
		if len(subArgs) > 0 {
			return true, fmt.Errorf("unknown arguments for list: %s", strings.Join(subArgs, " "))
		}
		if _, err := fmt.Fprintf(out, "Permission rules file: %s\n", cfg.Permission.RulesPath); err != nil {
			return true, err
		}
		if len(rules) == 0 {
			_, err = fmt.Fprintln(out, "No persisted permission rules.")
			return true, err
		}
		for i, rule := range rules {
			if _, err := fmt.Fprintf(out, "%d. %s -> %s\n", i+1, permissions.DescribeRule(rule), rule.Decision); err != nil {
				return true, err
			}
		}
		return true, nil
	case "clear":
		if err := permissions.ClearRules(cfg.Permission.RulesPath); err != nil {
			return true, err
		}
		_, err := fmt.Fprintf(out, "Cleared persisted permission rules at %s\n", cfg.Permission.RulesPath)
		return true, err
	case "add":
		jsonOutput, subArgs := consumeJSONFlag(args[3:])
		rule, err := parseRuleAddArgs(subArgs)
		if err != nil {
			return true, err
		}
		rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
		if err != nil {
			return true, err
		}
		replaced := false
		for i := range rules {
			if matcherEquals(rules[i].Matcher, rule.Matcher) {
				rules[i] = rule
				replaced = true
				break
			}
		}
		if !replaced {
			rules = append(rules, rule)
		}
		if err := permissions.SaveRules(cfg.Permission.RulesPath, rules); err != nil {
			return true, err
		}
		action := "Added"
		if replaced {
			action = "Updated"
		}
		if jsonOutput {
			ruleCopy := rule
			return true, json.NewEncoder(out).Encode(ruleMutationResponse{
				Path:           cfg.Permission.RulesPath,
				Action:         strings.ToLower(action),
				Rule:           &ruleCopy,
				RemainingCount: len(rules),
			})
		}
		_, err = fmt.Fprintf(out, "%s rule: %s -> %s\n", action, permissions.DescribeRule(rule), rule.Decision)
		return true, err
	case "remove":
		jsonOutput, subArgs := consumeJSONFlag(args[3:])
		if len(subArgs) < 1 {
			return true, fmt.Errorf("usage: permissions rules remove <index> | permissions rules remove --tool <name> [matcher flags]")
		}
		rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
		if err != nil {
			return true, err
		}
		if index, err := strconv.Atoi(subArgs[0]); err == nil {
			if index < 1 {
				return true, fmt.Errorf("invalid rule index: %s", subArgs[0])
			}
			if index > len(rules) {
				return true, fmt.Errorf("rule index %d out of range", index)
			}
			removed := rules[index-1]
			rules = append(rules[:index-1], rules[index:]...)
			if err := saveOrClearRules(cfg.Permission.RulesPath, rules); err != nil {
				return true, err
			}
			if jsonOutput {
				return true, json.NewEncoder(out).Encode(ruleMutationResponse{
					Path:           cfg.Permission.RulesPath,
					Action:         "remove",
					RemovedRules:   []permissions.StoredRule{removed},
					RemainingCount: len(rules),
				})
			}
			_, err = fmt.Fprintf(out, "Removed rule %d: %s -> %s\n", index, permissions.DescribeRule(removed), removed.Decision)
			return true, err
		}
		filter, err := parseRuleFilterArgs(subArgs)
		if err != nil {
			return true, err
		}
		removed := make([]permissions.StoredRule, 0)
		kept := make([]permissions.StoredRule, 0, len(rules))
		for _, rule := range rules {
			if ruleMatchesFilter(rule, filter) {
				removed = append(removed, rule)
				continue
			}
			kept = append(kept, rule)
		}
		if len(removed) == 0 {
			return true, fmt.Errorf("no persisted permission rules matched the provided filter")
		}
		if err := saveOrClearRules(cfg.Permission.RulesPath, kept); err != nil {
			return true, err
		}
		if jsonOutput {
			return true, json.NewEncoder(out).Encode(ruleMutationResponse{
				Path:           cfg.Permission.RulesPath,
				Action:         "remove",
				RemovedRules:   removed,
				RemainingCount: len(kept),
			})
		}
		_, err = fmt.Fprintf(out, "Removed %d rule(s) matching filter\n", len(removed))
		return true, err
	default:
		return true, fmt.Errorf("unknown permissions rules command: %s", args[2])
	}
}

func consumeJSONFlag(args []string) (bool, []string) {
	filtered := make([]string, 0, len(args))
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		filtered = append(filtered, arg)
	}
	return jsonOutput, filtered
}

func parseRuleAddArgs(args []string) (permissions.StoredRule, error) {
	filter, decision, err := parseRuleMatcherArgs(args, true)
	if err != nil {
		return permissions.StoredRule{}, err
	}
	if filter.toolName == "" || filter.currentMode == "" || filter.requiredMode == "" {
		return permissions.StoredRule{}, fmt.Errorf("add requires --tool, --current, and --required")
	}
	return permissions.StoredRule{
		Matcher: permissions.RuleMatcher{
			ToolName:      filter.toolName,
			CurrentMode:   filter.currentMode,
			Required:      filter.requiredMode,
			TargetKind:    filter.targetKind,
			TargetPattern: filter.targetPattern,
		},
		Decision: decision,
	}, nil
}

func parseRuleFilterArgs(args []string) (ruleFilter, error) {
	filter, _, err := parseRuleMatcherArgs(args, false)
	if err != nil {
		return ruleFilter{}, err
	}
	if filter.toolName == "" && filter.currentMode == "" && filter.requiredMode == "" && filter.targetKind == "" && filter.targetPattern == "" {
		return ruleFilter{}, fmt.Errorf("remove filter requires at least one matcher flag")
	}
	return filter, nil
}

func parseRuleMatcherArgs(args []string, requireDecision bool) (ruleFilter, permissions.Decision, error) {
	var filter ruleFilter
	var decision permissions.Decision

	for i := 0; i < len(args); i++ {
		flag := args[i]
		if !strings.HasPrefix(flag, "--") {
			return ruleFilter{}, "", fmt.Errorf("unexpected argument: %s", flag)
		}
		if i+1 >= len(args) {
			return ruleFilter{}, "", fmt.Errorf("missing value for %s", flag)
		}
		value := args[i+1]
		i++

		switch flag {
		case "--tool":
			filter.toolName = value
		case "--current":
			mode, err := permissions.ParseMode(value)
			if err != nil {
				return ruleFilter{}, "", err
			}
			filter.currentMode = mode
		case "--required":
			mode, err := permissions.ParseMode(value)
			if err != nil {
				return ruleFilter{}, "", err
			}
			filter.requiredMode = mode
		case "--decision":
			switch permissions.Decision(value) {
			case permissions.DecisionAllow, permissions.DecisionDeny:
				decision = permissions.Decision(value)
			default:
				return ruleFilter{}, "", fmt.Errorf("unknown decision: %s", value)
			}
		case "--command-prefix":
			filter.targetKind = permissions.RuleTargetCommandPrefix
			filter.targetPattern = value
		case "--host":
			filter.targetKind = permissions.RuleTargetHost
			filter.targetPattern = strings.ToLower(value)
		case "--path":
			filter.targetKind = permissions.RuleTargetPathPattern
			filter.targetPattern = filepath.ToSlash(filepath.Clean(value))
		default:
			return ruleFilter{}, "", fmt.Errorf("unknown flag: %s", flag)
		}
	}

	if requireDecision && decision == "" {
		return ruleFilter{}, "", fmt.Errorf("add requires --decision allow|deny")
	}
	return filter, decision, nil
}

func saveOrClearRules(path string, rules []permissions.StoredRule) error {
	if len(rules) == 0 {
		return permissions.ClearRules(path)
	}
	return permissions.SaveRules(path, rules)
}

func matcherEquals(left, right permissions.RuleMatcher) bool {
	return left.ToolName == right.ToolName &&
		left.CurrentMode == right.CurrentMode &&
		left.Required == right.Required &&
		left.TargetKind == right.TargetKind &&
		left.TargetPattern == right.TargetPattern
}

func ruleMatchesFilter(rule permissions.StoredRule, filter ruleFilter) bool {
	if filter.toolName != "" && rule.Matcher.ToolName != filter.toolName {
		return false
	}
	if filter.currentMode != "" && rule.Matcher.CurrentMode != filter.currentMode {
		return false
	}
	if filter.requiredMode != "" && rule.Matcher.Required != filter.requiredMode {
		return false
	}
	if filter.targetKind != "" && rule.Matcher.TargetKind != filter.targetKind {
		return false
	}
	if filter.targetPattern != "" && rule.Matcher.TargetPattern != filter.targetPattern {
		return false
	}
	return true
}
