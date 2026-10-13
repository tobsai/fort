# UTM source preparation revision R3

**Disabled source candidate for independent review.** The five R2 findings are
fixed in source and synthetic fixtures. Live confinement and complete execution
readiness remain unaccepted. Base: `ca29ad4849dc5f0b1c6dee3f6aa344efb374a069`.
Earlier source/design snapshots, logs and reviewer-owned files are preserved.

## Review corrections and their limits

| Finding | Change | Evidence and limit |
| --- | --- | --- |
| STD-1, P1 | Commands never poll/reap the direct child before group cancellation. Linux observes exit with `waitid(WNOWAIT)`; Mac source uses a process-exit kqueue. Cancellation precedes the sole final wait on every path. | Boundary doubles cover exited leaders with descendant-held pipes, output caps, normal completion and cancellation failure. No command child was launched. |
| STD-2, P2 | Count and validate each physical 512-byte USTAR header before interpreting a member. Reject PAX/GNU metadata, sparse/special/link headers before reading their payload. | The supplied 65-PAX fixture and oversized metadata declarations fail. Missing terminators and nonzero trailing data fail. No archive was downloaded. |
| STD-3, P2 | Separate 20 GiB raw and 192 MiB combined auxiliary budgets. Include retained compressed kernel plus decoded output and all other auxiliary members in peak accounting. | The supplied streamed 193 MiB fixture fails. A scaled gzip-kernel overlap fixture fails at its limit and passes with sufficient capacity. Decoded kernel size/hash enter the `Data/` file plan. |
| SPEC-1, P2 | A socket inode string cannot become a journal descriptor. The disabled ARM64 Linux collector duplicates target descriptors through pidfds and inspects kernel family/type, inode, peer path and `SO_PEERCRED`; it checks the root peer's executable, cgroup/start identity and root-owned journal path. | Unattributed, mismatched and management sockets fail; attributed journal fixtures pass. Collector effects are entirely doubled. Actual pidfd permissions and socket-activation credentials remain unrun. |
| SPEC-2, P2 | Runtime performs both fixed nonblocking `/dev/hvc0` and `/dev/hvc1` attempts before readiness. Reports carry ordered, typed denials bound to boot/invocation; root rechecks serial positive controls before grant. | Missing, successful, untyped, reordered or unbound reports fail. The original R2 fixture is frozen with its missing results. Actual runtime UID/device syscalls remain unrun. |

The runner requires a single-threaded caller with default SIGCHLD and exclusive
child-reaping ownership. Keeping its child unreaped reserves the PID/PGID until
cancellation; it never signals the group after its final wait. Deadline/output
failures continue to raise, even after sending cancellation. A signal and leader
wait do not establish that every descendant has exited or that an arbitrary
program could not escape its group. The runner serves fixed trusted inspection
commands; independently observed guest cgroup emptiness and host quiescence
remain separate acceptance requirements. Real Linux/Mac cancellation and timing
are unrun. Kernel stalls are not claimed to be preempted by Python deadlines.

Journal proof accepts the fixed root journald endpoint, with a root journald
peer or root PID 1 systemd peer for socket activation. It rejects unavailable
pidfd duplication, unfamiliar peer paths or process identity, and descriptor
changes during collection. It sends/receives no socket payload. This is trusted
root observation in one disposable guest; it is not a defense against guest root
or a shared-host security claim.

S1–S3 from the previous review remain fixed: durable single-use evidence before
collection, authority revocation on failure, strict boolean Stop evidence and
validation before creating a cleanup root. Earlier F1–F3 retain inactive
preflight, prepared runtime, separate grant, bound live evidence, prompt Stop
acknowledgement plus independent empty-tree proof, and actual temporary-listener
positive controls before/after the fixture denial.

## UTM source and schema

The only new retrieval was three authorized public Swift blobs at
`048ca7498ea3a374439149d51739d94c5300bcda`. Their decoded sizes, Git blob identities
and SHA-256 values are verified; legal notices and exact bytes are retained.
[UTM-SOURCE.md](evidence/r3/UTM-SOURCE.md) records provenance, cached-source reuse
and precise schema conclusions. There was no additional binary/archive download.

