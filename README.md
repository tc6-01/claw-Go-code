# claw-Go-code

<p align="center">
  <img src="./claw-hero.jpeg" alt="Claw hero image" width="280">
</p>

<p align="center">
  <strong>A personal, stronger harness for coding agents — with explicit permissions, scriptable rule management, and a cleaner runtime boundary.</strong>
</p>

`claw-Go-code` is a Go-based agent harness that experiments with a safer, more controllable, and more extensible execution loop for coding agents.

> [!IMPORTANT]
> This repository is **not** a marker dump for leaked Claude Code source, and it should not be read that way.
> It is an independent harness-oriented implementation focused on building a stricter, more inspectable, and more scriptable agent runtime in Go.

## Highlights

- stronger permission routing
- explicit tool execution boundaries
- session and persisted approval rules
- target-aware rule matching for commands, hosts, and paths
- human-friendly and JSON-friendly CLI rule management
- testable runtime behavior with small, reviewable components

## Quick Start

### 1. Pick a provider

```bash
export ANTHROPIC_API_KEY=your_key_here
```

If you want OpenAI instead:

```bash
export CLAW_PROVIDER=openai
export OPENAI_API_KEY=your_key_here
```

### 2. Run the smoke test

```bash
go run ./cmd/claw status
```

That verifies the CLI boots, loads config, builds the runtime, and completes a minimal request loop.

### 3. Pick a permission posture

```bash
export CLAW_PERMISSION_MODE=read-only
export CLAW_PERMISSION_ESCALATION_POLICY=prompt
```

The default mode is `workspace-write` with escalation denied.
If you set `prompt`, the interactive CLI can:

- allow once
- allow for the current session
- deny once
- deny for the current session
- allow as a persisted rule
- block as a persisted rule

### 4. Inspect saved rules

```bash
go run ./cmd/claw permissions rules list
go run ./cmd/claw permissions rules list --json
```

Persisted rules are stored by default at:

```text
~/.claude-go-code/permissions/rules.json
```

You can override that path with:

```bash
CLAW_PERMISSION_RULES_PATH=/custom/path/rules.json
```

## Rule Management

List persisted rules:

```bash
go run ./cmd/claw permissions rules list
```

List persisted rules as JSON:

```bash
go run ./cmd/claw permissions rules list --json
```

Add a persisted rule manually:

```bash
go run ./cmd/claw permissions rules add \
  --tool bash \
  --current workspace-write \
  --required danger-full-access \
  --decision allow \
  --command-prefix git
```

The same command can return JSON for scripts:

```bash
go run ./cmd/claw permissions rules add \
  --tool bash \
  --current workspace-write \
  --required danger-full-access \
  --decision allow \
  --command-prefix git \
  --json
```

Remove one persisted rule by index:

```bash
go run ./cmd/claw permissions rules remove 1
```

Remove persisted rules by matcher:

```bash
go run ./cmd/claw permissions rules remove \
  --tool web_fetch \
  --host example.com
```

Matcher-based removal also supports JSON:

```bash
go run ./cmd/claw permissions rules remove \
  --tool web_fetch \
  --host example.com \
  --json
```

Clear persisted rules:

```bash
go run ./cmd/claw permissions rules clear
```

## What this project focuses on

- `cmd/claw` — compact CLI entrypoint
- `internal/runtime` — request loop and tool execution flow
- `internal/tools` — builtin tool registration and execution
- `internal/permissions` — policy, confirmation, and persisted rules
- `internal/provider` — provider abstraction for model backends
- `internal/config` — environment-driven runtime configuration

## Permission Model

The harness supports three permission modes:

- `read-only`
- `workspace-write`
- `danger-full-access`

## Why it exists

Most agent CLIs blur together policy, tool execution, and model interaction.
This project tries to separate those concerns so they can be reasoned about, tested, and evolved independently.

That means the goal here is not to mimic a branded product artifact.
The goal is to build a better harness:

- easier to audit
- easier to extend
- easier to validate
- harder to accidentally over-permit

## Development

Run the main verification flow:

```bash
go test ./...
env GOCACHE=/tmp/go-build go vet ./...
env GOCACHE=/tmp/go-build go run ./cmd/claw status
```
