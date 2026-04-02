# Go 版 Claude Code v0.1 测试规格

## 1. 目标

本测试规格用于验证 Go 版 Claude Code v0.1 的核心闭环是否成立，重点覆盖：

- 双 provider 路径可用
- 单 agent tool loop 稳定
- 权限系统正确
- session / compact 可恢复长任务
- 指令文件与上下文装配正确

## 2. 测试分层

### 2.1 单元测试

覆盖纯逻辑与可隔离模块：

- `config`
- `permissions`
- `instructions`
- `prompt`
- `session`
- `compact`
- `provider` 请求/响应标准化
- 各 builtin tools

### 2.2 集成测试

覆盖跨模块流程：

- runtime + provider mock + tools
- session 持久化 + resume
- compact 触发与恢复
- slash command 与 runtime 交互

### 2.3 E2E 测试

覆盖用户真实使用路径：

- 交互模式
- one-shot 模式
- demo repo 编码修复任务

### 2.4 手工验证

覆盖难以稳定自动化的场景：

- 终端交互体验
- 流式输出观感
- 外部网络条件变化
- 长任务恢复体验

## 2.5 测试执行顺序

1. 单元测试先行，冻结接口与纯逻辑行为
2. 集成测试验证 provider 无关的 runtime 闭环
3. E2E 在 demo repo 上验证真实任务路径
4. 手工验证补齐终端体验、网络波动与权限确认等自动化盲区

**准入规则**
- 未通过对应层级前，不进入下一层级验收
- web 工具、skills/agents 相关测试不阻塞核心闭环主线，但在发布门槛前必须补齐范围内条目

## 2.6 Fixture / Mock 策略

### Provider
- Anthropic / OpenAI-compatible 均使用协议级 mock 响应，覆盖文本、stream、tool call、error、timeout
- 不依赖真实线上配额作为常规 CI 路径；真实 provider 验证留在发布前手工/受控集成阶段

### Tools
- 文件/搜索工具使用 `testdata/fixtures/` 中的稳定目录树与文本样本
- `bash` 使用可重复、无副作用命令；timeout 用可控阻塞命令或测试桩模拟
- `web_fetch` / `web_search` 使用本地测试服务器或录制响应，避免网络抖动污染 CI
- demo repo 固定在 `testdata/demo-repo/`，包含可预测失败测试、可修复缺陷与期望输出

### Session / Compact
- 长会话样本使用固定消息集与固定 tool trace 构造
- compact fallback 使用确定性输入，避免摘要断言依赖模型随机性

## 2.7 覆盖矩阵

| 能力域 | 单元 | 集成 | E2E | 手工 |
|---|---|---|---|---|
| config / permissions | 必须 | 可选补强 | 否 | 低权限提示抽检 |
| provider 标准化 | 必须 | 必须 | 发布前实链路 | 流式观感 |
| runtime tool loop | 否 | 必须 | 必须 | 否 |
| 文件/搜索/bash 工具 | 必须 | 必须 | 必须 | 否 |
| session / resume | 必须 | 必须 | 交互恢复 | 长任务恢复体验 |
| compact | 必须 | 必须 | 可选长任务 | 摘要可理解性 |
| slash commands | 单命令解析 | 必须 | 交互串联 | 终端体验 |
| web tools | 必须 | 建议 | 可选 | 网络波动 |

## 2.8 范围边界

- v0.1 非目标（plugin runtime、hooks、MCP、LSP、subagent/team、server/SSE API、browser automation）不纳入测试承诺
- skills / agents 仅验证本地发现与读取，不验证 team orchestration
- 外部真实网络仅用于发布前受控验证，不作为稳定 CI 依赖

## 3. 单元测试明细

## 3.1 Config

### 用例

- 读取用户配置
- 读取项目配置
- CLI 参数覆盖配置
- 缺省值生效
- 非法配置报错

### 断言

- 配置优先级符合 `CLI > project > user > default`
- 敏感字段打印时已脱敏

## 3.2 Permissions

### 用例

- `read-only` 下允许只读工具
- `read-only` 下拒绝写入工具与 shell
- `workspace-write` 下允许文件修改
- `danger-full-access` 下允许 shell
- 工具所需权限高于当前模式时返回拒绝

### 断言

- 权限拒绝信息含工具名、当前模式、需要模式

## 3.3 Instructions

### 用例

- 只存在单个 `AGENTS.md`
- 同时存在祖先目录与子目录 `AGENTS.md`
- 同时存在 `AGENTS.md` 与 `CLAW.md`
- 深层文件优先覆盖浅层文件

### 断言

- 收集顺序稳定
- 覆盖规则符合预期

## 3.4 Prompt Builder

### 用例

- 无历史消息时构造 prompt
- 有历史消息和 tools 时构造 prompt
- 有 instructions 时构造 prompt
- compact 后构造 prompt

### 断言

- 包含 system prompt、tools、历史消息、当前上下文
- compact 消息位于预期位置

## 3.5 Session

### 用例

- 新建 session
- 写入消息
- 序列化
- 反序列化
- resume 指定 session
- 版本字段不兼容时拒绝加载

### 断言

- session 文件格式稳定
- 时间戳、cwd、model、permission mode 正确保存

## 3.6 Compact

### 用例

- 未达到阈值不 compact
- 达到阈值触发 compact
- compact 模型调用失败时走降级摘要
- compact 后保留最近若干条消息

### 断言

- compact 输出包含：
  - 当前目标
  - 已完成工作
  - 未完成工作
  - 关键文件
  - 下一步建议
- compact 后 token 估算下降

## 3.7 Provider: Anthropic

