# Go 版 Claude Code 核心 PRD（v0.1）

## 1. 文档信息

- 文档目标：定义 Go 版 Claude Code 第一阶段（v0.1）产品边界、核心能力、整体架构、模块设计、实现约束与验收标准。
- 产品定位：本地优先、单机优先、单 agent 优先的 coding-agent CLI/runtime。
- 参考对象：
  - `../claudeCode/claw-rust`：作为已验证的裁剪版实现参考。
  - `../claudeCode/claude-code-source-code`：作为完整产品面参考，但不是 v0.1 的直接目标。

## 2. 背景与问题定义

Claude Code 真正的核心不是复杂 TUI、遥测、远程控制或多 agent 平台，而是一个能在真实代码仓库中稳定完成以下闭环的本地 agent runtime：

1. 读取仓库上下文与指令文件。
2. 根据用户任务调用模型。
3. 在模型决定时调用本地工具。
4. 将工具结果回填给模型继续推理。
5. 持久化会话并在长任务中恢复工作状态。
6. 在权限边界内完成代码修改与验证。

本项目目标不是复刻 TypeScript 版的所有产品功能，而是以 Go 实现一个更易迭代、发布简单、工程心智负担更低的核心 runtime。

## 3. 产品定位

### 3.1 一句话定义

一个带权限控制、会话记忆、工具执行和项目上下文装配能力的本地 coding-agent CLI。

### 3.2 目标用户

- 日常在本地仓库中工作的开发者
- 想把 LLM 从“聊天”提升为“可执行编码代理”的高级用户
- 希望通过 CLI/自动化方式完成代码阅读、修改、测试与修复的人

### 3.3 产品原则

1. 本地优先：核心流程不依赖远程控制平台。
2. 单机闭环优先：先解决单用户、单仓库、单 agent 的真实可用性。
3. 核心优先：先做 agent loop、工具、权限、会话，不做外围产品功能。
4. 清晰边界：所有可执行能力必须有显式权限模型。
5. 结构可扩展：v0.1 设计必须为插件、MCP、LSP、subagent 留出演进接口。

## 4. 产品目标与非目标

### 4.1 v0.1 产品目标

- 提供稳定的交互式 CLI 和 one-shot prompt 模式。
- 提供完整的单 agent tool loop。
- 提供最小但足够实用的本地工具集。
- 提供会话持久化、恢复与基础 compact 能力。
- 提供项目级指令文件加载能力（`AGENTS.md` / `CLAW.md`）。
- 提供基础 slash command 操作面。
- 提供明确的权限与执行边界。

### 4.2 v0.1 非目标

- 不追求 TypeScript 版命令/工具数量对齐。
- 不实现 voice、mobile、bridge、remote control、managed settings。
- 不实现 analytics/telemetry/growthbook/datadog。
- 不实现 team orchestration、复杂 subagent runtime。
- 不实现完整 plugin marketplace。
- 不实现完整 MCP 管理 UI。
- 不实现复杂 TUI 或 React/Ink 风格界面。

## 5. 成功标准

v0.1 发布时，应满足以下结果：

1. 在真实仓库内可完成“读代码 -> 改代码 -> 跑命令 -> 根据结果继续修”的闭环。
2. 能在多次工具调用下持续完成一个任务，而不是单轮问答。
3. 能保存并恢复会话。
4. 能在 `read-only` / `workspace-write` / `danger-full-access` 三种权限模式下工作。
5. 能从 `AGENTS.md` / `CLAW.md` 与会话历史构造稳定 prompt。

## 6. 竞品/参考实现取舍结论

### 6.1 TypeScript 原始版本的意义

TS 版代表“完整产品面”，包含大量外围能力：

- 大量 slash commands
- 大量 feature-gated tools
- analytics / policy / remote managed settings
- voice / bridge / remote / plugin/product integrations

这些能力说明长期上限，但不适合作为 v0.1 直接范围。

### 6.2 Rust 版本的意义

Rust 版代表“做过取舍后的核心能力集合”，更适合作为 Go 版起点：

- 已保留核心 runtime
- 已保留基础命令与基础工具
- 已保留 skills / agents / session / compact / config / permission
- 已为 hooks / plugins / LSP / MCP 留出实现空间

