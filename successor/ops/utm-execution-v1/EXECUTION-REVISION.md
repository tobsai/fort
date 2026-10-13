# UTM source preparation revision R1

Status: **source preparation, execution disabled, independent review pending**.
This revision does **not** satisfy the complete-execution-source gate in the
prior packet. It is ready to review as a bounded implementation slice; it must
not be submitted as the final owner execution request. Passing fixtures do not
mean a VM booted or confinement passed.

The fixed design base is `cfea28acd098b1751f71a5f7b927e101211db054`, whose packet
seal is `c3c897a1bfe4ac47e3b061b181b5faa30bdf49ac302c9e455f97ac76b1a6c2d4`.
The independent review at `reviews/utm-packet-cfea28a/REVIEW.md` supplied F1–F3.
Its exact file digest is recorded in `execution-revision.json`. The review and
prior sealed packet are preserved unchanged. This document supersedes only
those three contradictory design rules for this source slice.

## Implemented public seams

| Component | Implemented behavior | Evidence limit |
| --- | --- | --- |
| `serial.py` | Strict bounded UTF-8/LF JSON; duplicate keys, nonfinite numbers, extra responses, EOF and timeouts fail closed; one transmission per command | Actual anonymous socketpair I/O; no UTM PTY |
| `pty.py` | Exact owner 501/mode 0600 character-device identity, nofollow open, fstat comparison, exclusive open, raw mode and identity-checked restoration | Kernel boundary fixtures; no real PTY opened or permissions changed |
| `controller.py` | Exclusive durable consumed-attempt record before start; consecutive sequence and invocation binding; separate grant; lease/deadline classification; stop acceptance and observation; inspection-only restart | Real temporary files and fsync; synthetic systemd/guest facts |
| `evidence.py` | Separate inactive preflight, prepared-runtime controls and live process evidence validation | Facts are supplied by trusted guest-boundary fixtures; no Linux collector yet |
| `ipc.py` | Working root-side Unix connection before/after tool observation and exact denial errno | Real temporary listening/unlistening/removed sockets; tool errno supplied by fixture, not a different UID |
| `seed.py` | Deterministic in-memory meta-data/user-data/network-config preview; locked users, no packages, growth or cloud-init command stages; dormant resource-constrained units | Not an ISO, not booted, no runtime/tool/broker enabled |
| `lifecycle.py` | Exclusive creation of an absent private root; exact inode/device/type/mode/hash manifest; mutable content reconciled only after quiescence; nofollow exact deletion | Only owned temporary trees; host process absence is a supplied fixture observation |

There is no automatic retry, process substitution, alternate image, provider
connection, arbitrary command endpoint or shared-host claim. Fresh direct
construction of `OwnedTree` refuses any pre-existing root, including one left
by an interrupted earlier object. Recovery/adoption needs a separately reviewed
persisted creation receipt; this revision does not invent one.

## F1: phase ordering and live evidence

1. Root-only preflight requires runtime and tool inactive, empty owned cgroups,
   zero NICs/shares and root positive controls for the synthetic protected
   objects, seed, serial devices and working IPC listener.
2. Prepare sequence 0 durably consumes the sole attempt, then starts only the
   fenced runtime barrier. The tool remains inactive. A new invocation identity
   binds the guest boot ID and exact runtime cgroup to the packet/VM binding.
3. Grant sequence 1 requires fresh prepared-runtime controls: UID 2001, expected
   privileges/descriptors/cgroup, successful public provider-canary read and
   still-inactive tool. Only this grant starts the fixed tool.
4. Heartbeats are sequences 2 through 20. A single trusted live capture occurs
   after grant and before Stop; the integrated fixture captures at sequence 20.
   It binds the full attempt identity and current sequence to runtime/tool/
   descendant PID, start ticks, UID, capabilities, no-new-privileges, network/
   mount/IPC namespaces, descriptor classes and exact owned cgroups. Tool and
   descendant must share the tool namespace/cgroup; the runtime is distinct.
   A trusted observation must also prove the descendant's setsid status and
   C01–C08 outcomes. These fields are a collector contract, not proof that the
   collector exists.
5. Capture is single use, at most 500 ms old, and cannot update heartbeat time.
   Stale, replayed, incomplete or changed-runtime observations fail. Missing or
   invalid live evidence leaves `live_evidence_proven=false` even when cleanup
   succeeds. Post-stop emptiness cannot stand in for the missing live snapshot.

The future broker must independently service `tick()` and bound OS calls.
Current deadline checks classify an overrun after the supplied boundary returns;
they cannot preempt a hung external callback. No live backend is wired until
that scheduling and cancellation behavior is implemented and tested.

## F2: Stop has two responses

Control commands use exact `{op, sequence, identity}` fields after prepare.
Stop is sequence 21 and does not refresh the lease.

* `stopping`: exact identity and sequence, after one nonblocking stop submission;
  the guest acceptance operation has a 500 ms budget, the host acknowledgement
  750 ms. This receipt proves request acceptance only.
* `owned-attempt-empty`: the same identity and sequence, with measured stop
  elapsed time and the separate live-evidence flag. Independent observation must
  find both units inactive and every owned descendant cgroup empty within five
  seconds from the original Stop request, including the two-second unit stop
  and three-second observation budget.

