# Go 版 Claude Code v0.1 项目骨架定义

## 1. 目标

定义首版项目目录、核心接口、模块依赖方向与初始化顺序，保证实现时结构清晰、不返工。

## 1.5 范围边界与共享文件约束

本骨架文档只定义 v0.1 目录、接口、依赖方向与初始化顺序，不在本阶段展开：

- 具体 provider HTTP 细节
- 具体工具 schema 字段
- hooks / MCP / plugin / LSP / subagent runtime
- 复杂 TUI、远程控制、服务端 API

多人并行规划阶段的共享文件约束：

- `skeleton-go-claude-code-v0.1.md` 作为结构与依赖基线，只有在新增目录、接口或依赖约束时才修改
- `dev-plan-go-claude-code-v0.1.md` 承载实施顺序、里程碑、DoD、风险与发布门槛
- `test-spec-go-claude-code-v0.1.md` 承载覆盖矩阵、fixture/mock 策略、验证顺序与手工验证
- 若实施阶段需要突破本骨架约束，必须先回写计划文档，再允许代码实现

## 2. 目录骨架

```text
cmd/
  claw/
    main.go

internal/
  app/
    app.go
  cli/
    interactive.go
    oneshot.go
    render.go
  commands/
    router.go
    help.go
    status.go
    model.go
    permissions.go
    clear.go
    resume.go
    config.go
    memory.go
    init.go
    diff.go
    session.go
    export.go
  config/
    config.go
    load.go
    types.go
  context/
    workspace.go
    repo.go
  instructions/
    collect.go
    merge.go
    types.go
  prompt/
    builder.go
    system.go
    compact_prompt.go
  provider/
    provider.go
    events.go
    types.go
    anthropic/
      client.go
      stream.go
      normalize.go
    openai/
      client.go
      stream.go
      normalize.go
    shared/
      retry.go
      errors.go
      http.go
  runtime/
    engine.go
    turn.go
    state.go
    tool_loop.go
  tools/
    registry.go
    types.go
    validate.go
    bash/
      tool.go
    file/
      read.go
      write.go
      edit.go
      path.go
    search/
      glob.go
      grep.go
    web/
      fetch.go
      search.go
      html_extract.go
    todo/
      write.go
  permissions/
    mode.go
    engine.go
    decision.go
  session/
    store.go
    model.go
    file_store.go
    export.go
  compact/
    engine.go
    summarize.go
    fallback.go
  skills/
    discover.go
    load.go
  agents/
    discover.go
    load.go

pkg/
  types/
    message.go
    tool.go
    session.go
    usage.go

testdata/
  demo-repo/
  fixtures/
```

## 3. 依赖方向

依赖必须单向流动：

```text
cmd/claw
  -> internal/app
    -> cli / commands
    -> runtime
    -> config
    -> session
    -> provider
    -> tools
    -> permissions
    -> prompt
    -> instructions
    -> compact
```

约束：

- `runtime` 不依赖 `cli`
- `provider` 不依赖 `session`
- `tools` 不依赖具体 provider
- `commands` 不直接调用 provider，只通过 app/runtime/session/config

## 4. 核心接口定义

## 4.1 Provider

```go
type Provider interface {
    Send(ctx context.Context, req *MessageRequest) (*MessageResponse, error)
    Stream(ctx context.Context, req *MessageRequest) (StreamReader, error)
    NormalizeModel(model string) string
    Capabilities() ProviderCapabilities
}
```

## 4.2 StreamReader

```go
type StreamReader interface {
    Next() (*StreamEvent, error)
    Close() error
}
```

## 4.3 Tool

```go
type Tool interface {
    Spec() ToolSpec
    Execute(ctx context.Context, input json.RawMessage, env ToolEnv) (*ToolResult, error)
}
```

## 4.4 Permission Engine

```go
type Engine interface {
    Decide(ctx context.Context, req PermissionRequest) (*PermissionDecision, error)
}
```

## 4.5 Session Store

```go
type Store interface {
    Create(ctx context.Context, session *Session) error
    Save(ctx context.Context, session *Session) error
    Load(ctx context.Context, id string) (*Session, error)
    List(ctx context.Context) ([]SessionSummary, error)
}
```

## 4.6 Compact Engine

```go
type CompactEngine interface {
    ShouldCompact(session *Session) bool
    Compact(ctx context.Context, session *Session, deps CompactDeps) (*CompactResult, error)
}
```

