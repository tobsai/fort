# Process review P2 correction and isolation design

**The reproduced P2 is fixed in `db64b6f1d6a531daab9073c4488c63bcea94e611`.**
The full backend suite passes **145 tests** with strict TypeScript and unchanged
scope checks. Independent re-review is still required; OS confinement and provider
activation remain unaccepted.

Base: `de78b3596cfa95a706a852c1d7f5ebc7e369e573`.
Branch/worktree: `codex/fort-core-openmaus-20261011`,
`/tmp/fort-core-openmaus-20261011` (canonical `/private/tmp/...`).
[Review](../../reviews/process-confinement-de78b35/REVIEW.md) and its original
reproduction are preserved unchanged in parent-owned `reviews/`.

## Reproduction and correction

The unchanged terminal-descendant fixture emitted valid terminal output and exited
zero while a descendant held stdout open. Each receipt took 1.1 seconds; the first
started receipt was lost and its retry succeeded. Both uploads were within the
existing 2-second limit, but the parser waited for them before consuming remaining
stdout. The separate 2-second root-exit timer sealed the codec as disconnected.
Our added default-limit regression reproduced `terminal: unknown`,
`failure: pipeHeldOpen` with observed cleanup and only one start. Four related
slow-upload EOF controls passed in that same red run.

The fix removes the async upload wait and pause/resume coupling from stdout parsing.
Each bounded chunk is parsed synchronously in byte order before returning to the
event loop. Evidence delivery remains a separate serial queue with bounded retries;
terminal evidence still waits for observed process-group/pipe cleanup and all earlier
evidence remains ahead of it. No timeout, byte cap, event cap or retry allowance was
increased. There is no unbounded raw-output buffer or new transport abstraction.

Any burst exceeding the existing evidence queue fails closed and remains unconfirmed;
the parser does not manufacture contiguous ledger acceptance or issue another start.
That conservative overflow behavior now also applies to ordinary rapid progress,
not just a batch released by the codec before start confirmation. This remains a
bounded synthetic harness, not a production progress-stream throughput claim.

Six new process regressions cover the exact default-limit failure, delayed lost
acknowledgement with echo/exit-zero/partial-EOF/pipe-holding descendants, and ordered
progress plus terminal within the cap. The progress case delivers sequences
`1,1,2,3,4`, preserving exact replay and order with two queued progress events.
A seventh regression crosses the authenticated HTTP seam: a started event commits
before its delayed acknowledgement is lost, replay is deduplicated, exact final text
commits once, restart preserves the snapshot, and the later queued message can be
claimed. Genuine missing terminal evidence remains unknown.

## Validation

From `successor/`, using the existing dependencies and no install hooks:

```sh
/opt/homebrew/opt/node@26/bin/node --test service/service.test.ts service/synthetic-runner.test.ts runner/fixture.test.ts runner/codex.test.ts runner/synthetic-transport.test.ts runner/synthetic-process.test.ts runner/confinement-canary.test.ts
/opt/homebrew/opt/node@26/bin/node node_modules/typescript/bin/tsc -p tsconfig.json
```

| Check | Result |
| --- | --- |
| Default-limit reproduction and EOF controls before fix | **1 failed, 4 passed**, exit 1; [raw red log](process-backpressure-2026-10-12/red.log.gz) |
| Process suite after minimal fix | **22 passed**, exit 0; [log](process-backpressure-2026-10-12/first-green.log) |
| Added ordered-progress and durable-bridge cases | **2 passed**, exit 0; [log](process-backpressure-2026-10-12/ordered-bridge.log) |
| Complete final backend suite | **145 passed, 0 failed, 0 skipped**, exit 0; [log](process-backpressure-2026-10-12/node-tests.log) |
| Strict TypeScript | Exit 0, no diagnostics; [log](process-backpressure-2026-10-12/typecheck.log) |
| Fixture syntax and diff whitespace | Exit 0 |
| Donor/specs/native/contracts/codec/transport/service/bridge implementation | Unchanged from `de78b35`; [scope check](process-backpressure-2026-10-12/scope.log) |

The final suite reran the original crash, flood, backlog, cancellation and temporary
native-canary cases. The native diagnostic test still reports the restrictive-profile
SIGABRT and inherited descriptor bypass; its passing test status does not establish
production isolation. Source and retained-log hashes are in
[validation.json](process-backpressure-2026-10-12/validation.json). Whitespace-bearing
raw logs are preserved as deterministic gzip with decompressed hashes. Historical
logs document observed runs, not an independent reconstruction of old source edits.

## Isolation proposal and authority

The [decision proposal](../design/2026-10-12-managed-linux-isolation.md) selects a
dedicated Lima/VZ Linux guest, guest-PID-1-owned attempt units, separate provider-host
and tool scopes, and observed cleanup/restart fencing. It records the inspected Mac
capacity and Docker installation without querying its engine, and pins public Lima
2.2.1 and Ubuntu 24.04/20260926 metadata. The exact proposed future paths, resource
budget, management endpoint, fresh SSH key scope, guest accounts/units, mounts,
cleanup and permission boundaries are listed for review.

Only public release/checksum/package metadata was fetched. No VM package/image was
downloaded, no executable installed or launched for VM management, no VM provisioned,
and no account, service, entitlement, credential or host setting changed. The
proposal is not an executable provisioning bundle; full configuration artifacts,
image-signature verification and any missing dependencies must be sealed before
asking for concrete installation/provisioning approval. Existing tools do not by
themselves satisfy T07.

Unrun: VM boot/systemd validation, managed-unit canaries, actual-launch descriptor
inheritance and pre-ownership crash probes for that design, escaped descendants,
global Keychain/Mach IPC, official runtime/schema/authentication, native UI/device,
self-hosted and release checks. Provider activation remains disabled. No external
agent, real provider, existing secret, sign-in, startup service, persistent permission,
push or deployment was involved. No new approval is requested by this record.
