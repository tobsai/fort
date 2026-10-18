# Guarded runtime process ownership

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), unchanged local
implementation authority from [decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
October 18, 2026. This is synthetic process/RPC evidence. No Codex executable,
provider initialization, credential, permission profile or startup registration
was used. The production executor worker is not yet composed with this adapter.

## Implementation

`launchGuardedCodexProcess` requires macOS, exact normalized absolute guardian,
runtime and working-directory paths, explicit argv/environment and a work deadline
of at most 120 seconds. Each path is limited to 4096 bytes; argv to 256 entries
and 65536 bytes; environment to 128 entries and 65536 bytes. Strings must be valid
Unicode without NUL. There is no ambient executable/environment fallback.

The existing native guardian leads a detached process group and watches its direct
launcher parent through kqueue. The runtime and ordinary descendants inherit that
group. JavaScript disposal uses fixed 250 ms TERM and 750 ms KILL bounds and confirms
child close plus group absence. Abrupt parent death invokes the native watch even
when the runtime group differs from its outer executor group. No new idle poller
or production dependency was added. The portable core artifact excludes this adapter.

The first expanded test found a shared `owned-process.ts` cleanup race. A harmless
signal-zero observation of the guardian with a missing child executable recorded:

| Event | PID | Exit code | Group query |
| --- | --- | --- | --- |
| Spawn returned | 84803 | unset | present |
| stdout ended | 84803 | unset | EPERM |
| Child exit | 84803 | 125 | ESRCH |
| Child close | 84803 | 125 | ESRCH |

A direct missing executable had no PID, ruling out the initial invalid-PID
hypothesis. Cleanup now signals only an integer PID greater than one. Signal
failure does not determine cleanup success; bounded exit/absence checks do.
Unknown group status counts as present and leaves cleanup unconfirmed if it
persists. Historical process budgets, packet validation and callers are preserved.

## Verification

Machine: Apple M1 Max, 10 CPU cores, 64 GiB, macOS 27.2/26B5112a. Node 26.5.0,
TypeScript 6.0.3, Swift 6.4, Xcode 27.1/27A9275. Compilation used the existing
guardian source and local compiler cache. Synthetic children received explicit
empty or test-only environments. No real messages or account data were used.

- Initial missing-module red: `/tmp/fort059-guarded-codex-red.log`.
- Expanded behavior run: 5 passed, 1 failed, 1 timing check skipped. The failure
  was the exiting-guardian signal race, `/tmp/fort059-guarded-codex-behavior.log`.
- Independent public owned-process regression reproduced the same failure before
  the fix, `/tmp/fort059-owned-exit-red.log`.
- Guarded/owned green run: 10 passed, 1 isolated timing check skipped,
  `/tmp/fort059-guarded-codex-green.log`. Includes exact RPC text, invalid inputs,
  explicit environment, missing guardian/runtime, deadline and repeated disposal.
- Additional negative control and test discovery: 6 passed,
  `/tmp/fort059-owned-exit-final.log`. Only the exact test group's signal-zero
  query is made to report unavailable status; real termination remains active.
  The child/group are independently absent afterward, but cleanup still refuses
  to claim confirmation under the unavailable observation. The OS method is
  restored before the final independent absence checks.
- Full portable suite: 902 passed, zero failed/skipped,
  `/tmp/fort059-guarded-codex-portable.log`. This includes the existing callers,
  packet-budget controls and new failed-launch/uncertain-observation regressions.
- Strict TypeScript passed, `/tmp/fort059-guarded-codex-typecheck.log`.
- Full structural, implementation, staged-path and whitespace gates passed.

The green noncooperative test observed PIDs 84950, 84951 and 84953 and group 84950
absent after 272.323 ms. The parent-death test observed PIDs 84965, 84966, 84967
and 84968, runtime group 84966 and outer group 84965 absent after 11.303 ms.
These are individual cleanup observations, not percentile benchmarks.

The separate, otherwise idle benchmark ran 20 serial synthetic launch/ping/dispose
journeys. Each included Node startup, guardian startup, local RPC and independently
confirmed group absence. Compilation preceded the measurement; signing, provider
and network work were excluded. p50 74.090 ms, p95 80.461 ms, maximum 499.254 ms,
against a p95 target of 2000 ms. Command used `FORT_GUARDED_CODEX_BENCHMARK` with
`node --test --test-name-pattern='isolated budget' runner/guarded-codex-process.test.ts`.
Raw samples and log: `/tmp/fort059-guarded-codex-budget.json` and `.log`.

## Limits and remaining work

The launcher does not verify binary provenance or grant runtime/provider access.
It supervises the agent host process; it does not provide a computer-tool sandbox.
A trusted child deliberately creating another session can escape the group.
Production packaging still needs signed App Sandbox and exact runtime review.
An executor must await child disposal before reporting terminal success; observing
only its own group cannot establish absence of this separate runtime group.

No Swift product files changed. Native builds, signed packaging, real runtime,
installed UI, Apple account/provisioning and TestFlight checks were not rerun.
The existing 30-file/190826-byte core artifact is unchanged, manifest SHA256
`c1a19bd8e8f0def68efa45f2b913dc5b2123466a3da6a5c14ce0eb8fded7f527`.
Source implementation and synthetic tests do not establish a working provider
application or completed v1.
