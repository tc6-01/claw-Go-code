package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"claude-go-code/internal/config"
	"claude-go-code/internal/permissions"
)

func handlePermissionRuleCommand(cfg config.Config, args []string, out io.Writer) (bool, error) {
	if len(args) < 3 || args[0] != "permissions" || args[1] != "rules" {
		return false, nil
	}

	switch args[2] {
	case "list":
		rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
		if err != nil {
			return true, err
		}
		if len(args) > 3 && args[3] == "--json" {
			payload := struct {
				Path  string                   `json:"path"`
				Rules []permissions.StoredRule `json:"rules"`
			}{
				Path:  cfg.Permission.RulesPath,
				Rules: rules,
			}
			return true, json.NewEncoder(out).Encode(payload)
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
	case "remove":
		if len(args) < 4 {
			return true, fmt.Errorf("usage: permissions rules remove <index>")
		}
		index, err := strconv.Atoi(args[3])
		if err != nil || index < 1 {
			return true, fmt.Errorf("invalid rule index: %s", args[3])
		}
		rules, err := permissions.LoadRules(cfg.Permission.RulesPath)
		if err != nil {
			return true, err
		}
		if index > len(rules) {
			return true, fmt.Errorf("rule index %d out of range", index)
		}
		removed := rules[index-1]
		rules = append(rules[:index-1], rules[index:]...)
		if len(rules) == 0 {
			if err := permissions.ClearRules(cfg.Permission.RulesPath); err != nil {
				return true, err
			}
		} else if err := permissions.SaveRules(cfg.Permission.RulesPath, rules); err != nil {
			return true, err
		}
		_, err = fmt.Fprintf(out, "Removed rule %d: %s -> %s\n", index, permissions.DescribeRule(removed), removed.Decision)
		return true, err
	default:
		return true, fmt.Errorf("unknown permissions rules command: %s", args[2])
	}
}
