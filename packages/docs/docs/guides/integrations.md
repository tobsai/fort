---
sidebar_position: 16
title: Integrations
description: Gmail, Calendar, iMessage, Brave Search, and Browser integrations
---

# Integrations

Fort ships with five integrations that connect it to external services. Each integration publishes events on the ModuleBus and respects the permission tier system.

## Gmail

**Auth:** OAuth2 with Google.

**Capabilities:** List messages, read messages, create drafts, send email.

```bash
# List recent messages
fort gmail list --limit 20

# Read a specific message
fort gmail read <message-id>

# Create a draft
fort gmail draft --to "alice@example.com" --subject "Meeting notes" --body "..."

# Send (Tier 3 — requires approval)
fort gmail send --to "alice@example.com" --subject "Meeting notes" --body "..."
```

Sending email is gated at **Tier 3** (Approve). Fort will describe the email and wait for your explicit confirmation before sending. Drafting is Tier 2 — Fort creates the draft and shows it to you for review.

**Events:** `email:listed`, `email:read`, `email:drafted`, `email:sent`.

## Calendar

**Auth:** Google Calendar API via OAuth2.

**Capabilities:** List events, create events, update events, free time analysis.

```bash
# List today's events
fort calendar list --date today

# List events for a range
fort calendar list --from 2026-03-21 --to 2026-03-28

# Create an event
fort calendar create --title "Standup" --date 2026-03-22 --time 09:00 --duration 30m

# Find free time
fort calendar free --date 2026-03-22 --min-duration 60m
```

Event creation is **Tier 3**. The free time analysis is Tier 1 (read-only).

Free time analysis scans your calendar for gaps and returns available blocks:

```bash
fort calendar free --date 2026-03-22 --min-duration 60m
# Free blocks on 2026-03-22:
#   08:00 - 09:00 (60m)
#   10:30 - 12:00 (90m)
#   14:00 - 17:00 (180m)
```

**Events:** `calendar:listed`, `calendar:event:created`, `calendar:event:updated`, `calendar:free:analyzed`.

## iMessage

**Auth:** AppleScript bridge + read access to `chat.db`.

**Capabilities:** Read recent messages, send messages (to allowlisted recipients).

```bash
# Read recent messages
fort imessage list --limit 10

# Read conversation with a contact
fort imessage read --contact "Alice Smith" --limit 20

# Send a message (Tier 2 — creates draft for review)
fort imessage send --to "+15551234567" --body "Running 10 min late"
```

Sending is gated at **Tier 2** (Draft). Fort composes the message and shows it to you before dispatching via AppleScript.

**Recipient allowlist:** Only contacts on the allowlist can receive messages. Configure in `.fort/data/integrations/imessage.yaml`:

```yaml
allowlist:
  - "+15551234567"
  - "+15559876543"
  - "alice@icloud.com"
```

Attempts to send to contacts not on the allowlist are blocked.

**Events:** `imessage:listed`, `imessage:read`, `imessage:sent`.

## Brave Search

**Auth:** Brave Search API key.

**Capabilities:** Web search, search with summarization.

```bash
# Search the web
fort search "rust async runtime comparison"

# Search with AI summary
fort search "best practices for SQLite WAL mode" --summarize
```

Search is **Tier 1** (Auto). Results are returned as structured data with title, URL, and snippet.

The `--summarize` flag runs the search results through Fort's summarization pipeline, producing a concise answer with source citations.

**Events:** `search:completed`, `search:summarized`.

## Browser

**Auth:** Local Playwright instance.

**Capabilities:** Navigate to URLs, extract page content, fill and submit forms.

```bash
# Navigate and extract content
fort browser open "https://example.com/docs/api"
fort browser extract --url "https://example.com/docs/api" --selector "main"

# Fill a form (Tier 3 — requires approval)
fort browser fill --url "https://example.com/form" --fields '{"name": "Fort", "email": "..."}'
```

### Site Allowlist

The browser only navigates to allowlisted domains. Configure in `.fort/data/integrations/browser.yaml`:

```yaml
allowlist:
  - "docs.example.com"
  - "api.example.com"
  - "github.com"
```

Navigation to non-allowlisted sites is blocked.

### Content Sanitization

All content extracted from web pages is sanitized to strip prompt injection patterns. The sanitizer removes 12 known injection techniques:

1. Hidden text (CSS `display:none`, zero-size fonts)
2. Invisible Unicode characters
3. Base64-encoded instruction blocks
4. HTML comment injections
5. Data attribute instructions
6. Zero-width joiners carrying payloads
7. CSS content property text
8. Homoglyph-based obfuscation
9. RTL override characters
10. Markdown injection in extracted text
11. JavaScript protocol URIs
12. Meta refresh redirect chains

Form submission is **Tier 3** (Approve). Navigation and extraction are Tier 2.

**Events:** `browser:navigated`, `browser:extracted`, `browser:form:submitted`.

## Common Patterns

All integrations follow the same patterns:

- **ModuleBus events** for every action, enabling other modules to react.
- **Permission tiers** control what requires approval.
- **Structured logging** via the task graph.
- **Diagnose method** for health checks via `fort introspect modules`.
