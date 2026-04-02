# Context Snapshot

## Task statement
用户要求基于 `/.omx/plans/skeleton-go-claude-code-v0.1.md` 进入实现前准备阶段：先把实施计划补充完整，再把测试计划补充清楚，供用户先审阅；当前不要开始实现代码。

## Desired outcome
产出一版更完整、可审阅、可执行的计划文档集合，至少覆盖：
- 骨架/实施计划的缺口补全
- 与 PRD 对齐的 milestone、依赖、DoD、风险与非目标
- 测试计划的分层、覆盖矩阵、执行顺序、发布门槛、手工验证项
- 明确“审阅通过后再进入实现”

## Known facts / evidence
- 现有核心文档：
  - `.omx/plans/prd-go-claude-code-v0.1.md`
  - `.omx/plans/dev-plan-go-claude-code-v0.1.md`
  - `.omx/plans/skeleton-go-claude-code-v0.1.md`
  - `.omx/plans/test-spec-go-claude-code-v0.1.md`
- `skeleton` 已定义目录骨架、依赖方向、核心接口、启动顺序、P0/P1/P2 文件优先级、约束与验收标准。
- `dev-plan` 已有 M1-M11 milestone，但颗粒度偏粗，缺少阶段入口/出口、依赖、并行策略、风险与审阅门禁。
- `test-spec` 已有 unit/integration/e2e/manual 分层，但缺少覆盖矩阵、fixture 设计、mock 策略、执行节奏、失败归因与验收模板。
- 用户显式触发 `$team`，希望以 4 人团队推进。

## Constraints
- 当前阶段只补计划，不实现代码。
- 不新增超出 PRD v0.1 的范围。
- 计划需与现有 PRD、开发拆解、测试规格保持一致并补足缺项。
- 输出要让用户“先看清楚”，因此结构需清晰、审阅友好。

## Unknowns / open questions
- 最终采用“增补现有文档”还是“新增更细的实施计划文档”作为主审阅入口。
- 哪些内容放在 `skeleton`，哪些内容放在 `dev-plan` / `test-spec`，需要团队给出建议。
- 是否需要增加覆盖矩阵/里程碑验收表格。

## Likely touchpoints
- `.omx/plans/skeleton-go-claude-code-v0.1.md`
- `.omx/plans/dev-plan-go-claude-code-v0.1.md`
- `.omx/plans/test-spec-go-claude-code-v0.1.md`
- `.omx/plans/prd-go-claude-code-v0.1.md`
