---
sidebar_position: 9
title: Tools
---

# Tools

The Tool Registry is Fort's mechanism for enforcing **reuse-before-build**. Every capability — sending email, running tests, searching code — is a registered tool. Before building anything new, agents and developers must search the registry to check if an existing tool handles the need.

## How the Registry Works

The Tool Registry is backed by SQLite and stores tool definitions with:

- **Name** — Unique identifier (e.g., `email_send`, `git_diff`)
- **Description** — What the tool does
- **Capabilities** — List of specific abilities (e.g., `send`, `draft`, `attach`)
- **Tags** — Searchable keywords (e.g., `email`, `communication`, `notification`)
- **Usage count** — How many times the tool has been invoked

When the Orchestrator plans a task, it searches the registry for matching tools before considering new implementations. The Harness (Fort's code generation subsystem) also checks the registry before creating new tools.

## Registering Tools

Register a new tool:

```bash
fort tools register \
  --name email_send \
  --description "Send an email via configured SMTP or API" \
  --capabilities "send,draft,attach,reply" \
  --tags "email,communication,notification"
```

Tools can also be registered programmatically:

```typescript
await toolRegistry.register({
  name: 'email_send',
  description: 'Send an email via configured SMTP or API',
  capabilities: ['send', 'draft', 'attach', 'reply'],
  tags: ['email', 'communication', 'notification'],
});
```

## Searching Tools

Multi-term search finds tools matching any combination of name, description, capabilities, or tags:

```bash
fort tools search "email"
```

```
NAME          DESCRIPTION                              TAGS                          USES
email_send    Send an email via configured SMTP or API email,communication           47
email_read    Read emails from configured inbox         email,imap,communication     23
email_parse   Parse email content and extract fields    email,parsing                12
```

Search with multiple terms:

```bash
fort tools search "git review"
```

```
NAME          DESCRIPTION                              TAGS                    USES
git_diff      Show file differences between commits     git,code,review        89
git_log       Show commit history                       git,code,history       34
code_review   Analyze code for issues and style          code,review,quality   56
```

## Listing Tools

List all registered tools:

```bash
fort tools list
```

```
NAME            CAPABILITIES                TAGS                       USES
email_send      send,draft,attach,reply     email,communication        47
email_read      read,search,filter          email,imap                 23
git_diff        diff,compare                git,code,review            89
git_log         log,history,blame           git,code,history           34
test_runner     run,watch,coverage          testing,ci                 112
build_tool      build,bundle,optimize       build,ci,deploy            67
deploy_tool     deploy,rollback,status      deploy,infrastructure      28
metrics_api     query,aggregate,export      metrics,monitoring         41
```

## Inspecting Tools

Get full details on a tool including its usage history:

```bash
fort tools inspect email_send
```

```
Tool: email_send
Description: Send an email via configured SMTP or API
Capabilities: send, draft, attach, reply
Tags: email, communication, notification
Registered: 2026-03-01
Total Uses: 47

Recent Invocations:
  tk-201  2026-03-21 09:00  Morning Briefing (success)
  tk-198  2026-03-20 09:00  Morning Briefing (success)
  tk-195  2026-03-20 14:30  Email Triage (success)
  tk-190  2026-03-19 09:00  Morning Briefing (failed)

Used By Agents:
  Email Triager (31 times)
  Orchestrator (16 times)
```

## Usage Counting

Every time a tool is invoked through a flow or agent, its usage count increments. This data helps identify:

- **High-value tools** that justify investment in reliability
- **Unused tools** that can be retired
- **Missing tools** when agents frequently fall back to LLM reasoning for tasks that should be deterministic

## The Reuse-Before-Build Rule

Fort's development rules require searching the registry before creating new tools. The workflow:

1. Identify the needed capability
2. Run `fort tools search` with relevant terms
3. If a matching tool exists, use it
4. If no match, register the new tool before implementing it

This prevents duplicate implementations and keeps the tool surface area manageable. The Harness enforces this programmatically — it will not generate a new tool without first confirming no registry match exists.

## Updating and Removing Tools

Update a tool's metadata:

```bash
fort tools update email_send \
  --description "Send email via SendGrid API" \
  --tags "email,communication,sendgrid"
```

Remove a tool from the registry:

```bash
fort tools remove email_send
```

Removing a tool does not delete its implementation code. It only removes the registry entry, preventing it from appearing in searches and agent capability matching.
