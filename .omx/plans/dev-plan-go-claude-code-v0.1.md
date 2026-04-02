# Go 版 Claude Code v0.1 开发任务拆解

## 1. 目标

把 PRD 拆成可执行开发任务，确保实施顺序稳定，避免同时铺太多面。

## 2. 开发原则

- 先打通闭环，再补外围
- 先有接口和测试点，再实现细节
- 先 Anthropic + OpenAI-compatible 双 provider 抽象，再接工具
- compact 按“模型辅助 + 降级 fallback”一次设计好

## 3. Milestone 规划

### 3.1 Milestone 入口/出口、依赖与 DoD 约束

| Milestone | 依赖 | 入口条件 | 出口条件 / DoD |
| --- | --- | --- | --- |
| M1 基础骨架与类型系统 | 无 | PRD / skeleton / dev-plan / test-spec 已对齐；目录与命名方案冻结 | Go module、目录骨架、核心类型、provider/tool/permission/session 抽象可编译；依赖方向无环；至少有骨架级 smoke test |
| M2 Provider 打通 | M1 | 统一 stream event、provider capabilities、配置字段已冻结 | Anthropic 与 OpenAI-compatible 都能输出统一事件流；鉴权、base URL、错误语义可测试；provider mock 可复用 |
| M3 Runtime 核心闭环 | M2 | tool request/result 结构已冻结；session message model 已稳定 | 单轮支持多次 tool call；tool trace/message trace/usage 可落盘；工具失败可回填模型或用户 |
| M4 基础工具集 | M3 | registry、validation、workspace path guard、permission hooks 已可用 | read/write/edit/glob/grep/bash/todo/web_fetch/web_search 全部走统一 registry + schema + permission；文件/搜索/命令工具至少各有一条集成路径 |
| M5 权限系统 | M4 | 工具权限分级表已明确；CLI/interactive 确认交互策略已定 | 三种权限模式行为一致；危险工具拒绝/确认逻辑稳定；回归验证不出现绕过入口 |
| M6 Session / Resume / Export | M3, M5 | session JSON 结构与 version 策略已冻结 | save/load/list/export 可用；resume 不丢 messages/todos/tool results；损坏/版本不兼容时有清晰错误 |
| M7 Prompt / Instructions / Context | M3, M6 | instruction 优先级规则、prompt bundle 结构已确认 | `AGENTS.md` / `CLAW.md` 收集稳定；prompt 构造可测试；workspace/context 注入不依赖 CLI 细节 |
| M8 Compact | M6, M7 | compact prompt、阈值、fallback 规则已确认 | 长会话 compact 后仍能继续任务；摘要包含目标/已完成/未完成/关键文件/下一步；fallback 可用 |
| M9 CLI + Slash Commands | M3, M5, M6, M7 | runtime/app/session/config 接口稳定 | interactive、one-shot、首版 slash commands 路径齐全；命令不绕开 runtime/session/permission |
| M10 Skills / Agents 发现 | M7, M9 | 本地路径发现规则已确认 | 可列出/读取 skills 与 agents；不引入 team/plugin runtime 复杂度 |
| M11 测试与发布打磨 | M1-M10 | 测试矩阵、demo repo、release gate 已冻结 | 单元/集成/E2E/手工验证都达标；双 provider 路径通过；发布门槛满足 |

## 3.2 风险寄存器

| 风险 | 影响阶段 | 触发信号 | 缓解动作 |
| --- | --- | --- | --- |
| provider 事件归一化不稳 | M2-M3 | runtime 里出现 provider 分支判断 | 先冻结内部事件模型，再并行接入双 provider |
| runtime 状态机过早绑定 CLI 交互 | M3-M9 | slash command/流式输出逻辑侵入 runtime | 明确 `runtime` 仅输出结构化事件，渲染留在 CLI |
| 工具实现绕过权限入口 | M4-M5 | 单个工具自己做 allow/deny 决策 | 所有执行统一进入 registry + permission engine |
| session 结构膨胀影响 compact | M6-M8 | trace 冗余、resume 变慢 | 提前定义可裁剪字段与 compact 元数据 |
| web/search 细节拖慢主线 | M4/M9 | 核心闭环未通却花时间在 HTML 抽取 | 把 web tools 排到 CLI 基本完成后再做 |
| v0.1 范围膨胀 | 全阶段 | 提前引入 hooks/MCP/team runtime | 以 PRD 非目标和发布门槛作为变更拦截线 |

