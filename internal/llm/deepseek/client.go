package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"agent/internal/llm"
)

type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewClient(apiKey, baseURL, model string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: httpClient,
	}
}

func (c *Client) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (llm.Message, error) {
	resp, err := c.send(ctx, messages, tools, false)
	if err != nil {
		return llm.Message{}, err
	}
	defer resp.Body.Close()

	var result chatResponse
	//将响应体反序列化到result中
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return llm.Message{}, fmt.Errorf("decode DeepSeek response: %w", err)
	}
	if len(result.Choices) == 0 {
		return llm.Message{}, fmt.Errorf("DeepSeek response contains no choices")
	}
	return result.Choices[0].Message, nil
}

func (c *Client) send(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, stream bool) (*http.Response, error) {
	//组装请求
	payload := chatRequest{
		Model:      c.model,
		Messages:   messages,
		Tools:      tools,
		ToolChoice: "auto",
		Stream:     stream,
	}
	//序列化请求
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode DeepSeek request: %w", err)
	}
	//创建http请求报文
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create DeepSeek request: %w", err)
	}
	//设置头部
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	//发送
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send DeepSeek request: %w", err)
	}
	//读取错误
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return nil, fmt.Errorf("DeepSeek returned %s: %s", resp.Status, raw)
	}
	return resp, nil
}
