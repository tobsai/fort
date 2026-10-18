# Fort v1 acceptance ledger

Active design: [Spec 059](../specs/059-modular-node-v1.md). Owner decision:
[Node foundation](../docs/architecture/decisions/059-node-foundation.md).
Implementation owner continues end to end. Planned means no claim of completion.
Historical source through `09788c1` remains behavior/reference evidence; its 120
service/runtime and 56 native tests are not a pass for the new foundation.

| Requirement | Implemented | Tests | Build/package | Installed/provider acceptance | Remaining |
| --- | --- | --- | --- | --- | --- |
| Self-contained sandboxed Node/Swift bundle | Dependency closure and Swift sandbox probe | Relocation test plus 2 sandbox scenarios passed; strict typecheck passed | Signed Swift parent and inherited Node child execute SQLite | Synthetic denial, normal exit and hung-child cleanup passed; provider/physical unrun | Full runtime/IPC/lifecycle and distribution proof; [sandbox evidence](evidence/2026-10-17-node-foundation/sandbox.md), [timeout correction](evidence/2026-10-17-node-foundation/timeout-cleanup.md) |
| Provider-neutral core/native contracts | Explicit runtime descriptor and neutral model/skill selection | Current registered Node suite 752 passed; Swift 57 passed; strict typecheck passed | Source and Swift package only | Unrun | Independent sessionful/stateless contract suite, adapter-absent artifact and capability enforcement; [evidence](evidence/2026-10-17-node-foundation/provider-boundary.md) |
| Indexed compact ledger and durable outbox | Compact receipts/context references, indexed 100-message pages, per-conversation queue bound | Registered Node 753 passed; 200-turn indexed database 1,462,272 bytes; real Swift/HTTP conformance passed | Source and Swift client | Unrun | Synthetic copy/rollback passed ([migration](evidence/2026-10-17-node-foundation/migration.md)); indexed dispatch, global bound and outbox recovery remain; [evidence](evidence/2026-10-17-node-foundation/paging.md) |
| Service-owned attempts/cancel/evidence | Service ID/generation passed into Codex; mismatch rejected | 440 affected synthetic tests passed; strict typecheck passed | Source only | Unrun | Process-boundary envelope, stale/duplicate evidence, digests and limits; [evidence](evidence/2026-10-17-node-foundation/attempts.md) |
| Optional memory/voice/connectors/VM boundary | Planned | Planned | Unrun | Unrun | Module-absent artifact/behavior proof, scoped connector fixture |
| Bounded event-driven scheduler/refresh | Scheduler uses due-time wakeups; native active clients wait on bounded invalidation replay | 76 affected service/migration tests and 60 Swift tests passed; compiled native HTTP replay passed | Source and Swift package | Installed UI unrun | Measured idle budgets and installed UI remain; [scheduler](evidence/2026-10-17-node-foundation/scheduler.md), [replay](evidence/2026-10-17-node-foundation/replay.md); full Node suite now 759 passed after [readiness correction](evidence/2026-10-17-node-foundation/process-readiness.md) |
| Identity/model/avatar/settings/image display | Prior implementation | Prior fixtures only | Prior engineering apps | New installed QA open | Carry behavior through new contracts and exact builds |
| Reviewed skills and routines | Prior implementation | Prior fixtures only | Prior engineering apps | Open | Retain behavior and isolate optional context |
| Pairing/disconnect/removal/history choices | Intent/retry prototype | Prior fixtures only | Real adapter absent | Unrun | Owned process/startup cleanup, expiry/lost-response recovery |
| Native iPhone/Mac journeys | Prior clients | Prior fixtures only | Prior engineering apps | Physical TestFlight unrun | Reachable authenticated transport and actual provider |
| Performance budgets | Planned | Controlled measurements unrun | Unrun | Unrun | Record conditions and separate provider latency |
| Quality CI / signed Apple lane | Planned | Local commands exist | Unrun in CI | Unconfigured | GitHub workflow and authorized Xcode Cloud setup |

## Delivery matrix

| Platform | Source/tests | Build | Archive | Upload | Apple processed | Tester available | Physical acceptance |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Mac | New foundation planned | Prior engineering only | Unrun | Unrun | Unverified | Unverified | Unrun |
| iPhone | New foundation planned | Prior Simulator only | Unrun | Unrun | Unverified | Unverified | Unrun |

## Authority and current blockers

Ordinary coding, tests, routine fixes, CI files and requested delivery work are
authorized by the source decision. Do not request routine staged approvals.
Existing signing/account metadata needs verification; new credentials, grants,
security settings, agreements or destructive actions retain their mandatory
per-action gates. Expired demos are not reusable. No blocker may be relabeled
as completion; continue independent work and name any exact unavailable action.

The first read-only dependency inventory found Node 26 linked to Homebrew
libraries. The first packaging slice must remove that runtime prerequisite from
its produced bundle before any self-contained claim. No provider was started,
credential accessed or persistent service installed for this inventory.