## M1：基础骨架与类型系统

**入口条件**
- PRD、骨架定义与测试规格已审阅
- 目录骨架与包边界已冻结到 v0.1 范围

**任务**

1. 初始化 Go module 与目录骨架
2. 定义公共类型：
   - message
   - tool
   - session
   - usage
3. 定义 config 类型与加载器
4. 定义 provider 抽象接口
5. 定义 tool registry 抽象
6. 定义 permission engine 抽象
7. 定义 session store 抽象

**依赖**
- 无前置实现依赖，是所有后续 milestone 的基座

**交付**

- 项目可编译
- 核心接口命名稳定

**退出条件 / DoD**
- `main -> app -> runtime` 最小链路可编译
- 核心接口签名在后续 M2-M4 中无需破坏性调整
- 包依赖方向满足骨架文档约束，无环依赖
- 对应类型与加载路径具备最小单元测试壳

**主要风险**
- 过早细化接口导致后续 runtime/provider 返工
- 包边界不清导致 runtime、provider、tools 相互渗透

### M2：Provider 打通

**入口条件**
- M1 完成并冻结 provider 抽象、共享事件类型、配置入口

**任务**

1. 实现 Anthropic provider
2. 实现 OpenAI-compatible provider
3. 统一 stream event 模型
4. 完成模型名、base URL、API key 加载逻辑
5. 为两条 provider 路径补 mock 测试

**依赖**
- 依赖 M1 的 provider 接口、config 类型、共享 message/tool 类型

**交付**

- 两条 provider 路径都能返回统一事件流

**退出条件 / DoD**
- Anthropic / OpenAI-compatible 都能输出统一文本流、tool call、usage 事件
- provider 切换不影响 runtime 调用方式
- 鉴权缺失、超时、取消、base URL 覆盖均有明确错误语义
- provider mock 覆盖基础文本与工具调用路径

**主要风险**
- 两类协议归一化不完整，导致 runtime 仍需分支判断
- streaming 事件不稳定，影响 tool loop 可恢复性

### M3：Runtime 核心闭环

**入口条件**
- M2 已提供可消费的统一 provider 事件流
- M1 的 session/tool/permission 抽象可被 runtime 引用

**任务**

1. 实现 `runtime.Engine`
2. 实现 turn 状态机
3. 实现 tool request -> execute -> append tool result
4. 实现最终响应输出
5. 记录 usage、tool trace、message trace

**依赖**
- 依赖 M1、M2
- 为 M4-M9 提供统一执行主干

**交付**

- 能完成多次 tool call 的单任务闭环

**退出条件 / DoD**
- 单轮与多轮 tool loop 均可跑通
- 工具失败不会导致 runtime 崩溃，错误可回填模型或输出给用户
- usage、tool trace、message trace 在 session 中可持久化
- 具备至少 1 条集成测试覆盖“读 -> 工具 -> 回填 -> 完成”路径

**主要风险**
- 状态机与 streaming 耦合过深，导致恢复与 compact 难以接入
- tool result 追加格式不稳定，影响 provider 无关性

### M4：基础工具集

**入口条件**
- M3 已能承载 tool request / tool result 回填
- workspace/path 安全约束可复用

**任务**

1. `read_file`
2. `write_file`
3. `edit_file`
4. `glob_search`
5. `grep_search`
6. `bash`
7. `todo_write`
8. `web_fetch`
9. `web_search`

**依赖**
- 依赖 M1 的 tool registry 抽象、M3 的 runtime tool loop
- `web_*` 实际排在 M9 之后落地，避免打断核心闭环

**交付**

- 工具全部走统一 registry + schema + permission

**退出条件 / DoD**
- 文件、搜索、bash、todo 工具先满足核心闭环；web 工具在后续阶段补齐
- 每个工具具备 schema、权限级别、结构化输出、失败语义
- 至少 1 条集成测试覆盖多工具串联
- workspace 越界、替换失败、命令超时等关键错误可稳定复现与断言

**主要风险**
- 工具直接访问系统资源绕过统一权限入口
- `bash` 与文件工具错误结构不一致，影响模型消费

### M5：权限系统

**入口条件**
- M4 的工具 spec 与权限映射点已经固定

**任务**

1. 定义三种 permission mode
2. 实现工具权限映射
3. 实现 runtime 执行前校验
4. 实现用户确认策略（最小可用版）

**依赖**
- 依赖 M3、M4

**交付**

- 危险工具不会绕过权限引擎

