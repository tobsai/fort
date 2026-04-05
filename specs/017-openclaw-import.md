---
id: 017-openclaw-import
title: Import from OpenClaw
status: approved
---

## Goal
Add an "Import from OpenClaw" section to the Fort settings page that detects an existing OpenClaw installation, previews what would be imported, and performs the migration with one click.

## What Gets Imported

| OpenClaw source | Fort destination |
|---|---|
| `agents.list[]` → identity fields | `~/.fort/data/agents/<id>/identity.yaml` |
| Agent workspace `SOUL.md` | Agent `SOUL.md` (copied verbatim) |
| Agent workspace `MEMORY.md` | Agent `MEMORY.md` |
| `models.providers` + `env` API keys | `fort.llmProviders.addProvider()` |

Sessions/tasks are excluded from scope (v1).

## Approach

### Backend (`packages/core/src/import/openclaw.ts`)
- `scanOpenClaw(dataDir)` — detects `~/.openclaw/openclaw.json`, returns `OpenClawPreview` (agents, providers, warnings)
- `importOpenClaw(fort, options)` — performs the import; skips agents/providers that already exist

### API endpoints (added to `packages/core/src/server/index.ts`)
- `GET /api/import/openclaw/preview` — scan only, return preview JSON
- `POST /api/import/openclaw` — run import, return result JSON

### UI (`packages/dashboard/src/pages/SettingsPage.tsx`)
- New "Import from OpenClaw" section below LLM Providers
- "Detect" button → calls preview endpoint → shows what was found
- "Import" button (shown after detection) → calls import endpoint → shows results

## Affected Files
- `specs/017-openclaw-import.md` (this file)
- `packages/core/src/import/openclaw.ts` (new)
- `packages/core/src/server/index.ts` (two new HTTP routes)
- `packages/dashboard/src/pages/SettingsPage.tsx` (new UI section)

## Test Criteria
- Preview with no OpenClaw → `{ found: false }`
- Preview with valid `openclaw.json` → returns agent list and provider list
- Import creates agent dirs with `identity.yaml` + `SOUL.md`
- Import skips agents that already exist in Fort
- Import adds providers with encrypted API keys
- Import skips providers already in Fort

## Rollback Plan
Agents are created in `~/.fort/data/agents/` — delete agent directories and remove added LLM providers via settings to undo.
