# SPEC-006: LLM Provider Configuration

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Depends on:** Spec 001, Spec 004

---

## Problem

LLM configuration is currently hardcoded via environment variables (`ANTHROPIC_API_KEY`, `LLM_MODEL`). There is no way to:
- Configure different models per agent from the UI
- Add new providers (OpenAI, Groq, Ollama) without code changes
- See which model is currently in use or switch it without restarting

## Goals

1. Multiple LLM providers configurable at runtime (Anthropic, OpenAI, Groq, Ollama)
2. Per-agent model selection from the Agent Management UI
3. Global default provider/model settable from dashboard
4. API keys stored in SQLite (encrypted at rest with a derived key)
5. Provider health check — test connection before saving

## Non-Goals (v1)

- Per-task model routing (future)
- Model cost tracking (future — Spec 007)
- Fine-tuned model support
- Streaming provider failover

---

## Design

### Data Model

```sql
CREATE TABLE llm_providers (
  id TEXT PRIMARY KEY,          -- 'anthropic', 'openai', 'groq', 'ollama'
  name TEXT NOT NULL,
  api_key_encrypted TEXT,       -- null for local providers (Ollama)
  base_url TEXT,                -- for Ollama: http://localhost:11434
  default_model TEXT NOT NULL,
  enabled INTEGER DEFAULT 1,
  is_default INTEGER DEFAULT 0, -- only one can be default
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
```

### Encryption

API keys encrypted with AES-256-GCM using a key derived from `SESSION_SECRET` via PBKDF2. The encrypted blob is stored as `iv:ciphertext` hex. This is not enterprise security — it's protection against accidental log/backup exposure.

### LLMClient Changes

- Accept provider config dynamically instead of only from env vars
- `getProvider()` — returns active provider for a given agent (checks agent config → global default → env var fallback)
- `testConnection()` — send a minimal prompt and verify response

### WebSocket Handlers

```
'llm.providers.list'    → list all configured providers + status
'llm.provider.add'      → add new provider (validates API key first)
'llm.provider.update'   → update key, model, or default status
'llm.provider.delete'   → remove provider
'llm.provider.test'     → test connectivity
'llm.models.list'       → list available models for a provider (from API)
```

### Dashboard: Settings Page

New **Settings** section with:
1. **LLM Providers** card — table of providers with:
   - Provider name + icon
   - Status badge (connected / error / unconfigured)
   - Default model
   - "Set as default" button
   - Edit/Delete
2. **Add Provider** form — select provider type, enter API key, pick default model, test connection
3. **Global defaults** — which provider/model to use when agent has no preference

### Agent Integration

Agent creation form (Spec 004) gains a model selector dropdown populated from `llm.providers.list`. Options: `[provider default]`, then specific models from each provider.

---

## Supported Providers (v1)

| Provider | Auth | Models |
|---|---|---|
| Anthropic | API key | claude-opus-4-5, claude-sonnet-4-5, claude-haiku-4-5 |
| OpenAI | API key | gpt-4o, gpt-4o-mini, gpt-4-turbo |
| Groq | API key | llama-3.3-70b-versatile, llama-3.1-8b-instant |
| Ollama | No key (local URL) | Dynamic from `/api/tags` |

---

## Test Criteria

- Add Anthropic provider → persists to DB, API key encrypted
- Retrieve provider → key decrypts correctly
- Test connection → returns success/failure
- Set as default → other providers lose default flag
- Per-agent model override → used in LLMClient.chat()
- Env var fallback → if no DB providers, falls back to ANTHROPIC_API_KEY env

---

## Rollback

LLMClient has env var fallback at every level. If provider DB is missing or corrupt, it falls through to `process.env.ANTHROPIC_API_KEY`. Zero regression risk.
