# claw-Go-code

`claw-Go-code` is a Go-based agent harness that experiments with a safer, more controllable, and more extensible execution loop for coding agents.

It is intentionally positioned as a harness layer:

- stronger permission routing
- explicit tool execution boundaries
- session and persisted approval rules
- testable runtime behavior
- room for custom policy and orchestration logic

This repository is **not** a marker dump for leaked Claude Code source, and it should not be read that way.
It is an independent harness-oriented implementation that explores how to build a stricter and more inspectable agent runtime in Go.

## What this project focuses on

- A compact CLI entrypoint in `cmd/claw`
- A runtime loop in `internal/runtime`
- Tool registration and execution in `internal/tools`
- Permission policy and confirmation logic in `internal/permissions`
- Provider abstraction in `internal/provider`

## Permission model

The harness supports three permission modes:

- `read-only`
- `workspace-write`
- `danger-full-access`

When escalation policy is set to `prompt`, the CLI can:

- allow once
- allow for the current session
- deny once
- deny for the current session
- allow as a persisted rule
- block as a persisted rule

Persisted rules are stored by default at:

```text
~/.claude-go-code/permissions/rules.json
```

You can override that path with:

```bash
CLAW_PERMISSION_RULES_PATH=/custom/path/rules.json
```

## Rule management

List persisted rules:

```bash
go run ./cmd/claw permissions rules list
```

Clear persisted rules:

```bash
go run ./cmd/claw permissions rules clear
```

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