**退出条件 / DoD**
- `read-only` / `workspace-write` / `danger-full-access` 行为与测试规格一致
- 权限拒绝信息包含工具名、当前模式、所需模式
- runtime 与工具执行统一通过权限引擎

**主要风险**
- 交互确认策略分散到 CLI 和 runtime 两侧，后续难复用
- shell / write / edit 工具权限分级不一致

### M6：Session / Resume / Export

**入口条件**
- M3 已产出稳定 message/tool trace
- M5 权限模式可被会话保存

**任务**

1. 设计 JSON session 格式
2. 实现 file store
3. 实现 save/load/list/export
4. 把 runtime 与 session store 集成

**依赖**
- 依赖 M3、M5

**交付**

- 可中断恢复

**退出条件 / DoD**
- session 文件可离线检查
- save/load/resume/export 路径均可验证
- 版本字段不兼容时能拒绝加载并返回清晰错误

**主要风险**
- session 结构不稳定，导致 compact 与 resume 互相牵制
- trace 过大影响长会话性能

### M7：Prompt / Instructions / Context

**入口条件**
- M6 已可恢复会话
- instructions / workspace 信息来源已明确

**任务**

1. 收集 `AGENTS.md` / `CLAW.md`
2. 构建 prompt bundle
3. 注入工具定义
4. 注入 session 历史
5. 注入 workspace 信息

**依赖**
- 依赖 M1、M3、M6

**交付**

- prompt 构造稳定可测试

**退出条件 / DoD**
- 指令文件收集顺序、覆盖规则稳定
- compact 前后 prompt builder 均可工作
- prompt bundle 明确包含 system、instructions、history、tools、workspace

**主要风险**
- 指令优先级不稳定导致行为漂移
- prompt 装配与 runtime 耦合过深

### M8：Compact

**入口条件**
- M6 session 持久化稳定
- M7 prompt builder 可接收 compact 结果

**任务**

1. 设计 compact summarizer prompt
2. 实现 compact 触发条件
3. 实现模型辅助 compact
4. 实现 fallback 紧急摘要
5. 将 compact 结果写回 session

**依赖**
- 依赖 M6、M7

**交付**

- 长会话可继续

**退出条件 / DoD**
- 达到阈值可触发 compact，失败时可走 fallback
- compact 结果包含当前目标、已完成、未完成、关键文件、下一步建议
- compact 后 token 预算下降且会话仍可继续执行工具

**主要风险**
- 摘要质量不足导致长任务丢上下文
- fallback 信息结构与主路径不兼容

### M9：CLI + Slash Commands

**入口条件**
- M3、M5、M6、M7、M8 已提供完整交互底座

**任务**

1. interactive 模式
2. one-shot 模式
3. `/help`
4. `/status`
5. `/model`
6. `/permissions`
7. `/clear`
8. `/resume`
9. `/config`
10. `/memory`
11. `/init`
12. `/diff`
13. `/session`
14. `/export`

**依赖**
- 依赖 M3、M5、M6、M7、M8

**交付**

- 用户操作面完整

**退出条件 / DoD**
- interactive 与 one-shot 共用同一 runtime 主干
- slash commands 不破坏当前交互状态
- 至少覆盖 `/help` `/status` `/model` `/permissions` `/resume` `/session`

**主要风险**
- command router 侵入 runtime 业务状态
- 输出渲染过早复杂化，拖慢核心闭环

### M10：Skills / Agents 发现

**入口条件**
- CLI 已可稳定暴露本地能力
- 本地文件发现机制已确定

**任务**

1. skills 根目录发现
2. `SKILL.md` 读取
3. agents 根目录发现
4. list/read 命令补齐

**依赖**
- 依赖 M9

**交付**

- 本地扩展入口可见

**退出条件 / DoD**
- skills / agents 发现只做本地读取，不引入 team/subagent runtime
- list/read 能稳定返回结构化结果

**主要风险**
- 误把 team orchestration、marketplace 拉入 v0.1 范围

### M11：测试与发布打磨

**入口条件**
- M1-M10 的 v0.1 范围实现完成
- demo repo 与 fixtures 已准备

**任务**

1. 补齐单元测试
2. 补齐集成测试
3. 搭建 demo repo E2E
4. 补齐 README / quickstart
5. 验证 Anthropic / OpenAI-compatible 两条链路

**依赖**
- 依赖全部前序 milestone

**交付**

- 满足 release gate