### 6.3 Go 版策略

Go 版采用“Rust 的产品边界 + Go 的实现效率”：

- 功能范围参考 Rust 的取舍
- 结构清晰度和迭代速度优先
- 后续再按优先级追加插件、MCP、LSP、server

## 7. 总体架构

### 7.1 架构总览

```text
User / CLI
   |
   v
Command Layer
   |
   v
Session + Prompt Builder
   |
   v
Conversation Runtime
   |
   +--> Provider Client
   |
   +--> Tool Registry --> Tool Executor --> OS / FS / Network
   |
   +--> Permission Engine
   |
   +--> Session Store / Compact Engine
```

### 7.2 架构分层

#### A. CLI 层

负责：

- 交互模式
- one-shot prompt 模式
- slash commands
- 用户输入/输出

约束：

- 不承担核心业务逻辑
- 只做参数解析、模式切换、输出渲染

#### B. Runtime 层

负责：

- 会话驱动
- 模型调用
- tool loop
- turn 状态管理
- 错误恢复

约束：

- runtime 必须独立于 CLI，可被未来 server/API 复用

#### C. Tool 层

负责：

- 注册 builtin tools
- 参数校验
- 权限映射
- 执行结果标准化

约束：

- 所有工具必须有 schema
- 所有工具必须声明权限级别
- 所有工具输出必须统一为结构化结果

#### D. Context 层

负责：

- 读取 `AGENTS.md` / `CLAW.md`
- 读取工作目录信息
- 整理 system prompt / tool spec / 会话历史

约束：

- context 构造必须可测试
- 指令文件优先级规则必须确定

#### E. Session 层

负责：

- 保存历史
- 恢复历史
- 导出历史
- compact

约束：

- 存储格式必须稳定、可迁移
- session 文件必须可离线检查

#### F. Permission 层

负责：

- 工具权限校验
- 执行前确认
- 模式切换

约束：

- 默认行为必须保守
- 所有危险工具必须走统一入口

## 8. 建议代码组织

```text
cmd/claw/
internal/cli/
internal/commands/
internal/runtime/
internal/provider/
internal/provider/anthropic/
internal/provider/openai/
internal/provider/shared/
internal/prompt/
internal/context/
internal/tools/
internal/tools/bash/
internal/tools/file/
internal/tools/search/
internal/tools/web/
internal/tools/todo/
internal/session/
internal/compact/
internal/permissions/
internal/config/
internal/instructions/
internal/skills/
internal/agents/
pkg/types/
```

### 8.1 模块职责

- `cmd/claw`：主入口
- `internal/cli`：交互循环、prompt 模式、输出渲染
- `internal/commands`：slash command 解析与处理
- `internal/runtime`：conversation loop、turn execution、tool orchestration
- `internal/provider`：模型 provider 接口与实现
- `internal/prompt`：prompt 组装
- `internal/context`：仓库上下文、指令文件、环境信息
- `internal/tools`：工具注册表、schema、执行入口
- `internal/session`：session 持久化与恢复
- `internal/compact`：上下文压缩
- `internal/permissions`：权限决策
- `internal/config`：配置加载与解析
- `internal/instructions`：`AGENTS.md` / `CLAW.md` 收集与优先级处理
- `internal/skills`：本地 `SKILL.md` 发现与读取
- `internal/agents`：本地 agent 定义发现，不包含 team runtime

## 9. v0.1 功能清单与详细设计

## 9.1 功能一：交互式 CLI

### 目标

提供可持续对话的本地命令行入口。

### 用户价值

用户可以在同一个会话里连续完成多轮编码任务。

### 设计

- 支持启动后进入交互模式
- 支持读取用户输入
- 支持显示流式模型输出
- 支持 slash commands
- 支持显示工具执行摘要

### 实现描述

- 使用 `bufio.Reader` 或等价输入循环读取用户输入
- 将普通输入交给 runtime
- 将 `/xxx` 输入交给 command router
- 输出层先做纯文本渲染，不实现富组件系统

### 约束

- 首版不实现复杂终端 UI
- 首版不支持多窗格/多会话并发交互

### 验收标准

