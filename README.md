<p align="center">
  <img src="assets/fort-logo.png" alt="Fort" width="800" />
</p>

# NOTE: This is still in the prototype phase and not ready for use

# Fort

Fort is a task-centric personal AI agent platform. It is not a chat wrapper. Every interaction creates a task with clear ownership and status. Deterministic services handle orchestration, memory, scheduling, and reflection. Users create long-lived specialist agents through a web portal, each defined by a SOUL.md that shapes its personality and goals.

## Quick Start

```bash
# Install dependencies
npm install

# Build
npm run build

# Link the CLI globally
npm link --workspace=packages/cli

# Initialize Fort and open the portal wizard
fort init

# The portal opens at http://localhost:4077
# Walk through the setup wizard to create your first agent
```

## Architecture

Fort separates **services** from **agents**.

**Services** are deterministic infrastructure. They do not use LLMs unless they hit ambiguity. They live in `packages/core/src/services/`.

| Service | Role |
|---------|------|
| **Orchestrator** | Creates a task for every chat message and routes it to the correct agent |
| **Memory** | Manages the graph-based knowledge store |
| **Scheduler** | Executes cron-scheduled routines |
| **Reflection** | Periodically scans completed tasks for missed action items, creates follow-up tasks |

**Agents** are user-created specialists. Each agent has a SOUL.md that defines its personality, goals, rules, voice, and boundaries. The SOUL.md is injected into the LLM system prompt at runtime.

Chat messages go to the user's default agent (or a selected long-lived agent). The orchestrator service creates the task and routes it -- it does not answer questions itself.

### Task-Centric Design

Every chat message creates a task. Tasks have:

- `result` -- the outcome
- `assignedTo` -- `'agent'` or `'user'`
- Status -- To Do, In Progress, Done

The portal at `http://localhost:4077` shows a kanban board with clear ownership across these columns.

### Agent Lifecycle

Users create agents through the portal setup wizard or the CLI. The wizard collects name, goals, and emoji, then generates a SOUL.md.

Each agent gets a directory:

```
.fort/data/agents/<agent-slug>/
  identity.yaml   # name, status, metadata
  SOUL.md          # personality, goals, rules, voice, boundaries
  tools/           # agent-specific tool configs
```

Lifecycle commands: `fort agents create`, `fork`, `retire`, `revive`.

## CLI Reference

| Command | Description |
|---------|-------------|
| `fort init` | Initialize Fort, open portal wizard |
| `fort doctor` | Health checks across all modules |
| `fort status` | System status overview |
| `fort agents list` | List specialist agents |
| `fort agents create` | Create a specialist agent |
| `fort agents fork <id>` | Clone an agent with a new name |
| `fort agents retire <id>` | Deactivate an agent |
| `fort agents revive <id>` | Reactivate a retired agent |
| `fort agents inspect <id>` | Deep-inspect an agent |
| `fort tasks` | Show task graph (kanban view) |
| `fort memory stats` | Memory graph statistics |
| `fort tools list` | Tool registry |
| `fort tokens` | Token usage and cost |
| `fort tokens budget-set` | Set spending limits |
| `fort schedule list` | Cron jobs |
| `fort routines list` | Scheduled routines |
| `fort flags list` | Feature flags |
| `fort plugins list` | Loaded plugins |
| `fort harness start` | Start a self-coding cycle |
| `fort rewind` | Snapshot management |
| `fort introspect` | System profile and capability map |
| `fort llm setup` | Authenticate with Claude |
| `fort llm status` | LLM config and usage stats |
| `fort llm ask <prompt>` | Send a prompt (`--model fast\|standard\|powerful`) |

## Project Structure

```
packages/
  core/             TypeScript core
    src/
      services/       Orchestrator, Memory, Scheduler, Reflection
      agents/         Agent framework, SOUL.md handling
      task-graph/     Task tracking and kanban state
      memory/         Graph-based memory store
      module-bus/     Event-driven message bus
      llm/            Anthropic Claude API client
      threads/        Conversation threading
      tools/          Tool registry
      permissions/    Tiered action model
      flows/          Deterministic workflow engine
      tokens/         Usage tracking and budgets
      scheduler/      Cron scheduling
      routines/       Cron-scheduled routines
      diagnostics/    FortDoctor health checks
      harness/        Self-coding cycle
      integrations/   Gmail, Calendar, iMessage, Brave Search
      ipc/            WebSocket server
      plugins/        Plugin system
      rewind/         Snapshot backup and restore
      specs/          Spec-driven development
  cli/              Commander.js CLI
  swift-shell/      macOS menu bar app (AppKit + WebSocket)
  dashboard/        Tauri + React dashboard and portal (http://localhost:4077)
  docs/             Docusaurus documentation
```

## Tech Stack

- **Runtime**: Node.js / TypeScript
- **Tests**: Vitest
- **CLI**: Commander.js
- **DB**: better-sqlite3 (SQLite per module)
- **Config**: YAML + JSON Schema (ajv)
- **LLM**: Anthropic Claude (Haiku/Sonnet/Opus tiered routing)
- **macOS**: Swift/AppKit menu bar + Tauri dashboard
- **Docs**: Docusaurus

## License

[MIT](LICENSE)
