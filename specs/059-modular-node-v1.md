# 059: Modular Node foundation and native v1

Status: Accepted design for implementation by Tobias on 2026-10-17. Implementation
and delivery evidence remain separate. This supersedes Spec 058's architecture
and reassessment pause; historical evidence remains unchanged.

## Goal and authority

Ship a working single-owner Fort for native iPhone and Mac through TestFlight,
using Node/TypeScript core and Swift UI/lifecycle. Apply the four principles while
preserving durable exact messaging and the owner's first-wave features. The actual
owner direction and limits are recorded in [decision 059](../docs/architecture/decisions/059-node-foundation.md):
messages `Sentinel_9d1ca780b6d48191b1e19cdea66dc56e` and
`Sentinel_667fda138e308191a979ce2aad0906f2` in source thread
`01a0f307-3940-7333-96b0-a3b0b854e0e8`. Ordinary development/verification and
requested delivery work are authorized; routine confirmations are unnecessary.
Security, credentials, persistent access, legal agreements and destructive actions
retain their applicable per-action gates. Do not reuse expired demos or weaken a
denial. No provider cleanup, unrelated project or shared hosting is in scope.

## Architecture review
Review status: accepted
Review owner: Tobias
Decision reference: docs/architecture/decisions/059-node-foundation.md; source user messages Sentinel_9d1ca780b6d48191b1e19cdea66dc56e and Sentinel_667fda138e308191a979ce2aad0906f2, 2026-10-17 23:21/23:22 UTC, source thread 01a0f307-3940-7333-96b0-a3b0b854e0e8
Reviewed content SHA256: 78e2e49119a5efcba40c216bb967d8c846be6dceac32152d6e29ffb8d86de1da

### Vendor independence
Status: planned
Affected boundaries: AI/runtime adapters, public contract, core application, native model selection; separately identify hosting, storage, optional-module and Apple distribution dependencies.
Dependency/removal impact: Core/UI do not require OpenAI/Codex. Opaque exact identities and capability discovery replace provider literals. SQLite is the single initial storage adapter; export/restore retains identities. Apple native/TestFlight is the explicit distribution dependency; local/self-hosted contracts stay platform-neutral.
Acceptance criteria: Run the same send/accept/replay/cancel contract against a sessionful fixture and an independently shaped stateless non-OpenAI fixture with unsupported capabilities. Build core/native contracts with the OpenAI adapter absent. Verify official provider subscriptions separately without fallback/billing substitution.
Evidence: Preserved review at docs/architecture/reviews/20261017/REVIEW.md documents existing coupling. Owner decision selects Node explicitly; this is not yet a provider-independence result.
Tradeoffs: Keep official Codex/Claude runtime adapters and subscription preference, exposing real capability differences. Apple is a disclosed platform dependency. Do not add alternative databases/frameworks solely to claim independence; revisit with measured requirements.
Unresolved judgment: none

### Modularity
Status: planned
Affected boundaries: Conversation application, ledger/outbox, authority, dispatch, routines/scheduler, runtime adapters, memory/context, voice, service connectors, VM/execution placement and Swift platform lifecycle.
Dependency/removal impact: Core owns stable contracts; adapters depend inward; composition chooses implementations. Memory/voice/connectors/VM packages and artifacts can be omitted while text messaging persists/replays. A binding requiring an absent capability fails explicitly. Connector credentials/grants remain separate from Fort device and provider account state.
Acceptance criteria: Build/run core messaging with each optional implementation absent; exercise context/connector fixtures then remove/re-add them without losing history. Reject wrong scope/arguments/digest/expiry/revoked connector actions. Service-owned attempt/generation evidence is idempotent and stale evidence fails. Prove crash/restart, exact cancellation and honest unknown outcomes through the public service/executor contracts.
Evidence: Review identifies competing attempts, dense service composition, coupled memory and separate fixture/live behavior. Existing behavior tests and durable identity semantics are retained as migration requirements, not proof of the new boundaries.
Tradeoffs: Use one modular service and one writer, with an executor containment boundary; no microservices, general plugin market or mandatory VM. Optional future voice/VM/service integrations are extension points, not extra v1 products. Revisit isolation per concrete module authority.
Unresolved judgment: none