`utm_schema.py` binds version 4, required `IconCustom: false` and exact information
fields. The in-memory generator names all boot/storage data inside `Data/`.
Raw registry parsing requires the exact VM/package identity, rejects every
share or external drive before UTM can filter invalid bookmarks, and requires
independent no-UI/no-mount resolution of a nonempty package bookmark. The Mac
adapter now consumes that raw schema instead of a caller's preclassified list.
It requires persisted/in-memory agreement; the live capture/resolution provider
is still unavailable by default. No preferences, bookmark targets or live app
state were accessed. Python schema fixtures are not Swift/app decoder evidence.

## Source and execution boundary

The source includes fixed guest programs, guarded bootstrap, Linux/Mac collectors,
a timed two-channel broker, host session, in-memory NoCloud/plist generation,
archive inspection and verifier contracts. Existing fixture coverage of those
seams remains. Host-session fixtures use two real anonymous socketpairs with
synthetic guest facts; they do not run the real broker's OS loop or a VM.

`LIVE_EXECUTION_ENABLED = False` remains literal and has no environment override.
The public entrypoint exits 78. The gate prevents the supported live entrypoints;
it is a workflow control, not a sandbox against a trusted owner modifying source.
Independently packaging and booting the candidate would still run cloud-init
boot/user/file directives. No package, ISO, mount, app or guest boot was produced.
Root-owned guest code is readable by the synthetic runtime/tool; binding data and
persistent consumption markers remain root-only. Runtime/tool units have no
autostart target. There is no live provider or real credential in the experiment.

Bounds remain: 4096-byte frames including LF; 8192 pending bytes; 64 frames and
262144 bytes per direction; no resend. Prepare 0, grant 1, heartbeats 2–20 and
Stop 21 are fixed. Guest Stop acknowledgement budget is 500 ms, host 750 ms;
terminal proof has five seconds, with duplicate observation to that deadline.
Prepare lease is four seconds, control idle one second, runtime lease freshness
1.5 seconds, runtime/tool limits 40/38 seconds, runtime watchdog two seconds,
unit stop two seconds and host session 45 seconds. Command cleanup allows one
additional second to reap the leader; it does not convert errors into success.
The runtime's two serial attempts must finish within 250 ms before readiness.

Archive budgets count physical tar entries and logical extraction overlap.
They do not claim an RSS bound for this in-process parser. Actual large-input
acquisition/decompression needs the separately reviewed bounded worker described
in [OUTSTANDING-INSPECTION.md](OUTSTANDING-INSPECTION.md). Only an unpartitioned
ext4 root is supported; unknown layouts stop for an observed, reviewed decoder.
Header checks alone do not prove filesystem integrity or bootability.

## Remaining gates

- Authenticated Ubuntu and verifier artifacts, exact real members/format/label,
  bounded acquisition and verifier-module selection/loading are still absent.
  The six JavaScript contract fixtures use a supplied crypto double.
- Fresh live registry/bookmark capture, actual UTM process/helper and PTY
  attribution, cloud-init/systemd/proc/journal behavior and observed cancellation
  need a later authorized trial. Unknown facts fail closed.
- App symlinks, outside-root registry cleanup and interrupted pre-existing roots
  remain refused by generic lifecycle code. Owner UID alone cannot authorize
  cleanup; no broad kill, rm-rf, policy change or implicit adoption was added.
- Independent acceptance of this fixed source, actual-input binding, exact owner
  execution authority, live results, verified cleanup and T07 are still open.

## Validation

`validation.json` records exact commands, counts and evidence hashes. R3 retains
all five expected red reviewer failures and the passing original/new regressions.
Copied reviewer test bodies and the frozen R2 Linux fixture match the originals'
ASTs. Only import locations and standalone evidence writers were adapted.

Two additional failed fixture-development runs are retained: the first scaled
kernel fixture's actual overlap was below its proposed rejection threshold, and
a socket double returned Python's enum rather than the integer returned by the
kernel API. The corrected fixtures pass; neither failure involved a live effect.
The initial network attempt failed DNS in the sandbox, then the explicitly
scoped public-text retry succeeded. Existing Python/Node tools were used without
installation or package hooks. No current failing check is hidden as a pass.