- 可正常进入和退出交互模式
- 连续 10 轮输入后 session 状态仍正确
- slash command 与普通 prompt 路径互不冲突

## 9.2 功能二：One-shot Prompt 模式

### 目标

支持单次命令执行任务，适合集成到 shell workflow。

### 设计

- `claw prompt "..."` 或 `claw -p "..."` 风格
- 输出最终结果并退出

### 实现描述

- CLI 将用户输入包装为一个新 session
- runtime 走同样的 tool loop
- 运行完成后打印最终 answer

### 约束

- 与交互模式共用同一 runtime，不允许分叉两套实现

### 验收标准

- one-shot 与 interactive 调用同一业务核心
- one-shot 模式能完成至少一次多 tool task

## 9.3 功能三：Conversation Runtime / Agent Loop

### 目标

构建系统最核心的执行闭环。

### 设计

单轮流程：

1. 构造 prompt
2. 请求模型
3. 若返回文本，则输出
4. 若返回 tool use，则执行工具
5. 将 tool result 写回会话
6. 继续下一轮，直到完成

### 实现描述

- 维护统一的 `TurnExecutor`
- provider 返回统一事件流：
  - text delta
  - tool request
  - final message
  - usage
- runtime 根据事件驱动状态机推进

### 状态机建议

- `idle`
- `requesting_model`
- `streaming_response`
- `executing_tool`
- `awaiting_next_turn`
- `completed`
- `failed`

### 约束

- 不允许工具执行逻辑散落在 CLI 层
- runtime 必须可在未来被 server 模式复用
- 所有轮次必须可追踪 usage、tool trace、message trace

### 验收标准

- 支持连续多次 tool call
- tool 结果可以正确回填给下一次模型调用
- 遇到工具失败时，runtime 不崩溃，可给模型或用户错误反馈

## 9.4 功能四：Provider 抽象

### 目标

隔离模型接口差异，降低后续扩展成本。

### 设计

定义统一接口：

- `Send(request) -> response/stream`
- `ListModels()`
- `NormalizeToolSchema()`

### 首版范围

- 首版同时支持两个 provider 家族：
  - Claude / Anthropic 原生 messages 接口
  - OpenAI-compatible chat completions 接口
- 运行时允许通过配置、模型名或环境变量选择 provider

### 实现描述

- `provider.Provider` 接口统一抽象：
  - `Send(request)`
  - `Stream(request)`
  - `NormalizeModel(model)`
  - `Capabilities()`
- `provider/anthropic`：
  - 对接 Claude / Anthropic 原生 messages API
  - 支持原生 tool use 协议
  - 支持 SSE/streaming
- `provider/openai`：
  - 对接 OpenAI-compatible chat completions
  - 将 OpenAI tool call/stream delta 标准化成统一内部事件
  - 支持自定义 base URL，兼容 OpenAI-compatible 服务
- `provider/shared`：
  - 放置统一 request/response 类型、stream event、重试策略、错误规范化逻辑

### 约束

- provider 层不能感知本地 session 存储细节
- provider 层不处理权限逻辑
- provider 层必须输出统一内部事件模型，runtime 不关心上游协议差异
- provider 切换不能改变工具注册与权限判断方式

### 验收标准

- 能完成文本流式输出
- 能完成工具调用协议
- 错误、超时、取消语义一致
- 相同任务在 Anthropic 与 OpenAI-compatible 路径上都能完成基础 tool loop

## 9.5 功能五：基础工具系统

### 目标

为 agent 提供可执行能力。

### 首版工具清单

1. `bash`
2. `read_file`
3. `write_file`
4. `edit_file`
5. `glob_search`
6. `grep_search`
7. `web_fetch`
8. `web_search`
9. `todo_write`

### 设计

每个工具都包含：

- 名称
- 描述
- 输入 schema
- 权限级别
- 执行函数

### 实现描述

- `ToolSpec` 描述 schema 与权限
- `ToolExecutor` 负责统一执行
- tool 输出统一序列化为 JSON 结构

### 约束

- 所有工具必须可被 provider 暴露为标准 tool definition
- 不允许 ad-hoc tool
- 工具输入必须校验

### 验收标准

- 每个工具都有单元测试
- 至少有一个集成测试覆盖多工具串联场景

