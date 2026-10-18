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
Reviewed content SHA256: 5308615e66fcc7927d5fbb1f925e1b25f8feafee77773c93286f8fdbf88754e2

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

Separate native client behavior from its network carrier. A small Fort-owned
request/response contract carries a relative versioned path, method, installation
identity, optional device token, body and deadline; the response carries status
and body. The existing HTTPS/explicit fixture-loopback adapter keeps endpoint
validation and redirect refusal. A local transport can be supplied without a
network address, while the same client performs installation/replay validation
and service-error decoding. Migrate the interactive fixture and remove the old
optional HTTP-only seam. Test exact text and identity, encoded replay queries,
timeouts/cancellation, mismatched installations and preserved HTTP behavior.
No local XPC implementation or device-storage migration is implied by this seam.
This adds no runtime dependency, provider authority or persistent registration;
it keeps native contracts reusable with constant transport-selection overhead.

Implement the Mac transport using an embedded NSXPC service with exact validated
service identifier and signing team. Each relative request has one connection,
the existing deadline and cancellation; accept at most 2 MiB encoded request,
12 MiB encoded response and 8 MiB decoded response body. Reject malformed peer
responses and invalid status codes. Cancellation/timeout invalidates the owned
connection and settles once; it does not retry or select another carrier. Compile
the actual transport into the signed fixture and verify exact request/response,
both signature failures, timeout and cancellation with the same entitlements and
process-absence checks. The echo service remains a fixture; core forwarding,
owner setup and persistent host activation are separate. This is an Apple-only
adapter to the shared contract, with no AI SDK or new dependency and no polling.

Add a portable private-pipe relay for an already started core socket. It forwards
the shared native relative request to that exact private socket, preserving
installation identity, device token, body bytes, HTTP status and response body.
The caller owns application/runtime lifecycle and enrollment; the relay grants
no authority and creates no credential. JSON-line protocol version 1 uses strictly
increasing request IDs, correlated replies and explicit cancellation. Bound input
to 2 MiB, responses to 8 MiB decoded body, deadlines to 30 seconds and concurrent
requests to eight. Invalid framing, stale IDs or overload fail the channel closed;
EOF/abort cancels pending HTTP requests without retry or fallback. Writable
backpressure retains a bounded set of active calls, with a five-second output
drain bound that fails the channel rather than retaining buffers indefinitely.
Test a real owned fixture
process against the actual core: unauthorized/foreign requests, exact durable
text, replay/restart/idempotency, canceled waits, malformed/oversized frames and
EOF cleanup. Build and exercise the relay from the compiled vendor-free artifact.
This is a small hosting adapter with no production dependency, polling, TCP
listener or provider activation; Swift supervision/XPC forwarding remains next.

Add the Mac process transport behind the same native request contract. Explicit
composition supplies one exact executable, arguments, private directory and
environment; no shell, inherited environment, enrollment or provider selection.
Own a distinct process group and nonblocking stdin/stdout pipes. Accept readiness
within five seconds, at most eight outstanding calls, 2 MiB input and 12 MiB
output frames, 8 MiB response bodies and 30-second request deadlines. Validate
correlation/version/status and settle each continuation once. Cancellation sends
a correlated cancel and retains capacity until acknowledgment; failure to drain
an input write or acknowledge cancellation within five seconds closes the host.
Closing first sends EOF, then escalates to TERM after two seconds and KILL after
four, with a six-second total cleanup bound. Report clean close separately from
forced shutdown and require the child reaped and process group absent. These
are owned-process lifecycle checks, not containment of a hostile process that
escapes its group. Test exact source/compiled core requests, auth, replay/restart,
cancellation, malformed/oversized/stale output, absent readiness, blocked input
and hung-child cleanup. Keep real XPC composition, persistent startup and actual
providers as separate acceptance gates. This adds no package dependency or idle
polling; process exit and pipe readiness drive normal operation. Lifecycle timers
run only during launch, requests and shutdown. Existing setup authority is not
expanded; the synthetic fixture alone supplies enrollment outside the transport.

Connect the native XPC server adapter to a supplied Fort transport. Pin the exact
client identifier/team and effective user before accepting a connection. Each
connection admits one bounded request; invalidation cancels its task. Decode and
validate the same request/response limits without changing domain authority.
Prove the complete signed app → XPC service → owned Node pipes → private core
socket journey with synthetic bootstrap/restart/stop composition confined to the
packaging fixture. Preserve existing signature mismatch and sandbox denial tests;
record child/helper PIDs and independently prove their groups absent. Require
exact durable messages, unauthorized/foreign rejection, restart/idempotency,
transport cancellation and clean shutdown under unchanged entitlements. This
small Apple hosting adapter adds no dependency, credential, registration or
network listener. Real installation/bootstrap remains a separate authority gate;
fixture control routes must never enter the core or ordinary app composition.
The signed proof found execution of Node from the outer app bundle denied to the
XPC service. Package the exact runtime, dependency closure, licenses and compiled
core inside the service's own bundle; retain the outer-executable denial as a
negative control. Do not grant access to the outer bundle or expand entitlements.
The feasibility app retains its separate outer runtime for existing controls;
that duplication is test packaging, not the intended installed product layout.

Native saved connections must select an explicit network or exact Mac XPC
endpoint. Version the record, decode existing URL records without rewriting them,
and retain installation/device identifiers and the existing Keychain account.
Reject unknown/mixed formats, insecure network endpoints and malformed local
peer identities before credential access; never fall back to another carrier.
Restore/recovery choose the recorded endpoint, while disconnected local history
remains readable without a transport or token. A Mac endpoint on iPhone fails
explicitly. Existing HTTPS/fixture-loopback enrollment remains separate from
future owned setup; this migration does not enroll, activate startup or access
real credentials. Test old/new records, malformed input, read-only migration,
local history and unchanged network behavior using temporary storage and fake
credential providers. This removes URL coupling without adding a vendor or
dependency; work is constant per small record and preserves removal identity.

Owner stop/disconnect intent must survive core restart. Record it before waiting
for runtime shutdown; startup leaves dispatch/scheduling paused unless the native
owner explicitly requests a start. A pending disconnect cannot be resumed while
its device remains authorized. Record the latest exact owner device at enrollment
so trusted local recovery can verify an expired or already-revoked pairing without
inventing proof for an unknown device. Expose this recovery only as a composition
API for the pinned local host, never as an unauthenticated HTTP route. It checks
installation/device identity before stopping anything, waits for runtime disposal,
then revokes that device and enrollment codes while retaining history. Failed
disposal retains pairing and permits an explicit retry; another device/installation,
temporary demo or superseded pairing cannot be adopted. Explicit start may clear
a completed stop/revocation but cannot renew a credential. Test pause after restart,
expired/lost-reply recovery, identity mismatches, failure/retry and preserved history
through public core APIs using synthetic runtimes. This adds bounded local metadata,
no provider dependency, no new external authority and no polling. Host process
absence/startup unregistration remain the Swift owner's separate proof obligations.

Carry exact pairing recovery over a separate private-pipe frame, never through a
Fort HTTP request or bearer-token exception. The relay rejects this frame unless
its local composition explicitly supplies the owner recovery API. Keep the same
monotonic IDs, eight-call bound, 30-second deadline, cancellation and output limits.
Validate bounded installation/device identifiers before any callback. Swift checks
the returned exact identity and both confirmations, then separately closes/reaps
the core process and verifies its group absent. A timeout or canceled call cannot
be reported as confirmed recovery even if disposal completes later; retry uses the
durable intent. Test the real Swift/Node pipe using synthetic expired credentials,
retained history and foreign identity, plus disabled/malformed controls. This is
constant-size local lifecycle work with no extra dependency or idle activity. It
does not expose enrollment, change signed XPC admission, activate startup or add
network access. Ordinary app setup and the installed owner adapter remain separate.

The signed feasibility harness must distinguish first process entry from XPC
checks and Node execution. Emit an exact parent PID/group marker on entry, retain
the 15-second initial launch bound, then start a separate bounded parent-work
phase and the existing Node-child execution deadline. Record startup, parent work
and child execution durations separately; none establishes the product's measured
startup budgets. This corrects a reproduced timeout where macOS signing evaluation
occupied almost all of launch's deadline before XPC checks began. Preserve the
failed artifact and original result, independently prove owned processes absent,
and remove only that fixture's generated containers. Keep the hung-child timeout
negative control and all signed identity/sandbox checks. This test-only repair
adds no runtime dependency, product authority, idle work or installation change.

Add a distinct owner-recovery method to the pinned Mac XPC protocol. A host must
explicitly supply a Fort-owned owner control; ordinary transport construction
leaves it unavailable. Validate a small exact installation/device request and a
bounded deadline on both peers, admit one operation per connection, propagate
cancellation, and require exact returned identity plus runtime disposal,
revocation and core-process exit before accepting its receipt. The receipt does
not establish helper-process absence or startup unregistration. Keep this method
separate from Fort HTTP routing and credentials. Exercise the real signed
Swift/XPC/Node path with synthetic owner pairing, foreign identity and disabled
control checks, both signature mismatch directions, retained history/restart and
independent process/container absence. Unit-test bounds, missing controls and
unconfirmed/foreign replies. This extends the existing local lifecycle boundary,
adds no dependency/network capability or idle polling, and grants no real pairing,
startup activation or provider authority. A real owner installation adapter and
app setup still must compose these proofs before removal can be offered.

Protect the owned core group when its Swift supervisor dies abruptly. Require an
explicit lifecycle selection at launch: test-only cooperative supervision or an
exact bundled native guardian. The guardian verifies its expected parent and
own process-group leadership, synchronously registers an OS parent-exit event
before launching the exact child, and fails closed if monitoring cannot start.
Node inherits the guardian's group and only the selected environment and standard
pipes. Parent exit kills that owned group even when Node ignores EOF and TERM;
normal close retains the existing bounded reaping and group-absence checks.
Do not infer cleanup of detached groups from launchd or XPC restart behavior.

This unchanged-scope lifecycle refinement keeps AI/runtime independence and
adds only an Apple process adapter, with no package, provider or domain imports.
One bundled Swift executable can be removed with the app; no registration,
credential, network listener or entitlement expansion is added. The speed cost
is one blocking OS-event thread and a small process, with no idle polling.
Targets are at most 25 MiB added idle RSS, 0.1% added idle CPU and 100 ms p95
added warm launch overhead on the recorded Mac over ten equivalent launches;
these must be measured separately from core-only budgets. For installation and
removal, test killing the actual Swift supervisor while a child/descendant ignores
EOF/TERM, require exact PID/group absence within three seconds, and retain normal
close, invalid-parent admission and signed XPC/sandbox controls. Package and sign
the guardian inside the core service's own bundle. This is lifecycle ownership,
not containment of hostile descendants escaping their group, and does not yet
prove executor cleanup when Node itself dies or ordinary installed-host behavior.

Extend the same explicit lifecycle choice to the optional process executor:
portable cooperative group supervision or an exact Mac parent-exit guardian.
Reject a malformed guardian path or unsupported platform before launch and copy
the selected configuration so callers cannot change it later. The guardian owns
the executor group and watches the exact Node parent; retain the existing
dispatch/evidence protocol, deadlines, cancellation and clean-exit requirement.
Prove abrupt Node-parent death removes a noncooperative worker and descendant
within three seconds through a real core dispatch. Check guarded exact replies,
correlated cancellation, bad evidence and unchanged fixture permission denials.
The existing native guard is reused, with the same measured per-active-process
cost and no idle executor process. Vendor/core contracts remain unchanged and the
implementation stays removable; no new package or entitlement is introduced.
This improves owned-runtime removal without enabling a provider, persistent host
or computer-tool authority. Official runtime adapters and installed containment
remain separate migration/acceptance work.

Recover an established private socket after abrupt core death using durable exact
ownership. Store its directory device/inode/owner and socket device/inode/owner/
change-time identity in the same exclusively locked ledger after binding, before
publishing readiness. On restart, only the same selected name with an exact
record may be reclaimed; require a socket rather than a symlink/file and a bounded
250 ms connection probe that returns connection-refused. A live, timed-out or
otherwise uncertain endpoint fails closed. Recheck identity and cancellation
before unlinking. Preserve unknown/replaced sockets and malformed ownership
records, including a crash before the ownership record was committed; do not
invent authority for those paths. Retain the record until cleanup is confirmed.

This internal transport adapter adds no vendor/package dependency and keeps domain
contracts unchanged. Its metadata lives with the owner ledger and is removable
with that installation; no external credential or startup grant is introduced.
The only added normal work is a small startup/shutdown metadata write, with no
idle polling or writes; stale-socket probing has the stated 250 ms bound.
Test SIGKILL/restart with authenticated durable history and exact receipt replay,
active-owner refusal, replacement/symlink/regular-file preservation and canceled
recovery. This enables recovery of recorded resources, not unrestricted directory
cleanup or proof of installed-host recovery at every startup instruction.

Verify the combined lifecycle in the signed packaging fixture: keep its ledger in
one private directory inside the fixture's unique service container, kill the XPC
helper with SIGKILL after a completed turn, require exact helper/guardian/Node
group absence within three seconds, then relaunch through the pinned XPC client.
The new helper must recover the recorded socket and same installation, accept
the existing synthetic pairing, and return the original receipt/history without
executing the turn again. Include all three core generations and both helper PIDs
in independent cleanup checks before removing generated containers.

