# UTM source preparation revision R2

**Fixed source candidate for independent review; all activation gates remain
false.** This revision completes the requested adapter/program/generation source
seams using synthetic inputs. Actual UTM schema and archive binding remain
unresolved. It is not an accepted execution packet or a working tested VM.

Base source: `5dcc21697a933798cb941977cfff1bf66e68df8a`.
Original inert design: `cfea28acd098b1751f71a5f7b927e101211db054`.
Both prior snapshots and their evidence remain in Git. Reviewer files under
`reviews/` are preserved. Their digests are recorded in `execution-revision.json`.

## Source and evidence boundary

| Source | Behavior now implemented | Validation |
| --- | --- | --- |
| `guest_programs.py`, `guest_entry.py` | Fixed runtime canary/barrier, stale-lease exit, systemd readiness/watchdog notifications, fixed tool deny/positive-control syscalls, one double-fork/setsid descendant, root bootstrap and guest role dispatch | Program logic runs against IO doubles. Actual syscalls/bootstrap never run. |
| `linux.py`, `linux_kernel.py` | Fixed systemd commands and hardening/resource readback; boot/invocation binding; root target controls; proc PID/start/UID/capability/namespace/fd/cgroup collection; journal-bound tool report; independent empty-tree observation | Linux policy uses a raw OS boundary double; parser fixtures exercise real parsing. No `/proc`, systemd, journal or guest filesystem inspection occurs on this Mac. |
| `broker.py` | Separate fixed control/inspection operations, bounded framing/output, timer ticks without input, one terminal Stop response | Fixtures exercise timer expiry, malformed operations and two-phase Stop. |
| `bounded.py` | Sanitized-environment commands, bounded output and wall time, cleanup of only the spawned command group | Source inspected and syntax checked. Live constructor refuses before spawning anything. OS cancellation behavior is unrun. |
| `host.py` | Exact app pin/signature and process-start checks; PTY owner set; zero-network/share topology; resource/device checks; fresh process/handle/mount collection and quiescence interpretation | Host policy/parser fixtures only. Actual collector is disabled. Registry parsing is blocked on an uncached pinned schema, explicitly rejected rather than defaulting to no shares. |
| `host_session.py` | Preflight, prepare, one grant, 19 heartbeats, live evidence, two-response Stop and postflight through attributed channels | Integrated fixture uses two actual anonymous socketpairs, the actual broker/controller/codecs and synthetic guest facts. |
| `config.py`, `seed.py` | In-memory three-file NoCloud candidate containing guarded guest sources, offline bootstrap/unit definitions, boot service masks and explicit zero-NIC/share UTM plist; exact ISO command construction | Synthetic image and UTM schema facts only. No files are emitted into a VM package, no ISO command is run. |
| `archive.py` | Read-only bounded gzip/tar inspection, exact selected member binding, link/special/sparse rejection, hashes, ARM64 header and raw ext4 label check | Generated tar fixtures only. Currently supports an unpartitioned ext4 raw filesystem; other layouts stop for a reviewed decoder. Header checks do not prove filesystem integrity or bootability. |
| `verify-inputs.mjs` | Eight exact Ubuntu/verifier input pins; registry-signature and publisher-checksum verification functions adapted from the earlier reviewed Fort verifier | Six contract fixtures with a declared crypto boundary double. No actual OpenPGP library, registry signature or publisher signature was verified. No automatic import/download entry exists. |
| `controller.py`, `lifecycle.py`, `pty.py`, `ipc.py` | Durable one-attempt fencing, single-use evidence, exact typed termination facts, absent-root ownership, strict PTY checks and working-listener denial controls | Original suite plus reviewer regressions and expanded failure matrix. |

All guest source imports are included in the candidate seed. Guest code is
root-owned and readable by the fixed runtime/dynamic tool; binding data remains
root-only. Bootstrap and controller consumption markers use the persistent guest disk,
not `/run`, so reboot does not create a fresh attempt. Runtime and tool units
have no install/autostart target. Only the
fixed bootstrap can start the root broker. `LIVE_EXECUTION_ENABLED = False`
stops bootstrap, broker, runtime, tool and command-runner construction before
live operations. The public package entry still exits 78; no runtime flag or
environment override enables it. This is a workflow guard, not a security sandbox
against a trusted owner editing or importing source. The candidate includes
cloud-init boot/user/file provisioning directives; those would execute if someone
independently packaged and booted it. The Python gate does not suppress cloud-init
itself. No such package or boot was produced here.

## Reviewer corrections

The independent source review of `5dcc216` found three P2 issues. All four supplied
regressions now pass against the active source. The test copy changes only its
source import location and removes its standalone writer to reviewer-owned
files and normalizes the final blank line; test bodies and assertions remain unchanged.

* **S1:** capture consumption is persisted before asking the guest for evidence.
  A failed canary, malformed snapshot, timeout or persistence error revokes the
  attempt. Recapture and heartbeat cannot restore it, including after reopen.
  Successful cleanup remains a separate fact and never converts failed live
  evidence into success.