### Speed
Status: planned
Affected boundaries: Node packaging, TypeScript checks, Swift builds, indexed ledger, outbox dispatch, scheduler and native refresh.
Dependency/removal impact: Retain Node by owner choice; remove repeated full-history persistence, whole-kind scans and per-second idle checkpoints. Native refresh uses events/wakeups and bounded reconciliation. Avoid required third-party server frameworks until needed.
Acceptance criteria: On recorded MacBookPro18,2/64 GiB/10 CPU, macOS 27.2, Node 26.5.0, TypeScript 6.0.3 and Xcode 27.1/Swift 6.4, target clean core check/build <=10 s, incremental <=3 s; clean native Debug build <=120 s, incremental <=20 s; core cold startup <=2 s and warm <=500 ms; local acceptance p95 <=25 ms and 100-message page p95 <=50 ms at 10,000 messages; idle core RSS <=128 MiB, CPU <=0.5% of one core and <=1 wakeup/s over 60 s; zero scheduler DB writes with no due/state-changing work over 60 s; 10,000 1-KiB messages <=64 MiB ledger. Measure at least 5 clean/warm build/start samples and 100 request samples, fixed workload, cache conditions and p50/p95; record provider/process latency and memory separately. These are engineering targets, not measured claims or promises about remote providers.
Evidence: Existing timings are incidental; no controlled results yet. Node links to Homebrew libraries on the development host, so package size/dependency closure and jitless overhead must be measured. The owner's Node choice supersedes a cross-language selection experiment.
Tradeoffs: Start without JIT entitlement expansion and measure the effect; do not silently relax sandbox/security to meet a speed target. If targets fail, profile the actual bottleneck and fix within Node before proposing a material design change. Track bundled runtime bytes separately from the app/core.
Unresolved judgment: none

### Installation and removal
Status: planned
Affected boundaries: Bundled Node/runtime dependency closure, Swift app/helper and lifecycle, pairing/revocation, reachable iPhone transport, package identity, CI/CD and retained data.
Dependency/removal impact: One self-contained native delivery with an explicitly owned per-user host. No Homebrew executable path or owner HOME is a shipping prerequisite. No hidden startup. No pairing expiry by default; optional expiry, separate disconnect/stop/unregister/app removal/history deletion. Retain history by default and do not claim revocation from local forgetting.
Acceptance criteria: First prove a sandboxed Swift bundle signed with the existing development identity launching bundled pinned Node with synthetic input, IPC, file/network negative controls and cleanup, without provider credentials or startup registration. Then verify exact official-runtime packaging/auth under actual authority, recoverable lifecycle and physical iPhone connectivity. GitHub quality commands run locally and in CI; proposed Xcode Cloud signed lane is separate. Record implementation/tests/build/archive/upload/Apple processing/tester availability/physical accessibility/screenshot/provider/install acceptance independently.
Evidence: Existing TestFlight plan records identity/profile/connectivity gaps and no real host adapter. Apple helper/App Review guidance and Node jitless docs are feasibility inputs; local signing/build success cannot prove TestFlight approval.
Tradeoffs: Preserve the all-TestFlight local goal and do not silently substitute a server/unsandboxed executor. User-selected workspace/account grants, startup consent, signing access and new agreements retain mandatory approval where required. Continue independent work when a genuine external gate blocks delivery.
Unresolved judgment: none

## Adopted architecture

One Fort application owns agent/binding revisions, conversations, commands, runs,
approvals, routines and calendar decisions. A versioned client contract contains
bounded pages, replay cursors and capability descriptors without provider SDK types.
Native clients own presentation and durable pending intent, not execution truth.
The scheduler arms the nearest due occurrence with at most one clock-reconciliation
wakeup per minute, and no timer when no work remains. No due decision means no
ledger write. Missed-work policy uses occurrence lateness and startup recovery,
not the time since an idle write. Scheduler faults remain visible through an
authenticated status response and retry at a bounded one-minute interval.