This is fixture-only composition under unchanged entitlements and the existing
development identity. A reproduced launchd throttle deferred a crashed helper's
next spawn by ten seconds. Give only the recovery bootstrap a twelve-second
request deadline within the existing fifteen-second parent-work bound, and record
its wait separately from the unchanged three-second process-absence deadline.
Recovery must use the same short relative socket address as binding and forwarding
inside the validated owner working directory. Test an actual long data-directory
path through a short test-only client alias; filesystem identity checks remain
absolute and exact. Do not make the signed container's pathname an AF_UNIX address.
Do not disable the OS throttle or present its wait as core processing overhead.
This adds no product dependency, vendor coupling, network
capability, idle work, startup registration or new external authority. The prior
bounded startup/execution phases and guardian budgets remain in force. Preserve
identity mismatch, outside-file/TCP/outer-executable denials and owner-recovery
controls. This signed synthetic result is separate from ordinary installed-app,
official provider and TestFlight acceptance.

Prepare resumable owner setup at the trusted local composition boundary. A
five-minute setup offer records an opaque setup ID and exact installation before
pairing; repeated offers before expiry reuse it. An existing active pairing is
reported without issuing another offer. The client prepares its exact device ID
and cryptographically random token in local storage before submission. Completion
binds installation/setup/device/token and stores only the token hash. Its receipt
contains identity and expiry, never the token. Repeating the same completed
request confirms the existing pairing without extending expiry or creating a
device; different identity/token, expired, revoked or superseded requests fail.
Do not resume a stopped host, convert bounded sessions or replace an active owner.
Preserve legacy enrollment behavior while native setup migrates to this seam.

This first core slice adds Fort-owned types and local methods only. Do not expose
setup over HTTP or claim native storage/XPC/ordinary-app integration until those
are implemented and tested. AI/runtime/hosting dependencies stay unchanged;
durable setup state belongs to the same exclusive ledger and adds no package,
idle polling or startup service. Work is bounded to a few indexed metadata/device
operations and one setup record, with no provider calls. Test lost-reply retry,
restart, conflicting requests, expiry/revocation/supersession, paused-host refusal,
existing-owner reporting, bounded-session exclusion and authenticated API access.
Tests use synthetic credentials; ordinary owner activation remains an explicit
user action, with its persistent access scope and real-provider grants separate.

Carry owner setup over the existing private parent pipe through an explicit,
default-disabled composition callback. Add begin/complete operations, bounded
identifiers, canonical token validation, deadlines and cancellation under the same
eight-request/frame/input limits. Return only validated offer/receipt fields;
completion must match the submitted installation/device. A reply lost after a
commit remains uncertain until the exact prepared request is retried. Tests cover
source and compiled core, disabled/malformed paths, conflicting callback output,
timeouts/cancellation and authenticated access after retry. No HTTP exception,
provider dependency, startup activation or native integration is added by this
pipe slice. It reuses the same process/storage lifecycle and budgets; small local
control frames add no polling or background process. Swift/XPC and durable native
preparation remain separately required before ordinary setup can use it.

Add language-neutral owner setup values to FortKit and implement its setup
control protocol on the existing Mac core-process transport. Validate decoded
identities, canonical credential encoding, expiry and operation deadlines. The
completion receipt must match the exact submitted installation/device. Tests use
the compiled core and synthetic prepared tokens to prove offer/completion/retry,
restart, timeout/cancel uncertainty, authentication and owned process/socket
cleanup; malformed/foreign replies must fail. No Keychain write, credential
generation in ordinary setup, app/XPC integration or startup activation belongs
to this slice. Types remain usable by other native transports without importing
provider code. Reuse the same bounded channel/process lifetime, with no new
dependency, polling or idle work; installed setup performance remains unmeasured.
Include the new value types in standalone guardian and signed-probe compiler
source lists. Rerun their existing process/sandbox regression checks without
changing entitlements or claiming signed setup/XPC integration.

Extend the existing exact-peer XPC interface with a separate owner-setup selector.
The listener must explicitly supply `OwnerSetupControl`; omission stays disabled.
Reuse the same effective-user/signing-identity checks, one-operation connection,
invalidation/cancellation and at-most-30-second deadline. Limit setup request and
response payloads to 1 KiB, validate offer/receipt fields and completion identity,
and sanitize callback failures without returning tokens or internal errors. Share
the setup command codec with the parent pipe to keep native transports consistent.
Unit tests cover disabled/malformed/oversized inputs, mismatched receipts, errors
and cancellation. The signed synthetic journey prepares one token, confirms setup
across core restart and helper crash, retains history and verifies both directions
of peer mismatch, disabled setup, no HTTP exception and complete process/container
absence. Keep signed probes serial under their existing deadlines/cleanup budgets.
Update their exact compiler sources; no entitlement or signing identity change.
This adds no provider/storage dependency, polling or startup activation and keeps
the same local single-owner topology. No ordinary setup, Keychain preparation or
persistent host registration is authorized by the fixture composition; installed
setup performance and the real client lifecycle remain separate gates.

Prepare the Mac client's pending owner setup durably before submitting completion.
Bind its exact signed host endpoint, installation/setup/device IDs and offer expiry
in an owner-only record. A `preparing` phase precedes credential creation; save the
token through the existing device Keychain namespace, then persist `ready` with
its SHA256 digest before any completion request. Compare that digest before every
retry so an altered credential is rejected without submission. A lost reply,
restart, cancellation or final connection-file
failure retries the same prepared identity/token. Never replace a missing token in
the ready phase, replace an existing foreign connection, switch host or silently
discard a failed setup. If a credential write succeeded before reporting an error,
the preparing retry reads and reuses it. Store no token in metadata or evidence.
Confirm the exact receipt and retained token before durably saving the connection;
only then remove the pending record. Keep prior history/removal records untouched.

Use a nonblocking exclusive file lease across the local setup operation so overlapping
windows/processes cannot overwrite preparation. Anchor bounded record access to an
owner-only directory descriptor, reject links/nonregular files and preserve unknown
or malformed records. Atomically replace private records, flush file data with the
platform full-sync primitive and sync directory updates before proceeding. This is
the native client's setup store, separate from the core ledger; it adds no runtime,
AI, hosting or storage package dependency. The Apple Keychain dependency remains
explicit and separately requires installed acceptance. Expose an explicit coordinator
without invoking it from restore or the ordinary app in this slice. Tests inject
synthetic credential storage, never the real Keychain, and exercise lost replies,
credential/write failures, restart, host/receipt mismatch, concurrent setup, unsafe
files and interrupted finalization. Measure 20 local synthetic preparation/finalize
runs with p95 at most 500 ms, excluding XPC, actual Keychain prompts and providers;
no idle work is added. Installed setup timing and hardware power-loss durability
remain separate unverified gates. Ordinary persistent activation still needs its
explicit owner action and applicable credential/access authority.

Add trusted local cancellation for an uncertain owner setup before exposing native
discard/recovery. Bind installation/setup/device IDs without requiring the possibly
missing token. A pending offer is durably canceled before confirming that it has
no pairing, so a delayed completion cannot create access after local cleanup. For
an exact completed setup, persist canceling intent and the existing owner disconnect
intent atomically before disposal, then revoke only after runtime shutdown is
confirmed. Failure/restart retains stop intent and pairing for retry. Cancellation
is idempotent; it never renews, resumes or substitutes a device, touches a different
owner, converts bounded sessions, or deletes history. An obsolete setup may be
confirmed not paired only when its requested device is absent and that offer cannot
complete. Distinguish `notPaired` from `revoked` with confirmed runtime stop; neither
receipt proves helper absence or startup unregistration. Preserve canceled state
until an explicit new offer supersedes it. Test both completion/cancellation orders,
lost replies, restart, failed disposal, expiry, conflicting/superseded identities,
legacy enrollment and bounded-session refusal. This slice adds core types/local
methods only; pipe/XPC/native discard remain separate. It reuses the existing ledger
and stop boundary with a few indexed operations, no package/provider dependency,
idle work or activation, and preserves existing disposal/transport deadline gates.

Carry owner setup cancellation through the existing private pipe as the explicit
`ownerSetup` cancel operation. Its optional composition callback stays disabled
when absent. Accept only the installation/setup/device identity and a deadline;
reject credential fields, malformed operations and out-of-bound deadlines before
calling core. Validate exact identity and `notPaired` versus confirmed `revoked`
receipts, serializing known fields only. A timeout or transport cancellation does
not prove rollback: retain uncertainty until an exact cancellation retry confirms
the durable result. Use the existing eight-operation admission limit, monotonic
IDs, bounded frames, five-second output deadline and at-most-thirty-second request
deadline. Test source and compiled journeys, both cancellation orders, restart,
foreign/unconfirmed callback output, disabled composition, secret-free failures,
lost replies and concurrent requests. This adds no dependency, provider, process,
timer while idle, authority or HTTP route; the local core and computer-tool sandbox
boundaries remain unchanged. Native XPC/discard integration remains a later slice.

Expose setup cancellation as a separate optional Swift control capability, with
validated installation/setup/device identity and a discriminated receipt. A revoked
receipt decodes only with confirmed runtime stop; not-paired does not imply shutdown.
The owned core transport and pinned XPC transport implement this operation with
exact identity correlation and unchanged deadlines/frame limits. The host channel
accepts it only when the supplied setup control provides the cancellation capability,
rejects unknown/token fields before invocation, and sanitizes errors. Keep begin/
complete-only compositions usable without enabling cancellation. Exercise compiled
core restart, pending and completed cancellation, lost replies, invalid/foreign
proofs, malformed identities/deadlines, disabled host composition, single-operation
admission and cancellation on connection invalidation. Existing parent-exit and
signature gates remain required. This slice changes native contracts/bridges and
synthetic fixtures, not ordinary app setup or client-record deletion. It adds no
runtime/provider/storage dependency or startup activation; its data contract remains
usable by alternate clients. Existing thirty-second request, five-second drain and
six-second core-stop limits apply; installed latency and signed cancellation proof
remain separately reported acceptance gates.

Opt the synthetic signed XPC wrapper into setup cancellation and verify the complete
Swift client → pinned XPC → owned core pipe → ledger operation. Prove pending
cancellation blocks completion, exact completed cancellation revokes access, a fresh
core confirms that outcome without resuming, and the existing receipt survives.
Check all foreign identity fields, HTTP exclusion, absent composition and wrong
client/service pins. Preserve existing message/replay/crash recovery checks, add
the extra exact core process generation to cleanup evidence, and independently
verify every recorded PID/group, temporary container, signature, entitlement set
and compiled-core hash. The fixture adds no provider/account, startup registration,
ordinary credential or new entitlement. It retains the same Node/Swift modular
boundary and existing separate cleanup/relaunch timing budgets. Report this as
synthetic signed evidence, separate from ordinary client recovery and installed
product acceptance.

Add explicit native pending-setup inspection and cancellation recovery before app
composition. Inspection acquires the existing private file lease (creating its
empty lock file when absent) and reads validated metadata, with no credential
read, host request or activation. Cancellation accepts the exact displayed
installation/setup/device identity and pinned endpoint; stale identities, foreign
connections and absent cancellation capability preserve local state. Persist
canceling intent under the existing exclusive lease before invoking the host, and
block connect/re-enrollment while canceling or confirmed cancellation remains.
Persist an exact validated cancellation receipt before local credential cleanup.
Lost replies, task cancellation, host failures, storage/credential failures and
restart retain a recoverable phase. A saved confirmed receipt can finish local
cleanup without another remote claim or automatic host activation.

Keep ready-phase credential hashes and refuse deletion of an altered credential.
For interrupted preparation, validate any retained credential and bind its digest
before cleanup; missing credentials are allowed, read errors remain failures.
Delete only the exact prepared credential, confirming absence; preserve unexpected
changes. If final connection persistence had already succeeded, record its confirmed
removal and retained-history reference before removing that exact connection. Keep
history snapshots and unrelated records. Remove pending metadata last; repeating an
already-finished operation reports no pending setup, not a new revocation. Use the
same anchored, bounded private file operations and lock. Malformed/unsafe records
remain untouched. Test public inspection/cancel seams, missing/changed credentials,
partial writes/removals, stale identities/hosts/proofs, overlapping operations,
interrupted finalization, restart and the compiled core. Explicitly unlock the
lease before closing its descriptor so a duplicate retained during process
creation cannot extend the completed operation's lock; verify immediate reacquisition
while that duplicate remains open. No new provider/storage
dependency, idle work, real Keychain access, startup activation, app removal or
ordinary UI wiring is introduced. Existing request/cleanup deadlines and the
500 ms synthetic file-durability budget remain; installed Keychain timing and
hardware power-loss behavior remain separate gates.

Expose prepared owner setup in the native model and connection/settings views,
only when app composition explicitly supplies the exact pinned Mac host. Launch,
restore and sheet inspection read pending metadata without beginning/completing
setup, reading its credential or contacting the host. Pending or unreadable setup
blocks connection restoration, including a partially persisted connection; it
cannot silently resume delivery or be overwritten by legacy fixture enrollment.
An explicit connect/retry action uses the same coordinator. Explicit cancellation
captures the displayed setup identity, confirms that scope and keeps history.
Lost replies and failed cleanup remain visible, with retry actions selected from
the persisted phase. A stale confirmation cannot cancel a replacement setup.
Serialize model actions; repeated clicks, restore and fixture enrollment cannot
race an active setup/cancellation. After successful setup restore the ordinary
Fort client through its transport boundary; after cancellation restore only any
retained disconnected history. Unconfigured apps, iPhone, offline previews and
interactive fixtures must not infer a local host or gain setup authority.

