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
Reviewed content SHA256: 426085dba328c27e902b11bb6268c31edb7659881aec732550dcda6c46dd64f3

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
Status: implemented-unverified
Affected boundaries: Node packaging, TypeScript checks, Swift builds, indexed ledger, outbox dispatch, scheduler and native refresh.
Dependency/removal impact: Retain Node by owner choice; remove repeated full-history persistence, whole-kind scans and per-second idle checkpoints. Native refresh uses events/wakeups and bounded reconciliation. Avoid required third-party server frameworks until needed.
Acceptance criteria: On recorded MacBookPro18,2/64 GiB/10 CPU, macOS 27.2, Node 26.5.0, TypeScript 6.0.3 and Xcode 27.1/Swift 6.4, target clean core check/build <=10 s, incremental <=3 s; clean native Debug build <=120 s, incremental <=20 s; core cold startup <=2 s and warm <=500 ms; local acceptance p95 <=25 ms and 100-message page p95 <=50 ms at 10,000 messages; idle core RSS <=128 MiB, CPU <=0.5% of one core and <=1 wakeup/s over 60 s; zero scheduler DB writes with no due/state-changing work over 60 s; 10,000 1-KiB messages <=64 MiB ledger. Measure at least 5 clean/warm build/start samples and 100 request samples, fixed workload, cache conditions and p50/p95; record provider/process latency and memory separately. These are engineering targets, not measured claims or promises about remote providers.
Evidence: Controlled source/core and native build measurements are recorded in successor/evidence/2026-10-17-node-foundation/performance.md with exact samples and limitations. The core-only process passes all measured budgets with unchanged service code. Earlier combined-process memory failures remain alongside the corrected diagnostic/client separation. Final bundled-host and real-provider measurements remain open. The owner's Node choice supersedes a cross-language selection experiment.
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
Admission and dispatch check the selected runtime's declared memory/skill
capabilities. A request requiring unsupported context fails explicitly before
acceptance; already accepted work that loses support fails before execution.
Historical receipts, messages and saved context remain readable after capability
removal. Library editing and export remain available independently of invocation.
Clearing an owner note explicitly allows plain text without memory content.
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
most 100 unresolved turns per conversation and 1,000 per installation; duplicate receipts do not consume
capacity, and cancellation or terminal resolution releases it. These are
unchanged-scope implementation bounds under decision 059. Events explicitly distinguish replayable deltas
from invalidation wakeups; expired cursors trigger snapshot recovery. The initial
client stream explicitly carries invalidations, at most 256 per response and the
latest 10,000 events retained. Authenticated waits last at most 25 seconds, with
at most eight concurrent waits per single-owner service. Installation reads carry
a cursor captured before native refresh reads so concurrent edits cannot be lost.
Idle responses cause no native cache rewrite. Backgrounding cancels the wait.
Workspace edits and scheduler faults also wake clients; no-due checks do not.
An advancing read acknowledgement emits a durable conversation invalidation in
the same transaction. Duplicate or older acknowledgements preserve the current
read position without a ledger write or event, preventing refresh loops between
clients. Replay and restart retain the acknowledged position.

The service allocates attempt IDs/generations and dispatches immutable binding,
input/context/policy digests, limits, deadline and scoped grant references. Executors
cannot invent competing attempts or write the ledger. Evidence is correlated,
bounded and idempotent. Cancellation targets the exact attempt; ambiguous external
effects stay unknown and are not blindly replayed. Queues preserve per-conversation
order with conservative global/provider/connector limits and backpressure.
The initial executor protocol carries one terminal result over an owned process's
pipes. Its dispatch digest binds installation/epoch, exact binding/attempt,
input/configuration/history digests, a deadline and plain-text policy with no
external grants. Limits are 1 MiB input, 64 KiB reply and 256 KiB total evidence;
the default turn deadline is 120 seconds, with a bounded shutdown reserve.
Identical terminal repeats are idempotent; foreign, stale, conflicting, malformed
or oversized evidence cannot commit a reply. Success requires correlated evidence
and observed clean process/group exit. Pipe ownership and Node fixture permissions
are not OS containment or official-provider acceptance. This slice preserves the
existing in-process official adapter for migration without activating it.
The indexed outbox contains unresolved work only, changes atomically with its Run,
and retains restart holds and uncertain-outcome barriers. Historical completed
runs are not scanned to choose the next dispatch. Legacy ledgers backfill this
index under the exclusive service lock; no second writer is introduced.

Optional modules own context retrieval, voice draft/playback, reviewed connector
operations and VM placement. Authority checks exact action/scope/digest/expiry
outside model reasoning. Retrieved content is data, not authority. Agent-host
privileges and computer-tool sandbox are distinct. Use static registration and
fixture implementations first; no arbitrary plugin download mechanism.

