# Tool Execution — Give Lewis Hands

**ID:** SPEC-002
**Status:** draft
**Author:** Lewis
**Created:** 2026-03-23
**Depends on:** SPEC-001 (Chat MVP)

## Goal

Make tools callable, not just metadata. When Lewis needs to do something (read a file, search the web, check the calendar), the LLM should be able to invoke a tool mid-conversation using Claude's native `tool_use`, get the result, and continue reasoning. Tools are deterministic at runtime — the LLM decides *when* to call them, but the tool itself is a bounded function with predictable behavior.

## Current State

- `ToolRegistry` exists as a SQLite metadata store (name, description, capabilities, search)
- Specialist agent searches the registry during chat and injects matches as LLM context
- LLM can *propose* new tools via JSON in its response (tool-building flow)
- LLM client (`packages/core/src/llm/index.ts`) calls `messages.create` with no `tools` parameter — plain text completion only
- `PermissionManager` has a 4-tier model (Auto, Draft, Approve, Never) with action gates already defined
- No tools are actually registered — the registry is empty
- No tool execution runtime exists

**What's missing:**
1. A `Tool` interface with an `execute()` method
2. Claude `tool_use` wiring in the LLM client (tools param + tool_result handling)
3. A tool execution loop (call → result → continue) in the specialist agent
4. Permission checks before tool execution
5. Concrete built-in tools (at least a starter set)

## Design Principles

From CLAUDE.md:
- Fort never calls external tools directly — every capability is wrapped in a Fort-owned tool
- Tools are deterministic at runtime (AI powers creation, execution is predictable)
- Tool creation hierarchy: reuse existing → use industry tools as engine → build custom (requires approval)
- New tool *types* require Toby's approval; built-in tools ship with this spec

## Approach

### Phase 1: Tool Interface + Executor

Define a `FortTool` interface that the registry can hold and the executor can call:

```typescript
interface FortTool {
  name: string;              // Unique identifier (kebab-case)
  description: string;       // What it does (shown to LLM)
  inputSchema: object;       // JSON Schema for parameters
  tier: ActionTier;          // 1=auto, 2=draft, 3=approve, 4=never
  execute(input: unknown): Promise<ToolResult>;
}

interface ToolResult {
  success: boolean;
  output: string;            // Text result shown to LLM
  artifacts?: unknown[];     // Optional structured data
  error?: string;
}
```

Create `ToolExecutor` class in `packages/core/src/tools/executor.ts`:
- Takes a `FortTool` + input, checks permissions via `PermissionManager`, executes
- Publishes `tool.executed` / `tool.denied` / `tool.error` events on the ModuleBus
- Records execution in TokenTracker (tool calls are part of the task cost)
- Returns `ToolResult`

### Phase 2: Wire Claude tool_use into LLM Client

Modify `LLMClient.complete()` to accept an optional `tools` parameter:
- Convert `FortTool[]` to Claude's `tools` format (name, description, input_schema)
- When response contains `tool_use` blocks, return them as a new response type
- Add a `completeWithTools()` method that handles the multi-turn loop:
  1. Send message with tools
  2. If response has `tool_use` → execute via `ToolExecutor` → append `tool_result` → re-send
  3. Repeat until response is pure text (or max iterations hit)
  4. Return final text response + tool call log

### Phase 3: Specialist Agent Tool Loop

Modify `SpecialistAgent.onTask()`:
- Instead of calling `this.llm.complete()`, call `this.llm.completeWithTools()`
- Pass available tools from the `ToolRegistry` (filtered by agent permissions)
- Tool call log gets stored in the task metadata for transparency
- Dashboard receives tool execution events via WebSocket broadcast

### Phase 4: Built-in Tools (Starter Set)

Ship 5 concrete tools in `packages/core/src/tools/builtins/`:

| Tool | Tier | Description |
|------|------|-------------|
| `read-file` | 1 (auto) | Read a file from allowed directories |
| `write-file` | 2 (draft) | Write/create a file (creates draft, shows diff) |
| `list-files` | 1 (auto) | List directory contents |
| `web-search` | 1 (auto) | Search the web via Brave Search API |
| `shell-command` | 3 (approve) | Run a shell command (requires explicit approval) |

Each tool:
- Implements `FortTool` interface
- Has its own test file
- Wraps an underlying capability (fs, fetch, child_process) with constraints
- Respects `PermissionManager` tier and folder allowlists

### Phase 5: Dashboard Integration

- Show tool calls inline in the chat UI (collapsible, showing input → output)
- Show permission prompts when a Tier 3 tool needs approval
- Broadcast `tool.executed` events via WebSocket so the dashboard updates live

## Affected Files

**New files:**
- `packages/core/src/tools/types.ts` — FortTool, ToolResult, ToolCallLog interfaces
- `packages/core/src/tools/executor.ts` — ToolExecutor class
- `packages/core/src/tools/builtins/read-file.ts`
- `packages/core/src/tools/builtins/write-file.ts`
- `packages/core/src/tools/builtins/list-files.ts`
- `packages/core/src/tools/builtins/web-search.ts`
- `packages/core/src/tools/builtins/shell-command.ts`
- `packages/core/src/__tests__/tool-executor.test.ts`
- `packages/core/src/__tests__/builtins.test.ts`

**Modified files:**
- `packages/core/src/tools/index.ts` — Registry gains `registerTool(FortTool)` alongside metadata-only `register()`
- `packages/core/src/llm/index.ts` — Add `tools` param to `complete()`, add `completeWithTools()` loop
- `packages/core/src/agents/specialist.ts` — Use `completeWithTools()` in `onTask()`
- `packages/core/src/server/index.ts` — Broadcast tool events, add permission approval WebSocket handler
- `packages/core/src/types.ts` — New type exports
- `packages/dashboard/src/` — Chat UI: tool call display + approval prompt

## Test Criteria

1. `ToolExecutor` executes a Tier 1 tool without prompting
2. `ToolExecutor` blocks a Tier 4 tool and returns denial
3. `ToolExecutor` requests approval for a Tier 3 tool
4. `read-file` reads an allowed file and returns contents
5. `read-file` rejects paths outside allowed directories
6. `list-files` returns directory listing
7. `web-search` returns results (mock in tests)
8. `shell-command` is blocked by default (`commandLine: deny`)
9. LLM client sends tools in Claude format and handles `tool_use` responses
10. `completeWithTools()` loops correctly: message → tool_use → tool_result → text
11. Specialist agent uses tools during chat (integration test)
12. Tool calls appear in task metadata
13. All existing 344 tests still pass
14. Dashboard shows tool calls inline in chat

## Rollback Plan

Feature branch (`feat/tool-execution`). The `FortTool` interface is additive — existing `ToolDefinition` metadata entries continue to work. If anything breaks, the specialist agent falls back to plain `complete()` (no tools).

## Open Questions

1. **Max tool iterations** — How many tool calls per conversation turn? Suggest 10 as a safety cap.
2. **Parallel tool calls** — Claude can request multiple tools at once. Execute in parallel or serial? Suggest parallel for Tier 1, serial for Tier 2+.
3. **Tool result size limits** — Large file reads or command outputs need truncation. Suggest 50KB per tool result.
4. **Approval UX** — For Tier 3 tools, the dashboard needs to show a prompt and block until the user approves/denies. WebSocket-based or polling?
