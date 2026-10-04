package agent

import (
	"context"
	"encoding/json"
	"testing"

	"agent/internal/llm"
	"agent/internal/tool"
)

type fakeProvider struct{}

func (fakeProvider) Chat(context.Context, []llm.Message, []llm.ToolDefinition) (llm.Message, error) {
	return llm.Message{Role: llm.RoleAssistant, Content: "ok"}, nil
}

func (fakeProvider) ChatStream(context.Context, []llm.Message, []llm.ToolDefinition) (<-chan llm.StreamChunk, error) {
	out := make(chan llm.StreamChunk, 1)
	out <- llm.StreamChunk{Content: "ok", FinishReason: "stop"}
	close(out)
	return out, nil
}

func TestRun(t *testing.T) {
	a := New(fakeProvider{}, tool.NewRegistry(), 2)
	session := NewSession()
	result, err := a.Run(context.Background(), session, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Fatalf("unexpected result: %s", result)
	}
	messages := session.Messages()
	if len(messages) != 2 || messages[0].Role != llm.RoleUser || messages[1].Content != "ok" {
		t.Fatalf("unexpected session messages: %#v", messages)
	}
}

type recordingStreamProvider struct {
	calls     [][]llm.Message
	responses [][]llm.StreamChunk
}

func (p *recordingStreamProvider) Chat(context.Context, []llm.Message, []llm.ToolDefinition) (llm.Message, error) {
	return llm.Message{}, nil
}

func (p *recordingStreamProvider) ChatStream(_ context.Context, messages []llm.Message, _ []llm.ToolDefinition) (<-chan llm.StreamChunk, error) {
	p.calls = append(p.calls, cloneMessages(messages))
	response := p.responses[len(p.calls)-1]
	out := make(chan llm.StreamChunk, len(response))
	for _, chunk := range response {
		out <- chunk
	}
	close(out)
	return out, nil
}

type echoWeatherTool struct{}

func (echoWeatherTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "get_weather",
		},
	}
}

func (echoWeatherTool) Execute(_ context.Context, arguments string) (string, error) {
	var args struct {
		Location string `json:"location"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	return args.Location + "：25℃，晴天", nil
}

func TestRunStreamKeepsHistoryAcrossTurns(t *testing.T) {
	provider := &recordingStreamProvider{
		responses: [][]llm.StreamChunk{
			{{
				ToolCalls: []llm.ToolCallDelta{{
					Index: 0,
					ID:    "call-1",
					Type:  "function",
					Function: llm.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"杭州"}`,
					},
				}},
				FinishReason: "tool_calls",
			}},
			{{Content: "杭州当前 25℃，晴天。", FinishReason: "stop"}},
			{{Content: "上海也可以继续查询。", FinishReason: "stop"}},
		},
	}
	registry := tool.NewRegistry()
	if err := registry.Register(echoWeatherTool{}); err != nil {
		t.Fatal(err)
	}
	a := New(provider, registry, 3)
	session := NewSession()

	if err := a.RunStream(context.Background(), session, "杭州天气怎么样？", nil); err != nil {
		t.Fatal(err)
	}
	if err := a.RunStream(context.Background(), session, "那上海呢？", nil); err != nil {
		t.Fatal(err)
	}

	if len(provider.calls) != 3 {
		t.Fatalf("expected 3 model calls, got %d", len(provider.calls))
	}
	followUp := provider.calls[2]
	if len(followUp) != 5 {
		t.Fatalf("expected 5 messages in follow-up request, got %#v", followUp)
	}
	if followUp[0].Role != llm.RoleUser || followUp[0].Content != "杭州天气怎么样？" {
		t.Fatalf("first user message was not retained: %#v", followUp[0])
	}
	if followUp[1].Role != llm.RoleAssistant || len(followUp[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool call was not retained: %#v", followUp[1])
	}
	if followUp[2].Role != llm.RoleTool || followUp[2].Content != "杭州：25℃，晴天" {
		t.Fatalf("tool result was not retained: %#v", followUp[2])
	}
	if followUp[3].Role != llm.RoleAssistant || followUp[3].Content != "杭州当前 25℃，晴天。" {
		t.Fatalf("final answer was not retained: %#v", followUp[3])
	}
	if followUp[4].Role != llm.RoleUser || followUp[4].Content != "那上海呢？" {
		t.Fatalf("follow-up user message missing: %#v", followUp[4])
	}

	history := session.Messages()
	if len(history) != 6 || history[5].Content != "上海也可以继续查询。" {
		t.Fatalf("unexpected committed history: %#v", history)
	}
}
