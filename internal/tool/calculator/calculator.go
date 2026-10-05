package calculator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"agent/internal/llm"
)

type Tool struct{}

type Arguments struct {
	Num1      float64 `json:"num1"`
	Num2      float64 `json:"num2"`
	Operation string  `json:"operation"`
}

func New() *Tool {
	return &Tool{}
}

func (t *Tool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "get_calculator",
			Description: "计算两个数字的加、减、乘、除结果",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"num1": map[string]any{
						"type":        "number",
						"description": "第一个数字",
					},
					"num2": map[string]any{
						"type":        "number",
						"description": "第二个数字",
					},
					"operation": map[string]any{
						"type":        "string",
						"description": "运算类型",
						"enum":        []string{"add", "subtract", "multiply", "divide"},
					},
				},
				"required": []string{"num1", "num2", "operation"},
			},
		},
	}
}

func (t *Tool) Execute(_ context.Context, arguments string) (string, error) {
	var args Arguments
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("decode calculator arguments: %w", err)
	}
	if strings.TrimSpace(args.Operation) == "" {
		return "", fmt.Errorf("operation cannot be empty")
	}

	var result float64

	switch args.Operation {
	case "add":
		result = args.Num1 + args.Num2
	case "subtract":
		result = args.Num1 - args.Num2
	case "multiply":
		result = args.Num1 * args.Num2
	case "divide":
		if args.Num2 == 0 {
			return "", fmt.Errorf("cannot divide by zero")
		}
		result = args.Num1 / args.Num2
	default:
		return "", fmt.Errorf("unsupported operation %q", args.Operation)
	}

	return strconv.FormatFloat(result, 'f', -1, 64), nil
}