One SQLite WAL ledger owns indexed relational state plus compact receipts, events
and dispatch outbox in the same transaction. Store messages once; use immutable
context references/digests, not snapshots/history copied into each acceptance.
Offline migration writes a new private directory from a consistent SQLite backup,
compacts only validated redundant receipts/context, and preserves the original
for rollback. It refuses an existing destination or unknown/inconsistent records.
It does not start a service, retry work, refresh expired authority or infer missing
historical attempt identities. Verification uses synthetic copies only.
Snapshots expose at most 100 messages per page with an explicit older-page cursor.
Receipt responses carry the exact accepted message/run separately for recovery
outside that page. Native clients keep the latest page and at most one earlier
page in memory, with an explicit return to latest history. Admission allows at
most 100 unresolved turns per conversation; duplicate receipts do not consume
capacity, and cancellation or terminal resolution releases it. These are
unchanged-scope implementation bounds under decision 059. Events explicitly distinguish replayable deltas
from invalidation wakeups; expired cursors trigger snapshot recovery. The initial
client stream explicitly carries invalidations, at most 256 per response and the
latest 10,000 events retained. Authenticated waits last at most 25 seconds, with
at most eight concurrent waits per single-owner service. Installation reads carry
a cursor captured before native refresh reads so concurrent edits cannot be lost.
Idle responses cause no native cache rewrite. Backgrounding cancels the wait.
Workspace edits and scheduler faults also wake clients; no-due checks do not.

The service allocates attempt IDs/generations and dispatches immutable binding,
input/context/policy digests, limits, deadline and scoped grant references. Executors
cannot invent competing attempts or write the ledger. Evidence is correlated,
bounded and idempotent. Cancellation targets the exact attempt; ambiguous external
effects stay unknown and are not blindly replayed. Queues preserve per-conversation
order with conservative global/provider/connector limits and backpressure.

Optional modules own context retrieval, voice draft/playback, reviewed connector
operations and VM placement. Authority checks exact action/scope/digest/expiry
outside model reasoning. Retrieved content is data, not authority. Agent-host
privileges and computer-tool sandbox are distinct. Use static registration and
fixture implementations first; no arbitrary plugin download mechanism.

Swift owns native UI and Apple lifecycle/Keychain/XPC boundaries. The first
feasibility test uses no persistent registration or provider accounts. Actual
startup/background behavior requires explicit consent. Node/official-runtime
packaging must be self-contained and retain all license/provenance material.
Diagnostics are local/redacted; no default telemetry or auto-update service.

## V1 scope and sequence

1. Preserve the accepted decision, source review and explicit acceptance checklist.
2. Prove synthetic sandboxed bundle/runtime/IPC/cleanup feasibility; report exact
   limitations. This is not permission to bypass a sandbox failure.
3. One end-to-end native text slice: durable accept → outbox → service-owned attempt
   → bounded fixture evidence → message commit → reconnect/replay/cancel/restart.
   Run two independently shaped fixtures and optional-module-absent builds.
4. Migrate public/native provider assumptions and the existing live adapter; preserve
   identities and outcomes with copied-ledger migration/rollback tests, no dual writers.
5. Retain identity editing, exact model choice, animated bundled avatars with Reduce
   Motion, settings, local image display, reviewed skills, routines/schedules and
   no-expiry pairing/easy disconnect. Implement event-driven bounded refresh/scheduling.
6. Complete real recoverable owned-host lifecycle, secure reachable iPhone transport,
   existing authorized provider acceptance and full safe fixture/contract/removal suite.
7. Quality CI and signed delivery lane, performance targets, exact TestFlight artifacts
   and physical Mac/iPhone acceptance. Continue until the checklist is verified or
   only explicit external authority/access blockers remain; do not stop at a build.

Public seams adopted with this design: packaging CLI and produced bundle behavior;
versioned client command/replay API; executor dispatch/evidence protocol; lifecycle
client/platform adapter; native user journeys. Use one red → green slice at a time.

## Implementation paths

- `successor/runner/synthetic-process.ts`
- `successor/runner/synthetic-process.test.ts`
- `successor/evidence/2026-10-17-node-foundation/process-readiness.md`

