# Go Streaming Agent

一个使用 Go 标准库实现的最小流式 AI Agent Runtime。项目接入 DeepSeek Chat Completions API，支持 SSE 流式输出、Function Calling、可扩展工具注册、会话上下文、超时取消和 Agent Loop 步数限制。

当前内置两个演示工具：模拟天气查询 `get_weather` 和四则运算 `get_calculator`。

## 架构

```text
CLI (cmd/agent-cli)
  |
  +-- config        读取环境变量和 .env
  +-- agent         Agent Loop、Session、流式事件聚合
  +-- llm           Provider 接口和通用消息类型
  |    `-- deepseek DeepSeek HTTP 客户端与 SSE 解析
  `-- tool          Tool 接口、Registry
       +-- weather
       `-- calculator
```

核心依赖方向是 `Agent -> llm.Provider` 和 `Agent -> tool.Registry`。Agent 不依赖具体模型或具体工具实现，因此可以替换 Provider，也可以通过 Registry 注册更多工具。

`Session` 保存已成功提交的对话历史，并串行化同一会话中的 turn。某一轮发生 API、流读取或工具执行错误时，该轮会回滚，不会把不完整消息写入历史。

## Agent Loop 流程

每次调用 `RunStream` 时执行以下流程：

1. 从 `Session` 复制历史，并追加当前用户消息。
2. 携带消息历史与工具定义调用 `Provider.ChatStream`。
3. 聚合流式文本、推理内容、结束原因及分片的 tool call 参数。
4. 如果模型直接返回答案，则提交本轮消息并结束。
5. 如果模型返回一个或多个 tool call，则依次通过 Registry 执行，并把每个结果作为 `tool` 消息追加到上下文。
6. 将工具结果再次交给模型，直到得到最终答案，或达到 `MAX_AGENT_STEPS`。

达到最大步数会返回 `agent reached maximum steps` 错误，本轮对话不会提交。

## SSE 流式输出

DeepSeek 客户端以 `stream: true` 请求 `/chat/completions`，逐行解析 `data:` SSE 事件，并通过只读 channel 输出 `llm.StreamChunk`。

- 文本内容到达后立即交给 CLI 的 handler 打印。
- `reasoning_content` 会被聚合进 assistant 消息，但 CLI 默认不展示。
- tool call 的名称、ID 和 JSON 参数可能分散在多个事件中，Agent 按 `index` 合并。
- `[DONE]` 表示流完整结束；流意外关闭、JSON 损坏、空响应或异常结束原因都会返回错误。
- Context 取消会终止请求和流读取，避免 goroutine 或 HTTP 连接长期占用。

## Function Calling

工具以 DeepSeek/OpenAI 风格的 function schema 提供给模型。模型返回 tool call 后，Agent 使用函数名和 JSON 参数查找并执行工具，再将结果连同 `tool_call_id` 发回模型。

一次模型响应可以包含多个 tool call。Agent 会逐个执行并保留各自结果，然后进行下一次模型调用。

## Tool Registry

`internal/tool.Registry` 负责：

- 注册实现 `Tool` 接口的工具；
- 拒绝空名称和重复名称；
- 汇总工具定义供模型选择；
- 按名称执行工具；
- 对未知工具返回明确错误。

工具只需实现：

```go
type Tool interface {
	Definition() llm.ToolDefinition
	Execute(ctx context.Context, arguments string) (string, error)
}
```

## weather / calculator

### `get_weather`

参数：

```json
{"location":"杭州"}
```

这是一个演示工具，不访问真实天气服务，固定返回指定城市 `25℃，晴天`。缺少城市或 JSON 非法时返回错误。

### `get_calculator`

参数：

```json
{"num1":128,"num2":45,"operation":"multiply"}
```

支持 `add`、`subtract`、`multiply`、`divide`。非法 JSON、缺少 operation、不支持的运算以及除数为零都会返回错误。

## Context / Timeout

CLI 为每个用户输入创建带超时的 `context.Context`。它贯穿 Agent、Provider 和 Tool：

- `REQUEST_TIMEOUT` 控制单轮对话总时限，默认 `2m`；
- 超时返回 `context deadline exceeded`；
- 主动取消返回 `context canceled`；
- `MAX_AGENT_STEPS` 控制单轮最多模型调用次数，默认 `5`。

## 如何运行

要求 Go 1.24 或更高版本（以 `go.mod` 为准）。

在项目根目录创建 `.env`（该文件已被 Git 忽略）：

```dotenv
DEEPSEEK_API_KEY=your_api_key
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-flash
REQUEST_TIMEOUT=2m
MAX_AGENT_STEPS=5
```

只有 `DEEPSEEK_API_KEY` 是必填项，其余变量均有默认值。启动交互式 CLI：

```bash
go run ./cmd/agent-cli
```

也可以使用 Makefile：

```bash
make run
```

输入问题后会流式显示结果；输入 `exit` 退出。

常用质量检查：

```bash
go test ./...
go test -race ./...
go vet ./...
```

## 项目目录

```text
.
|-- cmd/agent-cli/                 # CLI 入口
|-- internal/
|   |-- agent/                     # Agent Loop、流式聚合、Session
|   |-- config/                    # 环境变量与 .env 配置
|   |-- llm/
|   |   `-- deepseek/              # DeepSeek 客户端与 SSE 解析
|   `-- tool/
|       |-- calculator/            # 四则运算工具
|       `-- weather/               # 模拟天气工具
|-- testdata/                      # SSE 测试数据
|-- go.mod
|-- Makefile
`-- README.md
```

## 后续规划

当前版本以最小、可读、可测试的 runtime 为目标，不再扩展功能。后续可以在新的迭代中考虑：

- 接入真实天气服务；
- 增加结构化日志、指标与 tracing；
- 支持更多模型 Provider；
- 为工具增加并发执行、重试和权限策略；
- 增加持久化 Session 与更完整的端到端测试。
