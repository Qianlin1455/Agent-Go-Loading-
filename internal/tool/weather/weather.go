package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent/internal/llm"
)

type Tool struct{}

type Arguments struct {
	Location string `json:"location"`
}

func New() *Tool {
	return &Tool{}
}

func (t *Tool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "get_weather",
			Description: "获取指定城市的天气",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{
						"type":        "string",
						"description": "城市名称，例如宣城、杭州、上海",
					},
				},
				"required": []string{"location"},
			},
		},
	}
}

func (t *Tool) Execute(_ context.Context, arguments string) (string, error) {
	var args Arguments
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("decode weather arguments: %w", err)
	}
	if strings.TrimSpace(args.Location) == "" {
		return "", fmt.Errorf("location cannot be empty")
	}
	return args.Location + "：25℃，晴天", nil
}
