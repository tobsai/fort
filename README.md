# Fort

A self-improving personal AI agent platform. Ground-up replacement for OpenClaw combining long-lived specialist agents, graph-based memory, deterministic workflows, and macOS native integration.

## Quick Start

```bash
# Install dependencies
npm install

# Build
npm run build

# Link the CLI globally
npm link --workspace=packages/cli

# Authenticate (uses your Claude subscription)
fort llm setup

# Verify everything works
fort doctor
fort llm status
```

## Architecture

Fort is a polyglot monorepo with three languages:

- **TypeScript** — Core logic, CLI, and all modules (`packages/core`, `packages/cli`)
- **Swift** — macOS menu bar app with native integrations (`packages/swift-shell`)
- **React/Tauri** — Dashboard UI (`packages/dashboard`)

### Core Modules

| Module | Description |
|--------|-------------|
| **ModuleBus** | Event-driven backbone — all modules communicate via typed events |
| **TaskGraph** | Every conversation creates a task. Tasks are the atomic unit of transparency |
| **AgentRegistry** | 4 core agents + specialist agents created via the Hatchery |
| **MemoryManager** | SQLite-backed graph store with MemU HTTP fallback |
| **LLMClient** | Anthropic Claude API with three-tier model routing (Haiku/Sonnet/Opus) |
| **FlowEngine** | YAML-defined deterministic workflows with 6 step types |
| **TokenTracker** | Usage logging, cost calculation, budget enforcement |
| **BehaviorManager** | Context-aware behavioral rules injected into LLM prompts |
| **RoutineManager** | Cron-scheduled routines with execution history |
| **ThreadManager** | Persistent conversation threads backed by tasks |
| **ToolRegistry** | SQLite-backed registry enforcing reuse-before-build |
| **PermissionManager** | Tiered action model (Auto / Draft / Approve / Never) |
| **FeatureFlagManager** | Flags with bake periods and auto-promote/rollback |
| **PluginManager** | Security-scanned plugin loading with 14 pattern checks |
| **Harness** | Self-coding cycle: spec, branch, implement, test, review, merge |
| **RewindManager** | Snapshot-based backup and restore |
| **IPCServer** | WebSocket server bridging core to Swift shell and dashboard |
| **OSIntegration** | Spotlight, Shortcuts, Finder, voice, Focus mode hooks |
| **Introspector** | Machine-readable system documentation and capability maps |
| **FortDoctor** | Health checks across all modules |

### Specialist Agents

Fort ships with 4 core agents (Orchestrator, Memory, Scheduler, Reflection) and supports creating specialist agents via the Hatchery:

```bash
# Create a new specialist
fort agents hatch --name "Research Agent" \
  --description "Deep research and source synthesis" \
  --capabilities "web_search,summarization" \
  --behaviors "Always cite sources,Prefer academic sources"

# Fork, retire, revive
fort agents fork research-agent --name "Academic Researcher"
fort agents retire research-agent --reason "Replaced"
fort agents revive research-agent
```

Specialists persist as YAML identity files, auto-load on restart, get their own memory partition, and have behavioral rules that serve as LLM system prompt context.

## CLI Reference

```
fort doctor              Health checks across all modules
fort status              System status overview
fort llm setup           Authenticate with Claude subscription
fort llm status          LLM config, auth method, usage stats
fort llm ask <prompt>    Send a prompt (--model fast|standard|powerful)
fort llm models          List model routing tiers
fort agents list         List registered agents
fort agents hatch        Create a specialist agent
fort agents identities   List all specialists (active + retired)
fort tasks               Show task graph
fort threads             Conversation threads
fort memory stats        Memory graph statistics
fort tools list          Tool registry
fort tokens              Token usage and cost
fort tokens budget-set   Set spending limits
fort behaviors list      Behavioral rules
fort routines list       Scheduled routines
fort schedule list       Cron jobs
fort flags list          Feature flags
fort plugins list        Loaded plugins
fort harness start       Start a self-coding cycle
fort rewind              Snapshot management
fort introspect          System profile and capability map
```

## LLM Authentication

Fort authenticates through your existing Claude subscription — no separate API billing needed.

```bash
fort llm setup    # Runs claude setup-token, opens browser, done
```

Auth is checked in priority order:

1. Explicit `apiKey` in `.fort/config.yaml`
2. `CLAUDE_CODE_OAUTH_TOKEN` (set automatically inside Claude Code sessions)
3. macOS Keychain token (set by `claude setup-token`)
4. `ANTHROPIC_API_KEY` environment variable

## Model Routing

Fort automatically routes to the cheapest model that can handle each task:

| Tier | Model | Use Case |
|------|-------|----------|
| `fast` | Claude Haiku 4.5 | Classification, extraction, simple queries |
| `standard` | Claude Sonnet 4.6 | Most tasks, coding, analysis |
| `powerful` | Claude Opus 4.6 | Complex planning, architecture, nuanced reasoning |

## Development

```bash
# Run tests (316 tests across 21 suites)
npm test

# Build all packages
npm run build

# Build Swift menu bar app
cd packages/swift-shell && swift build

# Build Tauri dashboard
cd packages/dashboard && npm install && npm run tauri dev
```

### Project Structure

```
packages/
  core/           TypeScript core (all modules)
    src/
      agents/       Agent framework + hatchery
      behaviors/    Context-aware behavioral rules
      diagnostics/  FortDoctor health checks
      feature-flags/ Feature flag management
      flows/        Deterministic workflow engine
      harness/      Self-coding cycle + garbage collector
      integrations/ Gmail, Calendar, iMessage, Brave Search, Browser
      introspect/   Machine-readable system docs
      ipc/          WebSocket server
      llm/          Anthropic Claude API client
      memory/       Graph-based memory store
      module-bus/   Event-driven message bus
      os-integration/ macOS native hooks
      permissions/  Tiered action model
      plugins/      Security-scanned plugin system
      rewind/       Snapshot backup and restore
      routines/     Cron-scheduled routines
      scheduler/    Cron scheduling
      specs/        Spec-driven development
      task-graph/   Task tracking
      threads/      Conversation threading
      tokens/       Usage tracking and budgets
      tools/        Tool registry
  cli/            Commander.js CLI
  swift-shell/    macOS menu bar app (AppKit + WebSocket)
  dashboard/      Tauri + React dashboard
```

## License

Private — not yet open source.