## 4.7 Prompt Builder

```go
type Builder interface {
    Build(ctx context.Context, in BuildInput) (*PromptBundle, error)
}
```

## 5. 启动顺序

`main.go` 启动建议顺序：

1. 解析 CLI 参数
2. 加载 config
3. 初始化 session store
4. 初始化 provider factory
5. 初始化 tool registry
6. 初始化 permission engine
7. 初始化 instruction collector
8. 初始化 prompt builder
9. 初始化 compact engine
10. 初始化 runtime engine
11. 初始化 command router
12. 进入 interactive 或 one-shot 模式

## 6. 首版必须先落地的文件

### P0 骨架

- `cmd/claw/main.go`
- `internal/app/app.go`
- `internal/config/*`
- `pkg/types/*`
- `internal/provider/provider.go`
- `internal/runtime/engine.go`
- `internal/session/model.go`
- `internal/tools/registry.go`
- `internal/permissions/mode.go`

### P1 核心闭环

- `internal/provider/anthropic/*`
- `internal/provider/openai/*`
- `internal/runtime/turn.go`
- `internal/runtime/tool_loop.go`
- `internal/tools/file/*`
- `internal/tools/bash/tool.go`
- `internal/tools/search/*`

### P2 可用性

- `internal/prompt/*`
- `internal/instructions/*`
- `internal/session/file_store.go`
- `internal/compact/*`
- `internal/commands/*`

## 6.5 里程碑入口/出口约束（骨架视角）

| Milestone | 入口条件 | 出口条件 |
| --- | --- | --- |
| M1 基础骨架与类型系统 | PRD、骨架、开发计划已冻结；目录命名和 provider/tool/session 抽象已达成一致 | `main -> app -> runtime` 可编译；核心接口命名冻结；依赖方向无环 |
| M2 Provider 打通 | M1 已稳定；统一 stream event 模型已确认 | Anthropic 与 OpenAI-compatible 都能映射到统一事件流；provider 切换不影响 runtime 接口 |
| M3 Runtime 核心闭环 | M2 可用；工具请求/结果内部数据结构已冻结 | 单轮多次 tool call 可闭环；tool trace / message trace / usage 已落盘 |
| M4-M5 工具与权限 | M3 闭环稳定；registry 与 permission engine 接口已定 | 所有首版工具必须经 registry + permission；危险路径存在一致拒绝/确认逻辑 |
| M6-M8 Session / Prompt / Compact | M5 完成；session 基础模型稳定 | resume/compact 不破坏任务连续性；prompt 组装与 compact 可测试 |
| M9-M11 CLI / 扩展发现 / 测试发布 | 前置里程碑完成，命令面和发布范围已冻结 | slash commands、skills/agents 发现、测试与 release gate 闭环完成 |

## 7. 明确约束

- 不允许先写复杂 UI，再补 runtime
- 不允许先做 plugin/MCP/LSP，再补核心闭环
- 不允许 provider 逻辑直接侵入 runtime 业务状态
- 不允许工具实现绕开统一 registry 和权限引擎

## 8. 建议测试目录

```text
internal/.../*_test.go
tests/integration/
tests/e2e/
testdata/demo-repo/
testdata/fixtures/
```

## 8.5 依赖关系与执行顺序补充

严格依赖链如下：

1. `pkg/types` / `internal/config` / `internal/session/model`
2. `internal/provider/provider.go` + `internal/provider/events.go`
3. `internal/tools/registry.go` + `internal/permissions/*`
4. `internal/runtime/*`
5. `internal/prompt/*` + `internal/instructions/*`
6. `internal/session/file_store.go` + `internal/compact/*`
7. `internal/commands/*` + `internal/cli/*`
8. `internal/skills/*` + `internal/agents/*`

约束说明：

- 4 不得早于 1-3，否则 runtime 会反向驱动抽象设计
- 5-6 不得在 runtime 未稳定前并行推进大改，否则 prompt/session/compact 会围绕不稳定接口返工
- 7 必须建立在 runtime、session、permission 基础能力已稳定可调用之上
- 8 只做发现与读取，不得反向要求改动 runtime 为 team/plugin 预埋复杂状态

## 9. 骨架验收标准

- 所有核心接口已稳定命名
- 包依赖方向无环
- `main -> app -> runtime` 链路可编译
- 新增工具或 provider 时不需要修改核心运行时状态机
