package tools

import "fmt"

func RequireTool(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool is required")
	}
	return nil
}