## 9.6 功能六：Shell/Bash 工具

### 目标

让 agent 能执行仓库相关命令，如测试、构建、grep、git 状态查看。

### 设计

- 接收命令字符串
- 可设置 timeout
- 返回 stdout/stderr/exit code

### 实现描述

- 基于 `os/exec`
- 绑定工作目录
- 透传环境变量白名单
- 支持取消与超时

### 约束

- 默认受权限模式约束
- 首版不做后台任务管理器
- 首版不做复杂 shell 语法分析

### 验收标准

- 能执行常见构建/测试命令
- timeout 生效
- 错误输出可回填模型

## 9.7 功能七：文件读写编辑工具

### 目标

让 agent 能查看与修改工作区文件。

### 设计

- `read_file`：支持 offset/limit
- `write_file`：整文件写入
- `edit_file`：基于 old/new string 的局部替换

### 实现描述

- 文件路径统一走 workspace 安全解析
- `edit_file` 先实现精确字符串替换
- 后续版本再考虑 patch/AST edit

### 约束

- 不允许越过 workspace root
- 默认使用文本模式
- 二进制文件不作为首版支持对象

### 验收标准

- 能稳定修改文本文件
- 文件不存在/权限不足/替换失败时错误信息清楚

## 9.8 功能八：搜索工具

### 目标

提升 agent 在大型仓库中的定位能力。

### 设计

- `glob_search`：找文件
- `grep_search`：找文本

### 实现描述

- 优先使用 Go 原生实现
- 若需要性能优化，可后续接 ripgrep

### 约束

- 首版优先正确性与一致返回结构
- 不做复杂索引系统

### 验收标准

- 中型仓库内搜索结果结构可被模型稳定消费

## 9.9 功能九：Web 工具

### 目标

让 agent 获取当前文档或网页信息。

### 设计

- `web_fetch`：拉取指定 URL 内容
- `web_search`：执行简单 web search

### 实现描述

- `web_fetch` 先用 HTTP GET + 文本抽取
- `web_search` 参考 Rust 版实现：
  - 默认使用 HTML 搜索结果页拉取方式
  - 支持通过环境变量覆盖搜索入口
  - 从 HTML 中抽取结果链接与标题
  - 支持 `allowed_domains` / `blocked_domains`
  - 去重、截断结果数量
  - 返回“说明文本 + 结构化搜索结果”双层输出

### 约束

- 必须受网络与权限边界控制
- 结果必须裁剪，避免长网页直接灌入上下文
- `web_search` 首版不接浏览器自动化，不做 JS 渲染
- 搜索实现必须可替换，不能把特定搜索站点硬编码进业务层

### 验收标准

- 能获取文本结果
- 网络失败与超时可恢复
- `allowed_domains` / `blocked_domains` 过滤行为可测试
- 搜索结果输出能稳定生成来源列表

## 9.10 功能十：Todo / 任务状态工具

### 目标

让模型在长任务中显式维护计划状态。

### 设计

- 维护当前 session 的 todo 列表
- 状态：`pending` / `in_progress` / `completed`

### 实现描述

- session 内保存结构化 todo
- 输出层可选择性显示摘要

### 约束

- 首版只做 session 级 todo，不做全局任务系统

### 验收标准

- 多轮任务中 todo 状态保持一致

## 9.11 功能十一：权限系统

### 目标

对工具执行施加明确约束。

### 模式

- `read-only`
- `workspace-write`
- `danger-full-access`

### 设计

- 每个工具声明最低权限
- runtime 在执行前判断当前模式是否允许
- 对危险工具支持确认机制

### 实现描述

- `PermissionMode`
- `PermissionEngine`
- `PermissionDecision`

### 约束

- 所有工具都必须声明权限等级
- 权限决策必须统一，不允许每个工具各自处理

### 验收标准

- 在不同权限模式下行为一致且可预期
- 危险命令不会在低权限模式下直接执行

## 9.12 功能十二：Session 持久化与恢复

### 目标

支持长任务断点续做。

### 设计

- 存储消息历史
- 存储 tool result
- 存储 todo
- 存储 metadata

### 实现描述

- 使用 JSON 文件作为首版格式
- 每个 session 一个文件
- 提供 list/get/resume/export