- `successor/evidence/2026-10-17-node-foundation/replay.md`

- `successor/evidence/2026-10-17-node-foundation/scheduler.md`

- `successor/service/migrate-ledger.ts`
- `successor/service/migrate-ledger.test.ts`
- `successor/evidence/2026-10-17-node-foundation/migration.md`

- `successor/apple/FortKit/Tests/FortKitTests/WorkspaceModelTests.swift`

- `successor/apple/FortKit/Sources/FortKit/FortClient.swift`
- `successor/apple/FortKit/Sources/FortKit/PendingStore.swift`
- `successor/apple/FortKit/Tests/FortKitTests/ContractTests.swift`
- `successor/apple/FortKit/Tests/FortKitTests/PendingStoreTests.swift`
- `successor/evidence/2026-10-17-node-foundation/paging.md`

- `successor/evidence/2026-10-17-node-foundation/timeout-cleanup.md`

- `successor/apple/FortKit/Sources/FortKit/InteractiveFixture.swift`

- `successor/apple/FortKit/Sources/FortNativeCheck/main.swift`

- `successor/apple/FortKit/Sources/FortKit/MessengerView.swift`
- `successor/apple/FortKit/Sources/FortKit/SkillBundleLibrary.swift`

- `successor/runner/codex-descriptor.ts`
- `successor/service/agent-settings.test.ts`
- `successor/service/owner-pairing.test.ts`
- `successor/service/openai-shutdown.test.ts`
- `successor/prototype/mvp-session.ts`
- `successor/prototype/session.ts`
- `successor/apple/openai-http-check.ts`
- `successor/apple/FortKit/Sources/FortKit/Contract.swift`
- `successor/apple/FortKit/Sources/FortKit/AgentSettings.swift`
- `successor/apple/FortKit/Sources/FortKit/MessengerModel.swift`
- `successor/apple/FortKit/Tests/FortKitTests/AgentSettingsTests.swift`
- `successor/evidence/2026-10-17-node-foundation/provider-boundary.md`

- `successor/evidence/2026-10-17-node-foundation/compact-ledger.md`

- `successor/contracts/v1.ts`
- `successor/runner/chat-runtime.ts`
- `successor/service/openai.ts`
- `successor/service/openai.test.ts`
- `successor/runner/codex-chat.ts`
- `successor/runner/codex-chat.test.ts`
- `successor/runner/codex-startup-contract.test.ts`
- `successor/evidence/2026-10-17-node-foundation/attempts.md`

