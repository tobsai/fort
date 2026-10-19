# Selected provider account correlation — October 19, 2026

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md), existing owner
implementation decision and the coordinator's explicit fixture-only correlation
instruction. Base `28dfa3b6111ac8a1656e064ac5ce93f75d0bf94a`; branch
`codex/fort-openai-rebuild-20261016`. This is source, build and synthetic evidence.
No real credential file was read, account looked up, provider signed in, Apple
agreement accepted, entitlement expanded or startup service activated.

## Behavior and boundary

The existing optional sign-in adapter now returns a bounded nonsecret Fort identity:
`accountId` and `userLabel`. Codex 0.157.1 maps these from
`workspaceRouting.chatgptAccountId` and `account.email` in the pinned official
[account protocol](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/account.rs).
Both must be present and valid. The workspace field is experimental; the existing
initialize request enables experimental API. No unsupported session-list RPC or
credential parsing is used. Missing evidence refuses identity establishment, even
when a successful login notification or ChatGPT account type exists.

The owning Mac shows the reported label and workspace. Only an explicit
`POST /v1/account/sign-in/select` matching the operation ID, revision, recovery epoch
and full reported identity selects it. Companion access, foreign/stale identities
and malformed evidence are refused. Status and restored UI never select implicitly.
Selection is persisted in the existing account-operation metadata; retries are
idempotent. Starting another account operation or replacing the owner/epoch
invalidates the prior selection. Restart preserves an unchanged owner's selection.
No new identity service, dependency, credential store or generic framework exists.

Each accepted turn pins the choice in its durable configuration. Before dequeuing,
the service compares that pin with the current owner choice; it cannot retarget old
work after an account change. The bounded executor carries the choice within existing
digests. Codex compares fresh supported account evidence before model discovery and
before every thread/turn. Runtime replacement rechecks its retained selection.
Old outcome/history records stay readable but cannot prove account selection.
The old MVP packet without a selected identity now refuses preflight and reaps its
synthetic owned child; it gains no compatibility bypass or new activation authority.

The public/native identity is provider-neutral: an independently shaped fixture uses
an opaque tenant and plain display label. Codex-specific response fields stay in its
adapter. No identity is inferred from Apple login, an old demo, email alone, or a
successful sign-in outcome. The selected account/model must still be chosen and
inspected in an authorized real session before installed acceptance.

Account labels/workspace IDs are nonsecret but personal data. They persist in
private operation metadata and accepted turn configurations alongside retained
history. Local provider-credential removal remains the separate metadata-only
`Runtime/state/auth.json` cleanup after confirmed host exit/unregistration. Keeping
history keeps its nonsecret identity pins. Nothing claims provider-side revocation.
`account/read` with `refreshToken:false` can still perform routing discovery and
auth recovery; every account response in this slice was a fixture.

Agent-host App Sandbox and computer-tool restrictions remain separate. Helper
outbound networking remains disabled in the review package. No LAN/TCP listener,
new pairing expiry or product turn/time limit was added. The earlier thirty-minute/
six-turn suggestion is only a possible attended verification session, not policy.

## Verification

- Public red: selected-account evidence was absent before implementation. Green:
  report/select/retry/stale/companion refusal and restart preservation pass through
  authenticated HTTP. Queue tests prove a changed choice never retargets prior work.
- Protocol red: missing/wrong selection previously completed a synthetic turn.
  Green: those requests stop before model/thread/turn calls. Missing/null/malformed
  email/workspace, a change after catalog discovery, strict native decoding and
  independent-provider executor identity transport are covered.
- Full portable suite: **1,326 passed / 16 skipped / zero failures**, 43.252 s.
  Command: pinned Node 26.5.0 `successor/verification/test-suite.mjs --suite portable`.
  Strict TypeScript passed. Core/executor/resource builders passed.
- Full serial native suite with fresh core/host resources: **328 passed / 16 skipped**,
  344 reported, zero issues, 53.504 s. Bundled Swift client → private host → compiled
  core → guarded synthetic runtime proves selection, model discovery, exact text,
  stop/resume, restart/history, re-pairing invalidation, fresh selection and cleanup.
