---
sidebar_position: 2
title: LLM Integration
---

# LLM Integration

Fort routes every LLM call through a three-tier model system with automatic complexity-based routing, context enrichment, and streaming support.

## Three-Tier Model Routing

Each tier maps to a Claude model optimized for different workloads:

| Tier | Model | Use Case |
|---|---|---|
| `fast` | Haiku 4.5 | Classification, simple extraction, yes/no decisions |
| `standard` | Sonnet 4.6 | Multi-step reasoning, code generation, analysis |
| `powerful` | Opus 4.6 | Complex planning, long-context synthesis, reflection |

### Auto-Routing

By default, Fort selects the tier automatically based on task complexity. The Orchestrator evaluates the prompt and task metadata to decide:

- Short prompts with simple intent go to `fast`
- Tasks requiring tool use or multi-step reasoning go to `standard`
- Tasks involving planning, cross-referencing memory, or reflection go to `powerful`

You can override auto-routing by specifying a tier explicitly.

## CLI Usage

Ask a question with auto-routing:

```bash
fort llm ask "What is the capital of France?"
# Routes to: fast (simple factual question)
```

Force a specific tier:

```bash
fort llm ask "Review this architecture and suggest improvements" --model powerful
```

List available models and their configuration:

```bash
fort llm models
```

```
TIER       MODEL        MAX_TOKENS   COST/1K_IN   COST/1K_OUT
fast       haiku-4.5    8192         $0.001       $0.005
standard   sonnet-4.6   16384        $0.003       $0.015
powerful   opus-4.6     32768        $0.015       $0.075
```

Check LLM subsystem health and current rate limits:

```bash
fort llm status
```

```
Status: healthy
API Key: configured
Rate Limits: 4000 req/min (2847 remaining)
Active Streams: 0
Today's Usage: 142,300 tokens ($1.24)
```

## Context Enrichment

Before every LLM call, Fort enriches the prompt with two sources of context.

### Behavior Injection

Active behaviors matching the current context are injected into the system prompt. For example, if an agent has the behavior "Always respond in British English", that rule is prepended to every call made by that agent.

```typescript
// Behaviors become system prompt instructions
const systemPrompt = [
  ...activeBehaviors.map(b => b.rule),
  baseSystemPrompt,
].join('\n');
```

### Memory Injection

The `injectMemory` step retrieves relevant memory nodes and adds them to the prompt context. Memory injection is selective — only nodes matching the current task's topic or agent partition are included.

```typescript
// Memory nodes are retrieved and injected
const memories = await memoryManager.search(task.topic, { limit: 10 });
const memoryContext = memories.map(m => `[${m.type}] ${m.content}`).join('\n');
```

This means the LLM always has access to relevant stored knowledge without the caller needing to manually fetch and format it.

## Streaming

Fort supports streaming responses for interactive use cases. The LLM manager emits `llm:chunk` events on the ModuleBus as tokens arrive:

```typescript
bus.on('llm:chunk', (event) => {
  process.stdout.write(event.data.content);
});

await llmManager.stream({
  prompt: "Explain the memory subsystem",
  model: 'standard',
});
```

The CLI `fort llm ask` command streams by default, printing tokens as they arrive.

## Token Tracking

Every LLM call is automatically logged with model, token count, and estimated cost. See the [Tokens and Budgets](./tokens-and-budgets.md) guide for details on monitoring and setting limits.

## Configuration

LLM settings live in `.fort/config.yaml`:

```yaml
llm:
  default_tier: standard
  auto_route: true
  api_key_env: ANTHROPIC_API_KEY
  max_retries: 3
  timeout_ms: 30000
  tiers:
    fast:
      model: claude-haiku-4-5-20250301
      max_tokens: 8192
    standard:
      model: claude-sonnet-4-6-20250514
      max_tokens: 16384
    powerful:
      model: claude-opus-4-6-20250514
      max_tokens: 32768
```

The `auto_route` flag controls whether Fort picks the tier or always uses `default_tier`. Set `auto_route: false` to lock all calls to a single tier.
