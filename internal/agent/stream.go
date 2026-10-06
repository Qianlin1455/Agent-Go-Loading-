package agent

import (
	"context"
	"fmt"
	"strings"

	"agent/internal/llm"
)

type StreamHandler func(chunk llm.StreamChunk) error

func (a *Agent) RunStream(ctx context.Context, session *Session, input string, handler StreamHandler) error {
	if session == nil {
		return fmt.Errorf("session is required")
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
		stepCtx, cancelStep := context.WithCancel(ctx)
		stream, err := a.provider.ChatStream(stepCtx, messages, a.tools.Definitions())
		if err != nil {
			cancelStep()
			return err
		}

		message := llm.Message{Role: llm.RoleAssistant}
		finishReason := ""

		for chunk := range stream {
			if chunk.Err != nil {
				cancelStep()
				return chunk.Err
			}
			if handler != nil {
				if err := handler(chunk); err != nil {
					cancelStep()
					return err
				}
			}

			message.Content += chunk.Content
			message.ReasoningContent += chunk.ReasoningContent
			if chunk.FinishReason != "" {
				finishReason = chunk.FinishReason
			}

			for _, delta := range chunk.ToolCalls {
				if delta.Index < 0 {
					cancelStep()
					return fmt.Errorf("invalid tool call index: %d", delta.Index)
				}
				for len(message.ToolCalls) <= delta.Index {
					message.ToolCalls = append(message.ToolCalls, llm.ToolCall{})
				}
				call := &message.ToolCalls[delta.Index]
				if delta.ID != "" {
					call.ID = delta.ID
				}
				if delta.Type != "" {
					call.Type = delta.Type
				}
				if delta.Function.Name != "" {
					call.Function.Name = delta.Function.Name
				}
				call.Function.Arguments += delta.Function.Arguments
			}
		}
		cancelStep()

		if err := ctx.Err(); err != nil {
			return err
		}
		if len(message.ToolCalls) == 0 && finishReason != "" && finishReason != "stop" {
			return fmt.Errorf("model stopped with reason %q", finishReason)
		}
		if strings.TrimSpace(message.Content) == "" && len(message.ToolCalls) == 0 {
			return fmt.Errorf("model returned empty response")
		}
		messages = append(messages, message)
		if len(message.ToolCalls) == 0 {
			session.commitTurn(messages)
			committed = true
			return nil
		}

		messages, err = a.executeTools(ctx, messages, message.ToolCalls)
		if err != nil {
			return err
		}
	}

	return fmt.Errorf("agent reached maximum steps: %d", a.maxSteps)
}
