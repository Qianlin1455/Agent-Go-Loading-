package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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

type startErrorProvider struct {
	err error
}

func (p startErrorProvider) Chat(context.Context, []llm.Message, []llm.ToolDefinition) (llm.Message, error) {
	return llm.Message{}, p.err
}

func (p startErrorProvider) ChatStream(context.Context, []llm.Message, []llm.ToolDefinition) (<-chan llm.StreamChunk, error) {
	return nil, p.err
}

// contextStreamProvider keeps a stream open until its context is cancelled.
// It gives cancellation tests a deterministic event to wait for instead of
// relying on arbitrary sleeps inside the mock.
type contextStreamProvider struct{}

func (contextStreamProvider) Chat(context.Context, []llm.Message, []llm.ToolDefinition) (llm.Message, error) {
	return llm.Message{}, nil
}

func (contextStreamProvider) ChatStream(ctx context.Context, _ []llm.Message, _ []llm.ToolDefinition) (<-chan llm.StreamChunk, error) {
	out := make(chan llm.StreamChunk)
	go func() {
		defer close(out)
		<-ctx.Done()
	}()
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

func TestRunStreamProviderError(t *testing.T) {
	wantErr := errors.New("model unavailable")
	a := New(startErrorProvider{err: wantErr}, tool.NewRegistry(), 3)
	session := NewSession()

	err := a.RunStream(context.Background(), session, "hello", nil)

	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf("expected empty session, got %#v", messages)
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

type failingWeatherTool struct {
	err error
}

func (failingWeatherTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "get_weather",
		},
	}
}

func (t failingWeatherTool) Execute(context.Context, string) (string, error) {
	return "", t.err
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

func TestRunStreamExceedsMaxSteps(t *testing.T) {
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
			{{
				ToolCalls: []llm.ToolCallDelta{{
					Index: 0,
					ID:    "call-2",
					Type:  "function",
					Function: llm.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"上海"}`,
					},
				}},
				FinishReason: "tool_calls",
			}},
		},
	}

	registry := tool.NewRegistry()
	if err := registry.Register(echoWeatherTool{}); err != nil {
		t.Fatal(err)
	}

	const maxSteps = 2
	a := New(provider, registry, maxSteps)
	session := NewSession()

	err := a.RunStream(
		context.Background(),
		session,
		"不断查询天气",
		nil,
	)

	if err == nil {
		t.Fatal("expected max steps error, got nil")
	}

	wantErr := "agent reached maximum steps: 2"
	if err.Error() != wantErr {
		t.Fatalf("got error %q, want %q", err.Error(), wantErr)
	}

	if len(provider.calls) != maxSteps {
		t.Fatalf(
			"got %d model calls, want %d",
			len(provider.calls),
			maxSteps,
		)
	}

	// 超过 maxSteps 后，本轮消息应被回滚。
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf("expected empty session, got %#v", messages)
	}
}

func TestRunStreamUnknownTool(t *testing.T) {
	provider := &recordingStreamProvider{
		responses: [][]llm.StreamChunk{
			{{
				ToolCalls: []llm.ToolCallDelta{{
					Index: 0,
					ID:    "call-unknown",
					Type:  "function",
					Function: llm.FunctionCall{
						Name:      "unknown_tool",
						Arguments: `{}`,
					},
				}},
				FinishReason: "tool_calls",
			}},
		},
	}

	// 使用空 Registry，不注册 unknown_tool。
	registry := tool.NewRegistry()

	a := New(provider, registry, 3)
	session := NewSession()

	err := a.RunStream(
		context.Background(),
		session,
		"调用一个不存在的工具",
		nil,
	)

	if err == nil {
		t.Fatal("expected unknown tool error, got nil")
	}

	wantErr := `execute tool unknown_tool: unknown tool "unknown_tool"`
	if err.Error() != wantErr {
		t.Fatalf("got error %q, want %q", err.Error(), wantErr)
	}

	// 找到未知工具后，不应该继续调用模型。
	if len(provider.calls) != 1 {
		t.Fatalf(
			"got %d model calls, want 1",
			len(provider.calls),
		)
	}

	// 失败的对话不应该写入 Session。
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf(
			"expected empty session, got %#v",
			messages,
		)
	}
}

func TestRunStreamInvalidToolArguments(t *testing.T) {
	provider := &recordingStreamProvider{
		responses: [][]llm.StreamChunk{
			{{
				ToolCalls: []llm.ToolCallDelta{{
					Index: 0,
					ID:    "call-invalid-arguments",
					Type:  "function",
					Function: llm.FunctionCall{
						Name: "get_weather",

						// JSON 不完整，属于非法参数。
						Arguments: `{"location":`,
					},
				}},
				FinishReason: "tool_calls",
			}},
		},
	}

	registry := tool.NewRegistry()
	if err := registry.Register(echoWeatherTool{}); err != nil {
		t.Fatal(err)
	}

	a := New(provider, registry, 3)
	session := NewSession()

	err := a.RunStream(
		context.Background(),
		session,
		"查询天气",
		nil,
	)

	if err == nil {
		t.Fatal("expected invalid arguments error, got nil")
	}

	// 不建议完整匹配 JSON 库的错误，因为不同 Go 版本的描述可能变化。
	if !strings.Contains(err.Error(), "execute tool get_weather") {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("expected JSON decode error, got: %v", err)
	}

	// 工具执行失败后，本轮对话应该回滚。
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf(
			"expected empty session, got %#v",
			messages,
		)
	}
}

func TestRunStreamToolExecutionError(t *testing.T) {
	wantErr := errors.New("weather service unavailable")
	provider := &recordingStreamProvider{
		responses: [][]llm.StreamChunk{
			{{
				ToolCalls: []llm.ToolCallDelta{{
					Index: 0,
					ID:    "call-tool-error",
					Type:  "function",
					Function: llm.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"杭州"}`,
					},
				}},
				FinishReason: "tool_calls",
			}},
		},
	}
	registry := tool.NewRegistry()
	if err := registry.Register(failingWeatherTool{err: wantErr}); err != nil {
		t.Fatal(err)
	}
	a := New(provider, registry, 3)
	session := NewSession()

	err := a.RunStream(context.Background(), session, "查询天气", nil)

	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want wrapped error %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "execute tool get_weather") {
		t.Fatalf("error does not identify the failed tool: %v", err)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("got %d model calls, want 1", len(provider.calls))
	}
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf("expected empty session, got %#v", messages)
	}
}

