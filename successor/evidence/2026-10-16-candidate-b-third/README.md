# Candidate B — third attempt ready for the UI operator

Source **`85d8544f395580e26a5244ba290701d5f300008c`**, branch `codex/fort-openai-rebuild-20261016`, worktree
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
[Exact pins](pins.json), [approval](approval.json), [verification](verification.json)
and [launch/pairing/sample instructions](../../ops/candidate-b-third-20261016/MANUAL.md).

The owner approved one further five-minute attempt at 20:17 UTC on October 16,
after the canonical-feature compatibility correction. This packet prepares a
separate `third-20261016` identity and fixed root:
`/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-third-20261016`.
Only UI task `01a1451b-81d7-76cf-8e5c-a85d7164201c` runs and pairs it. Preparation
has not invoked the live controller or runtime; the third root is absent.

## Limits and preservation

The manual entry now pins this revision and selects the third identity/root.
It uses the existing one-start policy: consume the start before native freshness,
block every subsequent start including a reopened controller, and refuse another
attempt's record before native inspection or cleanup. No reset or directory
substitution is offered. The same five literal samples run in one session;
sample 5 proves no restart restoration. Work stops at 292 seconds, reserving eight
seconds inside the five-minute total for cleanup. The credential lifetime is
bounded by the same total; late cleanup blocks success.

Canonical explicit-false checks are byte-identical to source `6e0ebb69f559695d89446cc53b5c566db2fb84c8`.
The entire runner, launch settings/flags, model, diagnostics, service, native
cleanup and Apple source/app are unchanged. The existing agent-host versus
computer-sandbox limits remain; no OS confinement or effective-model attestation
is newly claimed. All failed checks stop without a second start or extra prompt.

[Both consumed attempts](attempt-preservation.json) remain byte-identical,
including their SQLite ledgers and blocked state. Previous packet, approval and
evidence files remain untouched. No database contents or provider credentials
were inspected. Main checkout status matches the saved recovery baseline.

## Validation and UI handoff

**56 affected nonlive tests passed, zero failed**, and strict TypeScript exit 0:
[test output](affected-tests.log), [typecheck](typecheck.log). The third-attempt
case first failed its deadline assertion ([red](session-red.log)); the manual
wiring first failed on its old root ([red](manual-red.log)). Final tests cover
five allowed samples, sixth refusal, one persisted start, no restart after reopen,
late cleanup, freshness/preflight refusal, cancellation, old-attempt rejection,
and unchanged canonical-feature safety checks. Red outputs are expected prior
failures, not unresolved checks.

Pins verify **459 source files and 5 unchanged app files**.
Source-list SHA-256: `414976ca416f54d569b05f27bca3c2e03d9fad75234742cdd2733fb8cc007115`.
App-list SHA-256: `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`.
The official executable's hash was checked without executing it. No rebuild was
needed; live startup, native pairing/Keychain, provider turns, UI signoff,
Swift/build reruns and independent review of this delta remain unrun here.

The UI operator verifies these pins, prepares automation handles, fully quits
the exact FortMac app and confirms native freshness before the sole start. Use
the existing approved process-inspection permission path without changing OS
settings. Run in a PTY:

```sh
cd /Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016
/opt/homebrew/opt/node@26/bin/node successor/prototype/manual.ts
```

Enter `start` exactly once when ready. Automatic local enrollment uses the third
root's `local-enrollment.json` through non-emitting CUA variables, never chat or
screenshots. Follow the linked full instructions for all five samples. Quit Fort
fully before `done`. On failure, retain only the safe stage/check and cleanup
result; abort/stop without retry. No fourth attempt is authorized by this approval.
