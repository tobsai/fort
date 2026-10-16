# Candidate B independent review — 2026-10-16

**Decision: useful, honestly labelled reconstruction; keep live activation blocked.** One reproducible cancellation defect needs fixing. Provider compatibility, host confinement and owned-process shutdown remain unproven. This review does not consume Packet A or authorize execution.

Reviewed worktree: `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`; branch `codex/fort-openai-rebuild-20261016`; source `385277a07c2da772bac8a0b5efa0276b4c65e2a4`; evidence HEAD `0dd6363`. Diff base: `ac2dd2ae55b3df4235d352f34937c33f1fe932c3`. The reviewer model variant is not exposed by the available tools; this report does not claim verified Astra identity.

## Standards / runtime findings

**P1 — A canceled operation can still send a new provider turn.** `successor/runner/codex-chat.ts:73–75` subscribes, sends `turn/start`, then checks the abort signal. Cancellation after the valid `thread/start` response but before the continuation produces a new turn after cancellation. Pure in-memory reproduction at `/tmp/fort-candidate-b-abort-race.mjs` returned `sentAfterAbort:true` and then `outcomeUnknown`; independently rerun using `/opt/homebrew/opt/node@26/bin/node`. Fix: check cancellation immediately before dispatch, classify it as known not-started, and assert zero `turn/start` writes for this interleaving. This matters when shutdown or a bounded session deadline arrives during preflight.

**Activation blocker — Runtime lifetime is not owned by service shutdown.** `codex-chat.ts:63` keeps the connection after success; `runner/chat-runtime.ts:10` has no disposal operation; `service/openai.ts:290` aborts active work and closes HTTP/SQLite without disposing an idle runtime. The adapter test closes the connection manually. This agrees with the packet's admitted absent launcher; no actual process leak was observed. Before activation, add a bounded owned-process lifecycle with disposal acknowledgment and synthetic tests for idle shutdown, active shutdown, late output, ignored interruption and deadline expiry. Transport closure alone cannot prove child-process exit.

No additional material defect found in transactional request acceptance, immutable configuration, queued-context ordering or restart fencing. No blocking maintainability smell found beyond lifecycle ownership.

## Specification / native findings

**P2 — Stale workspace edits can overwrite newer settings.** `apple/FortKit/Sources/FortKit/WorkspaceEditor.swift:55–63` loads form values once but fetches the latest polled Agent/skill revision when Save is pressed. Another client's edit can therefore be overwritten by old form values carrying the new revision. Retain and submit the revision originally loaded into the form; test a second-client edit between Load and Save. Nonblocking for the prescribed single-client five-message path.

**Restart gate remains essential.** `MessengerModel.swift:106–120` automatically submits eligible native pending requests. The server only holds requests it already accepted. Before the second bounded start, verify all four first-session samples completed and the native pending queue is empty. Otherwise previously unaccepted work can execute with the fifth sample. Keep the original two-start/five-sample/per-start-duration bounds in the fresh candidate-specific execution packet; old pins and approval do not transfer.

**Synthetic native coverage does not exercise the actual messenger.** `FortNativeCheck/main.swift:33–82` calls `FortClient`/`PendingStore` directly and manually removes accepted requests. It does not exercise `MessengerModel`, editor interactions or SwiftUI. Add an OpenAI-mode model test for response loss, restart replay, exact recipient/skill retention and one durable run. Real model comprehension, skill adherence and UI behavior remain unaccepted.

## Provider / host boundary

The ordinary pinned [ThreadStartParams schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ThreadStartParams.ts) does not declare the requested empty environment/tool/capability-root fields. The pinned [SandboxPolicy schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/SandboxPolicy.ts) does not declare the adapter's restricted `access` object. Consequently these requests are not evidence of effective controls. The pinned [UserInput schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/UserInput.ts) also includes `text_elements`, absent from the emitted text item; actual defaulting/acceptance needs verification, not an assumed rejection.

Before any live proposal, verify the exact executable/configuration protocol and independently constrain host authority, ambient configuration/plugins, tools, provider credential access and non-provider network access. Rejecting a tool event is not proof that its side effect was prevented. Keep provider authentication in its designated provider store; do not extract tokens or alter existing sign-in as part of this review. Requested model selection is preserved, but effective-model attestation is correctly reported unavailable.

## Verified evidence and limits

- Recomputed all **748 source-file and five app-file hashes**: no mismatches. Source-to-evidence diff contains only the five evidence files.
- Independently ran **two adapter tests and seven candidate service tests: all passed** on Node **26.5.0**. The initial service attempt failed solely because sandbox policy denied loopback `listen`; the approved nonlive rerun passed. No provider runtime was invoked.
- Independently reproduced the cancellation race using only in-memory streams.
- Read the stored 128-test and five-turn synthetic logs. Swift 21-test, TypeScript and Mac-build results remain owner evidence, not independently rerun checks. App hashes establish identity, not UI or provider correctness.
- Exact receipts, recipient attribution, selected skill revisions, conversation-isolated context and successful-work restart are structurally implemented and supported by the targeted tests. Unknown closure, held-queue resumption and pending replacement remain acknowledged future operations.
- No implementation edits, real credentials, sign-in, security changes, deployment or provider calls. Owner working tree remained `?? .build/`. Only this review and the in-memory repro were written under `/tmp`.

Smallest next step: fix the abort race and stale edit revision, add the targeted nonlive tests, then review an owned launcher and verified runtime/host configuration. Build and pin the changed candidate before requesting a fresh bounded live execution decision.