func TestRunStreamRejectsClosedEmptyStream(t *testing.T) {
	provider := &recordingStreamProvider{
		responses: [][]llm.StreamChunk{{}},
	}
	a := New(provider, tool.NewRegistry(), 3)
	session := NewSession()

	err := a.RunStream(context.Background(), session, "hello", nil)

	if err == nil || err.Error() != "model returned empty response" {
		t.Fatalf("got error %v, want empty response error", err)
	}
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf("expected empty session, got %#v", messages)
	}
}

func TestRunStreamRejectsEmptyStopChunk(t *testing.T) {
	provider := &recordingStreamProvider{
		responses: [][]llm.StreamChunk{
			{{FinishReason: "stop"}},
		},
	}
	a := New(provider, tool.NewRegistry(), 3)
	session := NewSession()

	err := a.RunStream(context.Background(), session, "hello", nil)

	if err == nil || err.Error() != "model returned empty response" {
		t.Fatalf("got error %v, want empty response error", err)
	}
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf("expected empty session, got %#v", messages)
	}
}

func TestRunStreamCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := New(contextStreamProvider{}, tool.NewRegistry(), 3)
	session := NewSession()

	err := a.RunStream(ctx, session, "hello", nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want %v", err, context.Canceled)
	}
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf("expected empty session, got %#v", messages)
	}
}

func TestRunStreamTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	a := New(contextStreamProvider{}, tool.NewRegistry(), 3)
	session := NewSession()

	err := a.RunStream(ctx, session, "hello", nil)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got error %v, want %v", err, context.DeadlineExceeded)
	}
	if messages := session.Messages(); len(messages) != 0 {
		t.Fatalf("expected empty session, got %#v", messages)
	}
}
