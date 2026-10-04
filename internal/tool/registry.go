package tool

import (
	"context"
	"fmt"

	"agent/internal/llm"
)

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) error {
	name := t.Definition().Function.Name
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q is already registered", name)
	}
	r.tools[name] = t
	return nil
}

func (r *Registry) Definitions() []llm.ToolDefinition {
	definitions := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		definitions = append(definitions, t.Definition())
	}
	return definitions
}

func (r *Registry) Execute(ctx context.Context, name, arguments string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return t.Execute(ctx, arguments)
}
