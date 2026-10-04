package deepseek

import "agent/internal/llm"

type chatRequest struct {
	Model      string               `json:"model"`
	Messages   []llm.Message        `json:"messages"`
	Tools      []llm.ToolDefinition `json:"tools,omitempty"`
	ToolChoice string               `json:"tool_choice,omitempty"`
	Stream     bool                 `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message llm.Message `json:"message"`
	} `json:"choices"`
}

type streamResponse struct {
	Choices []struct {
		Delta struct {
			Role             llm.Role            `json:"role"`
			Content          string              `json:"content"`
			ReasoningContent string              `json:"reasoning_content"`
			ToolCalls        []llm.ToolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}
