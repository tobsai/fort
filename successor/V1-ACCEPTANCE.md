# Fort v1 acceptance ledger

Active design: [Spec 059](../specs/059-modular-node-v1.md). Owner decision:
[Node foundation](../docs/architecture/decisions/059-node-foundation.md).
Implementation owner continues end to end. Planned means no claim of completion.
Historical source through `09788c1` remains behavior/reference evidence; its 120
service/runtime and 56 native tests are not a pass for the new foundation.

| Requirement | Implemented | Tests | Build/package | Installed/provider acceptance | Remaining |
| --- | --- | --- | --- | --- | --- |
| Self-contained sandboxed Node/Swift bundle | Dependency closure, compiled core and signed Swift XPC peer feasibility | Relocation previously passed; 4 sandbox scenarios and strict typecheck passed | Signed private-socket core/restart and separate XPC signature-pin rejection proof | Synthetic denial, normal exit and hung-child cleanup passed; provider/physical unrun | XPC-to-core bridge, runtime/lifecycle and distribution proof; [XPC limits](evidence/2026-10-17-node-foundation/xpc.md), [compiled core evidence](evidence/2026-10-17-node-foundation/sandbox-core.md), [timeout correction](evidence/2026-10-17-node-foundation/timeout-cleanup.md) |
| Provider-neutral core/native contracts | Explicit runtime selection, neutral attempt contract and context capability enforcement | Independent sessionful/stateless artifact contracts passed; 818 portable tests and strict typecheck passed | Compiled core excludes Codex/prototype/donor implementations; native package 60 tests passed at replay slice | Actual providers unrun | Optional module removal and real runtime packaging remain; [core artifact](evidence/2026-10-17-node-foundation/core-artifact.md), [capabilities](evidence/2026-10-17-node-foundation/capabilities.md) |
| Indexed compact ledger and durable outbox | Compact receipts/context, indexed pages and durable outbox with conversation/installation bounds | Registered Node 762 passed; 79 affected tests and real Swift/HTTP passed | Source and Swift client | Unrun | [Copy/rollback](evidence/2026-10-17-node-foundation/migration.md), indexed dispatch and restart barriers passed; controlled load and process boundary remain; [outbox evidence](evidence/2026-10-17-node-foundation/outbox.md) |
| Service-owned attempts/cancel/evidence | Immutable dispatch with scope/digests/deadline/limits; optional process executor | 24 process/API checks and 816 portable tests passed; strict typecheck and compiled native HTTP passed | Source and isolated synthetic processes | Official process integration unrun | Official adapter migration, abrupt supervisor death and OS containment; [process evidence](evidence/2026-10-17-node-foundation/executor-process.md), [prior attempt evidence](evidence/2026-10-17-node-foundation/attempts.md) |
| Optional memory/voice/connectors/VM boundary | Versioned local context compiler removed from core/runtime adapter dependencies; saved routines pin its identity | 9 module checks and 827 portable tests passed; strict typecheck and compiled native HTTP passed | Core artifact omits optional implementation; independent runtime contracts pass | Installed presentation unrun | Voice/connector/VM contracts, scoped connector authority and lifecycle; [context evidence](evidence/2026-10-17-node-foundation/context-modules.md), [routine pins](evidence/2026-10-17-node-foundation/routine-context-pins.md) |
| Bounded event-driven scheduler/refresh | Due-time scheduling and bounded invalidation replay; read advances emit events and duplicate reads make no writes | 65 service and 829 portable tests passed; strict typecheck and compiled native HTTP passed; 60 Swift tests passed at the original replay slice | Source and Swift package | Installed UI unrun | Core idle budgets passed; installed UI remains; [scheduler](evidence/2026-10-17-node-foundation/scheduler.md), [replay and read correction](evidence/2026-10-17-node-foundation/replay.md), [performance](evidence/2026-10-17-node-foundation/performance.md) |
| Identity/model/avatar/settings/image display | Prior implementation | Prior fixtures only | Prior engineering apps | New installed QA open | Carry behavior through new contracts and exact builds |
| Reviewed skills and routines | Prior implementation | Prior fixtures only | Prior engineering apps | Open | Retain behavior and isolate optional context |
| Pairing/disconnect/removal/history choices | Intent/retry prototype, private-socket transport and explicit system-removal handoff | 63 Swift tests passed; prior 2 transport checks and 831 portable tests passed | Fresh unsigned Mac/iPhone Simulator builds passed; real Mac lifecycle adapter absent | Unrun | Owned process/startup cleanup and installed lifecycle; [removal handoff](evidence/2026-10-17-node-foundation/removal-handoff.md), [transport evidence](evidence/2026-10-17-node-foundation/private-transport.md) |
| Native iPhone/Mac journeys | Shared relative-request client; production Mac XPC adapter tested in the signed probe | 68 Swift tests, 4 signed scenarios and strict typecheck passed; preceding HTTP and 5 redirect checks passed | Fresh unsigned Mac/iPhone Simulator client builds passed | Physical TestFlight unrun | XPC-to-core bridge, device-storage migration, reachable iPhone transport and provider; [native XPC](evidence/2026-10-17-node-foundation/native-xpc.md), [transport boundary](evidence/2026-10-17-node-foundation/native-transport.md) |
| Private core host channel | Fixed private-socket relay and Swift-owned process transport with bounded I/O, cancellation and group cleanup | 74 Swift tests passed; native source/compiled core journeys passed; preceding 5 relay checks and 836 portable tests passed | 22-file vendor-free core artifact, all hashes checked; compiled core runs JIT-disabled through Swift | Signed XPC-to-core bridge unrun | XPC composition, bundle selection and real owner lifecycle; [Swift process evidence](evidence/2026-10-17-node-foundation/native-core-process.md), [relay evidence](evidence/2026-10-17-node-foundation/private-channel.md) |
| Performance budgets | Reproducible core CLI and captured native build procedure | 828 portable tests, strict typecheck and 18 gate tests passed; all 11 core budgets passed | Five clean/incremental samples per core and native platform passed | Installed/bundled-host/provider measurements unrun | Core-only idle RSS 100.27/94.42 MiB, acceptance p95 14.67 ms, page p95 13.94 ms; preserved earlier combined-process failures; [conditions and raw evidence](evidence/2026-10-17-node-foundation/performance.md) |
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
