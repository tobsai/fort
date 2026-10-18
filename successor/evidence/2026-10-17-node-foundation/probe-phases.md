# Separate signed probe launch and execution bounds

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md). Source base
`a390d93`, with exact input hashes and raw results in [probe-phases.json](probe-phases.json).

The signed compatibility attempt recorded in [private owner recovery](private-owner-recovery.md)
timed out at 15 seconds. Its system log shows nearly 14 seconds between launch
and the first XPC call, with macOS signing evaluation during that interval. XPC
work then consumed the remainder of the same initial deadline. The original
failure and independently verified cleanup are preserved in the JSON evidence.
No security setting or entitlement was changed.

The Swift probe now emits an exact parent PID/group marker on entry. The harness
retains its 15-second initial launch limit, starts a separate bounded parent-work
deadline at that marker, and retains the existing Node-child execution deadline.
Neither marker can repeatedly extend a phase. Startup, parent work and child
execution durations are reported separately. A successful probe must include its
parent marker and all previous process, identity and sandbox confirmations.

The new phase-report assertion first failed with `parentStartupMs was not measured`.
After the change, all four signed scenarios passed serially, with zero skips:
XPC peer controls plus the actual core bridge, compiled-core messaging/restart,
hung-child timeout/cleanup, and sandboxed SQLite/file/network controls. Strict
TypeScript and full/implementation spec gates passed. The hung-child negative
control still uses a one-second execution deadline and confirms absence within
its existing four-second execution/cleanup bound.

Retained artifact:
`successor/.build/xpc-recovery-phase-059-20261018/FortSandboxProbe.app`.
Its ordinary XPC/core journey and every existing denial check passed. Independent
verification confirmed the deep/strict signature, all six exact entitlement
targets, all 22 compiled-core file hashes, owned PID/group absence and removal of
all generated containers. No forced XPC cleanup was needed.

This single artifact recorded 8,520.81 ms before parent entry, 1,216.96 ms of
parent work and 211.71 ms of child execution/cleanup. These are diagnostic phase
measurements for a fresh signed synthetic bundle, **not** a product startup
benchmark or installed acceptance. The core manifest remains
`4bdc218107386e8592dbaf0e3b2fb59e66ff9d01d73f24a485a83098e9cc249b`.

Logs: `/tmp/fort059-probe-phases-{red,signed,typecheck,artifact}.log`.
The 848 portable and 81 native test passes belong to the preceding product slice;
they were not repeated for this harness-only change. New owner recovery over XPC,
ordinary app setup, persistent registration, actual providers and TestFlight remain
separate work. No account, credential or startup service was activated.