* **S2:** normal and abort Stop paths validate exact fields and boolean types.
  Only explicit inactive/inactive/empty values prove termination. Null, numeric,
  string, missing and extra fields quarantine. Valid remaining-process evidence
  continues bounded observation. Transitional systemd states count as active,
  while prepare separately waits for ready state.
* **S3:** process inventory and VM identity validation precede directory creation.
  Invalid input leaves the root absent and corrected construction succeeds.
  Pre-existing roots still cannot be adopted implicitly.

Earlier F1–F3 remain enforced: inactive root preflight, prepared runtime controls,
separate grant, single-use bound live snapshot without lease renewal; prompt
`stopping` plus independent terminal proof within five seconds; root-proven
working IPC listener and explicit tool denial. For asynchronous tool startup,
the broker holds the same listener across grant and live capture, bounded to six
seconds, and uses the shared denial predicate with successful root connections
before and after. Refusal, timeout, removal or successful tool connection fails.

## Bounds and trust

The fixed sequence remains prepare 0, grant 1, heartbeats 2–20 and Stop 21.
Control frames are at most 4096 bytes including LF, pending bytes 8192, frames 64
and aggregate bytes 262144 per direction. There is no resend. Stop acknowledgement
has a 500 ms guest budget and 750 ms host deadline; terminal proof has five
seconds, including stop and observation. The host watches for duplicate terminal
proof until that deadline, then makes the channel terminal.

Preparation lease is four seconds; control idle is one second; runtime lease
age is 1.5 seconds. Runtime/tool limits are 40/38 seconds, runtime watchdog two
seconds, unit stop two seconds and the host session 45 seconds. The broker's
selector ticks at 50 ms and its service has a 120-second cap. Command calls are
bounded individually; aggregate callback overruns fail controller acceptance.
Actual scheduler latency, filesystem stalls and command cancellation are unrun.
A stalled kernel call is not claimed to be preempted by Python deadline checks.

Root observations, synthetic program reports, code provisioning and same-owner
host state are trusted within the declared experiment. Reports are matched to
unit invocation/PID/UID and checked against independently collected process facts.
There is no cryptographic host/guest authentication or shared-host guarantee.
No real provider, credential, host-secret or adversarial VM-escape claim is made.

## Precise outstanding dependencies

[OUTSTANDING-INSPECTION.md](OUTSTANDING-INSPECTION.md) specifies the separate,
currently unauthorized acquisition and read-only inspection step. Two categories
cannot be resolved from the source and synthetic fixtures presently available:

1. The pinned Ubuntu archive and public verifier inputs are absent. Actual member
   names, archive layout, member hashes, kernel/initrd compatibility and filesystem
   label cannot be bound. Signature verification and bounded verifier-module
   extraction/loading require the actual authenticated artifacts. There is no
   download or extraction fallback. A partitioned root would require a reviewed
   decoder after the layout is observed, not invented offsets now.
2. Three files in the already pinned UTM source tree were not cached:
   `UTMConfiguration.swift`, `UTMConfigurationInfo.swift` and
   `UTMRegistryEntry.swift`. Their Git blob IDs and sizes are known. Actual config
   version/information encoding and registry entry/share/bookmark decoding must
   be checked before implementing the final registry reader and accepting the
   generated plist. The Mac adapter refuses a missing registry source.

Synthetic schema version 4 and synthetic archive paths in tests are **test data**,
not evidence about the real UTM or Ubuntu inputs. Actual UTM process/helper paths,
PTY modes, registry/state files, cloud-init behavior and systemd property formatting
remain later runtime observations. Unknown variants stop instead of falling back.

## Lifecycle and execution limits

The collector and policy code are available for review; they have not read live
UTM state. It rejects incomplete scans, wrong process start times, competing PTY
readers and unknown UTM/VZ processes. Removal of registration and VM stop remain
owner-operated facts; no Automation, Accessibility or UTM CLI control is added.
Known lsof empty-result status is accepted only with empty stdout and stderr;
partial/permission-failed scans are errors.

Exact temporary-tree cleanup is fixture-tested. It intentionally rejects app
symlinks, unexpected mutable inodes, new files, protected/shared outside-root
state and interrupted pre-existing roots. These conditions preserve evidence;
they are not accepted cleanup. Actual signed-app symlink and Mac registry formats
must inform any narrowly scoped follow-up cleanup adapter. No broad kill, rm-rf,
TCC/FDA change or adoption by owner UID alone is implemented.

No live adapter, download, mount, UTM/helper, VM, credential operation, security
change, provider call, deployment or push ran. The planned host installation root
remains absent. Independent source acceptance, actual-input binding, exact owner
execution authority, live results and verified cleanup still block execution.
T07 and product/provider acceptance remain open.

## Validation record

`validation.json` records exact suite counts and logs. R2 Python fixtures,
including the four reviewer tests and extended failure matrix, are in
`evidence/r2/full-suite.log`; verifier contract tests are separate. The existing
Python and Node executables were used without installation or package hooks.
The local Unix-socket fixture suite used the same narrowly scoped automatic
approval as R1. Earlier logs remain unchanged except the already documented R1
trailing-space normalization. R2 archive's first attempted green run exposed a
one-byte fixture label-length mistake; the corrected fixture run is retained
separately. No failed check is presented as a runtime pass.
