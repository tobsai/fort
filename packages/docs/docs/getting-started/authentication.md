---
sidebar_position: 2
title: Authentication
---

# Authentication

Fort uses Claude as its LLM backend. There are several ways to authenticate.

## Recommended: Claude OAuth via `fort llm setup`

The simplest method uses your existing Claude subscription -- no separate API billing needed.

```bash
fort llm setup
```

This runs `claude setup-token` under the hood, which:

1. Opens your browser for OAuth authentication
2. Stores the token in **macOS Keychain**
3. Fort reads it automatically on every request

No API key management, no extra billing.

## Alternative: API Key via Environment Variable

If you prefer pay-per-token billing through the Anthropic API:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

This is billed separately from a Claude subscription.

## Alternative: API Key in Config

You can also set the key in `.fort/config.yaml`:

```yaml
llm:
  apiKey: sk-ant-...
```

:::info
Storing keys in config files is convenient for local development but less secure than Keychain or environment variables.
:::

## Auth Priority Order

Fort checks credentials in this order (first match wins):

1. `apiKey` in `.fort/config.yaml`
2. `CLAUDE_CODE_OAUTH_TOKEN` environment variable
3. macOS Keychain (set by `fort llm setup`)
4. `ANTHROPIC_API_KEY` environment variable

## Verify Authentication

```bash
fort llm status
```

This shows which auth method is active and confirms connectivity to the Claude API.