This refinement remains under decision 059. Vendor independence: no provider or
paid path is selected; exact host identity is supplied by composition. Modularity:
keep credential/durable-phase rules in the existing coordinator and client transport
behind FortTransport; the UI projects state and invokes explicit public actions.
Speed: inspection/action projection adds no timers or background work; retain the
500 ms synthetic file-operation target and existing request deadlines. Verify 20
isolated model inspections at p95 <=50 ms, excluding host/credential operations.
Installation/removal: expose uncertainty and retained history without registering
startup, granting persistent access or claiming whole-app removal. Test public
model actions with in-memory credentials/control and transport, ordinary restore,
stale confirmations, missing capability/unsafe state, overlapping actions and
restart; build both platforms. Existing app construction remains unconfigured
until its actual host bundle/lifecycle is supplied. UI screenshots/accessibility,
actual Keychain/XPC and installed setup acceptance remain separate gates.

Prepare a distinct persistent-agent address before composing the production host.
The existing `macHost` endpoint denotes an embedded XPC service; do not reinterpret
it as a launch agent. Add a validated Mac agent endpoint containing the exact
service code identifier, signing team and explicit iOS-style App Group identifier.
Derive its Mach name as `<appGroup>.fort.host`, bounded to fewer than 128 ASCII bytes
including room for the bootstrap name's terminator. The group name is a required
configuration value, never a discovered or invented grant. Peer authentication pins
the code identifier/team independently of the Mach lookup name. Use the unprivileged
named-Mach-service NSXPC connection API, preserving existing deadlines, frame limits,
cancellation and exact receipts. No embedded-service, network or provider fallback.

Persist only this new endpoint using connection schema 3. Preserve legacy and schema
2 behavior without automatic rewrite, and reject schema 3 for other endpoint kinds,
schema 2 carrying an agent address, mixed/unknown formats and invalid group/name/pin
values. On iPhone, decode the record for retained history but refuse local transport
before any credential or host action. The native setup coordinator and explicit model
construction accept either validated local endpoint; normal app construction stays
unconfigured. Test round-trip storage, invalid boundaries, legacy preservation,
local recovery/model behavior and signed embedded-XPC regressions. Actual Mach-agent
communication remains unverified until exact App Group/profile and registration
authority exist. Neither a value constructor nor transport construction registers,
starts, signs or grants access to a service.

Vendor independence: this is an explicit Apple IPC dependency outside the portable
core and provider contracts. Modularity: keep Mach lookup, peer code identity and
persisted endpoint kind distinct; optional runtime implementations remain omitted.
Speed: validation and lookup selection add no timers, processes or idle work; retain
existing XPC request/cleanup and native inspection budgets. Installation/removal:
the eventual per-user SMAppService LaunchAgent needs an app-like signed wrapper and
authorized shared App Group, with explicit startup consent and asynchronous removal
confirmation. Current local Fort-specific profiles cover historical iOS/watch IDs
only; they do not establish Mac/agent/App Group authority. Keep the provisioning
inventory and Apple references with the boundary evidence. This slice does not create
profiles, expand signed entitlements, register/start a job or change the accepted
single-owner per-user host into a root daemon. Production identity selection, owned
host composition, update/rollback and installed acceptance remain separate work.

Add a Mac startup-registration boundary around SMAppService's per-user agent API.
It takes the validated agent endpoint and derives only its exact code-identifier
plist name under the containing app's LaunchAgents directory. It does not create
or copy a plist, select a different service, change entitlements or register during
construction/inspection. Expose not-registered, enabled/eligible, requires-approval,
not-found and unknown/unsupported states distinctly; none proves a running process,
runtime stop, pairing revocation or helper/descendant absence.

Explicit enable reconciles current state, returns existing enabled/approval-needed
state without another registration and refuses missing/unsupported configuration.
Explicit disable treats already-unregistered as a registration-only result, otherwise
awaits SMAppService's asynchronous completion and checks the resulting status.
Reconcile status after errors before deciding whether the desired registration state
was reached. A task canceled after an OS operation begins cannot undo that operation;
retain operation-in-progress until completion, refresh its observed state and report
cancellation without compensating re-registration. Reject overlapping mutations;
inspection cannot report an in-flight change as completed. Use an actor-owned system
adapter so synchronous ServiceManagement calls do not run on the UI actor. No polling,
automatic retry, launchctl shell, startup grant or OS-setting navigation is added.

Vendor independence: Apple-specific registration stays outside core/provider code.
Modularity: expose Fort-owned registration states and a bounded backend seam for
synthetic tests, separate from owned-runtime shutdown proof. Speed: no idle activity;
20 synthetic inspect/reconcile samples target p95 <=50 ms, excluding OS interaction.
Run this asynchronous timing check explicitly in isolation, without unrelated
test work queued on the UI actor; normal suite execution keeps its behavior checks
and records the timing check as excluded rather than enforcing a contended budget.
ServiceManagement's real call/completion latency requires installed evidence; a
callback that has not returned remains pending, never a shutdown success. Installation/
removal: preserve explicit consent, approval-needed and denied/uncertain outcomes;
the eventual installation adapter must combine startup absence with independent
runtime/core/helper proof before removing credentials. Test repeated calls, changed
external status, partial register/unregister failures, missing states, overlapping
actions, task cancellation before/after dispatch and delayed callback completion.
Compile both clients. Do not invoke the real registration backend in tests or wire
it into ordinary app construction until exact bundle/profile/activation gates pass.

Compose the private child transport behind a Mac core-host lifecycle controller.
The controller forwards the existing command, setup and cancellation contracts while
serving. Exact owner recovery occupies one shared operation, blocks other work,
validates the core's revocation/runtime-stop receipt, then awaits a clean process/group
exit and verifies the private socket is absent before reporting `coreStopped`.
Concurrent retries for the same identity share the operation; different identities
are rejected while it is pending. Successful repeated recovery returns the retained
receipt only for its exact identity. An uncertain recovery failure leaves the controller in a
recovery-only state. Distinguish an authoritative identity rejection made before
any stop mutation: the core emits a dedicated rejection type, the private pipe
returns only the exact requested identity and fixed rejection outcome, and Swift
validates those fields before restoring its prior serving state. Generic errors,
timeouts, foreign/malformed rejection bodies and runtime stop failures cannot
restore ordinary traffic. A rejection during an already uncertain recovery leaves
that prior recovery-only state intact. Before a valid revocation receipt, an explicit retry revalidates
its supplied identity with the core. After revocation, retain that exact proof and
retry only child/socket cleanup; reject foreign identities and never delete an
unexpected socket path. This retained proof is local to the controller incarnation;
a later host must obtain durable recovery evidence from the core again. It never
reopens traffic after uncertain recovery, restarts a process or resumes durable work
implicitly. Cancellation of a caller after dispatch cannot abandon the shared cleanup.

Explicit host shutdown waits for a pending recovery, then closes its remaining child.
Repeated shutdown shares its result. An unclean exit, remaining socket or unconfirmed
revocation never becomes a successful owner-recovery receipt. Retain history and all
credential/startup ownership outside this controller. Its receipt proves core/runtime
cleanup, not host-helper exit, startup unregistration, app removal or profile authority.
The composed process retains the existing transport bounds and caller-supplied launch
configuration; production launch must use the parent-exit guardian. Test through the
public host contracts with real isolated child-process fixtures, including overlapping
recovery, caller cancellation, failed cleanup, exact retry and post-stop request denial.
Use this controller in the existing signed embedded-XPC fixture and retain all
identity, cancellation, crash-recovery, sandbox and cleanup controls unchanged.

Vendor independence: this controller adds no provider or storage implementation.
Modularity: one lifecycle owner composes existing Swift transport/control contracts;
the production wrapper and installation adapter retain separate responsibilities.
Speed: no polling or idle work is introduced; existing 30-second command and six-second
child cleanup bounds apply. Record combined synthetic recovery duration separately,
with a target <=2 seconds for the normal no-active-turn fixture, excluding cold launch.
Installation/removal: no registration, credentials or real provider access occurs;
ordinary app composition remains gated by the exact bundle/profile and startup scope.

Add a production owner-local core composition function behind the private pipe.
Require the caller's exact runtime implementation and descriptor, an existing absolute
owner-only nonsymlink data directory matching the process working directory, and
explicit input/output streams. Use only `core.sock`; do not open a TCP listener,
issue enrollment, create device tokens or select a runtime/module by default. Pairing
has no expiry by default, with an explicit optional policy. Restore is the default
start mode; explicit resume remains a distinct caller decision and cannot bypass
pending revocation/cancellation. Expose existing owner setup/cancel/recovery controls
through the bounded private channel, without fixture bootstrap or debug routes.

The composition owns the supplied runtime for the call. On preflight/start/channel
failure, EOF or abort, await core cleanup, or dispose the supplied runtime when core
construction never completed. The supervising native guardian retains the process
cleanup deadline; this function cannot turn a hung/failed runtime disposal into a
successful shutdown. Reject invalid storage before any ledger creation or permission
change. Preserve history and stopped intent across reopen. Do not copy environment
credentials, register startup, import optional modules or add provider fallbacks.

Vendor independence: composition accepts Fort runtime/context contracts and contains
no vendor SDK. Modularity: package this production entry with the core; runtime
selection and Apple bundle/profile configuration remain outside it. Speed: no new
polling or idle work; retain the existing core cold/warm, request, replay and cleanup
budgets. Installation/removal: one explicit private directory and pipe lifetime,
with the caller responsible for canonical App Group resolution and signed runtime
placement. Prove private pairing/send/replay, retained history, restore versus explicit
resume, failed startup/EOF/abort cleanup and wrong/private/symlink directory refusal
through the actual child protocol. Run the compiled artifact and guarded native host
journey. This prepares composition; it does not install or register a production host.

Prepare the production agent's private ledger directory through the exact validated
App Group identifier. Resolve only Apple's group-container API; a missing lookup or
inaccessible container fails without a HOME, Application Support or temporary fallback.
On macOS the returned URL alone is not entitlement evidence; the production wrapper
must separately validate its signed package/profile before invoking this adapter.
Do not create an App Group container. Open the returned container without following its
final symlink, confirm current-user ownership, no group/other write permissions and
no extended ACL entries, and derive its canonical path from the open descriptor.
Create only the fixed `FortHost` child at mode 0700; existing contents remain intact.
Reject non-directory/symlink paths, foreign ownership, group/other permissions or
extended ACL entries on that child. Do not chmod, strip ACLs, migrate or delete to
make an unknown location acceptable. Sync new directory creation before returning.

Return an identity-bound storage value that rechecks the original container and child
before yielding the launch directory. Replaced, moved, deleted or permission-changed
paths fail; ordinary reopen preserves the same history. Descriptor-relative creation
and identity checks bound accidental path replacement; this is not isolation from a
hostile process with the same user's access, nor a claim to hold a lease on the ledger.
The core's existing exclusive ledger/socket ownership remains authoritative. No
credentials, runtime, startup registration or provider configuration are read here.

Vendor independence: this is an Apple placement adapter over the existing portable
private-directory contract; no AI/vendor dependency changes. Modularity: group lookup
and safe storage preparation stay outside the Node core and optional modules. Speed:
no timer or idle work; measure 20 isolated prepare/reopen/validate samples each, target
p95 <=50 ms per operation, excluding Apple lookup, launch, provider and UI latency.
Installation/removal: fixed group-relative storage is discoverable and history is
retained by default. Prove exact lookup/no fallback, preserved history, concurrent
preparation, invalid storage refusal, identity-change rejection and a guarded core
journey from the prepared directory. The system App Group lookup and actual profile
authorization remain unrun until matching production identities/profiles exist.

Extend the existing signed synthetic bundle with one explicit offline Codex executable
probe before choosing its production package placement. Require the exact selected
0.157.1 executable bytes (SHA256
`27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`), reviewed at
upstream commit `36650394c5b38c2990ccf2a3457165ca3e9d9726`. Copy only that binary
into the standard `Contents/MacOS` code directory, retaining pinned upstream LICENSE
and NOTICE plus original/bundled hashes. Re-sign the local copy with the existing
development identity and the same sandbox+inherit child entitlements; do not modify
the installed runtime. Keep the parent's sandbox-only entitlement and existing Node
outside-file/network denial checks. No entitlement expansion or startup registration.

Invoke only `--version`, with stdin closed and a five-second child limit inside the
existing 15-second parent/group bound. The fixture owns a fresh empty HOME, explicit
existing CODEX_HOME and temporary directory; pass only those paths and a fixed PATH.
Never read/copy the owner's home, settings, environment credentials or authentication.
Pinned source shows dotenv/PATH setup before CLI parsing, so this cannot be an ambient
shell version check. Retain stdout/stderr as bounded synthetic diagnostics, exact
exit/version result, child PID and absence checks. Confirm both exact process/group
and synthetic container cleanup before reporting success. Preserve a failed artifact
and report incompatibility; do not weaken the binary/version pin to make it pass.

Vendor independence: the optional runtime probe does not enter the vendor-free core
artifact or select a production provider. Modularity: copy no voice, code-mode host,
shell, rg or other optional payload, and make the scenario opt-in with an explicit
binary path. Speed: retain existing 15-second synthetic work/cleanup bounds and record
the isolated version-command duration against a five-second limit, not a provider or
production startup benchmark. Installation/removal: a local signed engineering copy
only, with unchanged process/container cleanup and legal provenance. Actual app-server
package metadata, helper discovery, third-party redistribution closure, signed
App Group/profile, authentication, provider operations and TestFlight remain separate
unrun gates. A version response cannot establish those capabilities or complete v1.

