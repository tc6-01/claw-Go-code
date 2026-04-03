# Claw — Agent Harness Technical Specification v2.0

> Web-first AI Agent Harness built in Go

---

## 1. Project Overview

### 1.1 Project Name
**Claw** — Modular AI agent control system (Go implementation, binary name `claw`, module name `claude-go-code`)

### 1.2 Core Positioning
A Web-first AI Agent Runtime that gives AI models tool execution capabilities, with multi-Provider support, OpenAI-compatible API, and streaming SSE output.

### 1.3 Design Goals
- **Performance**: Go-native concurrency (goroutine + channel), low-latency streaming responses
- **Modularity**: Components usable independently, interface-driven
- **Extensibility**: Tool registration, MCP protocol (planned)
- **Production-grade**: Full permission system, session management, HTTP API

> **Note**: This document also includes Rust-version architecture reference design (marked `[Rust reference]`).
> The Go version is the primary implementation; see Chapter 15 for details.

---

## 2. System Architecture `[Rust reference]`

### 2.1 Overall Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         NexusHarness                             │
├─────────────────────────────────────────────────────────────────┤
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐     │
│  │   CLI     │  │  Server   │  │   SDK     │  │  Plugin   │     │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘     │
│        │              │              │              │            │
│  ┌─────▼──────────────▼──────────────▼──────────────▼─────┐     │
│  │                    Runtime Core                        │     │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │     │
│  │  │ Conversation │  │   Session   │  │  Permission │     │     │
│  │  │   Runtime    │  │   Manager   │  │   Policy    │     │     │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │     │
│  └────────────────────────────────────────────────────────┘     │
│  ┌────────────────────────────────────────────────────────┐     │
│  │                    Tool Layer                           │     │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │     │
│  │  │ Builtin │  │   MCP   │  │ Skill   │  │ Plugin  │    │     │
│  │  │  Tools  │  │  Tools  │  │ Tools   │  │  Hooks  │    │     │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │     │
│  └────────────────────────────────────────────────────────┘     │
│  ┌────────────────────────────────────────────────────────┐     │
│  │                    API Layer                            │     │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐                  │     │
│  │  │  Claw   │  │   xAI   │  │ OpenAI │                  │     │
│  │  │ Provider│  │ Provider│  │Compat.  │                  │     │
│  │  └─────────┘  └─────────┘  └─────────┘                  │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Module Responsibility Matrix

| Module | Responsibility | Dependencies |
|--------|----------------|---------------|
| `runtime` | Core agent loop, session management | api, tools, plugins |
| `api` | AI Provider abstraction, streaming responses | - |
| `tools` | Tool registration, execution framework | runtime |
| `commands` | Slash command system | runtime, tools |
| `plugins` | Lifecycle hooks, Pre/Post interception | runtime |
| `lsp` | Language Server Protocol integration | runtime |
| `server` | HTTP/SSE API service | runtime |
| `mcp` | Model Context Protocol | runtime, tools |
| `oauth` | OAuth 2.0 + PKCE | - |

---

## 3. Core Design Principles

> This chapter defines core design principles common to all versions (Rust/Go) and is the foundation for detailed design in each version.

### 3.1 Core Architecture Design Principles

#### 3.1.1 Layered Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                   Layered Architecture Diagram                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                 API Interface Layer                      │   │
│  │  (CLI / HTTP Server / SDK)                              │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Runtime Core                          │   │
│  │  (conversation loop / session management / permissions)  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Extension Capability Layer                  │   │
│  │  (Tools / Commands / Plugins / MCP / Skills)            │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Provider Layer                        │   │
│  │  (Anthropic / xAI / OpenAI compatible)                   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Principles:**
- **Upper layers depend on lower layers**: The API layer depends on Runtime Core; Runtime Core depends on the extension capability layer
- **Lower layers do not know upper layers**: The extension capability layer does not directly depend on the API layer
- **Depend on abstractions, not implementations**: Decouple via Trait/Interface

#### 3.1.2 Core Component Responsibilities

| Component | Responsibility | Boundary |
|-----------|----------------|----------|
| **Runtime Core** | Conversation loop execution, session management, permission control | Does not handle HTTP/CLI directly |
| **Tool Registry** | Tool registration, execution, result return | Does not know conversation context |
| **Command Registry** | Slash command parsing and execution | Does not call the API directly |
| **Session** | Message history, context compaction, metadata | Stateless, persistable |
| **Permission Policy** | Permission checks, rule matching | Decides only; does not execute tools |

#### 3.1.3 Request Data Flow

```
User input (CLI / HTTP / SDK)
         │
         ▼
┌─────────────────────────────────────────┐
│  Command Preprocessor                   │
│  Detect /command, +skill, mcp_ prefix   │
└────────────────┬────────────────────────┘
                 │
         ┌───────▼───────┐
         │ Slash command? │
         └───────┬───────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
       Yes               No
        │                 │
        ▼                 ▼
┌───────────────┐  ┌─────────────────────┐
│ CommandHandler │  │ ConversationRuntime │
└───────────────┘  └─────────────────────┘
        │                       │
        └───────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │ Build MessageRequest │
         │  system_prompt +    │
         │  messages           │
         └──────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │  ApiClient.stream() │
         └──────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │ Parse AssistantEvent │
         │  - text_delta       │
         │  - tool_use         │
         │  - usage            │
         └──────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │  tool_call?         │
         └──────────┬───────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
       Yes                     No
        │                       │
        ▼                       ▼
┌───────────────┐      ┌─────────────────┐
│ PermissionCheck│      │ Return to user  │
└───────────────┘      └─────────────────┘
        │
        ▼
┌───────────────┐
│ ToolRegistry  │
│ .execute()    │
└───────────────┘
        │
        ▼
┌───────────────┐
│ Append result │
│ to messages   │
└───────────────┘
        │
        ▼
    Continue loop (back to ApiClient.stream)
```

### 3.2 Module Design Principles

#### 3.2.1 Tool System Principles

**Tools are atomic, composable operation units**

```rust
// Tool specification
pub struct ToolSpec {
    pub name: String,              // Unique id (e.g. "bash", "read_file")
    pub description: String,       // Description for the model
    pub input_schema: Value,      // Input schema in JSON Schema format
    pub output_schema: Option<Value>, // Optional output schema
    pub required_permission: Permission, // Permission level required to execute
}
```

**Tool categories:**

| Category | Examples | Characteristics |
|----------|----------|-----------------|
| Built-in tools | bash, read_file, write_file | Core functionality, always available |
| MCP tools | mcp__filesystem__read_file | From MCP servers |
| Skill tools | Available after Skill activation | Added temporarily at session scope |

**Naming conventions:**
- Built-in tools: `snake_case` (e.g. `read_file`, `glob_search`)
- MCP tools: `mcp__<server_name>__<tool_name>` (e.g. `mcp__filesystem__read_file`)
- Skill tools: Names as defined by the Skill

#### 3.2.2 Command System Principles

**Slash commands are user-initiated control actions**

```rust
pub struct CommandSpec {
    pub name: String,           // Command name (e.g. "commit", "branch")
    pub aliases: Vec<String>,   // Aliases (e.g. ["co"] for checkout)
    pub description: String,     // Description
    pub category: CommandCategory, // Category
    pub resumable: bool,        // Whether interrupt/resume is supported
}
```

**Command categories:**

| Category | Examples | Function |
|----------|----------|----------|
| Core | /help, /status, /compact | Core control |
| Session | /resume, /export | Session management |
| Workspace | /config, /diff | Workspace operations |
| Git | /commit, /branch, /pr | Git operations |
| Automation | /bughunter, /plugins | Automation tasks |

#### 3.2.3 Session Management Principles

**A session is stateful conversation context**

```rust
pub struct Session {
    id: SessionId,              // Unique id
    model: String,              // Current model
    messages: Vec<Message>,    // Message history
    created_at: DateTime<Utc>,  // Creation time
    metadata: SessionMetadata,  // Metadata
}
```

**Session lifecycle:**
1. **Create**: `POST /sessions` → returns session_id
2. **Interact**: Send message → stream response → tool calls → return results
3. **Compact**: Auto-compact when context exceeds threshold
4. **Resume**: `POST /sessions/{id}/resume` → reload session state
5. **Export**: Export session history in a chosen format
6. **Delete**: `DELETE /sessions/{id}`

**Context compaction strategy:**
```rust
pub struct CompactionStrategy {
    target_tokens: usize,       // Target token count
    preserve_system: bool,      // Keep system prompt
    preserve_last_n: usize,     // Keep last N messages
    min_messages_to_keep: usize, // Minimum messages to retain
}
```

### 3.3 Permission Design Principles

#### 3.3.1 Permission Levels

```rust
pub enum PermissionLevel {
    ReadOnly,        // Read-only operations
    WorkspaceWrite,   // Workspace write
    DangerFullAccess, // Dangerous operations (bash, network, etc.)
}
```

**Permission check flow:**
```
Tool execution request
     │
     ▼
Check session cache
     │
     ├── hit → allow execution
     │
     ▼
Check one-time cache
     │
     ├── hit → allow execution
     │
     ▼
Get required permission level for tool
     │
     ▼
Evaluate by permission mode
     │
     ├── bypass → allow execution
     ├── dont_ask + sufficient level → allow execution
     ├── accept_edits + write tool → allow execution
     │
     ▼
Check rules (sorted by priority)
     │
     ├── deny rule → reject
     ├── ask rule → prompt user
     ├── allow rule → allow execution
     │
     ▼
Default → prompt user
```

**Authorization types:**
- `one_time`: Valid once; re-authorize if input changes
- `session`: Valid for the session; repeatable execution
- `permanent`: Persisted to config file

### 3.4 Extensibility Design Principles

#### 3.4.1 Provider Extension

```rust
pub trait ApiClient: Send + Sync {
    async fn stream(
        &self,
        request: MessageRequest,
    ) -> Result<StreamingResponse, ApiError>;

    fn supported_models(&self) -> Vec<ModelInfo>;
}
```

**Supported Providers:**
| Provider | Models | Context length |
|----------|--------|----------------|
| ClawApi (Anthropic) | claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5 | 32K-64K |
| Xai | grok, grok-3, grok-3-mini | 64K |

#### 3.4.2 MCP Protocol Support

MCP (Model Context Protocol) allows extending with external tool servers:

