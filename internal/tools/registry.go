package tools

import (
	"fmt"

	"claude-go-code/pkg/types"
)

type Registry interface {
	Register(tool Tool) error
	Get(name string) (Tool, bool)
	Specs() []types.ToolSpec
}

type MapRegistry struct {
	tools map[string]Tool
}

func NewRegistry(initial []Tool) *MapRegistry {
	registry := &MapRegistry{tools: make(map[string]Tool, len(initial))}
	for _, tool := range initial {
		_ = registry.Register(tool)
	}
	return registry
}

func (r *MapRegistry) Register(tool Tool) error {
	spec := tool.Spec()
	if spec.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	r.tools[spec.Name] = tool
	return nil
}

func (r *MapRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *MapRegistry) Specs() []types.ToolSpec {
	out := make([]types.ToolSpec, 0, len(r.tools))
	for _, tool := range r.tools {
		out = append(out, tool.Spec())
	}
	return out
}