Keep a concrete Apple activation gate with the exact current engineering identifiers,
verified signing team, local profile inventory and proposed production client/helper/
App Group strings. Historical references and proposed names are not verified account
records or adopted release identities. Define the smallest read-only account check,
any conditional provisioning changes and separate per-user startup action. Do not
request activation approval before the actual signed package and its scope are ready
for review. This clarifies the existing Apple dependency/removal gate; it adds no
runtime dependency, performance work, account grant, provisioning change or activation.
Continue independent implementation while those external gates remain open.

Observe the helper lifetime separately from its owner-recovery receipt. Add an
explicit XPC recovery operation that returns the validated receipt plus an in-memory
process observation obtained from NSXPCConnection's authenticated peer PID/effective
user and the OS process start time. Never trust process identifiers in reply JSON.
Require the current effective user and a nonself PID greater than one. Capture the
peer metadata before invalidating the connection; a vanished/uninspectable peer
cannot produce a new observation. Preserve the existing receipt-only operation.

The observation can confirm exit or await it for an explicit finite deadline no
greater than 30 seconds. An absent PID or different OS start time means that original
process is gone; a still-matching process remains live even if its effective user
changes. Only ESRCH is absence. Permission/short-read/other inspection failures stay
unconfirmed. Cancellation or timeout ends only that caller's wait. Never send a
signal, enumerate unrelated processes, relaunch the helper or register/unregister
startup from this observer. Poll only during an explicit wait, at most 40 times per
second, with no timer or work while idle. This in-memory value is not a durable
removal journal and does not itself authorize credential deletion.

Vendor independence: process observation is confined to the Apple lifecycle adapter;
no runtime/provider/storage dependency changes. Modularity: keep OS inspection and
XPC peer metadata out of the portable recovery receipt and Node core. Speed: bounded
metadata calls and at most a 25 ms polling interval while waiting; measure 20 isolated
live inspections, target p95 <=5 ms, excluding helper shutdown/provider latency.
Installation/removal: prove actual helper absence separately from revocation and
startup state. Test live/exit/repeated wait, timeout/cancellation, invalid identities,
changed start time, inspection errors and same-process user change. In the existing
signed synthetic XPC journey verify exact peer correlation, forged reply PID refusal,
signed-peer rejection, and helper exit under the existing cleanup bounds and unchanged
entitlements. A restarted client, durable partial-removal proof, SMAppService
unregistration and production App Group/profile/installed acceptance remain separate
work. Reuse decision 059 for this unchanged lifecycle scope; no external activation.

Compose a durable Mac removal operation over exact named-agent recovery, the existing
SMAppService wrapper and authenticated process observation. Bind it to one recorded
agent endpoint/installation/device and the client's existing private storage. Use
`host-removal.json` under the existing owner-file lease and bounded atomic durable
writes. Refuse missing/foreign connection metadata, pending setup, malformed/unsafe
records and changed storage without repairing or deleting them. Reject extended ACLs
on the private directory, lease file and records, including ACL changes during an
operation; do not strip permissions to make a location acceptable. Ordinary construction
has no registration, connection, credential or provider effect.

Persist requested intent before recovery, then validated receipt and process identity
before unregistration. Restore only a validated recorded PID, original user and start
timestamp; no public process-observation decoder is exposed. A new boot or reused PID
cannot turn an unknown lookup into success: only absent PID or changed start time
confirms the original process is gone, and inspection errors remain unconfirmed.
No boot identifier or new OS grant is needed. If the agent is still registered on a
retry, recover its exact current peer again before unregistering. If it is already
not registered and a recovery receipt was durably saved, finish from that record
without XPC activation. An already-confirmed operation must still observe absent
original process and unregistered startup; unexpected re-registration fails rather
than applying old authority to a new host lifetime.

Await the actual startup callback/status and original helper absence, then recheck
startup and exact connection/record before persisting confirmation. Only this returns
all three OwnedHostStop assertions. Cancellation, timeout, write failure or uncertain
OS state preserves progress for explicit retry; it does not remove credentials,
history or app files. Conflicting/concurrent setup/removal is refused under the same
lease, and pending removal records block new owner setup. Production app composition,
removal-record reconciliation for an explicitly new installation, and history deletion
remain later integration work; this operation is not installed-product signoff.

Vendor independence: no provider/core/storage-engine dependency changes. Modularity:
OS startup, authenticated XPC and private client progress stay in the Apple lifecycle
adapter; the core recovery contract stays unchanged. Speed: no idle work; the existing
30-second recovery, explicit OS callback, <=30-second process-wait and storage limits
remain. Measure 20 isolated confirmed-record retries, target p95 <=50 ms, excluding
real startup callback/XPC/provider latency. Installation/removal: prove restart after
unregistration, failed and repeated recovery, exact identity/storage rejection,
callback/exit ordering, cancellation, stale confirmation, persistence-before-dispatch
and preserved history/credentials through the public operation with synthetic files
and an injected startup backend. Serialize signed verification and restore an encoded
process observation from the real authenticated fixture peer without changing its
entitlements. Do not invoke real SMAppService, Keychain, App Group or provider actions.
Reuse decision 059 for this unchanged local lifecycle implementation scope.

Prepare an explicit guarded process launcher for the optional Codex RPC adapter.
The existing legacy launcher owns a detached runtime group but has no parent-death
watch; preserve its historical callers/consumed packets. The new Mac-only entry
requires the reviewed native guardian, exact runtime executable/argv, working
directory and explicit environment. Compose the guardian as the owned group leader,
passing the current launcher PID so it independently watches its direct parent.
The actual runtime and its ordinary descendants inherit that group. Do not silently
fall back to an unguarded launch or select an ambient executable/environment.

Validate absolute normalized nonroot paths, well-formed/NUL-free bounded arguments
and environment before spawning; cap each path at 4096 bytes, arguments at 256
entries/65536 bytes and environment at 128 entries/65536 bytes.
Use an explicit positive work deadline <=120 seconds and fixed 250 ms TERM / 750 ms
KILL cleanup bounds. Reuse the CodexConnection wire/disposal implementation without
provider initialization, permission-profile changes, login or account reads. Normal
close must confirm the entire owned group absent. Correct the shared launcher's
exit race without changing its historical deadline/packet authority: only signal
a positive child PID greater than one, and treat failed signals or indeterminate
group status as unconfirmed until bounded child-close/group-absence proof. A
transient macOS EPERM during reaping must not permanently fail an already exiting
child, while a surviving or uninspectable group must still fail cleanup. Cover
missing executables and immediately exiting children at the owned-process interface.
Abrupt direct-parent death must
trigger the guardian even when that runtime group is separate from the outer
executor. A worker must still await this child's disposal before emitting success;
separate-group absence is not proven by observing only its outer executor group.

Vendor independence: this launcher is an optional Codex/Apple lifecycle adapter and
must stay outside the portable core artifact. Modularity: retain the RPC interface,
explicit process ownership and provider composition as distinct boundaries. No
provider/model/account defaults or paid-service path are introduced. Speed: no new
idle poller; retain the guardian's event-driven parent watch and existing bounded
cleanup polling. Measure 20 isolated synthetic launch/ping/dispose journeys, target
p95 <=2 seconds including both processes and local IPC, excluding compilation,
signing, network and provider work. Installation/removal: prove normal disposal,
ignored TERM, deadline, missing/invalid exact inputs and abrupt worker death with
runtime/guardian/descendant/group absence within three seconds. Classify compiler/
process supervision tests in the explicit macOS suite. No startup registration,
real Codex invocation, credentials or account configuration is authorized by these
synthetic tests. A child group can still escape if trusted runtime code deliberately
creates another session; process supervision is not a computer-tool sandbox or
proof of App Sandbox/provider packaging. Full executor-worker/provider composition
and signed/installed acceptance remain later gates under the same v1 direction.

## Performance verification

### Ordinary Mac host controls revision

Wire the ordinary Mac client to an inert `MacHostControl` constructed from its
exact application bundle URL. Construction does not inspect, register or connect.
The public controller obtains `Bundle.main.bundleURL` itself: ServiceManagement
registration is scoped to the running app, so production callers cannot select a
different app to verify. Alternate URLs/backends are internal test injection only.
Explicit inspection validates the bundle before reading registration status;
invalid packages expose an unavailable state without a fallback identity. Keep
the first verified client/helper/team/group identity for that controller lifetime,
refusing later drift. A separate user enable action repeats bundle verification
before invoking the existing startup boundary. Never register during inspection,
owner setup, restore, a view refresh or construction. OS approval-required, enabled,
not-registered and unavailable states remain distinct; enabled registration does
not claim a running provider or successful pairing. There is no raw disable button:
removal must use the durable stop/revoke/exit proof already specified.

Only an enabled verified registration permits new owner setup. Repeat inspection
before setup; failures keep existing records and credentials and send nothing.
Refuse startup enable while any durable host-removal record exists, including a
confirmed one awaiting explicit reinstall reconciliation. Hold the existing private
progress lease across an enable action, preserving its records. Reject saved
connection identity mismatches before credential lookup or host contact.
The ordinary app uses these controls, while explicit debug/UI fixtures keep their
separate paths. Display concise startup status, explicit enable/check actions and
the existing separate pairing flow. Missing unsigned engineering package metadata
must display unavailable rather than offer legacy fixture enrollment as a fallback.
No actual startup activation or provider invocation is part of this source slice.

The expanded native compilation reproduced Swift's sending-closure diagnostic in
the existing workspace URLProtocol fixture, including with a fresh scratch build.
Make its immutable request and already-Sendable protocol owner explicit task
captures; keep compiler concurrency checking enabled and run the full native suite.
This is a test-boundary repair, with no production transport change.

Vendor independence: Mac-specific lifecycle remains outside portable contracts and
provider adapters. Modularity: the controller composes bundle inspection and startup
status, with the existing setup coordinator retaining credential/protocol ownership.
Speed: no recurring polling; expensive bundle work stays asynchronous off the UI
actor. Twenty isolated controller inspections with fake signature/status backends
must meet p95 <=50 ms, separate from the unverified real package/OS budget above.
Installation/removal: explicit enable, fresh verification and fail-closed setup;
durable removal and complete installed acceptance remain separate. Test inert
construction/inspection, changed identity or signature before enable/setup, OS
approval states, repeated actions and cancellation without real registration.
Fresh unsigned Mac/iPhone builds and native tests cover integration; accessibility,
screenshots and actual startup remain unverified. Reuse decision 059 for this
ordinary local source integration; no new identifier/profile/grant is adopted.

### Native host bundle inspection revision

Add a read-only Mac bundle inspection before wiring startup controls. The signed
client and helper each carry identical bounded `Contents/Resources/fort-host.json`
configuration with exactly format `1`, client identifier, helper service identifier,
signing team and App Group. Names remain explicit build inputs; this does not adopt
the proposed production identifiers or register them. The fixed helper layout is
`Contents/Helpers/FortHost.app/Contents/MacOS/FortHost`; the LaunchAgent plist is
`Contents/Library/LaunchAgents/<serviceIdentifier>.plist`. Require exactly its Label,
BundleProgram and one true MachServices entry, with no environment, alternate
program, arguments, automatic RunAtLoad or KeepAlive keys.

Validate both code signatures with the exact Apple-anchored identifier/team
requirement and strict resource/all-architecture checks before using signing
metadata. Both independent apps require App Sandbox, the exact App Group and no
sandbox-inherit entitlement. Require each embedded provisioning-profile file to
exist as a bounded regular file, but explicitly report only profile presence;
profile eligibility and actual OS startup remain unverified. Do not call App Group
lookup, SMAppService, Keychain, XPC or any provider during inspection. Refuse missing,
symlinked, foreign, malformed or inconsistent metadata without rewriting it.
Do not treat inspection as protection against concurrent same-user bundle updates;
repeat it before activation, and keep actual installed/runtime checks separate.

Vendor independence: this is the existing explicit Apple distribution boundary,
with no provider knowledge. Modularity: package identity/layout inspection stays
separate from startup registration, storage and runtime readiness. Speed: perform
signature/file work off the UI actor with no idle work; measure 20 isolated metadata
inspections with a synthetic signature reader, p95 <=50 ms, and separate real
signature checks from those samples. A fresh complete packaged-host inspection has
a proposed p95 <=2-second budget, still unverified until its signed artifact exists.
Installation/removal: use temporary synthetic bundles to prove exact manifest,
layout, profile-presence and signature/entitlement refusal; exercise real Security
framework verification on an existing authorized signed artifact when available,
without launching or modifying it. No new entitlement, profile, credential,
registration or provider action is authorized by these tests. Reuse decision 059
for this local native integration scope.

### Artifact build-environment repair

Apply the executor builder's isolated compiler environment and destination ownership
rules to the independent core builder. Require Node 26.5.0 and a new normalized,
nonroot absolute destination; record its device/inode after creation. Compiler and
Git license-fallback subprocesses receive an empty environment and bounded timeouts.
Use the same empty/bounded license fallback in the executor builder. Inspect emitted
files without following symlinks and require regular single-link files. On failure,
remove only the created destination if its current directory identity still matches;
preserve a replacement directory. This is a point-in-time ownership guard, not an
atomic filesystem transaction against a same-user race.

Vendor independence: core output remains free of provider implementations. Modularity:
this repairs the existing two packaging functions without changing runtime contracts
or adding a dependency. Speed: five serial fresh core artifacts, warm toolchain/cache,
no incremental output reuse, each/p95 <=10 s; provider/network excluded. Installation/
removal: no install hooks or activation, no ambient Node preload/Git environment
inheritance, and no deletion of an unrelated existing destination. Reuse decision 059
for this recoverable build-path issue.