### 用例

- 普通文本响应
- 流式文本响应
- 工具调用响应
- usage 统计
- 认证缺失
- base URL 覆盖

### 断言

- 能标准化成统一内部事件流
- tool call 字段完整

## 3.8 Provider: OpenAI-compatible

### 用例

- 普通文本响应
- 流式 chat completion
- tool call 响应
- usage 统计
- base URL 覆盖
- 鉴权缺失报错

### 断言

- 能标准化成与 Anthropic 同一内部事件流
- runtime 无需区分 provider 分支

## 3.9 Tools

### `read_file`

- 读取完整文件
- 按 offset/limit 读取
- 文件不存在

### `write_file`

- 写入新文件
- 覆盖旧文件
- 超出 workspace 拒绝

### `edit_file`

- 单次替换成功
- 替换目标不存在
- `replace_all=true` 成功

### `glob_search`

- 匹配多个文件
- 指定 path 搜索

### `grep_search`

- 命中文本
- 无结果
- 带 context 参数

### `bash`

- 命令成功
- 命令失败
- timeout

### `web_fetch`

- 拉取 html 页面
- 拉取纯文本页面
- 网络失败

### `web_search`

- 默认搜索返回结果
- `allowed_domains` 生效
- `blocked_domains` 生效
- 去重和截断生效

### `todo_write`

- 初始化 todo
- 更新 todo 状态

## 4. 集成测试明细

## 4.1 Runtime 基础闭环

### 场景

- 模型请求 `read_file`
- runtime 执行后回填
- 模型继续请求 `grep_search`
- 最终输出总结

### 断言

- 多次 tool call 顺序正确
- 消息历史保存完整

## 4.2 Runtime + 编辑 + Bash

### 场景

- 模型读文件
- 模型 edit 文件
- 模型运行测试命令
- 根据测试结果继续修复

### 断言

- tool trace 连贯
- stderr 可作为下一轮输入

## 4.3 Provider 无关性

### 场景

- 相同 mock task 分别走 Anthropic / OpenAI-compatible

### 断言

- runtime 侧逻辑无分叉
- 最终都能完成工具闭环

## 4.4 Session Resume

### 场景

- 运行到中间退出
- 重新 resume
- 继续同一任务

### 断言

- 未丢失 todo、messages、tool results

## 4.5 Compact 后恢复

### 场景

- 构造长会话
- 触发 compact
- 继续后续任务

### 断言

- 模型仍能回答当前进度
- 仍能继续调用工具完成任务

## 4.6 Slash Commands

### 场景

- `/help`
- `/status`
- `/model`
- `/permissions`
- `/resume`
- `/session`

### 断言

- 不破坏当前交互状态
- usage/错误提示正确

## 5. E2E 测试明细

## 5.1 One-shot 模式

### 场景

- 在 demo repo 中执行一个包含 `read_file + grep_search + bash` 的任务

### 断言

- 命令可返回最终答案
- 退出码符合预期

## 5.2 Interactive 模式

### 场景

- 连续三轮以上任务
- 中途使用 slash command
- 最后恢复 session

### 断言

- 交互状态稳定

## 5.3 真实修复任务

### 场景

- 在 demo repo 中故意放入一个失败测试
- 让 agent 搜索、修复、运行测试、确认通过

### 断言

- 完成“读 -> 改 -> 测 -> 再改 -> 通过”的闭环

## 6. 手工验证

### 手工场景 1：流式输出

- 检查流式响应是否可读
- 检查工具执行摘要是否清楚
- 验证 Anthropic / OpenAI-compatible 两条链路的文本流与 tool use 反馈风格均可接受

### 手工场景 2：权限体验

- 在低权限模式执行危险操作，检查提示是否明确
- 验证拒绝后仍可继续当前会话，不破坏消息状态

### 手工场景 3：长会话 compact

- 检查 compact 前后用户理解成本是否可接受
- 验证 compact 后继续执行 1 个真实任务，不出现关键上下文丢失

### 手工场景 4：网络波动

- 检查 provider 重试与错误展示
- 检查超时、取消、鉴权缺失时的错误是否可区分

### 手工场景 5：resume 闭环

- 中断 interactive 会话后恢复
- 检查 todo、tool trace、messages、permission mode 是否完整恢复

## 7. 发布门槛

v0.1 对外可用前，至少满足：

- 单元测试通过
- 核心集成测试通过
- 至少 1 个 demo repo E2E 通过
- Anthropic 与 OpenAI-compatible 两条 provider 路径都至少验证一次完整 tool loop
- compact 与 resume 场景验证通过
- 手工验证清单全部完成且无 P0/P1 阻塞问题

## 8. 发布阻塞 / 不阻塞判定

### 阻塞发布
- runtime 无法稳定完成多 tool loop
- provider 任一路径无法完成基础文本 + tool use 闭环
- session/resume/compact 任一核心能力不可用
- 权限拒绝可被绕过或危险工具默认放行
- demo repo E2E 无法形成“读 -> 改 -> 测 -> 通过”闭环

### 可记录但不阻塞发布
- 流式输出观感可优化但不影响正确性
- web 工具性能或结果排序存在轻微波动
- skills / agents 发现的输出格式仍可继续打磨，但已满足本地读取能力

## 9. 验证结果记录模板

每个测试阶段记录：

- `Check`: 用例/场景名
- `Type`: unit / integration / e2e / manual
- `Status`: PASS / FAIL
- `Evidence`: 命令、日志、截图或输出摘要
- `Blocking`: yes / no
- `Notes`: 缺陷定位、重试信息、后续动作