### 约束

- 首版不引入数据库
- 文件格式必须显式 version 字段

### 验收标准

- 中断后可恢复
- 导出内容完整

## 9.13 功能十三：Compact

### 目标

在长会话中控制上下文长度。

### 设计

- 当历史超过阈值时，压缩旧消息
- 保留最近若干轮原始消息
- 生成摘要前缀消息

### 实现描述

- 首版直接采用模型辅助式 compact：
  - 选取“较早消息窗口 + 当前 todo + 最近关键工具结果”
  - 调用 compact summarizer prompt
  - 生成结构化摘要，至少包含：
    - 目标与当前任务
    - 已完成工作
    - 未完成工作
    - 关键文件/路径
    - 失败尝试与注意事项
    - 后续建议动作
  - 将摘要写入 session 作为前缀消息
- 当 compact 模型调用失败时，允许降级到最小规则式摘要，以避免 session 完全不可继续

### 约束

- compact 后不能破坏最近上下文
- 必须保留关键任务状态与未完成工作
- compact prompt 必须与普通任务 prompt 隔离，避免污染主对话
- compact 结果必须带来源 metadata，标记由哪次 compact 生成

### 验收标准

- compact 后任务仍能继续
- session token 估算显著下降
- compact 后模型仍能回答“当前做到哪一步”与“接下来该做什么”

## 9.14 功能十四：Prompt 构建与指令文件加载

### 目标

把仓库规则、系统规则、工具能力、会话状态稳定装配成 prompt。

### 设计

prompt 组成：

- system prompt
- 指令文件内容（`AGENTS.md` / `CLAW.md`）
- 工具定义
- 会话历史
- 当前任务

### 实现描述

- 实现 `InstructionCollector`
- 自下而上收集当前目录及祖先目录的指令文件
- 深层目录优先覆盖浅层目录

### 约束

- 指令文件优先级规则必须固定
- prompt builder 不直接读取 CLI 参数，全由调用者传入上下文

### 验收标准

- 多层目录存在多个 `AGENTS.md` 时优先级正确
- 不会漏读当前仓库重要指令

## 9.15 功能十五：基础 Slash Commands

### 首版命令

- `/help`
- `/status`
- `/model`
- `/permissions`
- `/clear`
- `/resume`
- `/config`
- `/memory`
- `/init`
- `/diff`
- `/session`
- `/export`

### 设计

- 命令仅作为运行时操作入口
- 命令实现与 runtime 解耦

### 约束

- 首版不做 20+ 命令扩张
- 不做 PR/GitHub 自动化

### 验收标准

- 命令行为可预测
- 非法参数有明确 usage

## 9.16 功能十六：Skills 与 Agents 发现

### 目标

保留本地扩展入口，但不实现完整运行平台。

### 设计

- 发现 `.codex/skills`、`.claw/skills` 等目录
- 读取 `SKILL.md`
- 发现 `.codex/agents`、`.claw/agents`

### 实现描述

- 首版只实现“列出/读取”
- 不实现复杂 worker/team runtime

### 约束

- skills/agents 只作为上下文与发现能力，不作为完整插件框架替代

### 验收标准

- 能稳定列出项目级与用户级 skills/agents

## 10. 配置设计

### 10.1 配置来源

- 环境变量
- 用户级配置
- 项目级配置
- CLI 参数

### 10.2 优先级

`CLI 参数 > 项目配置 > 用户配置 > 环境默认值`

### 10.3 首版配置项

- provider 类型
- model
- api base url
- api key 环境变量名
- permission mode
- session store 路径
- compact 阈值
- web tool 开关

### 10.4 约束

- 所有配置必须可打印与调试
- 敏感信息不直接明文输出

## 11. 数据模型建议

### 11.1 Session

- `id`
- `version`
- `created_at`
- `updated_at`
- `cwd`
- `messages`
- `todos`
- `model`
- `permission_mode`
- `usage`

### 11.2 Message

- `role`
- `blocks`
- `timestamp`

### 11.3 Tool Call

- `tool_name`
- `input`
- `output`
- `status`
- `duration_ms`

### 11.4 约束

- 所有持久化结构必须带 `version`
- 必须允许未来迁移

