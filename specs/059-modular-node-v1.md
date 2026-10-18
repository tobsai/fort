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
Reviewed content SHA256: fd2c795ffbd5336831d6bc147117cea6f8119734f650357749ccbb2b75f68cea

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

- `successor/host/mac-process-guardian.swift`
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