- Guarded executor suite: **19 passed / 2 opt-in budget skips**, zero failures.
  Synthetic account/cancel races, catalog/turn transport and owned process absence
  pass. No installed Codex process was executed.
- Exact native package refusal probe: **1 passed**. Public inspection refuses the
  unsigned app; an explicitly synthetic signature reader reaches the real missing
  profile gate. No signature/profile/activation is fabricated.
- Isolated account-selection HTTP transaction: five warmups plus twenty samples,
  p50 **1.898417 ms**, p95 **2.221708 ms**, maximum **2.851791 ms**, target p95 < 5 ms.
  Provider work is excluded. Measurement ran after builds and other tests stopped.
  Samples: `/tmp/fort059-account-selection-samples.json`.

The first loopback test was blocked by the sandbox and rerun under approved local
fixture execution. An intermediate nested transaction caused HTTP 500 and was fixed
by using the existing account operation's transaction boundary. The first broad
portable run exposed old fixtures without account selection and was interrupted
when one waited for a turn that was now correctly refused; those fixtures now select
through the public flow. One restart test incorrectly expected held work to run;
its account-change assertion now runs before restart, while selection restart has
separate coverage. The first complete native run reported three issues in one
bundled-host journey that lacked selection; the repaired journey and full suite
pass. A guarded cancel-race expected the obsolete kind-only result and now checks
the exact new nonsecret identity. These intermediate results are not counted as
passes. A focused protocol run waiting on old restart preflight was also interrupted;
the updated selection fixture passes in the complete portable suite.

Native skips are fourteen opt-in timings and two optional package/signature probes;
the new exact review-package probe passed separately. Other timing/signed suites,
Simulator UI, actual provider, final installed accessibility/screenshots, archive,
upload and TestFlight remain unrun. CoreSimulator 1171.7.0 remains below Xcode's
required 1174.9.2; no component upgrade was performed.

Hardware/toolchain: MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2; Node 26.5.0,
TypeScript 6.0.3, Swift 6.4/Xcode 27.1. Tests use disposable local storage and authored
protocol/process fixtures. Builds are unsigned and unlaunched.

## Exact review package

`/private/tmp/fort059-account-correlation-review-20261019/Fort.app`

Fresh Release Mac/helper and Debug iPhone Simulator builds passed. The new app has
**200 files / 404,454,902 bytes**; every output hash/mode/length and all **198 input
hashes/modes** matched independently. There are **zero profiles**. Report SHA256:
`f28747be6e5fb4ff8ba1088d7c8fc8e74582008d72b0804271a990e5ce7fb12d`.
Independent summary: `/tmp/fort059-account-package-verification.json`.

Fresh host resources: `/tmp/fort059-account-correlation-host-resources`, 112 payload
files, 113 including manifest, 491,737 bytes. Exact inventory/length/hashes matched.
Manifest SHA256 `da5b28e63ebb8fc1b2b41b99fda5d5f0f2a3e363b5848dd1873e86aa7f45c22d`.
Existing Node closure/guardian and exact Codex binary/legal payload were preserved.
Engineering app/helper/group IDs and helper outbound entitlement remain unchanged.
This is an inert review artifact, not a working tested installed application.

Logs under `/tmp/fort059-account-`: `red.log`, `green.log`, `adapter-red.log`,
`adapter-green.log`, `focused.log`, `compat.log`, `portable-final.log`,
`types-final.log`, `native-final.log`, `guarded-final.log`, `budget.log`,
`host-resources.log`, `mac-build.log`, `helper-build.log`, `ios-build.log`,
`assembly.log` and `package-probe.log`. Prior artifacts and logs are preserved.

## Remaining acceptance

No technical blocker required live sign-in to complete this implementation.
Source correlation, native status, tests and package preparation are complete.
The remaining acceptance needs the owner-selected real account/workspace/model,
verified Apple identities/profiles and final signed artifact, then exact authorized
host outbound/startup/provider activation and installed launch/chat/restart/reconnect
inspection. Account identity fields being absent in a real supported response would
remain an explicit refusal, not a reason to inspect credentials or substitute an
account. The pending Apple secure handoff and legal approval are unchanged.
No further independent source prerequisite was found for this bounded account slice;
this does not claim the broader v1/TestFlight acceptance is complete.
