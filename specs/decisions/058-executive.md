# Fort MVP decision

**Build native iPhone and Mac clients alongside a small Fort-owned service, using selected official agent runtimes.** Keep the imported OpenMaus source quarantined and extract only the protocol code that passes Fort's contracts.

Status: Proposed on 2026-10-12. [Spec 058](../058-native-first-owned-agent-mvp.md) contains the authoritative requirements if approved. This page does not authorize implementation or distribution.

## What the owner gets

| Decision | MVP result |
| --- | --- |
| Native first | SwiftUI iPhone **and** Mac apps are required from the first runnable slice. This is the explicit interpretation of the latest request |
| Own the Agent | Stable named Agents, permanent Home Conversations, selected official runtime/model/host, and durable history across devices |
| Own the workflow | Reviewed Markdown skill bundles and native forms for manual routines and schedules. No TypeScript authoring requirement |
| Choose execution | Local Mac or always-on self-hosted server, using the same contract and clients. Provider inference remains external |
| Trust the status | Durable acceptance, exact approval authority, restart/replay, and visible uncertainty when an external effect cannot be confirmed |
| Stay independent | No managed OpenMaus account, worker, telemetry, updater, or tunnel in the shipping product |

## Build in this order

1. Prove both native clients against one fixture service and transactional ledger.
2. Complete a real selected-model conversation through one official Codex runtime. Restart the service and recover the same history on both devices.
3. Add Claude Code through the same conformance tests, then reviewed skills and native manual routines.
4. Add durable schedules and prove the always-on self-hosted topology, including provider suitability and restore.
5. Finish accessibility, notifications, exact-device testing, and signed distribution for the clients already in use.

The web is an administration and diagnostics companion. It cannot replace either native app at an acceptance gate. Watch, CarPlay, voice, Google, Groups, and general workflow graphs remain later decisions.

## The choices that matter

The recommended backend is a small TypeScript service with one transactional SQLite ledger per single-writer installation. This is a selective-extraction proposal, not approval for a broad rewrite. SwiftUI implements the Apple clients; reviewed bundle content and native forms define the owner-facing extension and routine interfaces. These language decisions are independent.

Local execution stops being available when its service host sleeps. The iPhone preserves pending work and reconciles on reconnect; it does not run an independent background agent. An always-on self-hosted service can continue accepted work while the owner's devices sleep. Optional Apple push wakes clients; the ledger remains authoritative.

Official subscription access must be proved for each provider in each topology. The current source proves none of those four cells. Anthropic's restrictions on third-party subscription authentication make the exact personal-wrapper and server cases a feasibility gate. No fallback to API billing or a different provider is allowed without a decision. A failed required cell blocks a claim that the full MVP is complete.

## First success worth reviewing

On a physical iPhone and a Mac, send to the same Agent/Home Conversation, receive one real provider reply, approve one bounded action, interrupt a run, restart the service, and recover the exact accepted history and truthful state on both devices. Then repeat the common tests with provider two. Build success alone is not completion.

The existing foundation remains source-only: 72 inventoried entrypoints refuse to run and the quarantine checks pass 7/7. The proposed service, durable ledger, isolated runner, and native clients are not implemented or validated by those checks.

The next owner decision is approval or amendment of [Spec 058](../058-native-first-owned-agent-mvp.md). Provider suitability, exact host setup, release accounts, and any cost are resolved before their dependent work. No dates are promised.
