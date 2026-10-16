# Fourth bounded native-chat packet — prepared, not started

Source **`916fd00e84a75250425d7cd152cfd15252a7c0e5`**, based on offline-canary evidence
`52b928112b2e9298ddc28928749c23f7944c922c`, branch `codex/fort-openai-rebuild-20261016`.
Worktree: `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.

The [owner's 21:29 UTC approval](approval.json) authorizes one fresh attempt, executed
only by UI task `01a1451b-81d7-76cf-8e5c-a85d7164201c`. Preparation performed no live
controller/runtime/app start, provider call, real pairing or credential operation.
The fixed fourth root is **absent**, so no start has been consumed:
`/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-fourth-20261016`.
[Packet scope](packet.json), [exact pins](pins.json), [verification](verification.json)
and [operator instructions](../../ops/candidate-b-fourth-20261016/MANUAL.md).

## Narrow change and preserved gates

The live entry now selects `fourth-20261016`, its fixed fresh root and these pins.
The controller recognizes that identity as one start, with a maximum of one admitted
sample and one required completed receipt. It consumes the start before freshness
checks, refuses every subsequent start, and cannot adopt any older packet's state.
One original receipt may be replayed without a second run. The first three consumed
attempt file sets, blocked states and ledgers are byte-identical, each with zero
receipts; [preservation](attempt-preservation.json). No reset or relabeling occurred.

The one sample is Writer Home, no skill, exact `gpt-6-astra`, Writer instructions
`Reply briefly using supplied context.`, and:

> Remember the sample word cedar. Reply with that word only.

The UI operator expects `cedar`. Wrong selections/text and a second submission are
refused; an unexpected reply must be reported without a corrective prompt. Automatic
local pairing uses the existing native UI and non-emitting CUA variables. No owner
interaction is needed unless a protected system prompt requires it.

The five-minute total includes preflight, pairing, the sample and cleanup: work stops
at 292 seconds, credentials expire at 300 seconds, and success requires app exit,
empty native pending state and confirmed scoped cleanup before that total deadline.
Cancellation, failed checks, late work/cleanup or unconfirmed credential deletion
block the packet permanently. Timer supervision remains subject to host scheduling.

The entire runner, official launch plan, activation guard, service, native cleanup,
Apple source and app are unchanged from `fcc3685`. The named no-files profile,
empty environments/roots, disabled tools, exact provider/model and all remaining
startup checks are preserved. The owner acknowledged the same-user host boundary;
the computer-tool profile does not confine that host. Pre-opened descriptors remain
usable, full generated-policy OS enforcement is unverified, and effective-model
attestation is absent. No permission was broadened or check weakened.

## Verification

- **434 focused nonlive tests passed, zero failed**, exit 0: [log](focused-tests.log).
  Tests use synthetic in-memory/runtime and temporary loopback fixtures, never the
  real provider, app or native Keychain. They cover one sample, wrong selections,
  duplicate receipt replay, extra refusal, no restart after reopen, late/failed cleanup,
  native exit/pending, deadline expiry, freshness/preflight refusal, cancellation and
  old-attempt rejection, plus all current startup/permission protocol checks.
- Strict TypeScript passed, exit 0: [empty success output](typecheck.log).
- The new fourth case first failed its five-minute deadline assertion
  ([red](session-red.log.gz)); manual wiring first failed its old-root assertion
  ([red](manual-red.log.gz)). Both passed after the minimal changes. An earlier default
  command sandbox blocked a fixture listener; the authorized rerun reached the real
  assertion. A test-only replay expectation was corrected from 200 to the existing
  201 receipt-replay contract. Raw failure logs retain their exact bytes in deterministic gzip, with
  [raw hashes](compressed-logs.json). These intermediate failures remain labeled in
  [verification.json](verification.json).
- All **468 source files** match the source commit. Source-list SHA-256:
  `3a398e49c4c7e2494f7969c1f772e5cf2fd2d322380c60343e45ad74b8a04a32`.
- All **5 native app files** and their file set match the prior pins; app-list SHA-256
  `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`.
  App build source remains `de9e482bac99920d52a03b6904542a78226c5ba2`.
- Codex 0.157.1 executable SHA-256 remains
  `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`.
  Its bytes were checked without executing it during this preparation.
- Main checkout status, old evidence/approval packets, donor attribution and unrelated
  work are preserved. [Preservation log](preservation.log). No push, merge or deployment.

Unrun here: fourth live preflight/provider response; native pairing, Keychain cleanup,
accessibility/screenshots; Swift/native rebuild; unrelated full test suite; independent
review of this delta. The designated UI operator supplies the live result.

## Short launch handoff

After pin/freshness checks and CUA preparation, the designated UI operator runs in a PTY:

```sh
cd /Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016
/opt/homebrew/opt/node@26/bin/node successor/prototype/manual.ts
```

Send `start` once. Pair the exact pinned app automatically, create Writer and send
the one sample with no skill. Quit Fort fully, then `done`; use `abort` on failure.
Follow the linked operator note for secret-safe pairing and evidence. No second start,
extra message, fallback, settings change or fifth attempt is authorized.
