package deepseek

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"agent/internal/llm"
)

func (c *Client) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (<-chan llm.StreamChunk, error) {
	resp, err := c.send(ctx, messages, tools, true)
	if err != nil {
		return nil, err
	}

	out := make(chan llm.StreamChunk, 64)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		completed := false

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			//空行或deepseek注释则跳过
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			//没有data段就跳过
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			//去掉空格和前缀
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			//如果内容是done则结束
			if data == "[DONE]" {
				completed = true
				break
			}

			var event streamResponse
			//反序列化后写入out
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				sendChunk(ctx, out, llm.StreamChunk{Err: fmt.Errorf("decode SSE event: %w", err)})
				return
			}
			//接收到空响应
			if len(event.Choices) == 0 {
				continue
			}

			choice := event.Choices[0]
			chunk := llm.StreamChunk{
				Content:          choice.Delta.Content,
				ReasoningContent: choice.Delta.ReasoningContent,
				ToolCalls:        choice.Delta.ToolCalls,
			}
			if choice.FinishReason != nil {
				chunk.FinishReason = *choice.FinishReason
			}
			if !sendChunk(ctx, out, chunk) {
				return
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			sendChunk(ctx, out, llm.StreamChunk{Err: fmt.Errorf("read SSE stream: %w", err)})
			return
		}
		if !completed && ctx.Err() == nil {
			sendChunk(ctx, out, llm.StreamChunk{Err: io.ErrUnexpectedEOF})
		}
	}()

	return out, nil
}

func sendChunk(ctx context.Context, out chan<- llm.StreamChunk, chunk llm.StreamChunk) bool {
	//如果可写入管道return true，上游关闭则return false
	select {
	case out <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}
