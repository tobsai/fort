---
sidebar_position: 3
title: fort llm
---

# fort llm

Configure LLM providers and send prompts with optional memory context.

## Usage

```
fort llm <subcommand> [options]
```

## Subcommands

### setup

Configure the LLM provider and API key.

```
fort llm setup [--api-key <key>]
```

If `--api-key` is omitted, you will be prompted interactively.

### status

Show current LLM configuration and connectivity.

```
fort llm status [--json]
```

### models

List all available models from the configured provider.

```
fort llm models
```

### ask

Send a prompt to the LLM and print the response.

```
fort llm ask <prompt> [options]
```

| Option | Description |
|--------|-------------|
| `--model <tier>` | Model tier: `fast`, `standard`, or `powerful`. Default: `standard` |
| `--no-behaviors` | Disable behavior rules for this request |
| `--memory <query>` | Inject relevant memory nodes matching the query |

### diagnose

Run diagnostics on the LLM integration.

```
fort llm diagnose
```

## Examples

```bash
# Initial setup
fort llm setup --api-key sk-abc123

# Ask a question with memory context
fort llm ask "what did I decide about the auth module?" --memory "auth"

# Use the fast model for a quick answer
fort llm ask "summarize this error" --model fast
```