```rust
pub enum McpTransport {
    Stdio,      // Standard input/output
    Sse,        // Server-Sent Events
    Http,       // HTTP requests
    Ws,         // WebSocket
}
```

#### 3.4.3 Plugin Hook System

Plugins can intercept tool execution:

```rust
pub trait PreToolHook: Send + Sync {
    async fn call(&self, ctx: &HookContext) -> Result<HookDecision, HookError>;
}

pub trait PostToolHook: Send + Sync {
    async fn call(&self, ctx: &HookContext, result: &ToolResult) -> Result<(), HookError>;
}

pub enum HookDecision {
    Allow,                    // Allow execution
    Deny { reason: String },  // Deny execution
    Modify(Value),             // Execute after modifying input
}
```

### 3.5 Security Design Principles

#### 3.5.1 Tool Execution Security

**Bash tool safeguards:**
```rust
const BLOCKED_COMMANDS: &[&str] = &[
    "rm -rf /", "dd", ":(){:|:&};:", "mkfs",
    "shutdown", "reboot", "init 0",
];

// Path traversal protection
pub fn validate_path(path: &Path, allowed_dir: &Path) -> Result<PathBuf, ToolError> {
    let canonical = path.canonicalize()?;
    if !canonical.starts_with(allowed_dir) {
        return Err(ToolError::PathTraversal);
    }
    Ok(canonical)
}
```

#### 3.5.2 Credential Security

OAuth credentials are stored encrypted with AES-256-GCM:

```rust
pub struct CredentialStore {
    path: PathBuf,
    key: LessSafeKey,  // AES-256-GCM key
}
```

#### 3.5.3 API Authentication

HTTP Server uses API Key authentication:

```rust
pub struct AuthLayer {
    valid_keys: HashSet<String>,
}

// Auth check middleware
async fn auth_check(
    State(state): State<AuthLayer>,
    request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    match request.headers().get("Authorization") {
        Some(value) if state.is_valid(value) => Ok(next.run(request).await),
        _ => Err(StatusCode::UNAUTHORIZED),
    }
}
```

### 3.6 Skill System Principles

> A Skill is a session-scoped capability template: essentially a system prompt fragment + tool set

**Skill definition:**
```yaml
name: code-review
description: Code review expert
system_prompt: |
  You are a professional code review expert...
tools:
  - read_file
  - grep_search
  - bash
parameters:
  triggers: ["review", "audit"]
  max_uses: 10
```

**Skill vs Tool:**
| Dimension | Tool | Skill |
|-----------|------|-------|
| Nature | Executable operation | System prompt fragment |
| Invocation | Model decides to call | Session-scoped activation |
| Granularity | Atomic operation | Capability bundle |
| Persistence | Built-in + plugins | Customizable |

---

## 4. NexusHarness (Rust) Detailed Design `[Rust reference]`

> The Rust version is full-featured and production-grade, with full CLI, HTTP Server, Plugin system, and LSP integration.
> **Note**: This chapter is Rust architecture reference. For Go, see core principles in Chapter 3; implementation details in Chapter 15.

### 4.1 Runtime Module