**退出条件 / DoD**
- 单元、核心集成、至少 1 条 demo repo E2E 全部通过
- Anthropic 与 OpenAI-compatible 两条路径均完成至少 1 次完整 tool loop 验证
- compact / resume 手工验证通过
- quickstart 可按文档跑通

**主要风险**
- 测试收尾过晚，导致前期设计问题集中暴露
- demo repo 与真实使用路径偏差过大

## 4. 推荐实现顺序（严格）

### 4.1 主路径

1. 类型系统与骨架
2. provider 双实现
3. runtime 核心闭环
4. 文件/搜索/shell/todo 工具
5. 权限系统
6. session 持久化
7. prompt 与 instructions
8. compact
9. CLI / slash commands
10. web tools
11. skills / agents
12. 测试收尾

### 4.2 依赖链说明

- M1 -> M2 -> M3 是不可并行打散的主干
- M4 依赖 M3；其中 `web_*` 工具延后到 CLI 基本可用后落地，避免干扰闭环主线
- M5 必须在 M4 核心工具稳定后完成，否则权限映射会反复返工
- M6 依赖 M3 与 M5，确保 session 能保存权限模式与 trace
- M7 依赖 M6，避免 prompt builder 先依赖不稳定 session 结构
- M8 依赖 M6、M7，必须在 session 与 prompt 稳定后接入
- M9 依赖 M3、M5、M6、M7、M8，属于完整操作面封装，不应前置
- M10 只在 M9 之后进入，且仅做本地发现，不扩范围
- M11 贯穿补齐，但 release gate 只在全部范围内能力完成后判定

### 4.3 可并行窗口

- M2 中 Anthropic / OpenAI-compatible provider 可以并行实现，但共享事件模型必须先冻结
- M4 中文件工具与搜索工具可并行；`bash` 需在权限映射草案明确后接入
- M11 中单元测试补齐、demo repo E2E 准备、README/quickstart 可并行推进

### 4.4 明确禁止的并行方式

- 不允许在 M3 未稳定前提前开发复杂 CLI 渲染
- 不允许在 M5 未稳定前让工具各自实现权限判断
- 不允许在 M6/M7 未稳定前提前做 compact 最终方案
- 不允许把 web tools、skills/agents 作为“看起来有进展”的替代主线

## 5. 每阶段完成定义

### 5.1 完成定义模板

每个 milestone 完成时必须满足：

- 代码可编译
- 对应测试通过
- 至少一个示例路径可跑通
- 不引入新的结构性 TODO
- 本阶段新增接口、schema、状态字段已补最小文档或测试锚点

### 5.2 阶段验收记录模板

- `Milestone`: Mx
- `Entry checks`: 是否满足入口条件
- `Exit checks`: DoD 是否全部完成
- `Evidence`: 编译/测试/示例路径/日志
- `Open risks`: 剩余风险与缓解动作
- `Decision`: pass / conditional pass / fail

## 6. 发布门槛与手工验证闭环

### 6.1 Release Gate

v0.1 进入对外可用前，必须同时满足：

- M1-M10 范围内功能完成，且无范围漂移
- 单元测试通过
- 核心集成测试通过
- 至少 1 个 demo repo E2E 通过
- Anthropic 与 OpenAI-compatible 两条 provider 路径各完成至少 1 次完整 tool loop
- session / resume / compact 核心路径验证通过
- CLI interactive 与 one-shot 均通过基本可用性检查
- README / quickstart 可按文档独立跑通

### 6.2 手工验证闭环

发布前必须完成以下手工检查并记录证据：

1. 流式输出观感与工具摘要可读
2. 低权限模式危险操作提示明确
3. 中断后 resume 恢复完整
4. compact 前后长任务可继续
5. 网络波动下 provider 错误语义可区分
6. demo repo 真实修复路径可由人复核通过

### 6.3 阻塞发布的问题类型

- 任一 provider 无法完成基础 tool loop
- runtime 多工具串联不稳定
- 权限边界可被绕过
- session/resume/compact 任一路径不可用
- quickstart 与真实路径脱节，无法独立跑通

## 7. 暂不进入 v0.1 的任务

- plugin runtime
- hooks
- MCP
- LSP
- subagent/team
- server/SSE API
- browser automation

## 8. 最终交付物

v0.1 最终应交付：

- 可运行 CLI
- 双 provider 支持
- 核心工具闭环
- session/resume/compact
- 基础 commands
- 测试集
- demo repo 验证
