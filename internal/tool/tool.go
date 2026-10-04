package tool

import (
	"context"

	"agent/internal/llm"
)

type Tool interface {
	Definition() llm.ToolDefinition
	Execute(ctx context.Context, arguments string) (string, error)
}