Acceptance: first reproduce the core compiler executing an authored synthetic ambient
preload, then prove the marker stays absent and compilation succeeds. Verify malformed
and existing destination refusal with original contents intact, complete manifest
file/size/hash correspondence, typed/compiled core independence journeys and ordinary
owned-core restart/disposal. Run portable tests and strict TypeScript; no signed/native
build, provider/account or installed claim is needed for this packaging-only repair.

### Compiled private executor composition revision

Expose a production constructor in the optional Codex artifact with only exact
runtime executable, guardian, private directory and host identifier inputs. Require
the pinned Node version, macOS and the compiled `.js` module. Select that module
itself as the worker, with fixed jitless/no-TypeScript/no-warning flags, empty worker
environment and the private root as cwd. Accept no caller-supplied worker, argv or
environment. Construction performs read-only plan checks but starts no process.

When invoked directly as the compiled worker, accept only the four explicit string
options, reconstruct the private plan inside that process and serve the existing
guarded worker protocol. Use the same host/provider/version binding in both layers.
Reject missing/unknown options with a generic diagnostic and nonzero exit. A
directory/startup-file change after construction must fail before runtime launch.
Reuse the two native guardians, deadlines, cancellation and clean-exit checks; do
not create another owner of process cleanup or select a provider/model fallback.

Vendor independence: only the removable Codex artifact changes; the vendor-free core
artifact stays independent. Modularity: combine existing launch-plan/process/worker
seams through one compiled entry, without a framework or dependency. Speed: measure
20 serial compiled synthetic catalog-and-exact-text journeys, including both owned
process groups and their absence, p95 <=2 s excluding provider/network. Installation/
removal: construction remains inert and private locations must already exist; bundle
bootstrap, signatures, licenses, directory preparation, account and Apple grants remain
separate gates. No real runtime, credential, service activation or installation.

Acceptance: compiled-only catalog and exact text through the new constructor, no
thread/turn during catalog, foreign dispatch refusal, no inherited environment,
post-construction ambient-config refusal before a child runtime, unknown/missing CLI
options and independent group absence. Retain lower-level guardian regression tests,
portable independence tests and strict external declarations. Run affected Mac tests
and measure the isolated journey separately. Reuse decision 059 for this unchanged
local runtime-composition scope. This is not provider or installed-product evidence.

### Private Codex launch-plan revision

Add an inert, read-only launch-plan export to the optional executor artifact. The
trusted composition supplies exact normalized executable/guardian paths and an
existing canonical owner-only runtime directory. Fix its children as `home`,
`state`, `workspace` and `tmp`; all must be current-user directories with mode 0700,
without symlink components. Require empty `home` and `workspace`. Refuse any
`state/.env` or `state/config.toml` entry before launch, including dangling links.
If `state/auth.json` exists, inspect metadata only: private regular single-link,
current-user, nonempty and at most 128 KiB. Never read, copy, create or delete its
contents. Recheck directory identities before returning; this is a point-in-time
check, not an atomic launch lease or proof against a same-user mutation race.

Build fixed app-server stdio arguments using the existing no-tools controls and
permission declaration. Additionally request ChatGPT-only login, file-scoped CLI
and MCP auth stores, and explicitly disabled OpenTelemetry exporters/prompt logging.
The exact environment contains only the derived HOME/CODEX_HOME/TMPDIR, system PATH,
fixed locale and UTC. Return immutable arguments/environment and accept no ambient
environment/config overrides. Construction performs no process, network, login,
credential or persistent setup action. It does not choose a model or billing fallback.
Preserve historical demo launchers and observed startup fixtures unchanged.

Vendor independence: this plan belongs only to the removable Codex artifact; core
and clients keep their existing contracts. Modularity: share the reviewed control
constants, export one plan rather than another process owner, and retain the existing
guardian/disposal protocol. Speed: 20 serial inspections of fresh synthetic private
layouts, warm toolchain/filesystem caches, target p95 <=5 ms excluding provider and
process startup; no idle work. Installation/removal: derive all mutable paths from
the explicit owned root, perform no filesystem repair/deletion, and require separate
runtime signature/provenance, directory preparation, credential grant and installed
sandbox acceptance. Private environment and computer-tool restrictions do not confine
the agent host. No account migration or credential-store policy adoption is granted.

Acceptance: prove exact argv/cwd/environment through a synthetic child; reject unsafe
paths, missing/nonprivate/symlink directories, ambient startup entries and unsafe
synthetic auth metadata while preserving bytes. Prove compiled exports/declarations,
run portable tests, strict typechecking and affected guarded Mac artifact tests.
Keep actual Codex configuration/account behavior unrun and record pinned official
source evidence for dotenv and auth-store selection. Packaging bootstrap, creation
of private directories and real runtime/legal/account/Apple acceptance remain open.
Reuse decision 059 for this local source preparation, with no actual Codex invocation.

### Helper executable and fixed program revision

Add a background-only engineering `FortHost` app target and an inert fixed-program
inspection seam. The entry point derives the containing Fort app from its own
`Bundle.main` helper URL and requires that URL to equal the helper in the fully
inspected parent bundle. It accepts no arbitrary app path, executable, argument,
environment or configuration override from command-line or XPC data. The fixed
helper paths are `Contents/MacOS/node`, `Contents/MacOS/FortProcessGuardian` and
`Contents/Resources/host/main.mjs`. All are canonical, nonempty, regular, single-link,
not group/world writable files; native executables must be executable. Bound sizes
at 512 MiB, 8 MiB and 256 KiB respectively. Resource/code integrity remains covered
by the complete parent/helper signature check; this is not an atomic update lock.

Require both native tools to be Apple-anchored code under the helper's signing team,
with exact derived identifiers `<serviceIdentifier>.node` and
`<serviceIdentifier>.guardian`. Their entitlement key sets must be exactly App
Sandbox plus sandbox inheritance, both true, with no independently granted App
Group. Extend existing read-only code metadata to expose keys without changing the
client/helper entitlement requirements. Provisioning presence is still explicitly
unverified; actual profile eligibility and runtime/library/legal closure remain gates.

Only a successfully inspected program constructs the service lifecycle with fixed
Node flags (`--jitless`, `--no-experimental-strip-types`, `--no-warnings`), the bundled
bootstrap, an empty environment and the exact guardian. The helper handles SIGTERM
by canceling pending startup and awaiting shared service shutdown. Failure uses a
generic diagnostic and nonzero exit; no fallback runtime or registration is added.
The production bootstrap contents/runtime configuration remain a subsequent reviewed
packaging slice. Do not install a placeholder runtime or claim the helper can serve
without its required resources.

The target's local engineering identifier is `io.fort.spec058.fixture.mac.host`;
it is not adopted/registered as a release identifier. It has App Sandbox enabled,
no outgoing network capability, no App Group/profile and no automatic parent-app
embedding/activation. Those must come from the exact reviewed production package.
Creating the target grants no Apple capability or startup authority.

Vendor independence: the Swift program selects only the provider-neutral core
bootstrap boundary; the provider stays behind its optional artifact. Modularity:
reuse bundle/file/signature/lifecycle checks and keep Node/bootstrap supplied
separately. Speed: 20 isolated metadata-only program inspections with fake signatures,
p95 <=50 ms; real complete signature validation retains the <=2 s target and remains
unmeasured until a complete signed helper exists. Installation/removal: missing or
unsafe contents fail before App Group lookup/core launch; signal handling delegates
to the verified lifecycle, with no service registration or app deletion.

Acceptance: exact derived parent/helper, fixed paths, code requirements, permissions,
symlink/hardlink/directory/oversize and entitlement negative controls; public inspection
of the test runner refuses its non-helper layout. Full native tests, strict TypeScript,
fresh unsigned helper/Mac/iPhone builds and generated-project diff review. No helper
launch, actual signal delivery to the new target, provider, profiles, named listener
or TestFlight claim. Reuse decision 059 for this local executable preparation.

### Native helper service lifecycle revision

Compose the existing exact App Group storage, guarded core host and authenticated
Mac listener into one explicit helper lifecycle. Construction is inert. Starting
resolves and validates the exact group storage before core launch, waits for the
core's private-channel readiness, revalidates storage identity, then opens only the
configured Mach service with the existing same-user/exact-code requirement.
No launch registration, pairing, credential lookup or provider request is implicit.
The trusted helper composition supplies its already-inspected bundle and exact core
executable/arguments/environment/guardian; no XPC request selects those inputs.

Starting is single-use. Shutdown refuses new starts, invalidates the listener once,
awaits any in-progress core launch and closes the resulting core. Concurrent shutdown
callers share that operation. A canceled start or failed listener/storage check must
close the core before propagating failure. A failed launch or forced/uncertain cleanup
cannot produce a clean-stop receipt or trigger an automatic retry. Retain failure
state. Successful pairing recovery still leaves the helper alive until its explicit
shutdown/OS unregistration, so the recovery reply can return to the client first.
This wrapper does not itself claim startup unregistration or helper-process exit.

Vendor independence: the lifecycle imports only Fort-owned Swift/core contracts,
not a concrete provider. Modularity: retain existing core, storage and XPC seams;
the helper only composes their ownership. Speed: measure 20 serial fresh synthetic
guarded-core starts/readiness/stops with a fixture listener, p95 <=2 seconds,
including private storage preparation; no idle timer or polling is added.
Installation/removal: no service activation in this slice, no new permission or
credential; clean shutdown is separate from durable removal and OS registration.

Acceptance: public lifecycle tests with synthetic storage/listener boundaries cover
inert construction, ordering, repeated/concurrent calls, cancellation, storage change,
listener failure, launch failure and unconfirmed cleanup. Exercise the actual guarded
compiled core with synthetic runtime, verify readiness before listener activation and
prove its recorded PID/group absent after shutdown. Run the full native regression,
strict TypeScript and unsigned client builds. Actual named Mach registration/App Group
lookup, signed helper target/bootstrap, provider, installed UI and TestFlight remain
unrun. Reuse decision 059; no new external authority is inferred.

### Optional compiled executor artifact revision

Package the existing guarded Codex composition as a separate optional JavaScript
artifact with declarations, its Fort source license and a complete per-file size/
SHA256 manifest. Compile only the composition's static dependency graph. Exclude
service/database code, optional context implementations, prototypes, donor source,
fixtures, Node dependencies and actual Node/Codex/guardian executables. Those
executables and their legal/provenance closure remain separately supplied and
verified; this artifact is not a self-contained application or permission grant.
Publish no package and introduce no auto-install/update/sign-in behavior.
Run the pinned compiler with an empty environment so ambient Node preload options
cannot turn this build into an unreviewed hook invocation.

Use an absolute new destination, refuse overwrite, and clean up only a directory
created by the current build if compilation fails. Preserve existing paths. The
package exports the composition functions and TypeScript declarations; its runtime
imports must resolve entirely within the artifact or Node built-ins. No TypeScript
loader or source checkout is needed to consume the produced JavaScript. Prove the
published declarations from an external strict TypeScript consumer as well.

Vendor independence: the core artifact retains no Codex dependency and its existing
independently shaped provider contract suite must still pass. Removing this optional
artifact leaves that core build unchanged. Modularity: application composition
selects the package explicitly; no new core-domain import or lifecycle policy.
Speed: measure five serial builds into fresh directories with warm installed tools,
all file hashes and declarations included, p95 <=10 seconds; first sample retained.
No incremental cache is introduced. Installation/removal: explicit paths and
fail-closed destination ownership, no runtime installation, new credential or service.

Acceptance: portable build/inventory/hash/license/overwrite and declaration checks;
Mac synthetic compiled-artifact discovery and exact text execution with TypeScript
stripping disabled, followed by independent absence of both process groups. Use
separate fresh workspaces and explicit environments. Run strict TypeScript, the full
portable suite and existing Mac executor regressions. No real provider, signed
package, installed app or TestFlight proof is claimed. Reuse decision 059 for this
bounded packaging step. Production bootstrap/helper, runtime redistribution and
Apple/account activation gates remain separate.

### Guarded model discovery revision

Complete the existing model picker boundary through the owned worker. An explicitly
configured discovery binding pins provider/runtime version/host in both parent and
worker. A single bounded catalog request carries a fresh request ID, that binding,
a deadline no more than five seconds ahead and a consistency digest. It contains no
chat history, model selection, executable path, credential or grant. Hashes are not
authentication; the inherited private channel and locally configured binding remain
the trust boundary. Limit the request to 4 KiB and the optional correlated cancel
frame to 2 KiB. Refuse another operation, unsupported discovery or foreign identity
before constructing a runtime.

Validate the provider and catalog union, unique exact model IDs, availability,
bounded labels/details and ready-state timestamp. At most 1000 model rows and 1 MiB
combined process evidence are accepted. Return a catalog only after runtime disposal
and a clean zero worker exit with owned groups absent. A canceled, expired, malformed
or cleanup-unconfirmed query returns no ready catalog and cannot change the selected
model. Reuse the parent lifecycle implementation and its existing cancellation/
cleanup bounds; execution and discovery share one active slot and cleanup block.
Discovery is absent unless explicitly configured. Stopped/busy discovery starts
no process. Dispose cancels either operation and cannot hide uncertain cleanup.

