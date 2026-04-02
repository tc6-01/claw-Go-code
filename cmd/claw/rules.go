package main

import (
	"fmt"
	"io"

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
	default:
		return true, fmt.Errorf("unknown permissions rules command: %s", args[2])
	}
}