The serial client expects exactly these two phases, including coalesced frames.
It observes the channel until the terminal deadline to reject duplicate proof
within that bounded session. A duplicate after that closed observation window
cannot be ruled out; the channel is terminal and cannot send further commands.
Missing, wrong, late, duplicate or truncated terminal evidence leaves the host
quarantined/unknown. No resend or new grant follows. An acknowledged Stop alone
never proves completion. A delayed two-second teardown is covered by a controller
fixture; wire fixtures use shorter declared deadlines to exercise both phases.

Ordinary framing remains 4096 bytes including LF, 8192 pending bytes, 64 control
frames and 262144 aggregate bytes per direction. One request is in flight.
Prepared lease is four seconds; control idle is one second; runtime deadline is
40 seconds. The planned independent runtime watchdog, tool limit and host
45-second session bound still need the unwired runtime/host adapters.

## F3: working listener before denial

C06 requires a bounded successful trusted-root connection to the actual socket
before the tool call and another after it, with unchanged device/inode/owner.
The trusted broker must keep that listener open throughout the observation.
Only explicit `EACCES`/`EPERM`, or deliberately hidden `ENOENT` with these root
existence/identity controls, can pass. Refusal, timeout, missing endpoint,
replacement, failed root connect and successful tool connect fail the canary.
No command or payload is sent. Tests inject the tool result; they establish
acceptance logic, not a real permission denial under Linux.

## Remaining source gates

These are implementation gaps, not missing human approval for ordinary source
work. They prevent calling this a complete execution revision:

* A root serial broker and Linux systemd/proc collector, fixed runtime barrier,
  watchdog and tool/setsid-descendant programs are not wired. Current guest
  observations are explicit test doubles. C01–C11 need real collected receipts,
  including deny syscalls, root target identity, fd classification and cgroup
  cleanup. The trusted collector must derive C06 from `prove_denial`, not from
  an arbitrary success boolean.
* The NoCloud preview cannot guarantee boot suppression of the image's existing
  SSH/getty/update services. It contains no bootstrap or enabled broker and its
  unit commands deliberately exit disabled. Final service masks, seed ownership,
  offline nftables application, user/group setup, read-only seed attachment and
  actual cloud-init module behavior need an inspected image and a later reviewed
  seed revision. No ISO/config.plist is emitted now.
* A Mac collector must correlate exact app signature/PID/start identity,
  virtualization helper, VM UUID, exclusive PTY handles, actual post-registration
  device topology, processes, mounts and app-state paths. TIOCEXCL alone cannot
  detect old competing consumers. Real UTM PTY owner/mode remains unknown.
* Lifecycle observations are trusted inputs, not fresh OS scans. The host adapter
  must bound their age, prove all writers/handles/mounts gone and preserve unknown
  state. Mutable files may change content/size, but inode/owner/type/mode changes,
  new files, hard links or symlinks currently stop cleanup. App-bundle symlinks
  and outside-root app state need separate exact ownership handlers. No TCC,
  protected/shared metadata, broad kill or recursive deletion path exists.
* The Ubuntu-only nine-input downloader/verifier adaptation, bounded archive
  examiner, accepted-member extraction and UTM plist writer are not implemented
  in this directory. The old Lima downloader is not invoked or repurposed.
  The previous packet's pinned inputs, budgets and gate requirements still apply.

## Facts requiring later authorized inspection or execution

The pinned Ubuntu archive is absent and uninspected. Exact member names, hashes,
raw format/layout/label, ARM64 Image header, initrd compatibility and expanded
sizes remain unknown. No alternate name/image/kernel is guessed. Public package
versions in the earlier packet are metadata, not a verified boot filesystem.
The earlier accepted UTM static assessment remains evidence of the app only.

Real NoCloud startup, systemd hardening, UID/namespace/fd/cgroup facts, serial
identity/exclusivity, UTM device readback, the canary matrix, actual budgets and
complete Mac cleanup are **unrun**. T07 and product/provider acceptance remain
open. No real credential, host secret or provider confinement claim is made.

A later exact owner execution request must bind the completed source revision,
seal, independent review, authenticated inputs and remaining runtime gates.
Downloads, UTM launch, provisioning/boot, account work, host security changes,
provider cleanup and deployment were not authorized or performed in this slice.

## Validation and failure record

The final suite and disabled-entry receipt are in `evidence/`; machine-readable
results are in `validation.json`. The suite uses the existing `/usr/bin/python3`
with `-I -B`, no installed package and no environment credential loading.

Red logs preserve expected pre-implementation failures. The initial slice-08 red
run included a test-edit mistake, then a corrected red run was recorded. The
slice-14 red run also contains socket-peer cleanup errors after the absent API
failed. Slice-15's first attempted green run was blocked by the sandbox's Unix
socket policy; the narrowly approved run passed. Slice-17's first attempted green
run rejected the fixture's `/var` symlink as designed; the test now supplies the
canonical path of its own newly created temporary directory. These old failure
logs remain evidence and are not represented as successful checks. Trailing
whitespace in two failed-attempt logs was removed for Git hygiene; original
digests and exact removed bytes are in `evidence/log-normalization.json`.

Only this directory is staged. No original source, old sealed packet, provider
state, repository visibility, remote, main branch or reviewer artifact changed.
