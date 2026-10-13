# macOS memory-control decision after the R2 diagnostic

**Recommend a small, explicitly approved macOS correction: keep the existing parent-enforced RSS stop thresholds; remove the incompatible child `RLIMIT_RSS` assignment from the macOS policy. Do not silently catch its failure, raise the number, or retry acquisition.** This preserves the documented sampled-RSS behavior, not a hard memory ceiling. The removal changes an approved control and therefore needs an owner decision before implementation or another attempt.

## What the evidence establishes

The [R2 outcome](../successor/evidence/preexec-diagnosis-2026-10-13/R2-LIVE-OUTCOME.md), [raw events](../successor/evidence/preexec-diagnosis-2026-10-13/r2-live-execution.json) and [validation](../successor/evidence/preexec-diagnosis-2026-10-13/r2-live-validation.json) agree: core and CPU limits applied in the temporary child; setting resource 5 to 268,435,456 bytes failed with `ValueError`; file and descriptor limits were never reached. The child was reaped and parent limits were unchanged. This review verified the diagnostic's SHA-256 against the retained evidence and the worker source against R6 commit `7ef4215c7b850fc27918b71cc9b562301d305dd1`. It did not repeat the experiment.

The [actual R6 attempt](../successor/evidence/utm-input-acquisition-r6-2026-10-13/EXECUTION-OUTCOME.md) is consumed. It failed before downloader execution and retains three marker files, 178 logical bytes, in the private operation root. That is evidence supplied by the prior operation; this review did not inspect or modify that host path. There is no acquisition rerun authority.

## Why the setter is wrong for this purpose

Apple's published header makes `RLIMIT_RSS` an alias of `RLIMIT_AS`. The local SDK and Python constants independently recorded the same identifier, 5. Renaming the call would leave its behavior unchanged. [Apple resource definitions](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/resource.h#L481-L488).

Current published XNU sends `RLIMIT_AS` to `vm_map_set_size_limit`; the VM implementation rejects a limit below the current map size. That explains how a finite value can be refused despite an inherited infinite hard limit. It is an address-space check, not a distinct RSS check. This is a source-supported explanation consistent with the diagnostic, **not proof that this exact kernel branch caused the recorded error**: the child's map size and exact running-kernel/source match were not measured. [Apple setter](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_resource.c#L1560-L1566), [Apple VM validation](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/vm/vm_map.c#L20811-L20837).

Upstream CPython 3.9.6 maps a `setrlimit` EINVAL to this ValueError text. The text therefore does not establish that the requested soft limit exceeded the inherited hard limit. The Xcode runtime is not attested as identical to upstream, and the diagnostic's actual errno remains unavailable. [CPython implementation](https://github.com/python/cpython/blob/v3.9.6/Modules/resource.c#L219-L228).

The archived Apple manual describes an older resident-set pressure preference. That description was repeated in R6's approval packet and my earlier source review. It is inadequate for this host's observed alias/setter behavior and must be corrected in any successor packet. [Archived Apple manual](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/setrlimit.2.html), [R6 approval source](../successor/ops/utm-execution-v1/APPROVAL.md).

## Can the promised bound be preserved?

The approved packet explicitly described **sampled RSS stop thresholds with possible overshoot**, not a hard RSS quota. Those thresholds can remain unchanged:

| Stage | RSS stop threshold |
| --- | ---: |
| Acquisition | 256 MiB |
| Registry authentication | 1 GiB |
| Verifier archive inspection | 256 MiB |
| Publisher authentication | 1 GiB |
| Ubuntu inventory | 3 GiB |

The [existing supervisor](../successor/ops/utm-execution-v1/fort_probe/inspection_workers.py) already samples the owned child's RSS, rejects malformed/unavailable observations while the child remains live, supervises through pipe EOF and cancels the owned process group before reaping. It requests 100 ms sample spacing, with up to 250 ms per `ps` call and other loop/scheduling delays. That is not a guaranteed 100 ms response time. Keep this behavior and its fail-closed errors.

A guarantee that memory **never exceeds** the numbers above cannot be supplied by this monitor. Startup, sampling and termination gaps permit overshoot with no proven byte bound. A fast allocation can pressure the host before the supervisor reacts; loss or starvation of the supervisor removes the active memory control. RSS also measures resident pages rather than all allocated address space. The proposed correction preserves a stop policy, not containment against arbitrary memory exhaustion.

The same 256 MiB child resource-5 setting cannot be retained with the observed launch path: it fails. There is no evidenced drop-in public macOS setter here that establishes an equivalent hard RSS ceiling for these Python and Node workers. This does not claim every macOS isolation mechanism is impossible; it means this review found no justified minimal replacement offering that promise.

