package llm

import "context"

type Provider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error)
	ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition) (<-chan StreamChunk, error)
}
