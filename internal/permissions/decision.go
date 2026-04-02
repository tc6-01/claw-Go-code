package permissions

import "fmt"

type PermissionRequest struct {
	ToolName      string
	CurrentMode   Mode
	Required      Mode
	TargetKind    RuleTargetKind
	TargetPattern string
}

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionDeny   Decision = "deny"
	DecisionPrompt Decision = "prompt"
)

type PermissionDecision struct {
	Decision Decision
	Reason   string
}

type RuleTargetKind string

const (
	RuleTargetAny           RuleTargetKind = "any"
	RuleTargetCommandPrefix RuleTargetKind = "command_prefix"
	RuleTargetHost          RuleTargetKind = "host"
	RuleTargetPathPattern   RuleTargetKind = "path_pattern"
)

type RuleMatcher struct {
	ToolName      string
	CurrentMode   Mode
	Required      Mode
	TargetKind    RuleTargetKind
	TargetPattern string
}

type StoredRule struct {
	Matcher  RuleMatcher
	Decision Decision
}

func (r PermissionRequest) Matcher() RuleMatcher {
	matcher := RuleMatcher{
		ToolName:      r.ToolName,
		CurrentMode:   r.CurrentMode,
		Required:      r.Required,
		TargetKind:    r.TargetKind,
		TargetPattern: r.TargetPattern,
	}
	if matcher.TargetPattern == "" {
		matcher.TargetKind = RuleTargetAny
	}
	if matcher.TargetKind == "" {
		matcher.TargetKind = RuleTargetAny
	}
	return matcher
}

func DescribeRule(rule StoredRule) string {
	return describeMatcher(rule.Matcher)
}

func describeMatcher(matcher RuleMatcher) string {
	base := fmt.Sprintf("tool=%s current=%s required=%s", matcher.ToolName, matcher.CurrentMode, matcher.Required)
	if matcher.TargetKind == "" || matcher.TargetKind == RuleTargetAny || matcher.TargetPattern == "" {
		return base
	}
	return fmt.Sprintf("%s %s=%s", base, matcher.TargetKind, matcher.TargetPattern)
}