The Codex composition pins 0.157.1 and requires the explicit host identity. Its
discovery uses the same empty-workspace/guarded-runtime rules and existing account,
configuration and model-list checks. It creates no thread or turn and supplies no
provider sign-in/refresh operation. The user still chooses an exact available model;
unsupported text models remain visible as unavailable. Preserve all send/cancel/
crash contracts. Tests use synthetic official-protocol runtimes only; actual Codex,
accounts, provider requests, signed package changes and activation are unrun.

Vendor independence: this is the existing Fort-owned catalog contract, with provider
semantics behind composition. Modularity: optional discovery does not enter core
domain persistence; one shared subprocess lifecycle avoids a second cleanup policy.
Speed: no background polling or persistent catalog runtime. Measure 20 isolated
synthetic Mac guardian/worker/runtime catalog journeys, p95 <=2 seconds, including
startup and disposal but excluding real provider/network/signing costs. Installation/
removal: prove cancellation, invalid results, nonzero/forced exits and exact process
absence; no new service, dependency, credential or permission. Run portable tests,
existing guarded Mac execution regressions and strict TypeScript. Reuse decision
059 for this existing provider/model capability, without changing its authority.

### Guarded official protocol composition revision

Compose the owned worker with the existing exact Codex 0.157.1 protocol adapter
and guarded runtime launcher. Local composition supplies the guardian, executable,
argv, empty workspace, environment and host identity. Clone that configuration
before reading the dispatch. The incoming request cannot choose any process paths,
environment or account. Pin provider/version in the worker, use the exact selected
model, retain all existing runtime/permission/configuration checks, and derive the
child work deadline from the sealed attempt. Dispose through the owned connection
before terminal output. Actual executable provenance, package layout and provider
activation remain separate required gates; synthetic integration cannot supply them.

The outer executor's 250 ms cancellation allowance is too short for the official
adapter's existing two-second interrupt deadline plus up to one second of child
cleanup. Add an explicit optional cooperative-cancellation allowance bounded to
250..3500 ms, retaining the historical 250 ms default. Codex composition selects
3500 ms in the parent. Preserve the hard dispatch deadline and existing TERM/KILL
cleanup bounds. Report unknown when cancellation or cleanup cannot be confirmed.

Vendor independence: this composition stays outside the portable core and is
removable; other runtime adapters keep the existing worker interface. Modularity:
reuse worker, protocol and process ownership without copying the historical
prototype launch plan or its consumed activation packets. Speed: measure 20
isolated full synthetic parent/worker/guardian/runtime journeys, p95 <=2 seconds,
excluding compiler, signing and provider work; cancellation with delayed synthetic
interrupt confirmation must complete within 3500 ms plus the existing cleanup
reserve. Installation/removal: prove exact-model/config refusal, normal completion,
delayed cancellation, forced unknown outcomes and the entire nested process/group
chain absent after abrupt launcher death within three seconds. Tests use a new
synthetic protocol process with explicit empty environment and temporary files;
no actual Codex process, provider/account reads, credentials or startup actions.
Reuse decision 059 for this local adapter integration scope.

### Owned executor worker revision

Implement the receiving half of the existing executor protocol as a provider-neutral
worker library over explicitly owned private streams. Pin provider/runtime/host from
local composition, validate one bounded UTF-8 dispatch and its complete input shape,
digests, identity, plain-text/no-grant policy and deadline before constructing a
runtime. The hash binds bytes and identity; it is not authentication. Trust comes
from the inherited private pipe and selected executable, never a public listener.
Accept one exactly correlated cancellation. Reject additional dispatches, malformed,
oversized, foreign or expired input without constructing a runtime. Preserve exact
text and optional prepared context rather than selecting defaults or recompiling it.

Own the selected runtime through execution and disposal. Flush one correlated
terminal only after disposal resolves; cleanup failure, EOF, malformed input,
expired deadline or uncertain execution emits no successful terminal. Parent-side
exit/group proof remains required. No method can extend a dispatch deadline or
create a provider/executable/account selection from incoming data. The factory is
explicit and synchronous; actual runtime creation must retain cleanup ownership.

Vendor independence: the worker imports Fort contracts and validation, never a
provider implementation. Modularity: parsing and worker lifecycle remain small
internal modules; local composition alone chooses a runtime. Keep the real Codex
composition/provider authorization as a separate step. Speed: bound the initial
dispatch wait to five seconds, work to its <=120-second dispatch deadline, cleanup
to 1500 ms, each wire dispatch to input limit plus 16384 bytes, and cancellation
to 2048 bytes. No idle work beyond these active deadlines. Measure 20 isolated
fresh Node worker echo/disposal journeys, p95 <=2 seconds excluding provider/network
and compiler work. Installation/removal: exercise malformed and fragmented input,
foreign identity, digest tampering, exact cancellation, EOF, runtime/cleanup failure
and delayed disposal before a terminal at the public worker streams and through
the real ExecutorProcess. Preserve optional context and prove no process/group
survives completion. Use synthetic runtime implementations only; no accounts,
credentials, real Codex invocation or startup activation. Reuse decision 059 for
this existing local executor implementation scope.

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

### Fixed native host resource composition revision

Under unchanged decision 059, assemble the existing compiled core and optional
Codex executor with a separately compiled local context module and one fixed
`host/main.mjs`. Derive the bundled Codex and guardian paths from that module;
accept no caller path/provider/model overrides. The native helper validates Codex
with the same exact team, identifier and inheritance-only entitlements as Node
and the guardian. It prepares only `Runtime/{home,state,workspace,tmp}` beneath
its validated App Group FortHost directory, using the existing descriptor-relative
storage checks. Existing entries, permissions and contents are preserved; unknown
links, ACLs or shared directories fail without repair. Runtime construction stays
inert until discovery/dispatch and retains the existing no-tool/private-file policy.

Use the ledger's existing installation UUID plus stored provider/version/host
binding. `local-codex` is installation-scoped, not a globally unique machine ID or
physical-device attestation. Reopening with a changed host/provider/version is
refused. Do not introduce a second identity file or infer migration from a copied
ledger. The helper owns no credential migration, sign-in or legacy discovery.

Vendor independence: composition explicitly selects the optional Codex and local
context implementations; neither enters the standalone core artifact. Removal
cost remains replacing the selected composition and preserving ledger identities.
Modularity: reuse existing builders, storage and lifecycle; no new storage class,
bootstrap state machine, plugin framework or third-party dependency. Context stays
separate from the executor. Speed: five serial resource builds each <=10 seconds;
20 private-directory preparations p95 <=50 ms; a synthetic native setup/catalog/
text/reopen/revoke journey <=5 seconds excluding artifact compilation, measured
separately from provider latency. No polling or idle timer is added. Installation
and removal: resources include complete file hashes and source licenses, but
binaries, signed helper/profile, actual runtime legal/auth closure, registration
and installed acceptance remain separate. Temporary tests create no credentials,
call no provider and register no service. No external authority is expanded.

Acceptance: red/green at the resource builder and native storage/service seams;
verify no ambient compiler preload, complete manifests, existing destination
preservation, fixed bootstrap argv and separately removable core/context/executor.
Drive Swift FortClient through the actual compiled bootstrap/core/worker against an
authored synthetic Codex protocol process: owner setup, catalog, exact text,
shutdown/reopen with retained identity/history and no replay, then revocation and
independently absent process groups. Signature/container/listener fixtures do not
prove Apple provisioning or OS sandbox confinement. Run affected native/portable
checks and strict TypeScript; record failed and unrun checks. The existing
installation/privacy/lifecycle authority remains unchanged; no action-specific
approval is inferred from synthetic completion. Revisit machine identity only if
an explicit migration/device-attestation requirement arises.

### Ordinary Mac removal composition revision

Under unchanged decision 059, connect the existing durable MacHostRemoval controller
to the ordinary verified Mac host and native model. Reuse that controller as the
OwnedMacInstallation implementation, with an exact inspected app URL and connection;
do not add a second removal coordinator. Before external recovery/unregistration,
revalidate the same bundle identity. Setup remains blocked by saved removal progress,
but that progress must not hide the control needed to finish removal after restart.
The default path confirms revocation, core stop, startup unregistration and observed
helper exit before removing the exact device credential and offering system app
removal. Retain history and the recovery record. Disconnect recovery confirms core
revocation without silently unregistering startup. Construction remains inert.

Expose history deletion as a real capability of the installation, rather than
allowing an unavailable production callback to begin destructive preparation. The
production controller initially supports retained-history removal only; the separate
history-deletion boundary remains unimplemented and must not be represented as v1
completion. Refuse an unsupported deletion before shutdown, credential changes or
file removal. Existing fixture implementations opt into their authored deletion
capability. This is incremental wiring, not removal of the accepted deletion goal.

Vendor independence: Apple installation lifecycle stays outside service/provider
contracts; no AI or hosting dependency changes. Modularity: reuse the durable
controller and small installation contract; one capability expresses actual support,
with no generic adapter framework. Speed: 20 synthetic controller-to-removal metadata
journeys p95 <=50 ms excluding deliberate child-exit waits, signatures and provider
latency; existing bounded helper waits remain <=30 seconds and no new timer is added.
Installation/removal: exact package/connection identity, retained recovery progress,
no hidden registration or history deletion; wrong package, unsupported deletion,
foreign pairing, failed cleanup and retry remain explicit. The real Apple activation,
credential and installed-UI gates remain unchanged. No tests use a system registration,
real Keychain credential or existing user history.

Acceptance: public native lifecycle/model red-to-green tests for ordinary control
availability, saved-removal recovery, unsupported deletion before effects, exact
app/endpoint checks, retained-history completion and repeated cleanup. Re-run affected
native tests, unsigned Apple builds and structural/staged gates. Actual installed
accessibility, screenshots and provider inspection remain separate. Review deletion
semantics separately before enabling that capability; keep the owner's clarification
request visible while continuing the independently authorized retained-history path.

### Required Ponytail review revision

The owner's October 18 instruction, "Yes. Let's add a ponytail check as a required
review", is recorded in `docs/architecture/decisions/060-required-ponytail-review.md`.
Require a whole-repository baseline with explicit depth/omissions, followed by
proportionate semantic reviews of changed files, callers and affected boundaries.
Use the original read-only audit skill pinned by upstream commit and SHA256.
Never install its hooks/scripts or apply a global mode. Preserve Fort requirements,
security checks and explicit exceptions; simplicity does not justify removing them.

Keep one workflow in specs/README.md with short agent-document pointers. Store the
baseline and current change review in docs/architecture/reviews/ponytail; Git retains
earlier change reports. Each report records reviewed commit/scope, skill provenance,
reviewer, concrete findings/dispositions, test evidence and omissions/exceptions.
An open v1 finding remains in the acceptance ledger even when its reviewed slice
can be committed. Release-blocking exceptions cannot be relabeled as completion.

A small Node CLI verifies the staged snapshot or a trusted committed comparison
base. It requires the baseline, valid review metadata and SHA256 coverage of each
changed regular file, including deletions and both rename paths. Exclude only the
current report itself to avoid a circular digest. Read blobs from Git's selected
snapshot, never unstaged replacement content; reject missing/stale coverage, wrong
skill pin, invalid revision, empty comparisons and malformed evidence. This checks
evidence integrity and scope, not whether a semantic review happened or was sound.
Run it before completion/commit and in the existing read-only PR quality job. A
manual CI invocation must supply an explicit trusted base. No branch protection or
remote CI success is inferred from a local workflow file.

Vendor independence: Node's standard library and local Git only; no review service
or AI vendor dependency. Modularity: one standalone evidence gate, separate from
the spec validator and behavior tests. Speed: synthetic CLI cases complete within
10 seconds each; one staged repository check targets <=5 seconds, with no runtime
or idle-process cost. Installation/removal: no dependencies, credentials, hooks,
startup services or external mutation; removing this gate leaves product behavior
unchanged but would violate the owner's review requirement. Test the public CLI
against temporary repositories with stale, missing, renamed, deleted and unstaged
files. Reuse existing CI rather than adding a service or approval framework.

The baseline's bounded follow-up also fixes executor Unicode-path rejection,
native CI fixture composition and the stale successor README, and deletes the
unused MacHostRemoval constructor. Use an observed red-to-green public path test;
validate the existing native journey with both standalone and Homebrew Node layout
selection as available. The missing native resume journey stays an explicit v1
blocker pending its separate private-host slice. No automatic resume or new grant
is authorized by this review mechanism.

### Explicit owner resume revision

Resolve baseline Ponytail finding PT1 under unchanged decision 059. Reuse the
authenticated `/v1/runtime/restart` contract for an explicit owner-local resume;
do not add another XPC command, startup operation or bootstrap argument. The
stop receipt promises runtime stop, not removal of the local control process.
Keep that control channel and ledger available after owner stop/disconnect so
readable history, retry and recovery remain possible. Dispatch and scheduling
remain durably paused until an exact authorized resume. Full core/helper exit
and startup unregistration remain separate removal proof requirements.

Accept resume only for the latest exact, still-authorized owner device, current
installation/recovery epoch and a stopped owner intent. Reject temporary demos,
pending disconnect/setup cancellation, active stop/resume, changed identities,
failed cleanup and unsupported runtime capability before any restart. A fresh
core restored in paused state may resume its selected ready runtime without
replacing it. A runtime stopped in this process needs its explicit restart
capability and confirmed disposal. A failed or concurrent close cannot commit
running intent; cleanup waits for the owned resume operation. Preserve ambiguous
turns and idempotency; no accepted request is automatically resent after an
uncertain outcome. Ordinary reopen never clears paused intent.

