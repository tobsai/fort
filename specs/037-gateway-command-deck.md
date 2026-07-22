# 037 — Gateway Command Deck

**Status:** approved by the requested Fort dashboard rollout (Toby, 2026-07-22)
**Governed by:** [028-remote-gateway](028-remote-gateway.md) · [033-dashboard-redesign](033-dashboard-redesign.md) · [036-playbooks](036-playbooks.md)

## Goal

Make the authenticated Vercel gateway feel like the remote edition of Fort's
Command Deck instead of a separate utility shell. Opening a registered machine
must immediately show current, decrypted board data in the approved inbox-first
layout: **Needs you**, **Projects**, **Up next**, and **Crew**.

The gateway keeps its existing Google allowlist, daemon-key pinning, and
Noise/AEAD relay. This is a presentation and existing-client-capability change;
it adds no routing, inference, broker, or daemon HTTP contract.

## Approach

1. Give the gateway shell the normative Fort tokens, typography, attention
   hierarchy, language, responsive two-pane layout, and visible focus states
   from `design_handoff_fort_dashboard_redesign/`.
2. After TOFU pin validation, automatically establish a sealed session and read
   `/api/summary`, `/api/board`, `/api/backlog`, and `/api/machines`. Refresh the
   deck periodically and on demand. A machine/key change or explicit refresh
   clears the prior deck. A quiet poll may retain the same identity's
   last-known-good deck, but it must retain the last-success timestamp and show
   a persistent error if that poll fails.
3. Render the primary Command Deck natively in React from those decrypted
   payloads. Human decisions and handoffs continue to use sealed requests to
   the existing `/api/gate`, `/api/playbooks`, `/api/route`, `/api/chat`, and
   backlog-dispatch endpoints. Direction dispatch shows the resolved stages,
   lets the user switch to another catalog playbook, and requires confirmation
   before it pins the immutable route. Answer delivery always disables the plan
   gate.
4. Keep the raw HTML snapshot and encrypted event tail available as secondary
   diagnostics. The snapshot remains sandboxed and scripts-disabled; a fully
   proxied iframe is still deferred by spec 028.
5. Keep machine enrollment, revocation, fingerprint verification, and sign-out
   visible but visually subordinate to the work that needs attention.
6. Fail closed at the sealed-request boundary unless verification belongs to
   the current machine/key identity. Every short-lived relay session sends a
   `bye` frame after queued work; the Activity stream sends one when stopped.
7. Coalesce interval polls, but let post-action refreshes supersede an older
   poll and reject its late result. Deck-refresh errors persist until a
   successful refresh; action errors are scoped separately from deck state,
   and Activity stream errors remain inside the Activity diagnostic. Refresh
   the playbook catalog for every new handoff so route choices cannot go stale.
   The green connection state means a trusted sealed response succeeded, not
   merely that the broker reports a socket.

## Non-goals

- No plaintext board data in Vercel server components, logs, or the Worker.
- No change to the Worker, Durable Object, relay framing, or crypto.
- No new daemon endpoint and no relaxation of the snapshot iframe sandbox.
- No method-version, scheduler, or other data source deferred by spec 033.

## Affected files

- `gateway/web/app/layout.tsx`
- `gateway/web/app/page.tsx`
- `gateway/web/app/m/[id]/page.tsx`
- `gateway/web/app/signin/page.tsx`
- `gateway/web/app/add/page.tsx`
- `gateway/web/components/board-client.tsx`
- `gateway/web/components/command-deck-surface.tsx`
- `gateway/web/lib/command-deck.ts`
- `gateway/web/lib/relay-client.ts`
- `gateway/web/app/globals.css`
- `gateway/web/test/command-deck.test.ts`
- `gateway/web/test/relay-client.test.ts`

## Verification

- Presentation helpers deterministically map gates, statuses, checkpoints, and
  roster-based per-agent activity without using agent-estimated progress.
- Source contract checks pin the approved vocabulary, design tokens, automatic
  sealed data load, identity-bound pin validation, route pinning, relay-session
  cleanup, stale-snapshot clearing, and scripts-disabled iframe.
- `npm test --workspace web`
- `npm run typecheck --workspace web`
- `npm run build --workspace web`
- Browser QA at desktop and phone widths against handoff view 1a, including
  loading, empty, live-data, error, and expanded diagnostic states.
- Production deployment is READY and the canonical alias serves the new build.

## Rollback

Roll back the Vercel production alias to the preceding deployment. No persisted
data or transport migration is involved.
