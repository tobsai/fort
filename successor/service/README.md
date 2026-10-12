# Stage 1 fixture service

Newly authored code; no donor or historical Fort implementation is imported.
Use exactly `/opt/homebrew/opt/node@26/bin/node` v26.5.0. The dependency list is
Node built-ins only: `node:sqlite`, `node:http`, `node:crypto`, `node:fs`,
`node:path`, `node:url`, `node:child_process`; tests also use `node:test`,
`node:assert/strict`, `node:os`, and `node:fs/promises`. No package resolution,
install scripts, runtime download, provider login or network dependency exists.

The test entrypoint is:

```sh
/opt/homebrew/opt/node@26/bin/node --test successor/service/service.test.ts successor/service/synthetic-runner.test.ts successor/runner/fixture.test.ts successor/runner/codex.test.ts successor/runner/synthetic-transport.test.ts
```

The outer execution sandbox may require permission for ephemeral loopback tests.
The tests close their listeners and remove their temporary synthetic ledgers.
`crash-host.ts` is an IPC-only crash-test harness, not an installed service.

The trusted local harness API is `createFixtureService({dataDirectory,
autoRun:false})`, `await service.start()`, `service.issueEnrollment()`,
`await service.dispatchNextFixture()`, and `await service.close()`. `start()`
binds an ephemeral port on 127.0.0.1 only and sets `service.baseURL`. Set
`autoRun:true` explicitly for native fixture checks. Code issuance returns a
five-minute, single-use enrollment code. Each enrollment creates a separately
revocable 30-day device credential. Only token/code hashes are persisted.
`now` and `checkpoint` options are trusted fixture-test injection seams, not HTTP
input. Do not log the issued codes, tokens, or native test config.

Use a fresh explicit absolute directory with service-only access. Startup refuses
unrelated filenames, symlink roots/ledger files and incompatible existing files.
SQLite uses FULL synchronous durability, WAL, foreign keys, and an exclusive
connection lock for one active writer. Acceptance atomically records the command,
exact input digest/binding, message, run, outbox, event and original receipt.
Stored acceptance snapshots remain immutable. Provider-style start and terminal
evidence commits before HTTP snapshots or replay can expose it.

Fixture dispatch uses an attempt ID, token hash, generation and recorded lease.
The lease never authorizes retry. A claimed attempt found on restart becomes
`outcomeUnknown` and blocks later queued turns. There is no automatic retry/reconciliation. Uncertain work stays blocked until
the owner explicitly closes it without retry. A committed terminal result survives
restart, and the next queued turn can then run. SQLite locks release on SIGKILL.

Current tests include real HTTP auth, one-use/expired enrollment, revocation,
expiry, wrong installation, forged tokens, cross-origin denial, exact UTF-8,
12 concurrent duplicate sends, a 65,536-byte Unicode input and one byte over,
7 acceptance failure points, stable restart/replay, read scope/monotonicity,
fresh data-root refusal, single-writer exclusion, and actual SIGKILL after claim,
after start, before terminal commit and after terminal commit. These are bounded
T01/T02/T05/T13/T17 subsets, not full acceptance of those requirements.

Official provider execution remains disabled. The fixture worker has a fixed
model, no tools or credentials, an explicit two-variable environment, pipe-only
stdio, and Node permission restrictions. Synthetic canaries verify denied reads
of owner-token/ledger/sibling/provider/home files and symlinks, denied child
launch/network access, absent ambient environment and no inherited descriptor.
Node's permission model is not an OS security boundary for hostile code. A local
`sandbox-exec` probe required outer sandbox escalation; a broad read profile
started Node, but a narrow filesystem profile aborted. No enforceable provider
OS sandbox, Keychain boundary, full IPC boundary or T07 signoff was established.

Remaining service gates: OS runner isolation and provider conformance; full
production role/scope auth matrix; production runner transport and enrollment
(the synthetic receipt/event deduplication increment below is now tested);
diagnostic retention of rejected late events; agent/binding changes;
advanced provider recovery, restore and encrypted backups; real-provider approval/cancellation;
transcript pagination/SSE and large-history storage; TLS/physical-device setup;
minor-version compatibility. Node executes the TypeScript via stripping; the
coordinating workstream separately provisions strict TypeScript checking.

## Focused fixture controls increment

Trusted local `fixtureBehavior:'approval'` requests a fixed reply approval.
`fixtureBehavior:'hold'` is a cancellation-test fixture. Ordinary message text is
never interpreted as a control or fixture selector. The fixture's declared pause
contract exits without performing the reply step; a pending gate survives restart
and continues blocking later work. Only the exact approved fixture continuation
may resume under the same attempt with a fresh fencing generation/token. This
is not a claim that an official provider can safely resume after process loss.

Approval decisions are owner-only, compare-and-set, action/attempt/binding/scope
bound and expiry checked. Reject, expiry, or cancellation of the effect-free
parked fixture produces no reply. Active cancellation commits a request first,
then records cancellation only after process termination evidence. Unknown
outcomes can be explicitly closed by the owner without replay; the original
receipt, transcript and uncertainty remain durable.