The stateless guarded ExecutorProcess can explicitly re-enable dispatch only
after stopped state, completed disposal and confirmed process-group absence.
Keep its immutable executable/arguments/environment/binding and normal per-turn
limits. No child is launched by this restart method. Canceled, active, unconfirmed
or already-running restart requests fail. Existing bounded demo supervisors retain
their own restart budgets; owner resume cannot expand temporary session authority.

Native settings show an explicit resume action only when owner-local runtime
status declares support. The model retains access to control/history while work
is stopped, derives that state again after relaunch, and disables sending until
the service confirms resume. Keep exact credentials and stored conversations;
do not reenroll, change expiry or register startup. Existing generic runtime-recovery
behavior remains distinct from the owner-paused label.

Vendor independence: the existing optional runtime restart capability is used;
no vendor SDK, model or account change. Modularity: runtime disposal/restart stays
behind ChatRuntime, durable intent stays in the core, presentation stays native.
Speed: no new process or timer on resume; synthetic stop/read/resume/send journey
<=5 seconds excluding builds, with paused idle scheduling still disabled. No
new dependency or speculative host supervisor. Installation/removal: preserve
explicit stop, no-expiry pairing and exact revocation; a control process remaining
available is visible in wording and does not claim full uninstall/absence.

Acceptance: red-to-green public service stop/read/resume/send and restart/paused
reopen behavior, wrong epoch/device/auth, disconnect, failed/busy cleanup and
close-during-resume; guarded executor disposal/resume/cancellation behavior; native
model and complete compiled host stop/resume journey. Run relevant regressions,
strict checks, fresh native builds, and final Ponytail/staged gates. Installed UI,
actual provider and persistent activation remain separate unrun gates.

The full native regression exposed a fixture ordering error: its 10 ms deadline
could precede the synthetic setup commit under ordinary parallel load. Repair the
fixture with a held response after a durable commit marker. The test must observe
that marker before its bounded deadline, verify timeout, release the held response
and verify exact retry. This changes only test synchronization, not production
timeouts, cancellation behavior or cleanup guarantees. No shipping dependency or
idle cost changes; the fixture response is released on every exit path.

### Explicit local pairing after disconnect

Resolve the resume review's remaining local lifecycle gap under unchanged decision
059. After confirmed disconnect, an explicit local begin-setup call may create a
new offer while work stays paused. Completing that exact offer may prepare the
selected runtime, revalidate the unexpired offer and new device identity, then
atomically commit the new pairing and running intent. Only that successful commit
clears dispatch pause. Beginning, inspecting, canceling or reopening never resumes
work. The old token and device identity remain unusable; history is preserved.

Keep this operation on the existing trusted local setup contract. There is no new
HTTP route, XPC command or bootstrap argument. Allow it only after the latest
device has been revoked, no active pairing remains, setup cancellation is complete,
and runtime cleanup is confirmed. Ordinary Stop without disconnect still requires
the existing authenticated resume. Failed, unsupported or overlapping runtime
preparation and close during completion leave pairing absent and work paused.
Offer expiry or identity changes during preparation must be rejected before commit.
The existing exact completion retry handles a lost successful reply.

Make core setup completion asynchronous and migrate every caller; the private
channel already awaits it. Share the existing bounded runtime preparation and close
coordination with authenticated resume. Keep identity/offer validation on both sides
of the await. No new dependency, timer, startup registration or credential access
is introduced; synthetic credentials remain test data only.

Vendor independence: use ChatRuntime's existing optional restart capability, with
unsupported preparation visible. Modularity: setup/auth state stays in core and
runtime cleanup stays behind its adapter; native contracts are unchanged. Speed:
extend the isolated compiled native journey with re-pair and one new send, retaining
its <=5 second bound; setup inspection remains inert. Installation/removal: explicit
pairing makes retained history usable after disconnect, while removal progress and
pending revocation still block new setup. This does not clear completed uninstall
records or authorize actual new credentials, startup activation or history deletion.

Acceptance: same-core and reopened re-pair/send/history; rejected old token/device,
wrong/expired offer, stopped-but-paired host, failed or unsupported cleanup/restart,
overlapping completion/cancellation and close during preparation. Run migrated
setup, resume, private-channel, native and strict checks, rebuild exact resources,
then require the final semantic review and staged gates. Installed re-pairing,
Keychain and provider acceptance remain separate external gates.

### Retained-history reinstall reconciliation

Resolve the saved-removal blocker under unchanged decision 059. Add an explicit
native Set up Fort again action after completed removal. Recheck the exact current
app/helper/endpoint identity, confirmed recovery receipt, independently absent
recorded helper, startup not registered, absent active/pending connection, matching
confirmed local pairing-removal/disconnected records and absent exact credential.
Hold the existing private owner-file lease and revalidate records across waits.
Only then remove the completed host-removal block with durable directory sync.
Preserve history, disconnected/revocation markers and unrelated files. Repeated
inspection remains inert; a lost successful action leaves setup available without
starting anything. Pending, corrupt, foreign or changed progress remains blocked.

The action must not register startup, reconnect, create/delete credentials, launch
the helper, revoke again or remove history. Enabling and pairing remain separate
explicit actions. Existing completed-disconnect pairing handles the later connection.
No new runtime or public-service contract is needed. Keep the exact credential
absence check inside the explicit action; mere UI inspection must not read Keychain.

Vendor independence: no runtime/vendor change. Modularity: removal owns its proof
and transition; native model presents the existing host's capability. Speed: no
idle work, timer or new process; 20 isolated synthetic reconciliation samples must
have p95 <=500 ms, excluding fixture preparation, real Keychain and macOS callbacks.
Installation/removal: retained history is the default, and completed removal is
reconciled without bypassing unfinished cleanup or silently activating persistent
access. No actual owner metadata or credentials are changed during development.

Acceptance: public removal red-to-green, exact confirmation and credential-absence
checks, progress/identity/startup/helper mismatch refusal, interruption/retry and
native model eligibility; preserve setup guards and historical removal behavior.
Run native regressions, isolated timing, strict checks, fresh unsigned clients and
final Ponytail/staged gates. Actual reinstall, Keychain/startup, installed UI and
TestFlight remain separately gated.

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

### Explicit deletion of saved client history

Under decision 059 and the coordinator's continued-v1 instruction, implement a
separate confirmed action for saved history on this Mac after confirmed pairing
revocation and exact credential absence. Delete only this installation's cached
navigation/conversation snapshots, pending submissions and local image previews.
Preserve host ledger, settings, provider sessions, other installations, app files,
credential/revocation records and host-removal progress. State those limits in the
confirmation and result. This is a concrete client-data slice, not complete host
history deletion or installed-product signoff. Keep remains the default.

Vendor independence: this operation uses Fort's existing local formats and no AI
adapter. Modularity: storage owns the exact file scope; the native model owns
eligibility and clears in-memory projections only after confirmed completion.
Speed: at most 4096 inspected entries and 256 MiB decoded in a pass; reject larger
or malformed stores without deleting any files. Twenty synthetic deletion samples
with 100 snapshot files target p95 <=500 ms, excluding fixture creation and real
Keychain latency. No periodic work or new dependency. Installation/removal: use
an exclusive existing owner-file lease, descriptor-relative no-follow reads and
unlinks, file identity/content verification, preflight before any unlink, durable
directory sync, and fresh absence verification. A partial failure is reported
incomplete and can be retried; no backup/snapshot erasure is claimed.

Acceptance: public storage red-to-green with exact-installation data and unrelated
sentinels; absent/foreign/pending pairing, remaining credential, malformed metadata,
links, changed roots, unsafe modes and interrupted deletion refuse or remain
incomplete. Native action requires explicit confirmation and disconnected idle
state, cannot reconnect or register services, and removes visible cached contents
only on success. All tests use new disposable synthetic directories. Actual user
data deletion requires action-time authorization and is not executed by this work.
Research package licenses and Apple rules from read-only primary sources, record
concrete obligations and uncertainties, and do not infer account or startup grants.

### Inert Mac review-package assembly

Continue decision 059 with a public packaging CLI that assembles an existing unsigned
Mac client/helper, exact compiled host resources, a hashed standalone Node closure,
pinned Codex executable/licenses and compiled guardian into a fresh destination.
Require explicit client/helper/team/group configuration matching input bundle IDs.
Generate the identical client/helper host configuration and exact bundle-local
LaunchAgent plist required by MacHostBundle. No caller-selected startup arguments,
KeepAlive, automatic activation or network-policy expansion are added.

Vendor independence: runtime payloads remain at the existing optional adapter seam;
portable core is unchanged. Modularity: only composition assembles independent
artifacts; inputs retain their manifests, licenses and origin hashes. No donor,
voice companion, code-mode helper or cloud service is included. Speed: three warm
assembly samples of the real local payload target p95 <=15 seconds, excluding
compilation, signing and provider latency; no runtime or idle work is introduced.
Installation/removal: copy only into one new directory, reject links, path escapes,
input changes and preexisting destinations, clean only the exact directory created
by a failed invocation. Never overwrite source apps. Run native inspection tools
with an empty environment; inspect metadata only. A review bundle is explicitly
unapproved for execution/distribution: it has no invented profiles and is not
signed, installed, launched, uploaded or registered. Existing signatures cannot
establish the final required code identities after relocation.

Acceptance: CLI red-to-green on synthetic Mac input bundles; exact paths, both host
manifests and LaunchAgent definition; all resource/runtime/notice hashes preserved;
preexisting destination, invalid identity, unexpected/missing files, tampered hash,
linked input, and changed-source refusal. Assemble actual local engineering bytes
without execution and inspect the complete resulting file/mode manifest. Record
missing exact signing/profiles and Codex static notice closure as concrete remaining
work; the assembler cannot waive them or claim installed-product acceptance.

### Fixed OpenSSL configuration for packaged Node launches

Decision 059 covers correcting the concrete sandbox prerequisite recorded in
`sandbox.md`: the successful probe supplies a fixed local Node OpenSSL configuration,
while production host and private worker launch omitted it. Package the same
`nodejs_conf = fort_node` empty Node section in the executor artifact, copy it into
the host resource root, and pass the exact packaged path to both Node invocations.
Reject missing, unsafe or altered configuration during program/worker preparation;
a later malformed-file change must fail launch without executing the runtime.
Do not inherit machine OpenSSL variables, change global configuration, enable legacy
algorithms, relax sandbox/signature checks, or add caller-selected Node flags.

Vendor independence: Node remains the accepted runtime; the correction adds no AI,
hosting or account dependency. Modularity: the optional executor carries its own
configuration; host composition copies that resource, core contracts remain unchanged.
Speed: existing <=50 ms native metadata and <=2 s synthetic private executor p95
budgets remain; measure 20 isolated samples separately from builds/provider latency.
Installation/removal: the two tiny files live inside existing removable artifacts;
no machine files, credentials, account connections or startup registration change.

Acceptance: artifact manifest/content checks, native missing/link/mode/size/content
refusal, compiled executor late-mutation startup refusal with owned process absence,
and native service late-mutation refusal before listener/runtime availability.
Retain the complete synthetic native journey and verify final Mac review composition.
Use fresh temporary fixtures only; signature callbacks remain explicitly synthetic.
Source/build/fixture evidence cannot establish actual sandboxed provider or installed
TestFlight acceptance. Alternative ambient defaults already failed the sandbox proof;
fixed packaged configuration removes that prerequisite without expanding authority.
Exact paths for this slice are the executor/host builders and their tests, executor
contract fixture and private launch implementation, MacHostProgram/Bundle/AgentService
and associated native program/journey tests, Mac assembly/test, this spec, ledger,
current review and `node-openssl-configuration.md` evidence. Roll back this local
slice without touching the retained earlier packages or any owner data.

### Read-only Codex third-party attribution inventory

Continue the existing distribution audit with a checksum-bound source inventory
for the already selected binary/source commit. Preserve registry and exact upstream
notice provenance, source-header evidence and later-source comparisons separately.
The full lock graph includes build/test/other-platform packages and must not be
called a binary SBOM. Candidate collection is not final redistribution clearance.
No source/dependency execution, runtime replacement, feature activation or license
change is authorized or needed for this read-only audit.

Vendor independence and modularity: only optional-runtime attribution evidence changes;
core/client contracts and dependencies remain untouched. Speed: no runtime cost; bound
research downloads to 1 GiB, each archive to 50 MiB, and each selected text to 2 MiB.
Installation/removal: keep downloaded material in a task-owned review archive, never
install it or copy unknown code into an application. Verify hashes and archive membership;
record missing texts, source-availability obligations and limits honestly. Existing
owner direction covers this evidence refinement; no new agreement or generic legal
approval is inferred. Rollback removes only these evidence changes, preserving the
original notices and exact selected runtime. Relevant paths: the two audit evidence
files below, this spec, v1 ledger and current semantic-review record.

### Offline deletion of exact stopped-owner history

Continue the adopted explicit-history-removal scope with a provider-neutral offline
ledger operation, tested only on disposable synthetic installations. The caller must
supply the exact installation, current recovery epoch, disconnected device and a
fresh deletion ID from the explicit confirmation. This is a maintenance interface,
not an HTTP route, CLI, installation capability or substitute for native confirmation,
startup-unregistration, process-absence and credential-removal checks. Production
native history deletion remains unavailable until those lifecycle boundaries are wired.

