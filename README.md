# Go Streaming Agent

一个使用 Go 标准库实现的流式 AI Agent 示例，包含 DeepSeek SSE、工具调用和可取消的 Agent Loop。

## 运行

1. 复制 `.env.example` 为 `.env`，或直接设置对应环境变量。
2. 设置 `DEEPSEEK_API_KEY`。
3. 执行：

```bash
go run ./cmd/agent-cli
```

输入 `exit` 退出。

## 运行架构

```mermaid
flowchart TD
    U[用户终端] -->|输入问题| M["main.go<br/>CLI 交互循环"]
    M -->|加载环境变量| C["config.Load<br/>读取 .env"]
    C --> CFG["Config<br/>API Key / Model / Timeout / MaxSteps"]

    M -->|创建| DS["DeepSeek Client"]
    M -->|注册工具| R["Tool Registry"]
    R --> W["get_weather<br/>模拟天气工具"]
    M -->|组装| A["Agent"]

    CFG --> DS
    CFG --> A
    DS -.实现.-> P["llm.Provider 接口"]
    A --> P
    A --> R

    M -->|RunStream| LOOP

    subgraph LOOP["Agent Loop，最多 MaxAgentSteps 次"]
        direction TD
        MSG["构造 messages<br/>加入用户消息"]
        CHAT["provider.ChatStream"]
        HTTP["POST /chat/completions<br/>stream = true<br/>tool_choice = auto"]
        API["DeepSeek API"]
        SSE["读取 SSE 流"]
        MERGE["累计 Content、ReasoningContent<br/>拼接 ToolCalls 参数"]
        DECISION{"模型要求调用工具？"}

        MSG --> CHAT
        CHAT --> HTTP
        HTTP --> API
        API -->|SSE 数据| SSE
        SSE --> MERGE
        MERGE --> DECISION

        DECISION -->|否| PRINT["回调 handler<br/>实时打印 Content"]
        DECISION -->|是| EXEC["Registry.Execute"]
        EXEC --> WEATHER["Weather.Execute"]
        WEATHER --> RESULT["生成 tool 消息<br/>城市：25℃，晴天"]
        RESULT -->|追加到 messages| CHAT
    end

    A --> MSG
    PRINT --> U

    TIMEOUT["Context Timeout"] -.取消请求.-> LOOP
    CFG --> TIMEOUT
```

天气工具当前不访问真实天气服务，而是固定返回“城市：25℃，晴天”。

## 核心结构体关系

```mermaid
classDiagram
    direction LR

    class Config {
        +string APIKey
        +string BaseURL
        +string Model
        +Duration RequestTimeout
        +int MaxAgentSteps
    }

    class Agent {
        -Provider provider
        -Registry tools
        -int maxSteps
        +Run(ctx, session, input) string
        +RunStream(ctx, session, input, handler) error
        -executeTools(ctx, messages, calls)
    }

    class Session {
        -Message[] messages
        +Messages() Message[]
        +Reset()
    }

    class Provider {
        <<interface>>
        +Chat(ctx, messages, tools) Message
        +ChatStream(ctx, messages, tools) StreamChunkChannel
    }

    class DeepSeekClient {
        -string apiKey
        -string baseURL
        -string model
        -HTTPClient httpClient
        +Chat(ctx, messages, tools) Message
        +ChatStream(ctx, messages, tools) StreamChunkChannel
        -send(ctx, messages, tools, stream) HTTPResponse
    }

    class Registry {
        -map~string, Tool~ tools
        +Register(tool) error
        +Definitions() ToolDefinition[]
        +Execute(ctx, name, arguments) string
    }

    class Tool {
        <<interface>>
        +Definition() ToolDefinition
        +Execute(ctx, arguments) string
    }

    class WeatherTool {
        +Definition() ToolDefinition
        +Execute(ctx, arguments) string
    }

    class Message {
        +Role role
        +string content
        +ToolCall[] toolCalls
        +string toolCallID
        +string reasoningContent
    }

    class ToolCall {
        +string id
        +string type
        +FunctionCall function
    }

    class FunctionCall {
        +string name
        +string arguments
    }

    class ToolDefinition {
        +string type
        +ToolFunction function
    }

    class ToolFunction {
        +string name
        +string description
        +map parameters
    }

    class StreamChunk {
        +string content
        +string reasoningContent
        +ToolCallDelta[] toolCalls
        +string finishReason
        +error err
    }

    class ToolCallDelta {
        +int index
        +string id
        +string type
        +FunctionCall function
    }

    Agent o-- Provider : 持有
    Agent *-- Registry : 持有
    Agent ..> Session : 读取并提交会话
    Session *-- Message : 保存历史
    DeepSeekClient ..|> Provider : 实现

    Registry o-- Tool : map保存
    WeatherTool ..|> Tool : 实现

    Provider ..> Message : 输入/返回
    Provider ..> ToolDefinition : 接收
    Provider ..> StreamChunk : 流式返回

    Message *-- ToolCall
    ToolCall *-- FunctionCall

    ToolDefinition *-- ToolFunction

    StreamChunk *-- ToolCallDelta
    ToolCallDelta *-- FunctionCall
```

核心组合关系：

```text
Agent
├── Provider 接口
│   └── DeepSeek Client 实现
├── Registry
│   └── map[string]Tool
│       └── Weather Tool 实现
└── maxSteps

Session
└── []Message 会话历史
```

`Agent` 不直接依赖 DeepSeek 和 Weather 的具体实现，而是分别依赖 `Provider`、`Tool` 接口，因此可以替换模型实现或继续注册其他工具。`Session` 独立保存一段对话的历史，使同一个 Agent 可以安全地服务不同会话。

## 测试

```bash
go test ./...
go vet ./...
```
