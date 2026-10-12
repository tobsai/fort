# Stage 1 fixture service

Newly authored code; no donor or historical Fort implementation is imported.
Use exactly `/opt/homebrew/opt/node@26/bin/node` v26.5.0. The dependency list is
Node built-ins only: `node:sqlite`, `node:http`, `node:crypto`, `node:fs`,
`node:path`, `node:url`, `node:child_process`; tests also use `node:test`,
`node:assert/strict`, `node:os`, and `node:fs/promises`. No package resolution,
install scripts, runtime download, provider login or network dependency exists.

The test entrypoint is:

```sh
/opt/homebrew/opt/node@26/bin/node --test successor/service/service.test.ts successor/runner/fixture.test.ts
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
role/scope auth matrix; receipt/event ingestion deduplication for reconnecting
runners; diagnostic retention of fenced late events; agent/binding changes;
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