The Node flag should be described accurately too: `--max-old-space-size=256` limits V8 old space, not the whole JavaScript heap or total process memory. Young-generation, native and Buffer/ArrayBuffer allocations remain outside that single limit. Keep it as an additional constraint, not a replacement for RSS supervision. [Node 26.5.0 CLI](https://nodejs.org/download/release/v26.5.0/docs/api/cli.html#--max-old-space-sizesize-in-mib), [Node memory accounting](https://nodejs.org/download/release/v26.5.0/docs/api/process.html#processmemoryusage).

## Practical options

| Option | What it buys | Cost or risk | Decision |
| --- | --- | --- | --- |
| **Explicit macOS RSS supervision policy** | Retains all current RSS thresholds and the bounded streaming design; avoids the incompatible address-space setter. | Removes that child kernel control; memory overshoot and supervisor failure remain possible. No hard process-memory ceiling. | **Recommended for this bounded, attended input inspection**, subject to explicit acceptance of these limits. Small implementation change; no new service or dependency. |
| Separate address-space ceiling plus the RSS monitor | Adds a distinct kernel address-space limit if a compatible value can be established for both Python and Node. | Requires new numbers, process-map evidence and launch validation. A larger address-space ceiling is not the original RSS bound; do not auto-size it from host RAM or current usage. | A possible later additional control, not the smallest correction and not justified by current evidence. |
| Move execution behind a separately approved Linux memory controller/isolation boundary | Kernel accounting and group-level enforcement for the worker and descendants. | Changes execution placement, trust and bootstrap scope; requires separate authority and acceptance. It cannot reuse this consumed attempt or assume the unbuilt target VM is available. | Prefer if a kernel-enforced budget is a requirement rather than sampled supervision. Otherwise leave the operation stopped. |

For the third option, Linux cgroup v2 documents `memory.max` as its main hard memory control, with reclaim/OOM behavior and possible temporary exceedance; swap has a separate `memory.swap.max`. This is charged group memory, not identical to RSS. A later design would set its limits before starting the workload and separately budget the VM/host overhead if using a VM. Even this should not be advertised as a literal never-one-byte-over guarantee. [Linux kernel cgroup documentation](https://docs.kernel.org/admin-guide/cgroup-v2.html#memory-interface-files).

Do not substitute `RLIMIT_DATA`, process-global settings, private entitlement-dependent controls, or a larger numeric limit merely to make startup pass. None is an evidenced equivalent for the promised resident-memory policy. Do not add an automatic fallback that silently proceeds when memory-control setup fails.

## Smallest approved follow-up

1. **Make the control change explicit.** The macOS policy would apply the existing core, CPU, file and descriptor limits unchanged, and rely on the existing parent RSS monitor for memory. It would deliberately make no RSS/AS setter call. Do not set the inherited limit to infinity or alter the parent. Other setup errors still stop execution. Name this policy in the revised approval text and correct the old-space/total-heap description.
2. **Reuse the existing tests.** Add a failing fake-resource test that models the RSS/AS alias and proves the macOS path never requests it; assert the exact remaining calls and failure propagation. Keep the current worker fixtures for excessive/unobservable RSS, deadlines, EOF, cancel-before-reap and retained failure state. No new diagnostic framework is needed.
3. **Prove the replacement before acquisition.** Under specific later authority, use a small local child smoke check for the unchanged remaining setters, actual RSS observation and owned cancellation/reap, with no network or downloaded code. File/descriptor setup is still unrun; removing the RSS failure does not prove the next call or full worker launch will work. This review does not authorize that check.
4. **Review the new immutable source identity, then decide on another attempt.** A changed control requires a new commit/seal and explicit acceptance of sampled-memory risk, plus a distinct decision for any acquisition attempt. Preserve R6's consumed root and records; any new root or exact owned-state removal must be explicitly resolved. No automatic root adoption, reset or retry.

Suggested decision wording: “Approve the macOS policy correction that removes the child RSS/AS setter while retaining the existing RSS stop thresholds and all other limits. Accept that memory is supervised with possible unbounded overshoot rather than capped by a hard per-process quota. This authorizes preparation and verification of the revised source only; it does not authorize acquisition, removal of consumed state, downloaded-code execution or a VM.” Any future live verification permission must state its own concrete scope.

## Review boundaries and identities

This review read local evidence and source, checked file hashes, and consulted official Apple, Python, Node and Linux sources on 2026-10-13. No resource-limit experiment, acquisition, live worker, downloaded binary, secret access, VM launch, limit/settings change or product-source edit occurred. Only this decision document was written. No new review agents were used; actual serving model identity is unattested.

- Diagnostic SHA-256: `756eae914fa5ee1940bb52d6b1d02ca29c9f32a1eabb5406b01071c1857b7a2b`.
- Raw diagnostic evidence SHA-256: `adb7e539113f47d4b519f58ebdf6838198619034b76dbd98370290617f8f9732`.
- Diagnostic validation SHA-256: `124dc7ea9e97a5330bcc087050948f5408a767d00e598a8d75f8521e84bb2b12`.
- R6 seal remains `fd4c3110c319a1097d7bc419a7bd692be3d8f50b64d946a696076421b3f3b88b`; this document supplies no new execution authority.