## 12. 错误处理与恢复策略

### 原则

- 不因单次工具失败导致整个 CLI 崩溃
- 尽可能向模型或用户返回结构化错误

### 分类

- provider 错误
- tool 执行错误
- 权限拒绝
- session 读写错误
- 配置错误
- 上下文构建错误

### 处理策略

- provider 错误：提示重试/检查配置
- tool 错误：以 tool result 形式回填
- 权限拒绝：明确指出被拒绝原因
- session 错误：降级为内存会话或终止并提示

## 13. 安全与约束

### 13.1 路径安全

- 所有文件工具都必须限制在 workspace root

### 13.2 命令执行安全

- shell 工具必须记录 cwd、timeout、exit code

### 13.3 网络安全

- web 工具必须可被总开关关闭

### 13.4 Prompt 安全

- 指令文件与模型输出不能绕过本地权限引擎

### 13.5 首版明确不做

- 沙箱虚拟化
- 远程策略下发
- 复杂审计后台

## 14. 测试策略

详细测试规格另见 `/.omx/plans/test-spec-go-claude-code-v0.1.md`。

### 14.1 单元测试

- config 加载
- permission 判断
- session 序列化
- prompt builder
- 每个工具执行器

### 14.2 集成测试

- 一次多工具对话
- session 恢复
- compact 后继续任务
- 权限拒绝路径

### 14.3 手工验证场景

1. 在 demo repo 中要求 agent 搜索一个函数并修改实现
2. 运行测试并根据失败继续修复
3. 中断并恢复 session
4. 切换 permission mode 后验证工具行为变化

## 15. 里程碑与开发顺序

### M1：骨架搭建

- CLI 入口
- config
- types
- provider 抽象
- session 基础结构

### M2：核心闭环

- runtime loop
- basic prompt builder
- read/write/edit/bash/search tools
- permission engine

### M3：可用性补齐

- session save/resume/export
- compact
- basic slash commands
- todo tool

### M4：上下文增强

- `AGENTS.md` / `CLAW.md` 收集
- skills/agents 发现
- web tools

### M5：发布前打磨

- 错误处理完善
- 集成测试
- 文档与示例

## 16. v0.2 以后预留方向

- hooks
- plugins
- MCP
- LSP
- git 工作流命令
- subagent
- server / SSE session API

## 17. 风险与缓解

### 风险一：过早扩功能导致核心不稳

- 缓解：严格按核心闭环优先，不提前做外围能力

### 风险二：provider/tool 协议抽象不稳

- 缓解：先统一内部事件模型，再接 provider

### 风险三：上下文过长导致性能下降

- 缓解：尽早实现 compact 与 read_file limit

### 风险四：权限模型绕不过去

- 缓解：所有工具统一走 registry + permission engine

## 18. 最终验收标准（Release Gate）

v0.1 只有同时满足以下条件才可对外宣称“首版可用”：

1. 交互模式可稳定使用。
2. one-shot 模式可完成多工具任务。
3. 核心工具全部可用并有测试。
4. session 可恢复。
5. compact 不会破坏基本任务连续性。
6. `AGENTS.md` / `CLAW.md` 规则可正确进入 prompt。
7. 三种权限模式行为可验证。
8. 至少在一个真实 demo repo 中完成完整编码任务闭环。

## 19. ADR

### Decision

Go 版 v0.1 采用“单 agent、本地优先、核心闭环优先”的产品策略。

### Drivers

- 需要快速做出可工作的 coding-agent runtime
- 需要控制范围，避免陷入 TS 全量复刻
- 需要为未来插件/MCP/LSP 扩展留接口

### Alternatives Considered

- 直接追 TS 全量功能：范围过大，风险过高
- 只做极简聊天 CLI：无法体现 Claude Code 核心价值
- 先做复杂 TUI：投入大但不提升核心可用性

### Why Chosen

该方案最能平衡首版可交付性、结构清晰度与未来扩展空间。

### Consequences

- 首版功能会少于 TS 原版
- 但核心闭环会更快成熟
- 后续扩展路线更清楚

### Follow-ups

- v0.1 完成后评估 hooks/plugins/MCP 的优先级
- 基于真实使用数据决定是否引入更复杂的执行模型
