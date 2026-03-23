---
sidebar_position: 2
title: Authentication
---

# Authentication

Fort uses Claude as its LLM backend. Your API key is stored in a plain `.env` file you can inspect and edit anytime.

## Recommended: Claude OAuth via `fort llm setup`

The simplest method uses your existing Claude subscription — no separate API billing needed.

```bash
fort llm setup
```

This runs `claude setup-token` under the hood, which:

1. Opens your browser for OAuth authentication
2. Saves the token to **`~/.fort/.env`**

No extra billing, and the token is always inspectable:

```bash
cat ~/.fort/.env
```

## Alternative: Edit `.env` Directly

You can add or change your API key manually:

```bash
# Create or edit the file
nano ~/.fort/.env
```

```env
# Fort API Configuration
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

## Alternative: Environment Variable

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

This works but doesn't persist across terminal sessions unless added to your shell profile.

## Auth Priority Order

Fort checks credentials in this order (first match wins):

1. `apiKey` in Fort config (programmatic)
2. `~/.fort/.env` file (recommended)
3. `CLAUDE_CODE_OAUTH_TOKEN` environment variable (Claude Code sessions)
4. `ANTHROPIC_API_KEY` environment variable

## Verify Authentication

```bash
fort llm status
```

This shows which auth method is active, the file path, and confirms connectivity.
