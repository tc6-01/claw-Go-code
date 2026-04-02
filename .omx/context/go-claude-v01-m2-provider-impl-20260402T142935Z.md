# Context Snapshot

## Task statement
用户要求用 `$team 3` 继续开始实现，并且必须做测试验证。当前代码已完成 M1 最小骨架，下一步进入 M2：Provider 打通。

## Desired outcome
完成 M2 的第一版实现与验证：
- 建立 Anthropic provider 目录与最小可测试实现
- 建立 OpenAI-compatible provider 目录与最小可测试实现
- 冻结统一 provider 事件模型与工厂接线
- 增加对应单元/集成级测试，确保 `go test ./...` 通过
- 本轮不进入真实网络请求，不引入新依赖，不越级做 M3 tool loop

## Known facts / evidence
- 当前仓库已有 M1 骨架：`go.mod`、`cmd/claw/main.go`、`internal/app`、`internal/config`、`internal/provider`、`internal/runtime`、`internal/session`、`internal/tools`、`pkg/types`
- 当前通过验证：`go test ./...` 与 `go run ./cmd/claw status`
- 计划文档要求 M2 目标：双 provider 输出统一事件流；鉴权、base URL、错误语义可测试；provider mock 可复用
- 测试规格要求覆盖 Anthropic / OpenAI-compatible 的普通文本、流式、tool call、usage、鉴权缺失、base URL 覆盖
- 当前工作目录不是 git repo，没有 `.git`
- 上一轮已验证：OMX team 的 Codex worker 启动参数与本机 Codex CLI 不兼容，需优先使用 Claude workers

## Constraints
- 不新增第三方依赖
- 不做真实联网调用
- 保持修改文件分工清晰，减少冲突
- 必须保留 `go test ./...` 通过
- 团队 worker 不应依赖 git commit 步骤，因为当前目录没有 `.git`

## Unknowns / open questions
- 本轮 Anthropic/OpenAI provider 要做到多细：建议先做“可测试骨架 + 统一事件模型 + stub/adapter 层”，下一轮再接真实 HTTP 细节
- 是否需要在 M2 末尾补 1 条 provider factory 集成测试；倾向需要

## Likely touchpoints
- `internal/provider/provider.go`
- `internal/provider/events.go`
- `internal/provider/types.go`
- `internal/provider/anthropic/*`
- `internal/provider/openai/*`
- `internal/app/app.go`
- `internal/runtime/engine.go`
- `pkg/types/message.go`
- `pkg/types/tool.go`
- `internal/provider/*_test.go`
