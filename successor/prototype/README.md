# Candidate B — approved attended runner, awaiting Mac readiness

The owner approved this bounded replacement test and its minimal activation
revision; [the approval record](../evidence/2026-10-16-candidate-b-activation/approval.json)
preserves the exact scope. `requireCandidateBActivation()` now permits only the
manual entrypoint. It is an application routing check, not a host security boundary.
No provider process, real credential or local provider configuration was accessed
while preparing activation. The owner has not yet confirmed readiness at the Mac.

After the coordinator relays Mac readiness and the current pins are verified,
the entry is `/opt/homebrew/opt/node@26/bin/node successor/prototype/manual.ts` in
the candidate worktree. Do not invoke it yet. It verifies source/app bytes, owns
the two-session controller and exposes only `start`, `status`, `done` and `abort`
terminal commands. Runtime preflight and the session clock begin only on `start`.
The user opens/quits the exact pinned app and performs the five samples described
in [the manual](../ops/candidate-b-manual-packet-20261016/MANUAL.md).

## Implemented controls

- One initialized Codex connection is preflighted before enrollment or HTTP service
  exposure. `config/read` must confirm every pinned setting and the entire MCP map
  has no enabled/unknown server. The feature list and full MCP status list must
  terminate within eight pages. Missing fields, repeated cursors, an unexpected
  provider override or any MCP status entry fail closed. Account class must be
  existing ChatGPT and the exact model must be available.
- Every turn repeats effective controls, checks the ephemeral thread response and
  repeats feature/MCP inspection for that thread before the cancellation fence
  and `turn/start`. No tools, fallback, login or extra diagnostic turn is added.
- SIGINT, SIGTERM and terminal closure cancel pending startup as well as a ready
  session. The signal reaches preflight and listener startup; a late completion
  cannot publish enrollment or readiness. Cancellation consumes the start and
  leaves the packet blocked.
- The HTTP acceptance gate allows only the next of five literal samples, with the
  expected Agent, instructions, skill and conversation. It requires earlier runs
  completed and retains the same Agent/binding/Home identities. Extra or early
  samples are rejected before durable acceptance. Cached receipt replay creates
  no additional run.
- Durable start state plus an exclusive lock permits at most two sessions. A
  failed/crashed/uncertain session cannot be resumed automatically. Each session
  now stops work at **292 seconds**, reserving eight seconds within the unchanged
  five-minute limit for TERM/KILL, process checks and credential cleanup. The
  owned-runtime allowance is bounded by that remaining outer budget. This is
  timer-based supervision, not a hard-real-time or host-confinement guarantee.
- Restart requires four retained completed receipts, the actual installation's
  native pending file decoding to an empty array, exact app exit, and successful
  service/runtime cleanup. The same port, ledger and installation reopen for
  sample 5. The gate is checked again before native relaunch. A missing or malformed
  queue fails; the synthetic harness queue is never used as native evidence.

## Local credential lifetime and cleanup

The packet explicitly selects bounded enrollment. It can issue one enrollment
code and one device credential, with an immutable deadline at most ten minutes
from the first session start, including the gap. The second session cannot extend
it. This replaces the earlier packet's proposed 30-day credential; ordinary
fixture-service defaults are unchanged.

Final/failed shutdown deletes the candidate's enrollment/device authorization
rows and closes future enrollment. Only a fully confirmed first-session handoff
retains the device for session two. Exiting the controller between sessions
revokes it too. Native cleanup targets only `Fort.Spec058.Device` and the exact
installation/device account IDs issued by this ledger, without reading secret
values; it also removes that matching connection metadata after successful
Keychain deletion. It handles an issued item whose connection-file write failed.
The local enrollment-code file is removed on shutdown and never printed in logs.

If macOS refuses Keychain deletion, the result is `credentialCleanup:unconfirmed`
and the packet is blocked. The item/connection metadata may remain, but the server
has revoked its token. An uncatchable controller crash can also leave local state
or an inactive/expiring item; its persisted deadline is never extended and a stale
lock blocks further starts. Sample history, pending files and redacted receipts
are retained. No global credential sweep or system-permission change occurs.

## Runtime limits that remain unproven

The pinned 0.157.1 executable has not run in this increment. Its actual merged
settings may omit a field this strict preflight requires; the result is a stop,
not an assumption or relaxed default. A nonempty MCP status list also stops even
if an implementation reports disabled entries. These compatibility questions
require the eventual authorized no-turn preflight.

The official host starts before it can answer `config/read`. Retained HOME may
allow ambient discovery or startup side effects before that response. This code
cannot prove their absence without starting the runtime or inspecting sensitive
configuration, and neither was done. It does block thread creation/MCP discovery
requests until the effective map is confirmed, then blocks turns until both global
and thread checks pass. No direct user-config reader is added.

Requested read-only/untrusted/network-disabled computer-tool policy is not host
filesystem, Keychain, IPC or network confinement. Extra empty-environment/tool/root
fields may be ignored; tool-event refusal cannot undo an effect. Owned-group
cleanup cannot contain escaped descendants or prove remote provider cancellation.
The exact requested model is checked; effective-model attestation remains absent.
These are the previously disclosed host-risk class, not completed OS-isolation
acceptance. Actual launch, sign-in availability, native Keychain behavior, process
cleanup and manual visuals remain unrun. The Apple source/app is unchanged.

[Current source pins and validation](../evidence/2026-10-16-candidate-b-activation/README.md)
record the minimal activation revision. The coordinator reported final focused
review of the preceding candidate before the owner approved execution. This
preparation supplies nonlive activation checks; it claims no live acceptance.