The first removable implementation is a versioned local context compiler for
owner notes and reviewed text skill bundles. Composition supplies it explicitly;
the core and runtime adapters import only its contract. Missing, disabled or
failed context preparation permits ordinary text and retained-history reads but
refuses work that requires context. Accepted work pins the compiler identity and
version; a changed or removed compiler fails that work before dispatch. The
compiled content and configuration digest travel in the sealed execution input.
No module grants tools, credentials or connector authority. Core artifacts must
omit the local compiler implementation and still pass the send/persist/replay
contract. Test removal/re-addition, version change and compile failure through
the public API. Historical context-bearing queued work without a compiler pin
requires a new reviewed submission; history and receipts remain readable.
Saved context-bearing routine revisions also pin that compiler in their digest.
Manual and scheduled invocations require the saved pin, so a future occurrence
cannot adopt a changed compiler silently. A historical unpinned routine remains
readable but requires a newly saved revision before invocation. Updating a routine
does not update an existing schedule's pinned revision or renew its consent.

Swift owns native UI and Apple lifecycle/Keychain/XPC boundaries. The first
feasibility test uses no persistent registration or provider accounts. Actual
startup/background behavior requires explicit consent. Node/official-runtime
packaging must be self-contained and retain all license/provenance material.
Diagnostics are local/redacted; no default telemetry or auto-update service.
Before selecting the Mac core transport, extend the synthetic sandbox proof
with a bounded Unix-socket round trip inside its private container. Keep the
same parent sandbox and child inheritance entitlements, outside-file denials,
TCP-listener denial and owned-process cleanup checks. Record observed support
or refusal without adding a network entitlement or treating same-process local
IPC as completed XPC/helper lifecycle or reachable iPhone transport.
The core listener may explicitly select loopback TCP or a private Unix socket.
Private-socket mode requires a short validated filename, the ledger directory
as the process working directory, owner-only directory/socket permissions and
an absent socket path. It preserves the same HTTP authentication, Origin checks,
durable acceptance and replay. Startup refuses occupied paths and never removes
an unknown socket/file. A typed endpoint distinguishes the transports; private
mode provides no TCP URL. Test an actual core/client process exchange, denial
without authorization, restart/history, socket cleanup and invalid startup
scope. This prepares the host boundary without activating a background job or
claiming XPC, provider or iPhone acceptance.
Combine the compiled core artifact with the existing signed Swift/Node sandbox
probe as a separate synthetic scenario. Exercise authenticated private-socket
accept/reply/replay, duplicate recovery after restart and cleanup, retaining the
outside-file/TCP denials and unchanged entitlements. Its in-process provider and
HTTP client remain fixtures. This validates the actual core in the bundle but
does not establish Swift/XPC peer identity, official executor confinement or
persistent lifecycle.

Native removal prepares an explicit handoff to supported macOS/TestFlight app
removal. The lifecycle contract must confirm host exit, pairing revocation and
startup unregistration before credential cleanup. Keep history unless the user
separately chooses deletion; a failed cleanup must not produce a ready result.
Return the exact app URL and retained/deleted history choice, without deleting
the bundle or claiming it was removed. Migrate every active caller from the old
self-deletion interface. Test each missing confirmation, failed credential or
history cleanup, retry and honest native presentation through public lifecycle
and model actions. The real host/registration adapter and installed acceptance
remain separate gates. This refines the adopted removal scope: no AI dependency
changes, no new module/runtime dependency, constant work apart from explicitly
chosen history deletion, and no service activation or app deletion in tests.

Prepare the Swift control channel with a temporary embedded XPC proof in the
existing sandbox bundle. Pin both peers to exact generated bundle identifiers
and the selected signing team before resuming connections. Check an exact bounded
round trip and explicit rejection for mismatched client and service requirements;
timeouts do not count as rejection. Embedded helpers have bounded lifetimes and
must be proven absent before synthetic-container cleanup. Keep sandbox-only
entitlements, compiled-core denial controls and startup-registration prohibition.
This tests the Apple-specific control boundary without a provider dependency or
new production package; it does not implement the core bridge, login lifecycle
or executor confinement. Record the actual SDK/runtime and every cleanup result.

## Performance verification

