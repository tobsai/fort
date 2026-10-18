# Fort v1 acceptance ledger

Active design: [Spec 059](../specs/059-modular-node-v1.md). Owner decision:
[Node foundation](../docs/architecture/decisions/059-node-foundation.md).
Implementation owner continues end to end. Planned means no claim of completion.
Historical source through `09788c1` remains behavior/reference evidence; its 120
service/runtime and 56 native tests are not a pass for the new foundation.

| Requirement | Implemented | Tests | Build/package | Installed/provider acceptance | Remaining |
| --- | --- | --- | --- | --- | --- |
| Self-contained sandboxed Node/Swift bundle | Dependency closure, compiled core, guarded XPC setup/cancellation and abrupt helper recovery | 4 signed scenarios, 111 Swift tests and strict typecheck passed; last portable suite 892 passed | Retained signed cancellation/recovery app; 7 unchanged entitlement targets and both 26-file core copies verified | Synthetic setup/cancel/restart preserves receipt; 13 processes, 5 groups and 4 containers absent; provider/physical unrun | Durable client recovery/runtime/removal and distribution proof; [current proof](evidence/2026-10-17-node-foundation/owner-setup-cancellation-xpc.md), [earlier crash failures](evidence/2026-10-17-node-foundation/xpc-crash-recovery.md) |
| Provider-neutral core/native contracts | Explicit runtime selection, neutral attempt contract and context capability enforcement | Independent sessionful/stateless artifact contracts passed; 818 portable tests and strict typecheck passed | Compiled core excludes Codex/prototype/donor implementations; native package 60 tests passed at replay slice | Actual providers unrun | Optional module removal and real runtime packaging remain; [core artifact](evidence/2026-10-17-node-foundation/core-artifact.md), [capabilities](evidence/2026-10-17-node-foundation/capabilities.md) |
| Indexed compact ledger and durable outbox | Compact receipts/context, indexed pages and durable outbox with conversation/installation bounds | Registered Node 762 passed; 79 affected tests and real Swift/HTTP passed | Source and Swift client | Unrun | [Copy/rollback](evidence/2026-10-17-node-foundation/migration.md), indexed dispatch and restart barriers passed; controlled load and process boundary remain; [outbox evidence](evidence/2026-10-17-node-foundation/outbox.md) |
| Service-owned attempts/cancel/evidence | Immutable dispatch and explicit executor lifecycle with Mac parent-exit ownership | 8 guarded executor checks and 849 portable tests passed; strict typecheck passed | Source and isolated synthetic processes | Official process integration unrun | Official adapter migration, signed/installed executor containment and crash recovery; [parent-exit evidence](evidence/2026-10-17-node-foundation/executor-parent-exit.md), [process contract](evidence/2026-10-17-node-foundation/executor-process.md) |
| Optional memory/voice/connectors/VM boundary | Versioned local context compiler removed from core/runtime adapter dependencies; saved routines pin its identity | 9 module checks and 827 portable tests passed; strict typecheck and compiled native HTTP passed | Core artifact omits optional implementation; independent runtime contracts pass | Installed presentation unrun | Voice/connector/VM contracts, scoped connector authority and lifecycle; [context evidence](evidence/2026-10-17-node-foundation/context-modules.md), [routine pins](evidence/2026-10-17-node-foundation/routine-context-pins.md) |
| Bounded event-driven scheduler/refresh | Due-time scheduling and bounded invalidation replay; read advances emit events and duplicate reads make no writes | 65 service and 829 portable tests passed; strict typecheck and compiled native HTTP passed; 60 Swift tests passed at the original replay slice | Source and Swift package | Installed UI unrun | Core idle budgets passed; installed UI remains; [scheduler](evidence/2026-10-17-node-foundation/scheduler.md), [replay and read correction](evidence/2026-10-17-node-foundation/replay.md), [performance](evidence/2026-10-17-node-foundation/performance.md) |
| Identity/model/avatar/settings/image display | Prior implementation | Prior fixtures only | Prior engineering apps | New installed QA open | Carry behavior through new contracts and exact builds |
| Reviewed skills and routines | Prior implementation | Prior fixtures only | Prior engineering apps | Open | Retain behavior and isolate optional context |
| Pairing/disconnect/removal/history choices | Durable client preparation; exact cancellation through core, pipe and pinned native XPC | 111 Swift and 4 signed scenarios passed; prior 14 guardian, 33 core controls, 20 pipe and 892 portable remain separate evidence | Retained signed cancellation/restart passed; earlier fresh unsigned clients; real Mac lifecycle adapter absent | Signed synthetic cancellation passed; actual Keychain and installed lifecycle unrun | Native pending recovery, ordinary app and startup cleanup; [signed cancellation](evidence/2026-10-17-node-foundation/owner-setup-cancellation-xpc.md), [durable preparation](evidence/2026-10-17-node-foundation/owner-setup-preparation.md), [removal handoff](evidence/2026-10-17-node-foundation/removal-handoff.md) |
| Native iPhone/Mac journeys | Mac setup preparation and signed cancellation controls, shared values, XPC/core bridge and endpoint restoration | 111 Swift and four signed scenarios passed | Earlier fresh unsigned Mac/iPhone Simulator builds passed; CoreSimulator remains outdated | Physical TestFlight unrun | Pending recovery/ordinary app lifecycle, reachable iPhone transport and provider; [native build limits](evidence/2026-10-17-node-foundation/owner-setup-cancellation-native.md), [signed cancellation](evidence/2026-10-17-node-foundation/owner-setup-cancellation-xpc.md) |
| Private core host channel | Pinned setup/cancellation/recovery, parent-exit guards and recorded private-socket crash recovery | 111 Swift and signed cancellation/abrupt recovery passed; prior 15 socket, 20 pipe and 892 portable checks remain separate | Current 26-file vendor-free core and retained signed artifact, both copies verified | Synthetic setup/cancel/restart preserves receipt; installed/provider unrun | Unknown startup-window sockets and ordinary host setup; [signed cancellation/recovery](evidence/2026-10-17-node-foundation/owner-setup-cancellation-xpc.md), [socket ownership](evidence/2026-10-17-node-foundation/private-socket-recovery.md) |
| Performance budgets | Reproducible core CLI and captured native build procedure | 828 portable tests, strict typecheck and 18 gate tests passed; all 11 core budgets passed | Five clean/incremental samples per core and native platform passed | Installed/bundled-host/provider measurements unrun | Core-only idle RSS 100.27/94.42 MiB, acceptance p95 14.67 ms, page p95 13.94 ms; preserved earlier combined-process failures; [conditions and raw evidence](evidence/2026-10-17-node-foundation/performance.md) |
| Native setup file durability cost | Private atomic records with file/directory sync and no idle work | 20 isolated synthetic runs: p95 22.06 ms against 500 ms target | Fresh native engineering builds passed | Actual Keychain/XPC/installed timing excluded | [Conditions, all samples and limits](evidence/2026-10-17-node-foundation/owner-setup-preparation.md#local-preparation-timing) |
| Quality CI / signed Apple lane | Test discovery and read-only GitHub workflow prepared | 792 portable, 2 macOS and 18 gate tests passed; strict typecheck and YAML parse passed | Unrun in CI | Signed Apple lane unconfigured | Linux/GitHub execution, PR scope enforcement and authorized Xcode Cloud setup; [evidence](evidence/2026-10-17-node-foundation/quality-ci.md) |

## Delivery matrix

| Platform | Source/tests | Build | Archive | Upload | Apple processed | Tester available | Physical acceptance |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Mac | Foundation migration in progress | Fresh unsigned native client only | Unrun | Unrun | Unverified | Unverified | Unrun |
| iPhone | Foundation migration in progress | Fresh unsigned Simulator client only | Unrun | Unrun | Unverified | Unverified | Unrun |

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
