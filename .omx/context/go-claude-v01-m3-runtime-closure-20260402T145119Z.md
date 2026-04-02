# Go Claude Code v0.1 M3 Runtime Closure Context

## Task statement
Implement M3 runtime core closure for the Go claw prototype after the M1/M2 baseline commit.

## Desired outcome
Deliver a testable runtime loop that can consume provider responses, execute registered tools, append tool results back into session/message history, and finish a single task across one or more tool calls without provider-specific branching.

## Known facts / evidence
- Baseline commit exists at `29bc4f0`.
- M1 skeleton and M2 provider wiring are already implemented and passing `go test ./...`, `go vet ./...`, and `go run ./cmd/claw status`.
- Shared provider event model exists in `internal/provider/`.
- App wiring now selects Anthropic/OpenAI stub providers from config.
- Current runtime is only a bootstrap stub in `internal/runtime/engine.go`.
- Tool registry exists only as a skeleton in `internal/tools/`.

## Constraints
- User explicitly requested `$team 4` and wants tests/verification.
- Keep using the existing repo plan artifacts under `.omx/plans/`.
- No real network calls.
- No new dependencies unless absolutely unavoidable (prefer none).
- Preserve prior M2 behavior; do not regress provider tests.
- Repo is now a git repo; future code changes should be commit-ready, but workers must not rewrite history.
- Ignore unrelated `.omx/logs` / `.omx/state` churn unless needed for evidence.

## Unknowns / open questions
- Exact minimal tool shape needed for M3 closure without over-building M4.
- Best place to record message trace vs tool trace in session while keeping interfaces stable.
- How much of streaming behavior to simulate now versus defer.

## Likely code touchpoints
- `internal/runtime/engine.go`
- `internal/runtime/engine_test.go`
- `internal/tools/registry.go`
- `internal/tools/types.go`
- `internal/tools/validate.go`
- `internal/provider/provider.go`
- `internal/provider/events.go`
- `internal/session/model.go`
- `pkg/types/message.go`
- `pkg/types/session.go`
- `internal/app/app.go`

## M3 DoD to enforce
- Single-turn and multi-tool loop paths pass.
- Tool failures do not crash runtime.
- Usage, tool trace, and message trace persist in session.
- At least one integration-style test covers read -> tool -> append -> finish.
- `go test ./...` and relevant smoke verification remain green.