Vendor independence: no provider runtime, account, SDK or tool is constructed. Preserve
runtime identity, bindings and settings. Modularity: keep the operation with the SQLite
adapter; erase messages, runs, receipts, execution contexts, read positions, approvals,
cancellation records, occurrence history, events and outbox in one transaction. Retain
agents/bindings, conversation definitions and owner notes, skills, routine/schedule
revisions and schedule checked-through watermarks. Require every saved schedule to be
paused before deletion: removing shared occurrence deduplication while another schedule
lags could otherwise replay old work. Re-enabling already requires a fresh preview and
advances the schedule watermark through the existing public contract. Unknown schema/entities/metadata,
active credentials/setup, stale socket ownership, wrong identity or an occupied ledger
refuse before deletion. Rotate the recovery epoch in the transaction so old pending
commands cannot replay; retain an exact body-free receipt for idempotent cleanup retry.
A different request cannot reuse the old epoch. No deletion starts pending work.

Speed: one bounded 256 MiB ledger/sidecar set, owner-private canonical directory and
regular single-link files, exclusive SQLite lease, bulk deletes and no runtime process.
Measure 20 warm deletion/compaction samples at 10,000 synthetic 1-KiB message records
with p95 <=2 seconds on the recorded Mac/Node toolchain; fixture creation is excluded.
Preserve schedule progress to prevent erased occurrences from repeating. Test public
service setup/send/disconnect, offline deletion, reopen/re-pair, old-epoch rejection and
fresh sends; corruption and filesystem negative controls use synthetic fixture mutation.

Installation/removal: keep is unchanged by default. Secure-delete, checkpoint and vacuum
remove records from the live SQLite files, without promising forensic erasure of SSDs,
backups or filesystem snapshots. Failures after the atomic logical commit are reported
as incomplete cleanup; retry the exact deletion receipt. Never traverse Runtime/provider
files, delete app files, unregister startup or remove credentials here. Pin filesystem
identity before mutation; the local cooperative workflow does not claim protection
against a hostile same-UID writer. Rollback removes this new unused interface/tests;
no real data is touched. Existing implementation authority covers this bounded slice.
Exact paths: the new service operation/test and evidence below, this spec, v1 ledger
and current Ponytail review. Installed/native lifecycle and physical acceptance remain
separate gates; no new persistent or destructive action against owner data is authorized.

## Implementation paths

- `successor/service/delete-history.ts`
- `successor/service/delete-history.test.ts`
- `successor/evidence/2026-10-17-node-foundation/host-history-deletion.md`

- `successor/evidence/2026-10-17-node-foundation/codex-third-party-audit.md`
- `successor/evidence/2026-10-17-node-foundation/codex-third-party-audit.json`

- `successor/evidence/2026-10-17-node-foundation/node-openssl-configuration.md`

- `successor/evidence/2026-10-17-node-foundation/mac-review-package.md`

- `successor/packaging/mac-application.test.ts`

- `successor/packaging/mac-application.ts`

- `successor/evidence/2026-10-17-node-foundation/distribution-requirements.md`

- `successor/evidence/2026-10-17-node-foundation/saved-history-deletion.md`

- `successor/apple/FortKit/Tests/FortKitTests/SavedHistoryRemovalTests.swift`

- `successor/apple/FortKit/Sources/FortKit/SavedHistoryRemoval.swift`

- `successor/evidence/2026-10-17-node-foundation/owner-reinstall.md`
- `successor/service/owner-repair.test.ts`
- `successor/evidence/2026-10-17-node-foundation/owner-repair.md`
- `successor/service/owner-resume.test.ts`
- `successor/evidence/2026-10-17-node-foundation/owner-resume.md`

- `specs/README.md`
- `docs/architecture/decisions/060-required-ponytail-review.md`
- `docs/architecture/reviews/ponytail/baseline-20261018.md`
- `docs/architecture/reviews/ponytail/current.md`
- `ops/architecture/validate-reviews.mjs`
- `ops/architecture/validate-reviews.test.mjs`
- `successor/README.md`

- `successor/evidence/2026-10-17-node-foundation/native-removal-composition.md`
- `successor/evidence/2026-10-17-node-foundation/native-host-composition.md`
- `successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift`
- `successor/tsconfig.context.json`
- `successor/packaging/host-resources.test.ts`
- `successor/packaging/host-resources.ts`
- `successor/host/main.mjs`
- `successor/runner/private-codex-plan.ts`
- `successor/runner/private-codex-plan.test.ts`
- `successor/evidence/2026-10-17-node-foundation/private-codex-plan.md`
- `successor/evidence/2026-10-17-node-foundation/codex-auth-source-audit.json`
- `successor/evidence/2026-10-17-node-foundation/private-codex-executor.md`
- `successor/evidence/2026-10-17-node-foundation/artifact-build-environment.md`
- `successor/host/owned-core.ts`
- `successor/host/owned-core.test.ts`
- `successor/host/fixtures/owned-core.mjs`
- `successor/evidence/2026-10-17-node-foundation/owned-core.md`
- `successor/host/mac-process-guardian.swift`
- `successor/contracts/owner-recovery.ts`
- `successor/contracts/owner-setup.ts`
- `successor/service/owner-setup.test.ts`
- `successor/evidence/2026-10-17-node-foundation/owner-setup.md`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-pipe.md`
- `successor/apple/FortKit/Sources/FortKit/OwnerSetup.swift`
- `successor/apple/FortKit/Tests/FortKitTests/CoreSetupTests.swift`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-native.md`
- `successor/apple/FortKit/Tests/FortKitTests/MacOwnerSetupTests.swift`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-xpc.md`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-xpc.json`
- `successor/apple/FortKit/Sources/FortKit/OwnerSetupPreparation.swift`
- `successor/apple/FortKit/Sources/FortKit/OwnerSetupFiles.swift`
- `successor/apple/FortKit/Tests/FortKitTests/OwnerSetupPreparationTests.swift`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-preparation.md`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-cancellation.md`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-cancellation-pipe.md`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-cancellation-native.md`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-cancellation-xpc.md`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-cancellation-xpc.json`
- `successor/apple/FortKit/Tests/FortKitTests/OwnerSetupCancellationTests.swift`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-recovery.md`
- `successor/apple/FortKit/Sources/FortKit/OwnerSetupView.swift`
- `successor/apple/FortKit/Tests/FortKitTests/OwnerSetupModelTests.swift`
- `successor/evidence/2026-10-17-node-foundation/owner-setup-model.md`
- `successor/apple/FortKit/Sources/FortKit/MacAgentEndpoint.swift`
- `successor/apple/FortKit/Sources/FortKit/MacAgentStorage.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacAgentStorageTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-agent-storage.md`
- `successor/apple/FortKit/Tests/FortKitTests/MacAgentEndpointTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-agent-boundary.md`
- `successor/apple/FortKit/Sources/FortKit/MacCoreHost.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacCoreHostTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-core-host.md`
- `successor/evidence/2026-10-17-node-foundation/mac-core-host.json`
- `successor/apple/FortKit/Sources/FortKit/MacHostStartup.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacHostStartupTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-host-startup.md`
- `successor/evidence/2026-10-17-node-foundation/xpc-crash-recovery.md`
- `successor/evidence/2026-10-17-node-foundation/xpc-crash-recovery.json`
- `successor/service/private-socket.ts`
- `successor/evidence/2026-10-17-node-foundation/private-socket-recovery.md`
- `successor/host/fixtures/executor-supervisor.mjs`
- `successor/evidence/2026-10-17-node-foundation/executor-parent-exit.md`
- `successor/host/fixtures/core-supervisor.swift`
- `successor/host/guardian.test.ts`
- `successor/evidence/2026-10-17-node-foundation/parent-exit-guard.md`
- `successor/evidence/2026-10-17-node-foundation/parent-exit-guard.json`

- `successor/apple/FortKit/Sources/FortKit/MacOwnerControl.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacOwnerControlTests.swift`
- `successor/evidence/2026-10-17-node-foundation/xpc-owner-recovery.md`
- `successor/evidence/2026-10-17-node-foundation/xpc-owner-recovery.json`

- `successor/evidence/2026-10-17-node-foundation/probe-phases.md`
- `successor/evidence/2026-10-17-node-foundation/probe-phases.json`

- `successor/apple/FortKit/Tests/FortKitTests/CoreRecoveryTests.swift`
- `successor/host/fixtures/owner-core.mjs`
- `successor/evidence/2026-10-17-node-foundation/private-owner-recovery.md`

- `successor/service/owner-recovery.test.ts`
- `successor/evidence/2026-10-17-node-foundation/owner-recovery.md`

- `successor/apple/FortKit/Sources/FortKit/DeviceConnection.swift`
- `successor/apple/FortKit/Sources/FortKit/DeviceStorage.swift`
- `successor/apple/FortKit/Tests/FortKitTests/DeviceConnectionTests.swift`
- `successor/evidence/2026-10-17-node-foundation/device-endpoints.md`

- `successor/apple/FortKit/Sources/FortKit/MacHostChannel.swift`
- `successor/packaging/xpc-core-service.swift`
- `successor/packaging/xpc-core-client.swift`
- `successor/evidence/2026-10-17-node-foundation/xpc-core.md`
- `successor/evidence/2026-10-17-node-foundation/xpc-core.json`

- `successor/apple/FortKit/Sources/FortKit/MacCoreProcessTransport.swift`
- `successor/apple/FortKit/Tests/FortKitTests/CoreProcessTests.swift`
- `successor/host/fixtures/native-core.mjs`
- `successor/host/fixtures/native-channel.mjs`
- `successor/evidence/2026-10-17-node-foundation/native-core-process.md`

- `successor/host/private-channel.ts`
- `successor/host/private-channel.test.ts`
- `successor/host/fixtures/private-core.mjs`
- `successor/evidence/2026-10-17-node-foundation/private-channel.md`

- `successor/apple/FortKit/Sources/FortKit/MacXPCTransport.swift`
- `successor/apple/FortKit/Sources/FortKit/MacHostProcess.swift`
- `successor/apple/FortKit/Sources/FortKit/MacHostRemoval.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacHostRemovalTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-host-removal.md`
- `successor/apple/FortKit/Tests/FortKitTests/MacHostProcessTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-host-process.md`
- `successor/evidence/2026-10-17-node-foundation/native-xpc.md`
- `successor/evidence/2026-10-17-node-foundation/native-xpc.json`

- `successor/apple/FortKit/Sources/FortKit/FortTransport.swift`
- `successor/apple/FortKit/Tests/FortKitTests/TransportTests.swift`
- `successor/evidence/2026-10-17-node-foundation/native-transport.md`

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
- `successor/runner/guarded-codex-process.ts`
- `successor/runner/owned-process.ts`
- `successor/runner/owned-process.test.ts`
- `successor/runner/guarded-codex-process.test.ts`
- `successor/runner/fixtures/guarded-codex-host.mjs`
- `successor/runner/fixtures/guarded-codex-runtime.mjs`
- `successor/evidence/2026-10-17-node-foundation/guarded-codex-process.md`
- `successor/runner/executor-wire.ts`
- `successor/runner/executor-worker.ts`
- `successor/runner/executor-worker.test.ts`
- `successor/runner/executor-models.test.ts`
- `successor/evidence/2026-10-17-node-foundation/executor-models.md`
- `successor/runner/fixtures/owned-executor.mjs`
- `successor/evidence/2026-10-17-node-foundation/executor-worker.md`
- `successor/runner/executor-process.ts`
- `successor/runner/codex-executor.ts`
- `successor/apple/FortKit/Sources/FortKit/MacHostBundle.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacHostBundleTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-host-bundle.md`
- `successor/apple/FortKit/Sources/FortKit/MacHostControl.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacHostControlTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-host-control.md`
- `successor/apple/Apps/FortApp.swift`
- `successor/runner/codex-executor.test.ts`
- `successor/runner/fixtures/codex-executor-worker.mjs`
- `successor/runner/fixtures/codex-executor-runtime.mjs`
- `successor/runner/fixtures/codex-executor-host.mjs`
- `successor/evidence/2026-10-17-node-foundation/codex-executor.md`
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
- `successor/apple/FortKit/Sources/FortKit/MacAgentService.swift`
- `successor/apple/FortKit/Sources/FortKit/MacHostProgram.swift`
- `successor/apple/FortKit/Tests/FortKitTests/MacHostProgramTests.swift`
- `successor/apple/Host/FortHost.swift`
- `successor/apple/project.yml`
- `successor/apple/FortSpec058.xcodeproj/project.pbxproj`
- `successor/apple/FortSpec058.xcodeproj/xcshareddata/xcschemes/FortHost.xcscheme`
- `successor/evidence/2026-10-17-node-foundation/mac-host-program.md`
- `successor/apple/FortKit/Tests/FortKitTests/MacAgentServiceTests.swift`
- `successor/evidence/2026-10-17-node-foundation/mac-agent-service.md`
- `successor/packaging/codex-executor-artifact.ts`
- `successor/packaging/codex-executor-artifact.test.ts`
- `successor/packaging/codex-executor-artifact.macos.test.ts`
- `successor/packaging/fixtures/executor-contract.mjs`
- `successor/tsconfig.codex-executor.json`
- `successor/evidence/2026-10-17-node-foundation/codex-executor-artifact.md`
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
- `successor/APPLE-HOST-ACTIVATION.md`
- `successor/TESTFLIGHT-DELIVERY-20261017.md`
- `successor/packaging/vendor/codex-0.157.1/LICENSE`
- `successor/packaging/vendor/codex-0.157.1/NOTICE`
- `successor/evidence/2026-10-17-node-foundation/codex-bundle-version.md`
- `successor/evidence/2026-10-17-node-foundation/codex-package-audit.json`
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
