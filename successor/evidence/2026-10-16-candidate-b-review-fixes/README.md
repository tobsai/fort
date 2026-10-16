# Candidate B — targeted review fixes

Ready for another independent review. **Live activation remains blocked.** This
record addresses the supplied review; it does not claim independent acceptance of
the fixes or transfer Packet A authority to the changed candidate.

The original [independent review](independent-review.md) and
[in-memory reproduction](abort-race-original.mjs) were copied from `/tmp` before
editing. Their hashes are retained in [pins.json](pins.json). The original repro
first returned `sentAfterAbort:true` (receipt `1988ce`); after the fix it returned
`sentAfterAbort:false`, no `turn/start` method and `kind:notStarted` (`92bbd0`).

## Changes

- `38a1515` checks cancellation immediately before writing the provider turn.
  Known pre-dispatch cancellation becomes `notStarted` and a canceled service run.
  Runtime disposal is now required. Service shutdown aborts work, disposes the
  runtime even when idle, awaits cleanup and propagates failure.
- The same commit adds an owned process group. It drains bounded output, closes
  the transport to reject late frames, requests SIGTERM, escalates to SIGKILL,
  and requires both observed child close and absence of that owned group. A
  cleanup failure rejects disposal rather than reporting a successful restart.
  The candidate-specific launcher remains behind the always-refusing gate.
- `22ac54e` makes the SwiftUI editor use a `WorkspaceForm` that retains the loaded
  Agent or skill revision. Polling no longer supplies a newer revision for stale
  form contents. It also adds an actual MessengerModel nonlive response-loss and
  restart-replay test, and preserves four completed receipts in the HTTP harness.
- `de9e482` records the unchanged execution limits and validation commands.
- `73f6444` reserves the two-second TERM and two-second KILL allowances within
  the five-minute total session budget: the work cutoff is 296 seconds.

The text input now includes `text_elements: []`, as specified by the pinned
protocol. No fallback model, provider, authentication path or additional product
scope was introduced.

## Verification

| Check | Result |
| --- | --- |
| Targeted Node fix suite | 36 passed, 0 failed; [output](review-fix-tests.log) |
| Combined relevant Node suites | 135 passed, 0 failed at `de9e482`; [output](combined-review-tests.log) |
| Final session-budget/gate test | 1 passed, 0 failed at `73f6444`; [output](launcher-budget-final.log) |
| Swift unit/model suite | 23 passed, 0 failed; [output](native-review-tests.log) |
| Swift FortClient/PendingStore → temporary SQLite service | Five synthetic turns passed; four completed receipts retained; pending empty; resource-free runtime disposal confirmed before restart; [output](native-http-review.log) |
| Strict TypeScript | Passed after the final timing correction; receipt `8ea979` |
| Mac compile | Succeeded in new durable output directory; log `successor/.build/review/mac-review-build.log` |
| Main / previous app | Main status unchanged; previous pinned app hashes still match |

Synthetic process tests cover idle disposal, active shutdown, ignored interruption/
SIGTERM, forced exit, rejection of late output and automatic deadline expiry. The
active-shutdown service test verifies an unknown outcome, no assistant reply and no
automatic retry after reopening the ledger. These tests launch only explicit Node
test actors, not the Codex executable.

The form test loads revision one, polls revision two, then verifies that Save still
submits revision one and receives a conflict. The MessengerModel test loses an
acceptance response, restarts the model and verifies exact request replay with the
original recipient/skill revisions, one simulated run and an empty pending queue.
Its HTTP server is an in-memory test fixture. Actual SQLite idempotency and client
wire conformance are covered separately. Neither test exercises rendered SwiftUI,
real model comprehension or installed-product accessibility/screenshots.

The first new form test could not compile before `WorkspaceForm` existed; that red
output is preserved. The abort, retained-four-receipt, text-elements and total-budget
checks also failed before their fixes. The synthetic SIGTERM-ignoring child initially
exited on its closed stdout pipe; the fixture was corrected to tolerate EPIPE, then
the test confirmed the intended SIGKILL path. No required final nonlive check fails.

## Static executable and control checks

The exact installed executable was read as bytes, never executed:
`/opt/homebrew/Caskroom/codex/0.157.1/bin/codex`, arm64 Mach-O, 238,223,808 bytes,
SHA-256 `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`.
See [metadata](executable-static.json), [bounded marker checks](executable-markers.json)
and the [disabled proposed launch plan](proposed-launch-plan.json). Proposed session
paths were not created. No local auth or configuration file was read.

Five versioned public schemas were fetched and hashed in
[protocol-static.json](protocol-static.json). Every proposed feature key and top-level
configuration key exists in the [0.157.1 configuration schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/core/config.schema.json).
The [UserInput schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/UserInput.ts)
requires the text-elements array now emitted by the adapter. Schema presence and
binary markers do not prove effective settings or parser behavior.

The ordinary [thread-start schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ThreadStartParams.ts)
does not list the requested environment/tool/capability-root fields, and the
[sandbox schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/SandboxPolicy.ts)
does not list the restricted-access object. Those extra requests may be ignored.
Specific implementation-source URLs tried during inspection returned 404; no claim
of executable source-level enforcement is made.

Residual authority remains substantial: the host process runs as the owner, HOME
allows ambient configuration/provider-store discovery, only three named MCP servers
are explicitly overridden, and group cleanup does not contain a process that escapes
its group. Tool-event rejection cannot undo a side effect already performed. No
enforced host filesystem, Keychain, IPC or non-provider network boundary has been
demonstrated. Actual merged settings, executable protocol compatibility, provider
authentication behavior and live cleanup remain untested. These are review/activation
gates, not silently accepted limitations.

## New pins and next review

- Source: `73f644425a74463488cdce61aab8b83384ce8722`
- Source-list SHA-256: `0fb368a0e358a176db114a1085d83996af8c488b2f32d503eebd137970be6e45`
- App-list SHA-256: `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`
- App: `successor/.build/candidate-b-review-mac/Build/Products/Debug/FortMac.app`

The Mac build used `de9e482`; all Apple source is unchanged in the pinned revision.
Signing was disabled. The binary has a linker ad-hoc signature only: no team identity,
bound Info.plist or sealed resources. The app was neither installed nor launched.
Xcode's normal build registered its output path with LaunchServices.

A fresh candidate-specific packet must keep at most two supervised starts and five
exact samples. Before the conditional second start: retain all four completed first
receipts, verify the actual MessengerModel pending queue is empty, and confirm
service/runtime/app cleanup. Otherwise stop without extra prompts, uncertain retries
or queue flushing. The synthetic harness's proof is not a substitute for those live
checks. Packet A remains unconsumed; no approval is being inferred from it.

No live activation, provider call, real credential creation/access, security-setting
change, deployment, remote push, startup service or provider cleanup occurred. The
next action is independent review of these fixes and the residual authority, not a
live test.