- `AGENTS.md`
- `CLAUDE.md`
- `ARCHITECTURE.md`
- `specs/058-native-first-owned-agent-mvp.md`
- `docs/architecture/decisions/059-node-foundation.md`
- `successor/AGENTS.md`
- `successor/V1-ACCEPTANCE.md`
- `successor/package.json`
- `successor/tsconfig.json`
- `successor/packaging/node-bundle.ts`
- `successor/packaging/node-bundle.test.ts`
- `successor/packaging/sandbox-probe.swift`
- `successor/packaging/runtime-fixture.mjs`
- `successor/packaging/sandbox-feasibility.ts`
- `successor/packaging/sandbox-feasibility.test.ts`
- `successor/evidence/2026-10-17-node-foundation/README.md`
- `successor/evidence/2026-10-17-node-foundation/results.json`
- `successor/evidence/2026-10-17-node-foundation/sandbox.json`
- `successor/evidence/2026-10-17-node-foundation/sandbox.md`
- `docs/architecture/reviews/20261017/ARTIFACT-MANIFEST.json`
- `docs/architecture/reviews/20261017/DECISION-GRID.tsv`
- `docs/architecture/reviews/20261017/POLICY-REVIEW.md`
- `docs/architecture/reviews/20261017/PROVENANCE.json`
- `docs/architecture/reviews/20261017/REVIEW.md`
- `docs/architecture/reviews/20261017/dispatch-lifecycle.mmd`
- `docs/architecture/reviews/20261017/gate-results.json`
- `docs/architecture/reviews/20261017/placement-variants.mmd`
- `docs/architecture/reviews/20261017/policy-snapshot/AGENTS.md`
- `docs/architecture/reviews/20261017/policy-snapshot/ARCHITECTURE.md`
- `docs/architecture/reviews/20261017/policy-snapshot/CLAUDE.md`
- `docs/architecture/reviews/20261017/policy-snapshot/docs/architecture/reassessment-20261017.md`
- `docs/architecture/reviews/20261017/policy-snapshot/ops/architecture/VALIDATION.md`
- `docs/architecture/reviews/20261017/policy-snapshot/ops/architecture/historical-specs.json`
- `docs/architecture/reviews/20261017/policy-snapshot/ops/architecture/validate-specs.mjs`
- `docs/architecture/reviews/20261017/policy-snapshot/ops/architecture/validate-specs.test.mjs`
- `docs/architecture/reviews/20261017/policy-snapshot/specs/058-native-first-owned-agent-mvp.md`
- `docs/architecture/reviews/20261017/policy-snapshot/specs/README.md`
- `docs/architecture/reviews/20261017/policy-snapshot/specs/TEMPLATE.md`
- `docs/architecture/reviews/20261017/policy-snapshot/successor/AGENTS.md`
- `docs/architecture/reviews/20261017/policy-snapshot/successor/package.json`
- `docs/architecture/reviews/20261017/source/successor/TESTFLIGHT-DELIVERY-20261017.md`
- `docs/architecture/reviews/20261017/source/successor/apple/FortKit/Package.swift`
- `docs/architecture/reviews/20261017/source/successor/apple/FortKit/Sources/FortKit/Contract.swift`
- `docs/architecture/reviews/20261017/source/successor/apple/FortKit/Sources/FortKit/FortClient.swift`
- `docs/architecture/reviews/20261017/source/successor/apple/FortKit/Sources/FortKit/MessengerModel.swift`
- `docs/architecture/reviews/20261017/source/successor/apple/FortKit/Sources/FortKit/MessengerView.swift`
- `docs/architecture/reviews/20261017/source/successor/apple/FortKit/Sources/FortKit/OwnerLifecycle.swift`
- `docs/architecture/reviews/20261017/source/successor/contracts/runtime.ts`
- `docs/architecture/reviews/20261017/source/successor/contracts/v1.ts`
- `docs/architecture/reviews/20261017/source/successor/evidence/2026-10-17-interactive-qa/README.md`
- `docs/architecture/reviews/20261017/source/successor/package.json`
- `docs/architecture/reviews/20261017/source/successor/prototype/owned-codex.ts`
- `docs/architecture/reviews/20261017/source/successor/runner/chat-runtime.ts`
- `docs/architecture/reviews/20261017/source/successor/runner/codex-chat.ts`
- `docs/architecture/reviews/20261017/source/successor/runner/codex.ts`
- `docs/architecture/reviews/20261017/source/successor/runner/owned-process.ts`
- `docs/architecture/reviews/20261017/source/successor/service/openai.ts`
- `docs/architecture/reviews/20261017/source/successor/service/service.ts`
- `docs/architecture/reviews/20261017/source/successor/tsconfig.json`
- `docs/architecture/reviews/20261017/target-architecture.mmd`
- `docs/architecture/reviews/20261017/visual-brief.md`

## Test criteria

The first slice tests the packaging CLI and produced bundle, with actual installed
compiler/signing tools and synthetic inputs only. Capture exact runtime/lib hashes,
architecture, signatures, load commands, process exit and filesystem/network denial
results. No global install, real account enrollment or startup registration occurs.
Later slice paths are recorded before edits under the same unchanged-scope owner
authority, with refreshed spec digest and staged-path coverage. The mutable
successor/V1-ACCEPTANCE.md records observed results rather than retroactively
claiming this accepted plan was verified.

## Rollback and preservation

Keep existing source, Spec 058, historical specs/decisions, donor quarantine and
consumed approval ledgers. Each implementation slice is a local commit. Test
migrations on copies and keep original ledger/export material; no production data
purge or broad checkout reset. Remove only temporary artifacts created by the exact
fixture run. Undo a failed implementation through a scoped revert after preserving
its evidence; do not reactivate legacy services or expired provider grants.
