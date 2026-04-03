# Claw

<p align="center">
  <img src="./claw-hero.jpeg" alt="Claw hero image" width="280">
</p>

<p align="center">
  <strong>A personal, stronger harness for coding agents — with explicit permissions, scriptable rule management, and a cleaner runtime boundary.</strong>
</p>

Claw is a Go-based AI Agent Runtime that exposes agent capabilities via HTTP API. It supports multiple providers (Anthropic / OpenAI), multi-tenant session isolation, streaming SSE responses, built-in tool execution, and a permission engine.

> [!IMPORTANT]
> This repository is **not** a marker dump for leaked Claude Code source, and it should not be read that way.
> It is an independent harness-oriented implementation focused on building a stricter, more inspectable, and more scriptable agent runtime in Go.

## Why It Exists

The story behind this project starts with a real pain point: using Claude CLI through external scheduling systems quickly revealed that the bottleneck was never the model itself — it was everything around it. Prompt composition, context management, tool orchestration, session state, and runtime control were all being stitched together in ways that were fragile, opaque, and hard to maintain.

After studying how Claude Code approaches the core agent loop — the way it separates harness from model, enforces permission boundaries, and keeps the execution flow inspectable — we recognized that the most valuable lesson wasn't "how to call a model," but "how to build a runtime that calls a model well."

That realization led to building this project. Not as a recreation of a branded product, but as a Go-based implementation of the same core principles: a strict harness with explicit tool execution boundaries, permission routing, rule-based policy management, and a clean separation between runtime control and model interaction.

**Why Go?** Our team works primarily in Go. Moving the agent runtime into the same language our team already lives in means it's easier to maintain, extend, debug, and integrate into existing infrastructure — rather than treating it as a black box that only a few people can touch.

**Why Web-first?** Most agent runtimes are built CLI-first and bolted onto web later. We went the other direction — HTTP API as the primary interface from day one, with SSE streaming, multi-tenant session isolation, and Git worktree sandboxing designed in from the start. The CLI exists for debugging and local use, but the real target is serving agent capabilities to web applications at scale.

**Why open source?** Many engineering teams are in the same position: they use Go, they care about LLM harness quality, and they're looking for a principled way to build agent runtimes rather than duct-taping prompts and wrappers together. We hope sharing this implementation opens up a broader conversation — about harness design, tool use patterns, permission models, and what it really takes to move agent systems from "working demo" to "production-ready."

## Features

- **Web-first** — HTTP API as the primary interface, session-only model, SSE streaming
- **Multi-tenant** — API key isolation with independent sessions and working directories per tenant
- **Git Worktree Isolation** — Each session binds to its own worktree, sandboxing tool execution naturally
- **Permission Control** — Three-tier permission model + rule engine + interactive confirmation
- **Multi-Provider** — Anthropic (HTTP+SSE), OpenAI (HTTP+SSE)
- **Skill System** — YAML-defined capability templates with session-level activation
- **WebSocket** — Real-time bidirectional communication with skill integration
- **Go SDK** — `pkg/sdk` client library for programmatic access
- **Built-in Tools** — read/write/edit_file, glob/grep_search, bash
- **CLI** — Interactive REPL for debugging and local use

## Architecture

```
                          ┌─────────────────────────────────────────┐
                          │                 Claw                    │
                          └──────────────────┬──────────────────────┘
                                             │
                    ┌────────────────────────┴─────────────────────────┐
                    │                                                  │
          ┌─────────┴─────────┐                          ┌─────────────┴──────────────┐
          │     CLI (claw)    │                          │     HTTP Server             │
          │                   │                          │                             │
          │   Interactive     │                          │  POST /v1/sessions          │
          │   REPL            │                          │  POST /v1/sessions/:id/msg  │
          └─────────┬─────────┘                          │  GET  /health               │
                    │                                    └─────────────┬───────────────┘
                    │                                                  │
                    └────────────────────────┬─────────────────────────┘
                                             │
               ┌─────────────────────────────┴──────────────────────────────┐
               │                       Runtime Core                        │
               │                                                           │
               │   Engine ──── Session Store ──── Tool Registry            │
               │      │                                                    │
               │   Permissions ──── WorkDir Manager                        │
               └─────────────────────────────┬──────────────────────────────┘
                                             │
               ┌─────────────────────────────┴──────────────────────────────┐
               │                      Provider Layer                       │
               │                                                           │
               │       Anthropic (HTTP+SSE)     │     OpenAI (HTTP+SSE)    │
               └────────────────────────────────────────────────────────────┘
```

## Quick Start

### CLI Mode

```bash
# Set API key
export ANTHROPIC_API_KEY=your_key_here

# Interactive REPL
go run ./cmd/claw

# One-shot execution
go run ./cmd/claw "show me the directory structure"
```

### HTTP Server Mode

```bash
# Start server
go run ./cmd/claw serve --port 8080

# Create a session
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"model": "claude-sonnet-4-5"}'

# Send a message (SSE streaming response)
curl -N -X POST http://localhost:8080/v1/sessions/<session-id>/messages \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"content": "review the code structure"}'
```

### Docker

```bash
# Build and run
docker compose up -d

# With custom API keys
ANTHROPIC_API_KEY=sk-... CLAW_API_KEYS=your-api-key docker compose up -d
```