Additional behavior tests cover two-device approve/reject races, forged authority,
parked restart/barrier/continuation, stale approvals, queued cancellation, active
termination, and owner-audited uncertainty closure without retry. Strict TypeScript
checking is now provisioned separately by the coordinating workstream and passes.
Full hostile-provider isolation, real provider action approvals/cancellation and
provider-specific safe resumption remain unproved and disabled.

## Reviewed authorization race regression

The independent fixed-snapshot review demonstrated that a partially uploaded
submission could commit after another device revoked its credential. All owner
mutations now execute through `ownerTransaction`, which rechecks the credential
inside the same SQLite transaction as the command. Body parsing happens first;
authentication before reading the body is only an early rejection, never commit
authority. This applies to submit, read, revoke, approval resolution, cancellation
and uncertainty closure. Twelve real HTTP regressions cover revocation and expiry
while each route's body is incomplete; all return 401 without mutating state.
The combined fixture suite passes 45 tests, and strict TypeScript checking passes.

## Synthetic runner bridge

`1804345` adds an explicitly selected `createFixtureService({dataDirectory,
syntheticRunner:true})` mode for a **fresh temporary ledger**. The selection is
persisted; reopening with the ordinary fixture mode is refused, and
`dispatchNextFixture()` cannot launch the old fixture-process path in this mode.
The native v1 wire schema and fixture branding remain unchanged. No native UI
session or real provider is needed for these tests.

Trusted local harness methods `enrollSyntheticRunner('fixture-local')` and
`revokeSyntheticRunner(sessionId)` supply/revoke synthetic authority. There is no
HTTP enrollment shortcut and no provider identity lookup. Session credentials and
per-dispatch capabilities are random, only their hashes persist, and neither
credential class is accepted as an owner-device credential. Session rows bind
installation, recovery epoch, host, expiry and revocation; every runner command
authenticates inside its ledger transaction after body parsing. Existing device
tokens cannot authorize runner commands.

Only this selected mode enables the private fixture POST routes:

| Route | Authenticated command and result |
| --- | --- |
| `/fixture/runner/claim` | Exact protocol/installation/epoch/host/session scope; returns one recorded synthetic dispatch or null. A lost claim response does not authorize another dispatch |
| `/fixture/runner/events` | Same scope, exact dispatch capability and normalized evidence; returns a committed sequence/cursor receipt. Identical canonical replay is harmless; changed replay or a gap conflicts |
| `/fixture/runner/status` | Same scope, attempt ID and capability; returns the current run state, including an owner's cancellation request |

All routes require the runner bearer credential and installation header. Loopback,
no Origin, a copied session ID, or provider text grants no authority. Origin-bearing
requests are denied. Raw provider wire types stay outside the public native API.

Acceptance now also stores the immutable binding in the same transaction. Claims
copy that binding and accepted text, preserve its digest, and record an attempt,
generation, thread, session and 40-second lease before returning work. Bridge events
must match all recorded correlation, including selected model and runtime version.
The first valid start pins the provider turn ID. Only the next exact sequence is
new evidence; terminal/closed/unknown attempts are permanently fenced. Authentication
is still required for harmless duplicates after restart. Recovery-epoch changes
invalidate old session authority even when a request copies current identifiers.

Event schema checks are closed and bounded: 4,096 sequence positions, 1,000,000
encoded evidence bytes per attempt, 65,536 cumulative delta bytes, 65,536 final
text bytes and 128 unique final items. Unknown fields, malformed strings and coerced
array/object status/phase labels are rejected. Progress remains private evidence.
Commentary is preserved in private terminal evidence; only final/unreported-phase
assistant text enters the canonical transcript. All messages, run/outbox state,
ordered public events and the evidence receipt commit together. No successful
reply or model attestation is inferred from an interrupt ACK, timeout or exit-zero.

Lease expiry is materialized on authenticated owner observation or runner claim/
status, and service restart fences claimed work immediately. Neither path requeues
it. Explicit owner closure can release later work while retaining the original
uncertainty. Rejected late evidence does not modify the ledger; broader diagnostic
retention remains open. There is no replay/resume launch command in this increment.

The [new tests](synthetic-runner.test.ts) exercise actual HTTP/SQLite transactions,
partial-upload expiry/revocation, canonical/conflicting replay, model/runtime binding,
multi-message rollback, lost acknowledgements, cancellation and closure races,
synthetic deadlines, seven service-kill points and five runner-kill/exit cases.
The complete backend suite now passes **116 tests**, including **41 bridge tests**
and **8 transport tests**, plus strict TypeScript checking. See the
[retained evidence](../evidence/2026-10-12-synthetic-runner-bridge.md).

This authenticates synthetic fixture events; it does not enroll a production host,
establish TLS, sandbox an agent or prove official-provider/subscription feasibility.
No live runtime, provider credentials, native UI, deployment or persistent host
setting is touched. Before real provider execution, implement and prove the selected
OS/credential boundary, exact executable/schema conformance and a production
runner lifecycle, then obtain provider-native authentication in that environment.
