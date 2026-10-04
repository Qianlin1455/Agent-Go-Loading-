package agent

import (
	"context"
	"fmt"

	"agent/internal/llm"
	"agent/internal/tool"
)

type Agent struct {
	provider llm.Provider
	tools    *tool.Registry
	maxSteps int
}

func New(provider llm.Provider, tools *tool.Registry, maxSteps int) *Agent {
	if maxSteps <= 0 {
		maxSteps = 5
	}
	return &Agent{provider: provider, tools: tools, maxSteps: maxSteps}
}

func (a *Agent) Run(ctx context.Context, session *Session, input string) (string, error) {
	if session == nil {
		return "", fmt.Errorf("session is required")
	}

	messages := session.beginTurn()
	committed := false
	defer func() {
		if !committed {
			session.rollbackTurn()
		}
	}()
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: input})

	for step := 0; step < a.maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		message, err := a.provider.Chat(ctx, messages, a.tools.Definitions())
		if err != nil {
			return "", err
		}
		messages = append(messages, message)
		if len(message.ToolCalls) == 0 {
			session.commitTurn(messages)
			committed = true
			return message.Content, nil
		}

		messages, err = a.executeTools(ctx, messages, message.ToolCalls)
		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("agent reached maximum steps: %d", a.maxSteps)
}

func (a *Agent) executeTools(ctx context.Context, messages []llm.Message, calls []llm.ToolCall) ([]llm.Message, error) {
	for _, call := range calls {
		result, err := a.tools.Execute(ctx, call.Function.Name, call.Function.Arguments)
		if err != nil {
			return messages, fmt.Errorf("execute tool %s: %w", call.Function.Name, err)
		}
		messages = append(messages, llm.Message{
			Role:       llm.RoleTool,
			ToolCallID: call.ID,
			Content:    result,
		})
	}
	return messages, nil
}