### Go SDK

```go
import "claude-go-code/pkg/sdk"

client := sdk.NewClient("http://localhost:8080", "your-api-key")

sess, _ := client.CreateSession(ctx, nil)

client.ChatStream(ctx, sess.ID, "review the code", func(event *sdk.StreamEvent) bool {
    if event.Type == "text_delta" {
        fmt.Print(event.Text)
    }
    return event.Type != "done"
})
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/sessions` | Create session (optionally specify repo_url) |
| `GET` | `/v1/sessions` | List sessions |
| `GET` | `/v1/sessions/:id` | Get session details |
| `DELETE` | `/v1/sessions/:id` | Delete session (cleanup worktree) |
| `POST` | `/v1/sessions/:id/messages` | Send message (SSE streaming) |
| `GET` | `/v1/sessions/:id/messages` | Get message history |
| `GET` | `/v1/models` | List available models |
| `GET` | `/v1/skills` | List available skills |
| `POST` | `/v1/skills` | Register a new skill |
| `DELETE` | `/v1/skills/:name` | Delete a skill |
| `GET` | `/ws/:id` | WebSocket connection |
| `GET` | `/health` | Health check |
| `GET` | `/metrics` | Prometheus metrics |

## Configuration

### Environment Variables

```bash
# Provider
export CLAW_PROVIDER=anthropic          # anthropic | openai
export CLAW_MODEL=claude-sonnet-4-5     # default model
export ANTHROPIC_API_KEY=sk-...

# Permissions
export CLAW_PERMISSION_MODE=workspace-write           # read-only | workspace-write | danger-full-access
export CLAW_PERMISSION_ESCALATION_POLICY=deny          # deny | prompt
export CLAW_PERMISSION_RULES_PATH=~/.claw/rules.json
```

### Permission Modes

| Mode | Description |
|------|-------------|
| `read-only` | Only read-only tools allowed (read_file, glob_search, grep_search) |
| `workspace-write` | File read/write allowed, dangerous operations like bash blocked |
| `danger-full-access` | All operations allowed |

### Rule Management

```bash
# List rules
go run ./cmd/claw permissions rules list

# Add a rule
go run ./cmd/claw permissions rules add \
  --tool bash --current workspace-write \
  --required danger-full-access --decision allow \
  --command-prefix git

# Remove a rule by index
go run ./cmd/claw permissions rules remove 1

# Clear all rules
go run ./cmd/claw permissions rules clear
```

## Project Structure

```
claude-go-code/
├── cmd/claw/                  # CLI entrypoint
├── internal/
│   ├── app/                   # Application assembly (DI)
│   ├── config/                # Config (defaults + env vars)
│   ├── runtime/               # Core Engine (multi-turn chat loop)
│   ├── provider/              # Provider abstraction
│   │   ├── anthropic/         # Anthropic HTTP+SSE implementation
│   │   └── openai/            # OpenAI HTTP+SSE implementation
│   ├── session/               # Session store (in-memory / file) + GC
│   ├── tools/                 # Built-in tools
│   ├── permissions/           # Permission engine + rule persistence
│   ├── server/                # HTTP Server + WebSocket
│   ├── skill/                 # Skill system (YAML loader + session activation)
│   ├── sandbox/               # Tool sandbox (path isolation)
│   ├── sysprompt/             # System prompt builder
│   └── workdir/               # Git worktree manager
├── pkg/
│   ├── types/                 # Shared types
│   └── sdk/                   # Go SDK client
├── SPEC.md                    # Detailed technical specification
└── go.mod
```

## Development

```bash
# Run tests
go test ./...

# Static analysis
go vet ./...

# Launch REPL
go run ./cmd/claw
```

## Roadmap

**Phase 1 — MVP (Done)**

- [x] Core Runtime multi-turn chat loop
- [x] Anthropic Provider (HTTP + SSE streaming)
- [x] Built-in tools (6: file read/write/edit, search, bash)
- [x] Permission engine + rule persistence
- [x] CLI interactive REPL
- [x] Streaming Runtime (`RunPromptStream`)
- [x] HTTP Server (gin + middleware + SSE)
- [x] Session API + file persistence
- [x] API Key authentication

**Phase 2 — Production (Done)**

- [x] Git Worktree isolation (bare repo + per-session worktree)
- [x] System Prompt assembly (identity + environment + tools)
- [x] Tool sandbox (Web mode path isolation)
- [x] Structured logging (slog + JSON)
- [x] Prometheus Metrics (`/metrics` endpoint)
- [x] Session TTL + automatic garbage collection
- [x] Per-key rate limiting

**Phase 3 — Advanced (Done)**

- [x] OpenAI Provider (real HTTP+SSE implementation)
- [x] Skill system (YAML loader + session activation + trigger matching)
- [x] WebSocket support (bidirectional, skill integration)
- [x] Go SDK (`pkg/sdk`)
- [x] Docker + Compose deployment

**Phase 4 — Future**

- [ ] TypeScript SDK
- [ ] Context compaction (LLM-based summarization)
- [ ] MCP (Model Context Protocol) integration
- [ ] Multi-instance deployment (Redis session store)