The local verification CLI creates a new private output directory and refuses
overwrite. A smoke mode verifies its report/control flow but cannot satisfy
performance budgets. The core measurement mode uses five fresh compile/package
and process-start samples, five incremental edits in a copied source tree, at
least 10,000 synthetic 1-KiB messages, 100 acceptance/page samples and 60 seconds
of idle observation with a future schedule enabled. Node runs compiled code with
JIT disabled. Record exact source/artifact/toolchain, cache conditions and process
topology; no system cache purge or provider/account access is required. The
controller owns the synthetic HTTP client; the measured worker owns only the
compiled core and immediate provider fixture. Report client/HTTP round trips
and fixture dispatch/IPC time separately. The parent may inspect
only its owned worker's OS resource counters; OS interrupt and package-idle
wakeups are separate metrics, not a claim about total energy use. The parent
hashes ledger and WAL files outside the idle CPU/counter sampling interval, so
those diagnostic allocations do not enter worker memory. Native app builds use
five fresh Debug DerivedData directories and five copied-source incremental edits
per Mac/iPhone Simulator target, with signing disabled and no install or launch.
Preserve the exact measurement procedure, source hashes and last clean apps.
Native app builds and final bundled-host measurements remain separate from this
core baseline. Verification adds no product runtime dependency, optional-module
import, provider authority or startup registration.

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
7. Quality CI discovers all synthetic tests, with explicit portable, macOS and
   existing-identity signed suites; ordinary jobs need no provider or release
   credentials. The workflow has read-only repository access and runs dependency
   installation with hooks disabled. Local preparation does not prove a remote
   CI run or authorize pushing. Signed delivery lane, performance targets, exact TestFlight artifacts
   and physical Mac/iPhone acceptance. Continue until the checklist is verified or
   only explicit external authority/access blockers remain; do not stop at a build.

Public seams adopted with this design: packaging CLI and produced bundle behavior;
versioned client command/replay API; executor dispatch/evidence protocol; lifecycle
client/platform adapter; native user journeys. Use one red → green slice at a time.

## Implementation paths

- `successor/packaging/xpc-protocol.swift`
- `successor/packaging/xpc-service.swift`
- `successor/packaging/xpc-client.swift`
- `successor/evidence/2026-10-17-node-foundation/xpc.md`
- `successor/evidence/2026-10-17-node-foundation/xpc.json`

- `successor/apple/FortKit/Sources/FortKit/OwnerLifecycle.swift`
- `successor/apple/FortKit/Tests/FortKitTests/OwnerLifecycleTests.swift`
- `successor/apple/FortKit/Sources/FortKit/PairingSettings.swift`
- `successor/evidence/2026-10-17-node-foundation/removal-handoff.md`

- `successor/verification/performance.mjs`
- `successor/verification/performance-worker.mjs`
- `successor/verification/performance-client.mjs`
- `successor/verification/process-metrics.c`
- `successor/verification/performance.test.mjs`
- `successor/evidence/2026-10-17-node-foundation/performance.md`
- `successor/evidence/2026-10-17-node-foundation/performance.json`
- `successor/evidence/2026-10-17-node-foundation/native-measurement.py`
- `successor/evidence/2026-10-17-node-foundation/local-ipc.md`
- `successor/evidence/2026-10-17-node-foundation/local-ipc.json`
- `successor/service/socket-transport.test.ts`
- `successor/service/fixtures/socket-host.ts`
- `successor/evidence/2026-10-17-node-foundation/private-transport.md`
- `successor/packaging/core-fixture.mjs`
- `successor/evidence/2026-10-17-node-foundation/sandbox-core.md`
- `successor/evidence/2026-10-17-node-foundation/sandbox-core.json`

- `successor/contracts/context.ts`
- `successor/modules/local-context.ts`
- `successor/service/context-modules.test.ts`
- `successor/runner/fixtures/context-input.ts`
- `successor/runner/skill-context.ts`
- `successor/prototype/mvp-admission.test.ts`
- `successor/evidence/2026-10-17-node-foundation/context-modules.md`
- `successor/evidence/2026-10-17-node-foundation/routine-context-pins.md`

- `successor/contracts/executor.ts`
- `successor/runner/executor-process.ts`
- `successor/runner/executor-process.test.ts`
- `successor/runner/fixtures/executor-worker.mjs`
- `successor/evidence/2026-10-17-node-foundation/executor-process.md`
- `successor/evidence/2026-10-17-node-foundation/capabilities.md`

- `.github/workflows/fort-quality.yml`
- `successor/verification/test-suite.mjs`
- `successor/verification/test-suite.test.mjs`
- `successor/verification/README.md`
- `successor/runner/process.ts`
- `successor/service/service.test.ts`
- `successor/service/synthetic-runner.test.ts`
- `successor/evidence/2026-10-17-node-foundation/quality-ci.md`

- `successor/contracts/attempt.ts`
- `successor/contracts/runtime.ts`
- `successor/contracts/runner.ts`
- `successor/runner/codex.ts`
- `successor/packaging/core-artifact.ts`
- `successor/packaging/core-artifact.test.ts`
- `successor/packaging/fixtures/provider-contract.mjs`
- `successor/tsconfig.core.json`
- `successor/evidence/2026-10-17-node-foundation/core-artifact.md`

- `successor/evidence/2026-10-17-node-foundation/outbox.md`

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
