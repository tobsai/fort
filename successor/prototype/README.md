# Candidate B — third attempt prepared

The first two attempts remain consumed and blocked with zero receipts. Their
state, ledgers and evidence are unchanged. The owner approved one separate third
five-minute attempt at 20:17 UTC on October 16, after the [canonical-feature
compatibility correction](../evidence/2026-10-16-codex-feature-alias-compatibility/README.md).
Canonical observations must still be explicitly false; absence is a failure.

The [third-attempt packet](../ops/candidate-b-third-20261016/MANUAL.md) and
[its pins and approval](../evidence/2026-10-16-candidate-b-third/README.md) identify
the new fixed directory. Only the designated UI operator executes it, with
automatic local pairing. This preparation starts no runtime or attempt. One
start is allowed, with no restart or extra sample; earlier attempt records
cannot be adopted. The implemented limits below apply unchanged.

`manual.ts` verifies the source and unchanged app bytes before preparing the
fixed third-attempt directory. It passes the explicit `third-20261016`
attempt identity to `CandidatePacket`. No environment switch, retry or second
session is offered. Any persisted third-attempt state prevents another start.
The old two-session controller behavior remains available only for historical
nonlive regression coverage; the live manual entry selects the third attempt.
The application entry guard is a routing check, not host isolation.

The replacement accepts the same five literal samples in order with the same
Agent names, instructions, bindings, skills and conversations. Earlier runs must
complete before the next sample is admitted. Sample 5 now occurs in the same
session, so it does not establish recall across a restart. Extra or mismatched
messages are refused before durable acceptance; cached receipt replay does not
create another run. Failed, canceled, crashed or uncertain attempts cannot resume.

Work stops at 292 seconds. Eight seconds are reserved within the five-minute
total for owned-process termination, listener closure and exact credential
cleanup. The replacement credential expires no later than five minutes from
start and is never retained for a restart. Success requires all five completed
runs, the exact native app fully exited, an empty actual native pending queue,
confirmed cleanup, and completion of cleanup by the total deadline. Overshoot
is blocked. These are supervised timers and bounded subprocess waits, not a
hard-real-time guarantee. SIGINT, SIGTERM and terminal closure cancel pending
preflight as well as ready sessions. Cancellation consumes the start.

The pinned Codex 0.157.1 launcher, gpt-6-astra selection, existing ChatGPT account
requirement, effective configuration/feature/MCP checks, per-turn checks, refusal
rules and allowlisted startup diagnostics are unchanged. No configuration fault
is guessed or bypassed. A failed preflight may be the replacement's useful result;
it does not authorize another attempt. Raw provider payloads, stderr, credentials
and arbitrary errors are excluded from the diagnostic `{stage, check}` pair.

Cleanup revokes the exact service enrollment/device rows and attempts deletion
of only the issued installation/device Keychain items under `Fort.Spec058.Device`.
It reads no secret value. Successful deletion also removes matching connection
metadata; history and pending files remain. The local enrollment file is removed
on shutdown. Failed Keychain deletion is reported as `unconfirmed` and blocks the
packet. A crash can leave files or an inactive expiring item; no reset or global
credential sweep is authorized.

The official host starts with the Mac user's HOME/access before effective config
can be checked. Ambient startup effects remain possible. Computer-tool policy is
not host filesystem, Keychain, IPC or network confinement; group cleanup cannot
contain escaped descendants or prove remote cancellation. Effective-model
attestation is absent. No provider sign-in, OS settings change, permanent service,
public ingress or deployment is added. Native/provider acceptance and screenshots
must be supplied by the designated UI task after the exact pinned run.
