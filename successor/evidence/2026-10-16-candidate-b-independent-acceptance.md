# Candidate B targeted fixes — independent review

Reviewed source `73f644425a74463488cdce61aab8b83384ce8722`, evidence checkpoint `772410e`, branch `codex/fort-openai-rebuild-20261016`, in `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`. Comparison base: `0dd6363a695dee97f650b1c6c1fd62f2db67e695`.

**Conclusion: the three previous findings are resolved within the bounded prototype scope. No new material code blocker was found in this targeted review. Live activation remains blocked pending exact-runtime control evidence and a fresh candidate-specific execution decision.**

## Runtime / standards

- **Cancellation race resolved.** `successor/runner/codex-chat.ts:76` checks the signal before writing `turn/start`. The original independent in-memory reproduction now returns `sentAfterAbort:false`, methods only through `thread/start`, and `kind:notStarted`. The service maps this to canceled, not an uncertain dispatched turn.
- **Idle runtime ownership resolved.** `successor/service/openai.ts:293` requires and awaits runtime disposal even without active work, coalesces repeated close calls and propagates cleanup failure. The idle-service test verifies that close does not resolve before disposal confirmation.
- **Bounded owned-process cleanup demonstrated synthetically.** `successor/runner/owned-process.ts:32` closes transport, sends TERM, escalates to KILL, and requires both observed child close and absence of its process group. Tests establish forced exit after ignored TERM, rejection of late frames, idle deadline expiry and idempotent disposal. The active service test verifies an unknown outcome, no late assistant reply and no automatic retry after reopening the ledger.
- **Budget corrected.** `successor/prototype/owned-codex.ts:14` uses 296,000 ms for work plus 2,000 ms each for TERM and KILL, totaling 300,000 ms. This is the intended timer-based budget, not a hard-real-time guarantee. Group cleanup is not confinement of a child that escapes its group; the record accurately acknowledges that limit.

## Native / specification

- **Stale-form revision bug resolved.** `WorkspaceEditor.swift:67` retains the originally loaded Agent or skill in immutable `WorkspaceForm.source`. Save submits that revision despite polling. The new test advances revision one to two, then verifies both stale forms submit revision one and receive conflicts.
- **Relevant model coverage added.** `WorkspaceModelTests.swift:95` exercises actual MessengerModel send/restore behavior with a lost acceptance response, original recipient and skill revisions, identical replay after revision changes, one simulated run and an empty pending queue. The server is a simulation; real SQLite deduplication is independently covered by service tests.
- **Conditional restart evidence strengthened.** FortNativeCheck retains four distinct completed receipts, checks pending emptiness again, and revalidates the four durable completions before the fifth sample. The harness requires successful service close and synthetic runtime disposal first. The live packet must check the actual app/model queue and cleanup; harness state cannot stand in for them.

## Independent verification

- Recomputed **759 source-file hashes and five app-file hashes**, plus both canonical list hashes: all match.
- Verified preserved review and original reproduction hashes. Confirmed Apple source is unchanged between build commit `de9e482` and pinned source `73f6444`; the source-to-evidence delta contains only evidence files.
- Statically hashed the installed executable without running it: **238,223,808 bytes**, SHA-256 `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`, matching the launch pin.
- Independently ran **15 focused Node tests, all passing**, on the declared Node **26.5.0**: adapter, owned process, disabled launcher/budget, and candidate service. These used temporary loopback listeners and explicit Node test actors. The original cancellation reproduction also passed independently.
- Read the stored 135-test, 23-Swift-test and five-turn HTTP results. TypeScript and Mac compilation remain owner-reported checks; this review did not rebuild or rerun those suites. These are synthetic/compiler results, not UI or provider acceptance.
- No provider execution, credential access, security changes, deployment or publication. Candidate source remained unchanged; git status remains `?? .build/`. This report was written only under `/tmp`.

## Smallest remaining prerequisite

The remaining issue is **effective launch configuration and its agreed authority**, not another implementation redesign. The proposed launch retains the owner's HOME and disables only three named MCP entries. The [versioned configuration schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/core/config.schema.json) supports per-server configuration; static key presence does not establish the actual merged server set or effective tool restrictions. Binary markers likewise do not establish protocol behavior.

Prepare one fresh, exact candidate packet with a no-turn preflight gate: establish that the pinned executable accepts the exact arguments, that effective settings expose only the intended provider and capabilities, and that access to the existing provider store and residual same-user host authority are explicitly within the authorized scope. Do not expose credential contents. If the agreed scope requires stronger host confinement, demonstrate it before inference. If the preflight is uncertain or fails, stop before any sample rather than relax controls.

Retain the original bounds: at most two supervised starts, five exact samples, five minutes total per start, and second start only after four completed receipts, an empty actual native pending queue and confirmed service/runtime/app cleanup. Packet A does not authorize these changed pins. Real inference, actual provider cleanup and rendered native behavior are evidence the eventual manual test must collect. No unrelated hardening or wholesale rewrite is required by this review.
