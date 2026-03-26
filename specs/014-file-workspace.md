# SPEC-014: Agent File Workspace

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** MEDIUM — enables real coding/research work  
**Depends on:** Spec 002 (Tool Execution), Spec 004 (Agent Management)

---

## Problem

Agents can read and write files via the existing `read-file` and `write-file` tools, but they operate on arbitrary paths which is dangerous (can touch system files) and invisible (no audit trail, no dashboard visibility). Agents doing real work (research, coding, writing) need a sandboxed workspace — a dedicated directory per agent where they can freely create, edit, and organize files, with full visibility in the dashboard.

## Goals

1. Each agent has a sandboxed workspace directory: `~/.fort/workspaces/{agentId}/`
2. `read-file` and `write-file` tools resolve paths relative to the agent's workspace (no path traversal)
3. Dashboard file browser: view all files in an agent's workspace
4. File viewer: click to read file content in dashboard
5. Download file from dashboard
6. Workspace statistics: file count, total size

## Non-Goals (v1)

- Git integration for agent workspaces
- Shared workspaces between agents
- File upload from dashboard to workspace

---

## Design

### WorkspaceManager

File: `packages/core/src/workspace/index.ts`

```typescript
export class WorkspaceManager {
  constructor(private baseDir: string) {}    // ~/.fort/workspaces/
  
  getAgentDir(agentId: string): string
  ensureAgentDir(agentId: string): void     // mkdir -p
  
  // Safe path resolution — throws if traversal detected
  resolvePath(agentId: string, relativePath: string): string
  
  // File operations (used by tools)
  readFile(agentId: string, path: string): string
  writeFile(agentId: string, path: string, content: string): void
  listFiles(agentId: string, subdir?: string): WorkspaceFile[]
  deleteFile(agentId: string, path: string): void
  
  // Stats
  getStats(agentId: string): WorkspaceStats   // file count, total bytes
}
```

Path traversal protection: `resolvePath()` resolves the full path, then verifies it starts with `agentDir`. If not, throws `WorkspaceAccessError`.

### read-file tool (enhanced)

Current behavior: reads arbitrary absolute paths.  
New behavior: paths are relative to agent workspace. Absolute paths → reject (WorkspaceAccessError).

Backward compatibility: if path starts with `/` and FORT_STRICT_WORKSPACE=false (default during transition), fall through to absolute path read.

### write-file tool (enhanced)

Same: paths relative to workspace. Auto-creates parent directories within workspace.

### list-files tool (enhanced)

Returns workspace-relative paths instead of absolute.

### Fort.ts wiring

- `WorkspaceManager` initialized with `~/.fort/workspaces/`
- Passed to ToolExecutor
- New WS handlers

### WS Handlers

```
'workspace.list'          → { agentId, subdir? } → WorkspaceFile[]
'workspace.read'          → { agentId, path } → { content: string }
'workspace.stats'         → { agentId } → WorkspaceStats
'workspace.delete'        → { agentId, path } → void
```

### Dashboard: Agent Workspace Tab

In the Agent detail panel, add a "Workspace" tab (alongside Memory from Spec 012):
- File tree: expandable directories, file list with icons by extension
- Click file → viewer panel (syntax-highlighted for .ts/.js/.md/.json)
- Stats bar: X files, Y KB total
- Download button per file (triggers `workspace.read` then blob download)

---

## Test Criteria

- WorkspaceManager.resolvePath() returns correct path for relative inputs
- WorkspaceManager.resolvePath() throws on `../` traversal attempts
- writeFile() creates parent directories
- listFiles() returns workspace-relative paths
- read-file tool rejects absolute paths when FORT_STRICT_WORKSPACE=true
- All existing 587 tests still pass