The Rust Runtime module follows [Core Design Principles](#3-core-design-principles); key types:

```rust
// Core orchestrator
pub struct ConversationRuntime<C: ApiClient, T: ToolRegistry> {
    api_client: C,
    tool_registry: T,
    permission_policy: PermissionPolicy,
    session: Session,
    max_iterations: usize,
}

impl<C, T> ConversationRuntime<C, T>
where
    C: ApiClient,
    T: ToolRegistry,
{
    pub async fn run_turn(
        &mut self,
        system_prompt: &str,
        user_message: &str,
    ) -> Result<TurnSummary, RuntimeError>;

    async fn execute_tool_loop(
        &mut self,
        messages: &mut Vec<ConversationMessage>,
    ) -> Result<Option<ConversationMessage>, RuntimeError>;

    fn check_permission(&self, tool: &ToolSpec) -> Result<(), PermissionDenied>;
}
```

**Session management** (see [Session Management Principles](#323-session-management-principles)):

```rust
pub struct Session {
    id: SessionId,
    messages: Vec<ConversationMessage>,
    created_at: DateTime<Utc>,
    metadata: SessionMetadata,
}
```

### 4.2 API Module

#### 4.2.1 Provider Trait

See [Core Design Principles — Provider Extension](#341-provider-extension).

**Provider implementations:**

```rust
// ClawApi Provider (Anthropic)
pub struct ClawApiProvider {
    api_key: String,
    base_url: Url,
    http_client: reqwest::Client,
}

#[async_trait]
impl ApiClient for ClawApiProvider {
    async fn stream(&self, request: MessageRequest) -> Result<StreamingResponse, ApiError>;
    fn supported_models(&self) -> Vec<ModelInfo>;
}

// Xai Provider
pub struct XaiProvider {
    api_key: String,
    base_url: Url,
}

#[async_trait]
impl ApiClient for XAiProvider {
    async fn stream(&self, request: MessageRequest) -> Result<StreamingResponse, ApiError>;
    fn supported_models(&self) -> Vec<ModelInfo>;
}
```

### 4.3 Tools Module

Rust tools follow [Core Design Principles — Tool System Principles](#321-tool-system-principles).

**Tool list:**

| Tool name | Handler | Permission | Function |
|-----------|---------|------------|----------|
| `bash` | BashTool | DangerFullAccess | Run shell |
| `read_file` | ReadFileTool | ReadOnly | Read file |
| `write_file` | WriteFileTool | WorkspaceWrite | Write file |
| `edit_file` | EditFileTool | WorkspaceWrite | Edit file |
| `glob_search` | GlobSearchTool | ReadOnly | Glob search |
| `grep_search` | GrepSearchTool | ReadOnly | Content search |
| `WebFetch` | WebFetchTool | ReadOnly | Fetch URL |
| `WebSearch` | WebSearchTool | ReadOnly | Web search |
| `TodoWrite` | TodoWriteTool | WorkspaceWrite | Task management |
| `Agent` | AgentTool | DangerFullAccess | Sub-agent |
| `NotebookEdit` | NotebookEditTool | WorkspaceWrite | Jupyter edit |
| `PowerShell` | PowerShellTool | DangerFullAccess | PowerShell |

### 4.4 Commands Module

Rust commands follow [Core Design Principles — Command System Principles](#322-command-system-principles).

**Command list:**

| Command | Alias | Category | Function | Resumable |
|---------|-------|----------|----------|-----------|
| `/help` | - | Core | Show help | ✓ |
| `/status` | - | Core | Session status | ✓ |
| `/compact` | - | Core | Compact session | ✓ |
| `/model` | - | Core | Switch model | ✗ |
| `/permissions` | - | Core | Permission mode | ✗ |
| `/clear` | - | Core | Clear session | ✓ |
| `/cost` | - | Core | Token stats | ✓ |
| `/resume` | - | Session | Resume session | ✗ |
| `/export` | - | Session | Export session | ✓ |
| `/session` | - | Session | Session management | ✗ |
| `/config` | - | Workspace | View config | ✓ |
| `/memory` | - | Workspace | View memory | ✓ |
| `/init` | - | Workspace | Initialize project | ✓ |
| `/diff` | - | Workspace | Git diff | ✓ |
| `/branch` | - | Git | Branch management | ✗ |
| `/worktree` | - | Git | Worktree | ✗ |
| `/commit` | - | Git | Commit | ✗ |
| `/commit-push-pr` | - | Git | Commit + PR | ✗ |
| `/pr` | - | Git | PR management | ✗ |
| `/bughunter` | - | Automation | Bug scan | ✗ |
| `/ultraplan` | - | Automation | Deep planning | ✗ |
| `/plugin` | `plugins` | Automation | Plugin management | ✗ |

### 4.5 MCP Module

See [Core Design Principles — MCP Protocol Support](#342-mcp-protocol-support).

Rust-supported transports: `Stdio`, `Sse`, `Http`, `Ws`, `Sdk`, `ManagedProxy`.

### 4.6 Plugins Module

See [Core Design Principles — Plugin Hook System](#343-plugin-hook-system).

Rust supports full lifecycle hooks and Pre/Post tool interception.

### 4.7 LSP Module

Rust supports LSP (Language Server Protocol) integration for code analysis and context enrichment:

- `go_to_definition`: Go to definition
- `find_references`: Find references
- `collect_diagnostics`: Collect diagnostics
- `context_enrichment`: Context enrichment

### 4.8 Server Module (Web API)

#### 4.8.0 Design Goal: Web API Equivalent to CLI

```
CLI experience                         Web API experience
─────────────────────────────────────────────────────────────────────
nexus run                          ←→  POST /sessions/{id}/message (SSE)
nexus run --model claude-opus     ←→  Header: X-Model: claude-opus
nexus resume <session-id>         ←→  POST /sessions/{id}/resume
/compact                           ←→  POST /sessions/{id}/compact
/export                            ←→  GET /sessions/{id}/export
/config                            ←→  GET /config
/memory                            ←→  GET /memory
/git/commit                        ←→  POST /sessions/{id}/commands/commit
/plugins                           ←→  GET /plugins
nexus serve --port 8080           ←→  Standalone deployment
```

#### 4.8.1 HTTP Service Architecture

```rust
pub struct Server {
    config: ServerConfig,
    app_state: Arc<AppState>,
    listener: TcpListener,
}

pub struct ServerConfig {
    pub host: String,              // default "127.0.0.1"
    pub port: u16,                 // default 8080
    pub api_keys: Vec<String>,     // API key list
    pub cors_origins: Vec<String>, // CORS allowed origins
    pub request_timeout: Duration, // default 300s
    pub max_concurrent_sessions: usize,  // default 1000
    pub storage_path: PathBuf,     // Session storage directory
}

pub struct AppState {
    pub runtime: RuntimeRef,
    pub session_store: SessionStore,
    pub command_registry: CommandRegistryRef,
    pub tool_registry: ToolRegistryRef,
}

impl Server {
    pub async fn new(config: ServerConfig) -> Result<Self, ServerError>;
    pub async fn serve(self) -> Result<(), ServerError>;
    pub fn shutdown(&self) -> oneshot::Sender<()>;
}
```

#### 4.8.2 API Endpoint List

See [Appendix — API Endpoints Summary](#appendix-a-api-endpoint-summary).

#### 4.8.3 SSE Event Types

```rust
pub enum SseEvent {
    SessionStarted(SessionStarted),
    SessionEnded(SessionEnded),
    MessageStart(MessageStart),
    TextDelta(TextDelta),
    ToolUse(ToolUse),
    ToolStart(ToolStart),
    ToolEnd(ToolEnd),
    MessageEnd(MessageEnd),
    CommandStart(CommandStart),
    CommandOutput(CommandOutput),
    CommandEnd(CommandEnd),
    Error(Error),
    Warning(Warning),
    Info(Info),
    Ping(Ping),
    Done(Done),
}

pub struct SessionStarted {
    pub session_id: SessionId,
    pub model: String,
}

pub struct MessageStart {
    pub message_id: MessageId,
    pub role: Role,
}

pub struct TextDelta {
    pub message_id: MessageId,
    pub text: String,
}

pub struct ToolUse {
    pub message_id: MessageId,
    pub tool: String,
    pub input: Value,
    pub id: ToolUseId,
}

pub struct ToolStart {
    pub tool_use_id: ToolUseId,
    pub tool: String,
}

pub struct ToolEnd {
    pub tool_use_id: ToolUseId,
    pub tool: String,
    pub success: bool,
    pub result: Option<Value>,
    pub error: Option<String>,
    pub duration_ms: u64,
}

pub struct MessageEnd {
    pub message_id: MessageId,
    pub stop_reason: StopReason,
    pub usage: TokenUsage,
}

pub struct CommandStart {
    pub command: String,
    pub args: Vec<String>,
}

pub struct CommandOutput {
    pub command: String,
    pub output_type: OutputType,  // stdout | stderr | info
    pub content: String,
}

pub struct CommandEnd {
    pub command: String,
    pub success: bool,
    pub exit_code: Option<i32>,
}

pub struct Error {
    pub code: ErrorCode,
    pub message: String,
    pub details: Option<Value>,
}

pub struct Done {}
```

**SSE event types**: See [Appendix — SSE Event Types](#appendix-b-sse-event-types).

#### 4.8.4 CLI Equivalence Guarantees

The Rust version ensures Web API and CLI are fully equivalent:

```rust
// 1. Single Runtime instance — server and cli share the same Runtime
pub fn create_runtime(config: &Config) -> Arc<ConversationRuntime> {
    let runtime = RuntimeBuilder::new()
        .with_tools(tools::builtin())
        .with_commands(commands::all())
        .with_mcp(mcp::discover().await?)
        .with_permissions(permissions::from_config(config))
        .build();
    Arc::new(runtime)
}

// 2. Unified error handling — CLI and API return errors in the same shape
pub enum HarnessError {
    Tool(ToolError),
    Session(SessionError),
    // ... same error types for API and CLI
}

// 3. Unified logging/trace
pub fn with_tracing(runtime: RuntimeRef, request_id: String) -> RuntimeRef {
    runtime.with_span(tracing::info_span!("request", request_id = %request_id))
}
```

#### 4.8.5 Client SDK

See [Appendix — Client SDK](#appendix-d-client-sdks).

---

### 4.9 OAuth Module

See [Core Design Principles — Extensibility](#34-extensibility-design-principles) and [Core Design Principles — Security](#35-security-design-principles).

Rust supports OAuth 2.0 + PKCE for third-party app authorization.

### 4.10 Permissions Module

See [Core Design Principles — Permission Design](#33-permission-design-principles).

Rust permission system:

```rust
pub struct PermissionPolicy {
    default_mode: PermissionMode,
    tool_overrides: HashMap<String, PermissionMode>,
}
```

---

## 5. Data Flow Design `[General]`

### 5.1 End-to-End Request Flow

```
User input (CLI / HTTP API / SDK)
         │
         ▼
┌─────────────────────────────────────────┐
│  Command Preprocessor                   │
│  Detect /command, +skill, mcp_ prefix   │
└────────────────┬────────────────────────┘
                 │
         ┌───────▼───────┐
         │ Slash command? │
         └───────┬───────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
       Yes               No
        │                 │
        ▼                 ▼
┌───────────────┐  ┌─────────────────────┐
│ CommandHandler │  │ ConversationRuntime │
└───────────────┘  └─────────────────────┘
        │                       │
        └───────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │ Build MessageRequest │
         │  system_prompt +     │
         │  messages            │
         └──────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │  ApiClient.stream() │
         └──────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │ Parse AssistantEvent │
         │  - text_delta       │
         │  - tool_use         │
         │  - usage            │
         └──────────┬───────────┘
                    ▼
         ┌─────────────────────┐
         │  tool_call?         │
         └──────────┬───────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
       Yes                     No
        │                       │
        ▼                       ▼
┌───────────────┐      ┌─────────────────┐
│ PermissionCheck│      │ Return to user  │
└───────────────┘      └─────────────────┘
        │
        ▼
┌───────────────┐
│ ToolRegistry  │
│ .execute()    │
└───────────────┘
        │
        ▼
┌───────────────┐
│ HookExecutor  │
│ post_tool_use │
└───────────────┘
        │
        ▼
┌───────────────┐
│ Append result │
│ to messages   │
└───────────────┘
        │
        ▼
    Continue loop (back to ApiClient.stream)
```

---

## 6. Project Structure `[Rust reference]`

```
nexus-harness/
├── Cargo.toml                 # Workspace configuration
├── crates/
│   ├── api/                   # API Provider layer
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── client.rs      # Trait definitions
│   │       ├── errors.rs
│   │       └── providers/
│   │           ├── mod.rs
│   │           ├── claw.rs    # Anthropic
│   │           └── xai.rs      # xAI
│   │
│   ├── runtime/               # Core runtime
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── conversation.rs
│   │       ├── session.rs
│   │       ├── compaction.rs
│   │       ├── permissions.rs
│   │       ├── tools/
│   │       │   ├── mod.rs
│   │       │   ├── bash.rs
│   │       │   ├── file.rs
│   │       │   ├── web.rs
│   │       │   └── agent.rs
│   │       └── mcp/
│   │           ├── mod.rs
│   │           ├── transport.rs
│   │           └── client.rs
│   │
│   ├── tools/                 # Tool framework
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── registry.rs
│   │       ├── spec.rs
│   │       └── result.rs
│   │
│   ├── commands/              # Slash commands
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── registry.rs
│   │       ├── context.rs
│   │       ├── output.rs
│   │       └── commands/
│   │           ├── mod.rs
│   │           ├── core.rs
│   │           ├── git.rs
│   │           └── automation.rs
│   │
│   ├── plugins/               # Plugin system
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── hooks.rs
│   │       ├── manifest.rs
│   │       └── executor.rs
│   │
│   ├── lsp/                  # LSP integration
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── manager.rs
│   │       ├── client.rs
│   │       └── types.rs
│   │
│   ├── server/               # HTTP Server
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── routes.rs
│   │       ├── handlers.rs
│   │       └── sse.rs
│   │
│   ├── oauth/                # OAuth 2.0
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── flow.rs
│   │       ├── pkce.rs
│   │       └── store.rs
│   │
│   └── cli/                  # CLI entrypoint
│       ├── Cargo.toml
│       └── src/
│           ├── main.rs
│           ├── repl.rs
│           └── commands.rs
│
├── tests/
│   ├── integration/
│   └── unit/
│
└── examples/
    └── basic.rs
```

---

## 7. Dependency Versions `[Rust reference]`

```toml
[workspace]
rust-version = "1.75"
edition = "2021"

[workspace.dependencies]
tokio = { version = "1.36", features = ["full"] }
axum = "0.7"
reqwest = { version = "0.12", features = ["json"] }
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
thiserror = "1.0"
anyhow = "1.0"
tracing = "0.1"
futures = "0.3"
async-trait = "0.1"
tower = "0.4"
tower-http = { version = "0.5", features = ["cors"] }
tokio-util = { version = "0.7", features = ["io"] }
```

---

## 8. API Design `[Rust reference]`

### 8.1 CLI Mode

```bash
# Interactive REPL
nexus run

# Specify model
nexus run --model claude-opus-4-6

# Resume session
nexus resume <session-id>

# Execute command
nexus -- <message>
```

### 8.2 HTTP API Mode

```bash
# Start server
nexus serve --port 8080

# Create session
curl -X POST http://localhost:8080/sessions

# Send message
curl -X POST http://localhost:8080/sessions/{id}/message \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello"}'

# SSE streaming response
curl http://localhost:8080/sessions/{id}/events
```

---

## 9. Configuration Design `[Rust reference]`

### 9.1 Config File Locations

| Priority | Path |
|----------|------|
| Project-level | `./.nexus/config.toml` |
| User-level | `~/.nexus/config.toml` |
| Default | Built-in defaults |

### 9.2 Config Structure

```toml
[general]
model = "claude-opus-4-6"
permission_mode = "ask"

[api]
provider = "claw"
# provider = "xai"

[api.claw]
api_key = "${ANTHROPIC_API_KEY}"
base_url = "https://api.anthropic.com"

[api.xai]
api_key = "${XAI_API_KEY}"
base_url = "https://api.x.ai/v1"

[mcp]
enabled = true
servers = [
    { name = "filesystem", transport = "stdio", command = "npx", args = ["-y", "@modelcontextprotocol/server-filesystem", "."] }
]

[lsp]
servers = [
    { name = "rust", command = "rust-analyzer", args = [] }
]

[plugins]
enabled = true
directory = "~/.nexus/plugins"
```

---

## 10. Error Handling `[Rust reference]`

### 10.1 Error Types

```rust
pub enum HarnessError {
    Api(ApiError),
    Tool(ToolError),
    Permission(PermissionDenied),
    Session(SessionError),
    Mcp(McpError),
    Plugin(PluginError),
    Io(IoError),
    Config(ConfigError),
}
```

### 10.2 Error Recovery Strategy

| Error Type | Strategy |
|------------|----------|
| Transient API failure | Retry (exponential backoff) |
| Tool execution failure | Return error to the model so it can retry or try another approach |
| Permission denied | Return denial message to the model |
| Session lost | Return error; prompt user to recover |

---

## 11. Testing Strategy `[General]`

### 11.1 Test Layering

```
┌─────────────────────────────────────┐
│         E2E Tests (full flow)       │
├─────────────────────────────────────┤
│    Integration Tests (cross-module) │
├─────────────────────────────────────┤
│         Unit Tests (single module)  │
└─────────────────────────────────────┘
```

### 11.2 Test Cases

**Runtime Tests**
- Full agent loop
- Context compaction
- Permission checks

**API Tests**
- Provider streaming response parsing
- Error retry logic

**Tools Tests**
- Correct execution of each tool
- Timeout and error handling

**Commands Tests**
- Command parsing
- Command execution

---

## 12. Performance Targets `[Rust reference]`

| Metric | Target |
|--------|--------|
| Cold start time | < 500ms |
| Tool execution latency | < 100ms (local) |
| Memory footprint | < 100MB (idle) |
| Concurrent sessions | > 1000 |

---

## 13. Security Considerations `[General]`

1. **Permission isolation**: Tool execution is based on Permission level
2. **Hook interception**: Pre/Post interception of tool execution
3. **MCP sandbox**: Isolation of external tool servers
4. **OAuth credentials**: Stored securely on the local filesystem
5. **Input validation**: All tool inputs validated against JSON Schema

---

## 14. Review Feedback and Improvements (v1.1) `[Rust reference]`

> Review date: 2026-04-03 | Review: 3-way concurrent agent review

### 14.1 Review Summary

| Dimension | Score | Main Issues |
|-----------|-------|-------------|
| Architecture | 7/10 | Circular dependency risk |
| Runtime core | 7/10 | Compaction algorithm undefined |
| Extensibility | 6/10 | Plugin needs refactor |
| Security | 6.5/10 | Multiple security gaps |
| **Overall** | **7/10** | - |

### 14.2 High-Priority Fixes

#### Issue 1: Circular dependency
**Current state**: `runtime ↔ tools/commands` forms a circular dependency
**Fix**: Introduce intermediate traits to break the cycle

```rust
pub trait ToolExecutor: Send + Sync {
    async fn execute(&self, name: &str, input: Value) -> Result<ToolResult, ToolError>;
}

pub trait RuntimeCommands: Send + Sync {
    fn execute_tool(&self, name: &str, input: Value) -> Result<ToolResult, ToolError>;
    fn get_session(&self) -> SessionRef;
}
```

#### Issue 2: Plugin Hook string list
**Current state**: `Vec<String>` instead of real function registration
**Fix**: Use trait objects

```rust
pub trait PreToolHook: Send + Sync {
    async fn call(&self, ctx: &HookContext) -> Result<HookDecision, HookError>;
}

pub trait PostToolHook: Send + Sync {
    async fn call(&self, ctx: &HookContext, result: &ToolResult) -> Result<(), HookError>;
}

pub enum HookDecision {
    Allow,
    Deny { reason: String },
    Modify(Value),
}

pub struct PluginHooks {
    pub pre_tool_use: Vec<Arc<dyn PreToolHook>>,
    pub post_tool_use: Vec<Arc<dyn PostToolHook>>,
}
```

#### Issue 3: BashTool command blocklist
**Current state**: `DangerFullAccess` permission is too broad
**Fix**: Add blocklist and path validation

```rust
pub struct BashTool {
    timeout: Duration,
    working_dir: PathBuf,
    blocked_commands: Vec<String>,
}

impl BashTool {
    const BLOCKED: &[&str] = &[
        "rm -rf /", "dd", ":(){:|:&};:", "mkfs",
        "shutdown", "reboot", "init 0",
    ];

    pub fn validate_command(cmd: &str) -> Result<(), ToolError> {
        let lower = cmd.to_lowercase();
        for blocked in Self::BLOCKED {
            if lower.contains(blocked) {
                return Err(ToolError::CommandBlocked(blocked.to_string()));
            }
        }
        Ok(())
    }
}

pub fn validate_path(path: &Path, allowed_dir: &Path) -> Result<PathBuf, ToolError> {
    let canonical = path.canonicalize()
        .map_err(|_| ToolError::PathTraversal)?;
    if !canonical.starts_with(allowed_dir) {
        return Err(ToolError::PathTraversal);
    }
    Ok(canonical)
}
```

#### Issue 4: HTTP Server authentication
**Current state**: Endpoints have no authentication
**Fix**: Add API Key authentication middleware

```rust
pub struct ApiKeyAuth {
    valid_keys: HashSet<String>,
}

impl Middleware<Router> for ApiKeyAuth {
    fn layer(&self, inner: Router) -> Router {
        inner.layer(from_fn_with_state(self.clone(), auth_check))
    }
}

async fn auth_check(
    state: State<ApiKeyAuth>,
    request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    match request.headers().get("X-API-Key") {
        Some(key) if state.valid_keys.contains(key) => Ok(next.run(request).await),
        _ => Err(StatusCode::UNAUTHORIZED),
    }
}
```

### 14.3 Medium-Priority Fixes

#### Issue 5: OAuth credential encryption
```rust
use ring::aead::{Aad, LessSafeKey, Nonce, UnboundKey, AES_256_GCM};

pub struct CredentialStore {
    path: PathBuf,
    key: LessSafeKey,
}

impl CredentialStore {
    pub fn new(path: PathBuf, master_key: &[u8; 32]) -> Result<Self, CryptoError> {
        let unbound = UnboundKey::new(&AES_256_GCM, master_key)
            .map_err(|_| CryptoError::InvalidKey)?;
        Ok(Self {
            path,
            key: LessSafeKey::new(unbound),
        })
    }

    pub fn save(&self, tokens: &OAuthTokenSet) -> Result<(), IoError> {
        let plaintext = serde_json::to_vec(tokens).map_err(|_| IoError)?;
        let nonce = Nonce::assume_unique_for_key([0u8; 12]);
        let sealed = self.key.seal_in_place_append_tag(nonce, Aad::empty(), &mut plaintext)
            .map_err(|_| IoError)?;
        std::fs::write(&self.path, sealed)?;
        Ok(())
    }
}
```

#### Issue 6: Compaction algorithm definition
```rust
pub struct CompactionStrategy {
    pub target_tokens: usize,
    pub preserve_system: bool,
    pub preserve_last_n: usize,
    pub min_messages_to_keep: usize,
}

impl Session {
    pub fn compact(&mut self, strategy: &CompactionStrategy) -> Result<(), RuntimeError> {
        let current_tokens = self.estimate_tokens();
        if current_tokens <= strategy.target_tokens {
            return Ok(());
        }

        let messages: Vec<_> = self.messages.drain(..).collect();

        // 1. Split system messages (always kept) and the last N messages
        let (system, rest): (Vec<_>, Vec<_>) = messages
            .into_iter()
            .partition(|m| m.role == Role::System);

        let rest_len = rest.len();
        let preserve_n = strategy.preserve_last_n.min(rest_len);
        let (candidates, recent) = rest.split_at(rest_len - preserve_n);

        // 2. Drop oldest messages until target token count is reached
        let mut kept: Vec<_> = system;
        kept.extend_from_slice(recent);

        let mut sorted_candidates: Vec<_> = candidates.to_vec();
        sorted_candidates.sort_by(|a, b| a.priority.cmp(&b.priority));

        for msg in sorted_candidates.into_iter().rev() {
            if Self::estimate_tokens_for_messages(&kept) <= strategy.target_tokens {
                break;
            }
            if kept.len() <= strategy.min_messages_to_keep {
                break;
            }
            continue;
            kept.push(msg);
        }

        // 3. Reorder chronologically
        kept.sort_by_key(|m| m.created_at);
        self.messages = kept;
        Ok(())
    }
}
```

> **Note**: The above is a simplified Rust reference. The Go version uses a different compaction strategy—calling an LLM to summarize early dialogue rather than simply deleting messages. See section 15.x.

#### Issue 7: max_iterations behavior definition
```rust
impl<C, T> ConversationRuntime<C, T> {
    async fn execute_tool_loop(
        &mut self,
        messages: &mut Vec<ConversationMessage>,
    ) -> Result<Option<ConversationMessage>, RuntimeError> {
        for iteration in 0..self.max_iterations {
            // ... execution logic ...

            if !self.should_continue(&messages) {
                break;
            }
        }

        Err(RuntimeError::MaxIterationsExceeded {
            limit: self.max_iterations,
            last_message: messages.last().cloned(),
        })
    }
}

#[derive(Error, Debug)]
pub enum RuntimeError {
    #[error("Max iterations {limit} exceeded, possible loop detected")]
    MaxIterationsExceeded {
        limit: usize,
        last_message: Option<ConversationMessage>,
    },
}
```

### 14.4 Next Steps (Revised)

1. [x] Technical design review ✓
2. [ ] **Resolve circular dependency** (high priority)
3. [ ] **Refactor Plugin Hooks** (high priority)
4. [ ] **Add security mechanisms** (Bash blocklist, path validation, API auth)
5. [ ] Create project skeleton
6. [ ] Implement core Runtime
7. [ ] Implement API Provider
8. [ ] Implement built-in tools (with hardening)
9. [ ] Implement HTTP Server (with auth)
10. [ ] Implement MCP support
11. [ ] Implement Commands
12. [ ] Implement Plugins (new Hook API)
13. [ ] Implement LSP
14. [ ] Integration tests
15. [ ] Performance tuning

---

## 15. Claw-Go Detailed Design

> HTTP-first lightweight Agent Harness, providing upstream with high-speed streamed responses

### 15.1 Project Positioning

| Dimension | NexusHarness (Rust) | NexusHarness-Go |
|-----------|---------------------|-----------------|
| **Positioning** | Full-featured, production-grade | HTTP-first, lightweight, high-speed |
| **CLI** | Full CLI experience | Basic CLI |
| **Web API** | Full REST + SSE | Optimized SSE streaming output |
| **Core strengths** | Complete architecture, extensible | **Streaming speed, simple deployment** |
| **Target users** | IDE integration, power users | **Web applications, API consumers** |

**Design philosophy**: The Go version is not a replacement for the Rust version, but an **optimized implementation for HTTP API scenarios**.

### 15.2 Core Advantages

```
┌─────────────────────────────────────────────────────────────────┐
│                    Why choose the Go version?                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Sub-millisecond cold start  - faster compile/deploy than Rust│
│  2. Native concurrency model    - goroutines more intuitive     │
│  3. Simpler dependencies        - single binary, zero runtime   │
│  4. SSE optimization            - chunked transfer, low-latency │
│  5. Rich ecosystem              - mature HTTP libs, cloud-native│
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 15.3 System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          Claw-Go                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐     ┌─────────────────────────────────────┐  │
│  │   CLI       │     │       HTTP Server (gin)              │  │
│  │  (claw)     │     │                                     │  │
│  └──────┬──────┘     │  ┌─────────────────────────────────┐ │  │
│         │            │  │  POST /v1/sessions              │ │  │
│         │            │  │  POST /v1/sessions/:id/messages │ │  │
│         │            │  │  GET  /v1/sessions/:id/messages │ │  │
│         │            │  │  GET  /health                   │ │  │
│         │            │  └─────────────────────────────────┘ │  │
│         │            └──────────────┬──────────────────────┘  │
│         │                           │                          │
│  ┌──────▼───────────────────────────▼───────────────────────┐ │
│  │                     Runtime Core                          │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │ │
│  │  │  Engine      │  │   Session    │  │   Tools     │   │ │
│  │  │  Run/Stream  │  │   Store     │  │   Registry  │   │ │
│  │  └──────────────┘  └──────┬───────┘  └──────────────┘   │ │
│  │                           │                               │ │
│  │  ┌──────────────┐  ┌─────▼────────┐  ┌──────────────┐   │ │
│  │  │ Permissions  │  │  WorkDir     │  │  Provider    │   │ │
│  │  │   Engine     │  │  Manager     │  │  Factory     │   │ │
│  │  └──────────────┘  │ (git wt)     │  └──────────────┘   │ │
│  │                    └──────────────┘                       │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                     Provider Layer                        │ │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐         │ │
│  │  │ Anthropic  │  │   xAI      │  │  OpenAI   │         │ │
│  │  │[Implemented]│  │ [Planned]  │  │[Implemented]│        │ │
│  │  └────────────┘  └────────────┘  └────────────┘         │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 15.4 Architecture Decisions

| # | Decision | Options | Conclusion | Rationale |
|---|----------|---------|------------|-----------|
| 1 | Tenant model | Single-tenant / multi-tenant | **Multi-tenant** | Web service must support multiple users; API key isolation |
| 2 | API mode | Stateless chat/completions / Session-only | **Session-only** | Tool execution is stateful; git worktree bound to session |
| 3 | Working directory | Fixed dir / per-session isolation | **data dir + git worktree** | Default data directory; with repo integration use bare repo + worktree isolation |
| 4 | HTTP framework | net/http / chi / gin | **gin** | Rich ecosystem, mature middleware |

#### 15.4.1 Session-only API Mode

All interaction is session-based; there is no standalone stateless `chat/completions` endpoint. Reasons:

- Tool execution (bash, file read/write) is stateful and must be bound to a working directory
- Multi-turn tool loops require the server to maintain message history
- Git worktree is tied to session lifetime
- **SSE response format remains compatible with OpenAI chunk format**, so clients can parse with existing SDKs

```
Client                                    Server
  │                                         │
  │── POST /v1/sessions ──────────────────▶ │  Create Session
  │◀── {"id":"sess_xxx", "model":...} ──── │  (allocate worktree)
  │                                         │
  │── POST /v1/sessions/sess_xxx/messages ─▶│  Send message
  │   {"content": "Help me review the code"}│
  │                                         │
  │◀── SSE: event: message_delta ────────── │  Streamed response
  │◀── SSE: event: tool_use ─────────────── │  (OpenAI chunk compatible)
  │◀── SSE: event: tool_result ──────────── │
  │◀── SSE: event: message_delta ────────── │
  │◀── SSE: event: done ────────────────── │
  │                                         │
  │── DELETE /v1/sessions/sess_xxx ────────▶│  Destroy Session
  │                                         │  (clean up worktree)
```

#### 15.4.2 Working Directory and Git Worktree

```
$CLAW_DATA_DIR/                          # default ~/.claw/data/
├── repos/                               # bare repo cache
│   └── <repo-hash>/                     # git clone --bare
│       ├── HEAD
│       ├── objects/
│       └── refs/
│
├── worktrees/                           # per-session isolated working dirs
│   ├── sess_abc123/                     # git worktree add
│   │   ├── .git                         # → points to bare repo
│   │   ├── src/
│   │   └── ...
│   └── sess_def456/
│       └── ...
│
└── sessions/                            # session metadata persistence
    ├── sess_abc123.json
    └── sess_def456.json
```

**Workflow:**

1. `POST /v1/sessions` with `repo_url` → clone bare repo (or reuse cache) → `git worktree add` to create an isolated directory
2. All tool execution within the session is confined to that worktree directory
3. `DELETE /v1/sessions/:id` → `git worktree remove` → clean up directory
4. Without `repo_url`, create an empty directory under `$CLAW_DATA_DIR/worktrees/:id/`

```go
type CreateSessionRequest struct {
    Model   string `json:"model,omitempty"`
    RepoURL string `json:"repo_url,omitempty"`  // gitlab/github repo URL
    Branch  string `json:"branch,omitempty"`     // default main
}
```

### 15.5 HTTP API Design (Session-only)

> All interaction is session-based; no standalone stateless chat/completions endpoint.
> SSE response format is compatible with OpenAI chunk format; clients can parse with existing SDKs.

#### 15.5.1 Endpoint List (Phased)

**Phase 1 — Core (MVP)**

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/v1/sessions` | Create session (optional `repo_url`) |
| `GET` | `/v1/sessions` | List sessions |
| `GET` | `/v1/sessions/:id` | Get session details |
| `DELETE` | `/v1/sessions/:id` | Delete session (clean up worktree) |
| `POST` | `/v1/sessions/:id/messages` | Send message (SSE streamed response) |
| `GET` | `/v1/sessions/:id/messages` | Get message history |
| `GET` | `/v1/models` | Available models |
| `GET` | `/health` | Health check |

**Phase 2 — Extensions**

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/v1/sessions/:id/resume` | Resume session |
| `GET` | `/v1/tools` | Tool list |
| `POST` | `/v1/tools/:name/execute` | Execute tool directly (session required) |
| `GET` | `/v1/sessions/:id/permissions` | Get session permissions |
| `POST` | `/v1/sessions/:id/permissions` | Set session permissions |
| `GET` | `/metrics` | Prometheus metrics |

**Phase 3 — Advanced `[implemented]`**

| Method | Path | Purpose |
|--------|------|---------|
| `GET/POST/DELETE` | `/v1/skills/*` | Skill management |
| `GET` | `/ws/:id` | WebSocket connection |

#### 15.5.2 Core Request/Response Examples

**Create session**
```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "repo_url": "https://gitlab.example.com/team/project.git",
    "branch": "main"
  }'
```

```json
{
  "id": "sess_a1b2c3d4",
  "model": "claude-sonnet-4-5",
  "created_at": "2026-04-03T10:00:00Z",
  "cwd": "/data/worktrees/sess_a1b2c3d4",
  "repo_url": "https://gitlab.example.com/team/project.git",
  "branch": "main"
}
```

**Send message (SSE stream)**
```bash
curl -N -X POST http://localhost:8080/v1/sessions/sess_a1b2c3d4/messages \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"content": "Help me see what files are in the current directory"}'
```

**SSE response stream**
```
event: message_start
data: {"message_id":"msg_001","role":"assistant"}

event: message_delta
data: {"text":"Sure, let me"}

event: tool_use
data: {"id":"tu_001","name":"bash","input":{"command":"ls -la"}}

event: tool_start
data: {"tool_use_id":"tu_001","name":"bash"}

event: tool_result
data: {"tool_use_id":"tu_001","content":"total 24\ndrwxr-xr-x  7 user staff 224 ...","is_error":false}

event: message_delta
data: {"text":"The current directory has the following files:\n- src/\n- README.md"}

event: message_end
data: {"message_id":"msg_001","stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":50}}

event: done
data: {}
```

#### 15.5.3 SSE Event Types

```go
type SSEEventType string

const (
    EventMessageStart SSEEventType = "message_start"   // Message start
    EventMessageDelta SSEEventType = "message_delta"   // Text delta
    EventMessageEnd   SSEEventType = "message_end"     // Message end (includes usage)
    EventToolUse      SSEEventType = "tool_use"        // Model requests tool call
    EventToolStart    SSEEventType = "tool_start"      // Tool execution start
    EventToolResult   SSEEventType = "tool_result"     // Tool execution result
    EventToolError    SSEEventType = "tool_error"      // Tool execution error
    EventError        SSEEventType = "error"           // System error
    EventDone         SSEEventType = "done"            // Request complete
)
```

#### 15.5.4 Standardized Error Responses

All API errors return a unified format:

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "session not found: sess_xxx",
    "code": "session_not_found"
  }
}
```

| HTTP Status | type | Description |
|-------------|------|-------------|
| 400 | `invalid_request_error` | Invalid request parameters |
| 401 | `authentication_error` | Invalid API key |
| 403 | `permission_error` | Insufficient permissions |
| 404 | `not_found_error` | Session/Tool not found |
| 429 | `rate_limit_error` | Too many requests |
| 500 | `internal_error` | Internal server error |
| 502 | `provider_error` | Upstream Provider API error |

### 15.6 Core Implementation Design

#### 15.6.1 Project Layout

```
claw-go/
├── cmd/
│   └── claw/
│       └── main.go           # CLI + server entry
│
├── internal/
│   ├── server/
│   │   ├── server.go        # HTTP Server
│   │   ├── handlers.go      # API handlers
│   │   ├── middleware.go     # Middleware
│   │   ├── websocket.go     # WebSocket handler
│   │   ├── sse.go           # SSE writer
│   │   ├── metrics.go       # Prometheus metrics
│   │   └── ratelimit.go     # Rate limiting
│   │
│   ├── runtime/
│   │   ├── engine.go        # Core runtime
│   │   └── stream.go        # Streaming runtime
│   │
│   ├── provider/
│   │   ├── anthropic/       # Anthropic HTTP+SSE
│   │   └── openai/          # OpenAI HTTP+SSE
│   │
│   ├── tools/               # Built-in tools
│   ├── session/             # Session store + GC
│   ├── permissions/         # Permission engine
│   ├── skill/               # Skill system
│   ├── sandbox/             # Tool sandbox
│   ├── sysprompt/           # System prompt builder
│   ├── workdir/             # Git worktree manager
│   ├── config/              # Configuration
│   └── app/                 # Application assembly (DI)
│
├── pkg/
│   ├── types/               # Shared types
│   └── sdk/                 # Go SDK client
│
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

#### 15.6.2 Core Types

```go
type Runtime struct {
    apiClient   API
    tools       *ToolRegistry
    sessions    *SessionManager
    config      *Config
}

type API interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error)
}

type Session struct {
    ID        string
    Model     string
    Messages  []ChatMessage
    CreatedAt time.Time
    UpdatedAt time.Time
    Metadata  map[string]interface{}
}

type SessionManager struct {
    sessions  sync.Map
    maxCount int
    ttl      time.Duration
}
```

#### 15.6.3 Streaming Core: ChatLoop

```go
func (r *Runtime) ChatStream(ctx context.Context, sessionID string, req *ChatRequest) (<-chan *StreamEvent, error) {
    out := make(chan *StreamEvent, 100)

    go func() {
        defer close(out)

        session, err := r.sessions.Get(sessionID)
        if err != nil {
            out <- &StreamEvent{Type: EventError, Error: err.Error()}
            return
        }

        messages := append(session.Messages, req.Messages...)
        toolSpecs := r.tools.Specs()

        // Multi-turn tool loop: synchronous; complete all tool calls per round before the next
        for turn := 0; turn < r.maxTurns; turn++ {
            reader, err := r.apiClient.Stream(ctx, &MessageRequest{
                Model:    req.Model,
                Messages: messages,
                Tools:    toolSpecs,
            })
            if err != nil {
                out <- &StreamEvent{Type: EventError, Error: err.Error()}
                return
            }

            var toolCalls []*ToolCall

            // Consume all streamed events; forward to client in real time
            for {
                event, err := reader.Next()
                if err == io.EOF {
                    break
                }
                if err != nil {
                    out <- &StreamEvent{Type: EventError, Error: err.Error()}
                    return
                }
                switch event.Type {
                case StreamEventMessageDelta:
                    out <- &StreamEvent{Type: EventChatDelta, Text: event.Text}
                case StreamEventToolCall:
                    toolCalls = append(toolCalls, event.ToolCall)
                    out <- &StreamEvent{Type: EventToolUse, ToolCall: event.ToolCall}
                case StreamEventUsage:
                    out <- &StreamEvent{Type: EventUsage, Usage: event.Usage}
                case StreamEventError:
                    out <- &StreamEvent{Type: EventError, Error: event.Error.Error()}
                    return
                }
            }
            reader.Close()

            // No tool calls → conversation ends
            if len(toolCalls) == 0 {
                break
            }

            // Execute all tools synchronously; append results then continue
            for _, call := range toolCalls {
                out <- &StreamEvent{Type: EventToolStart, ToolCall: call}
                result, execErr := r.tools.Execute(ctx, call.Name, call.Input)
                if execErr != nil {
                    out <- &StreamEvent{Type: EventToolError, ToolCallID: call.ID, Error: execErr.Error()}
                    messages = append(messages, toolErrorMessage(call, execErr))
                } else {
                    out <- &StreamEvent{Type: EventToolResult, ToolCallID: call.ID, Content: result.Content}
                    messages = append(messages, toolResultMessage(call, result))
                }
            }
        }

        // Persist session
        session.Messages = messages
        r.sessions.Save(ctx, session)

        out <- &StreamEvent{Type: EventDone}
    }()

    return out, nil
}
```

#### 15.6.4 Low-latency SSE Encoding

```go
type SSEWriter struct {
    w       http.ResponseWriter
    flusher http.Flusher
    buf     bytes.Buffer
}

func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, _ := w.(http.Flusher)
    return &SSEWriter{w: w, flusher: flusher}
}

func (s *SSEWriter) WriteEvent(event string, data interface{}) error {
    s.buf.Reset()
    fmt.Fprintf(&s.buf, "event: %s\ndata: ", event)
    if err := json.NewEncoder(&s.buf).Encode(data); err != nil {
        return err
    }
    s.buf.WriteByte('\n')

    if _, err := s.w.Write(s.buf.Bytes()); err != nil {
        return err
    }
    if s.flusher != nil {
        s.flusher.Flush()
    }
    return nil
}
```

> Reuse `bytes.Buffer` to reduce allocations; `Flush` after each write so the client receives data in real time.

### 15.7 Comparison with the Rust Version

| Feature | Rust version | Go version |
|---------|-------------|------------|
| **Concurrency** | async/await (Tokio) | goroutine + channel |
| **Memory** | Zero GC, ownership | GC, but fast enough |
| **Cold start** | ~100ms | ~10ms |
| **Binary size** | ~20MB | ~15MB |
| **CLI** | Full REPL | Basic CLI |
| **SSE latency** | Low | Very low |
| **API compatibility** | Proprietary format | **OpenAI-compatible** |
| **MCP** | Full | Basic |
| **Plugin system** | Full | None |
| **LSP** | Yes | No |
| **OAuth** | Full | Simplified |
| **Use cases** | IDE integration | **Web API** |

### 15.8 Streaming Runtime Design `[implemented]`

> This is the core design for the web version—turning synchronous `RunPrompt` into a streamed, event-driven model.

#### 15.8.1 Problem: Synchronous vs Streamed

Today `Engine.RunPrompt()` blocks synchronously until the entire multi-turn tool loop finishes. That is acceptable for CLI REPL, but the Web API must push every delta in real time.

```
Synchronous (CLI)                        Streamed (Web)
══════════════                           ══════════════

RunPrompt() ─── block ──── return         RunPromptStream() ─── chan ──── SSE
    internally: Stream → accumulate            internally: Stream → forward per event
          Tool → execute                              Tool → push execution state
          Stream → accumulate                       Stream → forward per event
          return PromptResult                        close(chan)
```

#### 15.8.2 Streaming Engine Interface Extension

```go
type Engine interface {
    // ... existing methods ...
    RunPromptStream(ctx context.Context, sessionID, prompt string) (<-chan StreamEvent, error)
}

type StreamEvent struct {
    Type       StreamEventType
    Text       string          // EventTextDelta
    ToolCall   *types.ToolCall // EventToolUse / EventToolStart
    ToolResult *types.ToolResult // EventToolResult
    ToolCallID string          // EventToolError
    Error      error           // EventError
    Usage      *types.Usage    // EventUsage
    Message    *types.Message  // EventMessageEnd
}

type StreamEventType string

const (
    EventTextDelta   StreamEventType = "text_delta"
    EventToolUse     StreamEventType = "tool_use"
    EventToolStart   StreamEventType = "tool_start"
    EventToolResult  StreamEventType = "tool_result"
    EventToolError   StreamEventType = "tool_error"
    EventUsage       StreamEventType = "usage"
    EventMessageEnd  StreamEventType = "message_end"
    EventError       StreamEventType = "error"
    EventDone        StreamEventType = "done"
)
```

#### 15.8.3 Unifying Streamed and Synchronous

```go
// Synchronous path reuses streaming implementation; collects all events then returns
func (e *engine) RunPrompt(ctx context.Context, sessionID, prompt string) (*PromptResult, error) {
    ch, err := e.RunPromptStream(ctx, sessionID, prompt)
    if err != nil {
        return nil, err
    }
    return collectStreamResult(ch)
}
```

#### 15.8.4 Cancellation and Interruption

```go
// HTTP handler propagates cancellation via context
func handleChat(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithCancel(r.Context())
    defer cancel()

    ch, err := engine.RunPromptStream(ctx, sessionID, prompt)
    // ...
    // Client disconnect → r.Context().Done() → cancel() → propagates to provider and tools
}
```

### 15.9 HTTP Server Architecture `[implemented]`

#### 15.9.1 Server Structure

```go
type Server struct {
    engine   runtime.Engine
    sessions session.Store
    config   ServerConfig
    router   http.Handler
    server   *http.Server
}

type ServerConfig struct {
    Host              string        // default "0.0.0.0"
    Port              int           // default 8080
    APIKeys           []string      // API key list
    ReadTimeout       time.Duration // default 30s
    WriteTimeout      time.Duration // default 300s (streamed responses need longer)
    MaxConcurrent     int           // Max concurrent sessions, default 100
    ShutdownTimeout   time.Duration // Graceful shutdown timeout, default 30s
}
```

#### 15.9.2 Middleware Stack

```
Request in
    │
    ▼
┌─────────────────┐
│   Recovery       │  panic recovery, return 500
├─────────────────┤
│   RequestID      │  generate X-Request-ID
├─────────────────┤
│   Logger         │  structured logs (method, path, status, latency)
├─────────────────┤
│   Metrics        │  Prometheus request counters/histograms
├─────────────────┤
│   CORS           │  cross-origin config
├─────────────────┤
│   Auth           │  Bearer token validation
├─────────────────┤
│   RateLimit      │  per-key rate limit (optional)
├─────────────────┤
│   Handler        │  business logic
└─────────────────┘
```

#### 15.9.3 Core Handler: SSE Chat

```go
func (s *Server) handleSendMessage(c *gin.Context) {
    // Parse request, validate session, then:
    if streaming {
        s.handleStreamMessage(c, id, req.Content)
        return
    }
    // Non-streaming fallback
    result, err := s.engine.RunPrompt(c.Request.Context(), id, req.Content)
    // ...
}

func (s *Server) handleStreamMessage(c *gin.Context, sessionID, content string) {
    ch, err := s.engine.RunPromptStream(c.Request.Context(), sessionID, content)
    // ...
    sse := NewSSEWriter(c.Writer)
    for event := range ch {
        sse.WriteEvent(string(event.Type), event)
    }
}
```

#### 15.9.4 Graceful Shutdown

```go
func (s *Server) ListenAndServe() error {
    s.server = &http.Server{
        Addr:         fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
        Handler:      s.router,
        ReadTimeout:  s.config.ReadTimeout,
        WriteTimeout: s.config.WriteTimeout,
    }

    errCh := make(chan error, 1)
    go func() { errCh <- s.server.ListenAndServe() }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    select {
    case err := <-errCh:
        return err
    case <-quit:
        ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
        defer cancel()
        return s.server.Shutdown(ctx)
    }
}
```

#### 15.9.5 Standardized Error Responses

All errors return a unified format aligned with OpenAI API style:

```go
type ErrorResponse struct {
    Error struct {
        Type    string `json:"type"`
        Message string `json:"message"`
        Code    string `json:"code,omitempty"`
        Param   string `json:"param,omitempty"`
    } `json:"error"`
}

const (
    ErrInvalidRequest   = "invalid_request_error"
    ErrAuthentication   = "authentication_error"
    ErrRateLimit        = "rate_limit_error"
    ErrSessionNotFound  = "session_not_found"
    ErrProviderError    = "provider_error"
    ErrToolExecFailed   = "tool_execution_error"
    ErrInternalError    = "internal_error"
)
```

### 15.10 Web-side Session Management `[implemented]`

#### 15.10.1 Session Lifecycle

```
Created ──── Active ──── Idle ──── Expired ──── Deleted
   │            │          │          │
   │            │          │          └── TTL expired, automatic cleanup
   │            │          └── No requests beyond idle_timeout
   │            └── Active requests/responses
   └── Created via POST /v1/sessions
```

#### 15.10.2 Persistence Strategy

```go
// Tiered storage: in-memory hot data + file/DB persistence
type HybridStore struct {
    hot    *InMemoryStore     // Active sessions
    cold   PersistentStore    // Persistent backend
    ttl    time.Duration      // Hot data TTL
}

type PersistentStore interface {
    Store
    GC(ctx context.Context, olderThan time.Duration) (int, error)
}
```

The initial implementation uses file storage (`~/.claw/sessions/`); SQLite / Redis can be added later.

#### 15.10.3 Concurrency Control

```go
// Only one write operation per Session at a time
type sessionLock struct {
    mu    sync.Map // sessionID → *sync.Mutex
}

func (l *sessionLock) Lock(id string) {
    val, _ := l.mu.LoadOrStore(id, &sync.Mutex{})
    val.(*sync.Mutex).Lock()
}
```

#### 15.10.4 Session Isolation (Multi-tenancy)

API Key → bound to a set of Sessions; different Keys cannot see each other's sessions:

```go
func authMiddleware(apiKeys map[string]string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := extractBearerToken(r)
            identity, ok := apiKeys[key]
            if !ok {
                writeError(w, 401, ErrAuthentication, "invalid api key")
                return
            }
            ctx := context.WithValue(r.Context(), ctxKeyIdentity, identity)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 15.11 System Prompt Design `[implemented]`

#### 15.11.1 Assembly Flow

```
┌─────────────────────────────────────────────┐
│              System Prompt Assembly          │
├─────────────────────────────────────────────┤
│                                             │
│  1. Base identity prompt                     │
│     "You are an AI coding assistant..."      │
│                                             │
│  2. Environment context                      │
│     OS / CWD / time / project info           │
│                                             │
│  3. Tool descriptions (auto-generated)       │
│     Available tools + JSON Schema            │
│                                             │
│  4. Skill prompts (session-level)            │
│     Extra instructions from active Skills    │
│                                             │
│  5. API caller customization                 │
│     system_prompt field overrides/appends     │
│                                             │
└─────────────────────────────────────────────┘
```

#### 15.11.2 API Caller Control

```go
type ChatCompletionRequest struct {
    // ...
    SystemPrompt string `json:"system_prompt,omitempty"` // Fully overrides default system prompt
    SystemAppend string `json:"system_append,omitempty"` // Appended after default system prompt
}
```

### 15.12 Authentication and Security `[implemented]`

#### 15.12.1 API Key Authentication

```go
type AuthConfig struct {
    Keys []APIKeyEntry `json:"api_keys"`
}

type APIKeyEntry struct {
    Key      string `json:"key"`
    Name     string `json:"name"`       // Display name
    RateLimit int   `json:"rate_limit"` // Requests per minute, 0 = unlimited
}
```

#### 15.12.2 Tool Sandbox (Web Mode)

In CLI mode tools operate directly on the user filesystem; Web mode requires isolation:

```go
type SandboxConfig struct {
    Enabled     bool     `json:"enabled"`
    RootDir     string   `json:"root_dir"`     // Root directory tools may access
    AllowedDirs []string `json:"allowed_dirs"` // Allowlisted directories
    DenyExec    bool     `json:"deny_exec"`    // Disallow bash tool
}
```

### 15.13 Observability `[implemented]`

#### 15.13.1 Structured Logging

Use `slog` (Go 1.21+ standard library):

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

logger.Info("chat completion",
    "request_id", reqID,
    "session_id", sessionID,
    "model", model,
    "latency_ms", elapsed.Milliseconds(),
)
```

#### 15.13.2 Metrics (Prometheus)

```go
var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "claw_requests_total"},
        []string{"method", "path", "status"},
    )
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{Name: "claw_request_duration_seconds"},
        []string{"method", "path"},
    )
    activeStreams = prometheus.NewGauge(
        prometheus.GaugeOpts{Name: "claw_active_streams"},
    )
    tokenUsage = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "claw_token_usage_total"},
        []string{"model", "type"}, // type: input/output
    )
)
```

### 15.14 Configuration Design (Go Version) `[implemented]`

#### 15.14.1 Current Implementation

Environment variable loading:

| Variable | Description | Default |
|----------|-------------|---------|
| `CLAW_PROVIDER` | Provider name | `anthropic` |
| `CLAW_MODEL` | Default model | `claude-sonnet-4-5` |
| `ANTHROPIC_API_KEY` | Anthropic API Key | - |
| `ANTHROPIC_BASE_URL` | Anthropic Base URL | `https://api.anthropic.com` |
| `OPENAI_API_KEY` | OpenAI API Key | - |
| `CLAW_PERMISSION_MODE` | Permission mode | `workspace_write` |
| `CLAW_DATA_DIR` | Data directory | `~/.claw/data` |
| `CLAW_API_KEYS` | Server API keys (comma-separated) | - |
| `CLAW_SERVER_PORT` | Server port | `8080` |
| `CLAW_SERVER_RATE_LIMIT` | Requests per minute per key | `0` |

#### 15.14.2 Server Configuration

```toml
# ~/.claw/config.toml or environment variables

[server]
host = "0.0.0.0"
port = 8080
api_keys = ["sk-xxx", "sk-yyy"]
read_timeout = "30s"
write_timeout = "300s"
max_concurrent = 100

[session]
storage = "file"               # "memory" | "file" | "sqlite"
storage_dir = "~/.claw/sessions"
ttl = "24h"
idle_timeout = "1h"

[sandbox]
enabled = false
root_dir = "/tmp/claw-sandbox"

[log]
level = "info"                 # debug | info | warn | error
format = "json"                # json | text
```

### 15.15 WebSocket Real-time Communication Design `[implemented]`

#### 15.15.1 WebSocket vs SSE

| Aspect | SSE (HTTP) | WebSocket |
|--------|------------|-----------|
| **Direction** | Server push | Bidirectional |
| **Connection** | HTTP long-lived | Separate protocol |
| **Reconnect** | Manual handling | Auto reconnect |
| **Use cases** | Real-time stream output | **Real-time collaboration, bidirectional** |
| **Compatibility** | Slightly weaker | Better |

**Recommendations**:
- Pure streaming output (chat delta) → SSE
- Client needs real-time intervention (cancel, retry, interrupt) → WebSocket
- Real-time collaboration (multi-user editing) → WebSocket

#### 15.15.2 WebSocket Connection

**Endpoint**: `ws://localhost:8080/ws/{session_id}`

**Authentication**: `ws://localhost:8080/ws/{session_id}?api_key=xxx` or via Header

#### 15.15.3 Client → Server Messages

```go
type WSClientMessage struct {
    Type string          `json:"type"`
}

const (
    WSClientSendMessage     = "send_message"
    WSClientExecuteTool     = "execute_tool"
    WSClientActivateSkill   = "activate_skill"
    WSClientDeactivateSkill = "deactivate_skill"
    WSClientInterrupt       = "interrupt"
    WSClientPing            = "ping"
)
```

#### 15.15.4 Server → Client Messages

```go
type WSServerMessage struct {
    Type  string `json:"type"`
    ReqID string `json:"req_id,omitempty"`
}

const (
    WSServerMessageStart     = "message_start"
    WSServerTextDelta        = "text_delta"
    WSServerToolUse          = "tool_use"
    WSServerToolStart        = "tool_start"
    WSServerToolResult       = "tool_result"
    WSServerToolError        = "tool_error"
    WSServerMessageEnd       = "message_end"
    WSServerSkillActivated   = "skill_activated"
    WSServerSkillDeactivated = "skill_deactivated"
    WSServerInterrupted      = "interrupted"
    WSServerError            = "error"
    WSServerPong             = "pong"
    WSServerDone             = "done"
)
```

#### 15.15.5 Full Bidirectional Conversation Example

```
Client                              Server
   │                                   │
   │──── WebSocket connect ────────────▶│
   │    ws://localhost:8080/ws/sess_1  │
   │                                   │
   │◀──── Connected + session state ────│
   │    {"type": "connected", "session_id": "sess_1"}
   │                                   │
   │──── Activate Skill ───────────────▶│
   │    {"type": "activate_skill", "id": "1", "skill": "code-review"}
   │                                   │
   │◀──── Skill activated ──────────────│
   │    {"type": "skill_activated", "req_id": "1", "skill": "code-review"}
   │                                   │
   │──── Send message ──────────────────▶│
   │    {"type": "send_message", "id": "2", "message": "Review /src/main.go"}
   │                                   │
   │◀──── Text deltas (streaming) ──────│
   │◀──── Tool call ────────────────────│
   │◀──── Tool result ──────────────────│
   │◀──── Message end ──────────────────│
   │◀──── Done ─────────────────────────│
   │                                   │
   │──── Ping ──────────────────────────▶│
   │◀──── Pong ─────────────────────────│
   │                                   │
   │──── Close connection ──────────────▶│
```

### 15.16 SDK Design `[implemented]`

#### 15.16.1 Go SDK (`pkg/sdk`)

```go
type Client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

func NewClient(baseURL, apiKey string) *Client

func (c *Client) CreateSession(ctx context.Context, opts *CreateSessionOpts) (*Session, error)
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error)
func (c *Client) ListSessions(ctx context.Context) ([]SessionSummary, error)
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error
func (c *Client) Chat(ctx context.Context, sessionID, message string) (*ChatResult, error)
func (c *Client) ChatStream(ctx context.Context, sessionID, message string, handler StreamHandler) error
func (c *Client) GetMessages(ctx context.Context, sessionID string) ([]Message, error)
```

### 15.17 Deployment Architecture `[implemented]`

```
┌─────────────────────────────────────────────────────────────────┐
│                     Deployment Architecture                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐      │
│   │   Client A  │     │   Client B  │     │   Client C  │      │
│   │  (Web App)  │     │  (Mobile)   │     │   (CLI)     │      │
│   └──────┬──────┘     └──────┬──────┘     └──────┬──────┘      │
│          │                    │                    │             │
│          └────────────────────┼────────────────────┘             │
│                               │                                  │
│                    ┌──────────▼──────────┐                       │
│                    │   Load Balancer     │                       │
│                    │   (Nginx/Traefik)  │                       │
│                    └──────────┬──────────┘                       │
│                               │                                  │
│          ┌────────────────────┼────────────────────┐             │
│          │                    │                    │             │
│   ┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐      │
│   │  Instance 1  │     │  Instance 2  │     │  Instance 3  │      │
│   │ claw         │     │ claw         │     │ claw         │      │
│   │ :8080        │     │ :8080        │     │ :8080        │      │
│   └─────────────┘     └─────────────┘     └─────────────┘      │
│                               │                                  │
│                    ┌──────────▼──────────┐                       │
│                    │   Redis/SQLite      │                       │
│                    │   (session store)   │                       │
│                    └─────────────────────┘                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 15.18 Performance Targets

| Metric | Target | Notes |
|--------|--------|-------|
| **Cold start** | < 10ms | Single binary, no JVM/runtime |
| **SSE time to first byte** | < 50ms | From request to first chunk |
| **Throughput** | > 5000 req/s | Single instance |
| **Memory** | < 50MB | Idle |
| **Goroutine count** | < 10000 | High concurrency |

### 15.19 Comparison with Claude API

| Aspect | Claude API | Claw-Go |
|--------|------------|---------|
| **Endpoint** | `/v1/messages` | `/v1/sessions/:id/messages` (Session-based) |
| **Streaming** | Server-Sent Events | Server-Sent Events |
| **Delta format** | `content_block_delta` | `message_delta` (OpenAI chunk compatible) |
| **Tool calls** | `tool_use` + `tool_result` | Same semantics; server executes automatically |
| **Auth** | Bearer Token | Bearer Token |
| **Working directory** | N/A | Per-session independent worktree |

### 15.20 Implementation Plan

**Phase 1 — MVP (core loop)**

1. [x] Confirm Go positioning (Web-first, multi-tenant, Session-only, gin)
2. [x] Core Runtime multi-turn loop (`engine.go`)
3. [x] Anthropic Provider (HTTP + SSE)
4. [x] Built-in tools (read/write/edit_file, glob/grep_search, bash)
5. [x] Permission engine
6. [x] CLI REPL
7. [x] **Streaming Runtime** (`RunPromptStream`)
8. [x] **HTTP Server** (gin + middleware stack + SSE handler)
9. [x] **Session API** (create/list/delete/send message)
10. [x] **Session persistence** (file storage)
11. [x] **API Key authentication**

**Phase 2 — Production hardening**

12. [x] Git worktree integration (bare repo + per-session isolation)
13. [x] System Prompt assembly
14. [x] Tool sandbox (Web mode path isolation)
15. [x] Structured logging (slog)
16. [x] Prometheus Metrics
17. [x] Session TTL + automatic cleanup
18. [x] Rate Limiting

**Phase 3 — Advanced features**

19. [x] OpenAI Provider (real HTTP+SSE implementation)
20. [x] Skill system (YAML load + session activation + trigger matching)
21. [x] WebSocket support (bidirectional + Skill integration)
22. [x] Go SDK (`pkg/sdk`)
23. [x] Deployment (Docker + Compose)

---

## Appendices

### Appendix A. API Endpoint Summary

#### A.1 Rust Version API Endpoints

| Method | Path | CLI Equivalent | Function |
|--------|------|----------------|----------|
| `POST` | `/sessions` | `nexus run` | Create new session |
| `GET` | `/sessions` | - | List sessions |
| `GET` | `/sessions/{id}` | - | Session details |
| `DELETE` | `/sessions/{id}` | `/clear` | Delete session |
| `POST` | `/sessions/{id}/resume` | `nexus resume` | Resume session |
| `GET` | `/sessions/{id}/export` | `/export` | Export session |
| `POST` | `/sessions/{id}/compact` | `/compact` | Compact session |
| `POST` | `/sessions/{id}/message` | `nexus run` | Send message (SSE) |
| `GET` | `/sessions/{id}/events` | - | SSE event stream |
| `GET` | `/sessions/{id}/messages` | - | Message history |
| `POST` | `/sessions/{id}/commands/{cmd}` | `/{cmd}` | Execute slash command |
| `GET` | `/commands` | `/help` | List all commands |
| `GET` | `/tools` | - | List all tools |
| `POST` | `/tools/{name}/execute` | Model invocation | Execute tool directly |
| `GET` | `/config` | `/config` | Get configuration |
| `PATCH` | `/config` | - | Update configuration |
| `GET` | `/models` | - | Available models |
| `GET` | `/sessions/{id}/permissions` | `/permissions` | Get permission mode |
| `POST` | `/sessions/{id}/permissions` | `/permissions` | Set permission mode |
| `GET` | `/plugins` | `/plugins` | List plugins |
| `GET` | `/mcp/servers` | - | MCP server list |

#### A.2 Go Version API Endpoints (Session-only)

| Method | Path | Function | Phase |
|--------|------|----------|-------|
| `POST` | `/v1/sessions` | Create session (optional `repo_url`) | 1 |
| `GET` | `/v1/sessions` | List sessions | 1 |
| `GET` | `/v1/sessions/:id` | Session details | 1 |
| `DELETE` | `/v1/sessions/:id` | Delete session (clean up worktree) | 1 |
| `POST` | `/v1/sessions/:id/messages` | Send message (SSE streaming) | 1 |
| `GET` | `/v1/sessions/:id/messages` | Message history | 1 |
| `GET` | `/v1/models` | Available models | 1 |
| `GET` | `/health` | Health check | 1 |
| `GET` | `/metrics` | Prometheus metrics | 2 |
| `GET/POST/DELETE` | `/v1/skills/*` | Skill management | 3 |
| `GET` | `/ws/:id` | WebSocket connection | 3 |

### Appendix B. SSE Event Types

#### B.1 Rust Version SSE Events

```rust
pub enum SseEvent {
    SessionStarted(SessionStarted),
    SessionEnded(SessionEnded),
    MessageStart(MessageStart),
    TextDelta(TextDelta),
    ToolUse(ToolUse),
    ToolStart(ToolStart),
    ToolEnd(ToolEnd),
    MessageEnd(MessageEnd),
    CommandStart(CommandStart),
    CommandOutput(CommandOutput),
    CommandEnd(CommandEnd),
    Error(Error),
    Warning(Warning),
    Info(Info),
    Ping(Ping),
    Done(Done),
}
```

#### B.2 Go Version SSE Events (Session-only Mode)

```go
const (
    EventMessageStart SSEEventType = "message_start"   // Message start
    EventMessageDelta SSEEventType = "message_delta"   // Text delta
    EventMessageEnd   SSEEventType = "message_end"     // Message end (includes usage)
    EventToolUse      SSEEventType = "tool_use"        // Model requests tool call
    EventToolStart    SSEEventType = "tool_start"      // Tool execution start
    EventToolResult   SSEEventType = "tool_result"     // Tool execution result
    EventToolError    SSEEventType = "tool_error"      // Tool execution error
    EventError        SSEEventType = "error"           // System error
    EventDone         SSEEventType = "done"            // Request complete
)
```

### Appendix C. Type Definition Summary

#### C.1 Permission Types

```rust
// Rust
pub enum PermissionLevel {
    ReadOnly,
    WorkspaceWrite,
    DangerFullAccess,
}

pub enum PermissionMode {
    Ask,
    Allow,
    Deny,
}
```

```go
// Go
type PermissionLevel string
type PermissionMode string

const (
    LevelReadOnly       PermissionLevel = "read_only"
    LevelWorkspaceWrite PermissionLevel = "workspace_write"
    LevelDangerFull    PermissionLevel = "danger_full"
)
```

#### C.2 Session Types

```rust
// Rust
pub struct Session {
    id: SessionId,
    model: String,
    messages: Vec<ConversationMessage>,
    created_at: DateTime<Utc>,
    metadata: SessionMetadata,
}
```

```go
// Go
type Session struct {
    ID        string
    Model     string
    Messages  []ChatMessage
    CreatedAt time.Time
    UpdatedAt time.Time
    Metadata  map[string]interface{}
}
```

#### C.3 Tool Specifications

```rust
// Rust
pub struct ToolSpec {
    pub name: String,
    pub description: String,
    pub input_schema: Value,
    pub output_schema: Option<Value>,
    pub required_permission: Permission,
}
```

```go
// Go
type Tool struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Parameters  interface{} `json:"parameters"`
}
```

### Appendix D. Client SDKs

#### D.1 Rust SDK

```rust
pub struct NexusClient {
    base_url: Url,
    api_key: String,
    http_client: reqwest::Client,
}

impl NexusClient {
    pub async fn create_session(&self) -> Result<SessionInfo, ClientError>;
    pub async fn send_message(&self, session_id: &SessionId, message: &str) -> Result<AssistantMessage, ClientError>;
    pub fn send_message_stream(&self, session_id: &SessionId, message: &str) -> impl Stream<Item = SseEvent> + '_;
    pub async fn execute_command(&self, session_id: &SessionId, command: &str, args: Vec<String>) -> Result<CommandResult, ClientError>;
    pub async fn list_tools(&self) -> Result<Vec<ToolSpec>, ClientError>;
    pub async fn execute_tool(&self, tool_name: &str, input: Value) -> Result<ToolResult, ClientError>;
}
```

#### D.2 Go SDK

```go
type Client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

func NewClient(baseURL, apiKey string) *Client

func (c *Client) CreateSession(ctx context.Context, opts *CreateSessionOpts) (*Session, error)
func (c *Client) Chat(ctx context.Context, sessionID, message string) (*ChatResult, error)
func (c *Client) ChatStream(ctx context.Context, sessionID, message string, handler StreamHandler) error
func (c *Client) ListSessions(ctx context.Context) ([]SessionSummary, error)
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error
func (c *Client) GetMessages(ctx context.Context, sessionID string) ([]Message, error)
```

---

*Document version: v2.0*
*Revision date: 2026-04-03*
*Major revisions: Go version repositioned as Web-first; Session-only API; Multi-tenancy; gin framework; git worktree isolation; corrected ChatLoop/compaction algorithm/SSE pseudocode; unified chapter numbering and naming*
