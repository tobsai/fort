# Candidate B — blocked attempt and redacted diagnostics

Final diagnostic source: **`0e52d9215ed39f465a8e39cff0fcf828edb55e42`**, following
`d7641857dd7184d75149a1c9efa3c2c930109c33`, on
`codex/fort-openai-rebuild-20261016` in
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
[Current pins](pins.json) cover this nonlive revision. No live retry is authorized.

## What the original evidence establishes

The UI task `01a1451b-81d7-76cf-8e5c-a85d7164201c` ran the approved source
`24010dd0a1bb9024f6b00abc8cb0a1c3f95317c7` with evidence
`57f15830a0ec8f63a995354cb8f2cc60c7a240bc`. It accepted one start and failed closed.
The command record spans 7,740 ms, including setup and waiting for start. This
rules out the 292-second work cutoff; it is not a measured provider duration.

The [retained redacted attempt](blocked-attempt.json) records `blocked`, one
consumed start, zero receipts, no saved port, no enrollment file and credential
cleanup `deleted`. Creating that state occurs after native freshness checks.
Failure therefore happened after those checks and before a listener address was
saved/published. The UI task reported the exact runtime and app stopped, with
native storage empty. This diagnosis did not repeat those processes or inspect
provider configuration, auth storage, database contents or secret values.

**The exact original check cannot be recovered from the retained artifacts.**
`CodexChat.preflight`, `CandidatePacket.startLocked` and the manual entry replaced
errors with generic messages. Owned-process stderr was counted and discarded.
The state kept no checkpoint. There is no hidden failure log in the packet root;
its files were the state and SQLite ledger, with empty workspace/tmp directories.
The actual confirmed implementation defect is this loss of diagnostic evidence.

## Narrow hypotheses and their limits

The leading failure class is startup/configuration acceptance or confirmation:
an early CLI/config rejection could end the connection while `initialize` was
pending; alternatively the effective-config/feature checks could reject a missing
or unexpected field. Authentication/model inventory and explicit cancellation
remain possible. No retained trace ranks a precise check conclusively.

One concrete schema-compatible candidate is
`verifyEffectiveConfig` requiring `check_for_update_on_startup === false` when the
response omits that field. The pinned [Config response type](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/Config.ts)
permits extra fields but does not guarantee this one. A synthetic omitted-field
case now identifies that exact check and still rejects it. This is a reproduced
candidate failure, not evidence that the actual response omitted the field.

The launch overrides also create named disabled MCP entries without transport
values. The public [raw configuration schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/core/config.schema.json)
does not by itself establish that this was rejected. That hypothesis is unconfirmed;
the overrides and every other runtime setting remain unchanged. No guessed
configuration repair or relaxed check was applied.

## Minimal correction

Startup errors now carry only allowlisted `{stage, check}` pairs. These identify
launch, initialization, configuration fields, feature controls, MCP inventory,
account/model checks, listener/enrollment setup or cancellation/expiry. Arbitrary
provider names, values, endpoints, error messages, stderr and response payloads
are excluded. Unrecognized or forged errors receive a fixed fallback checkpoint.

The controller persists that pair through cleanup and the manual entry prints the
same redacted pair before its existing refusal message. Cancellation takes
precedence over a protocol rejection caused by that cancellation. Tests verify
that the original failure still consumes its start, blocks retry, disposes once,
and exposes neither enrollment nor a listener address.

This does not fix or claim to identify an unobserved provider configuration fault.
It preserves the diagnostic that the original code discarded. A future live
diagnostic would require separate authority; neither the old state nor its start
count was reset, and no second session was attempted.

## Checks and preservation

- **32 affected nonlive tests passed, zero failed**, [output](affected-tests.log).
  They exercise synthetic protocol frames and temporary loopback/SQLite fixtures.
- Missing-check preservation and protocol-check propagation failed before the
  fixes; [session red](session-red.log), [preflight red](preflight-red.log).
  Cancellation misclassification was also reproduced and corrected:
  [cancellation red](cancellation-red.log).
- Strict TypeScript passed, exit 0: [diagnostics](typecheck.log).
- **455 source files and five unchanged app files** are pinned. The launcher,
  process ownership/cleanup, native cleanup, service and Apple source are unchanged.
  Runtime settings, preflight predicates, admission and all time/message limits
  remain the same. Main checkout status matches the recovery baseline.
- The actual live state remains byte-identical, SHA-256
  `19be731c00c39548b57717256780a48532029cc20443e6900e68b8f6917abafe`.
  No real controller/provider, enrollment, credential access, OS setting change,
  deployment, reset or retry occurred during this work.
- Broad Node/Swift/build/UI checks were not rerun; the affected suites and strict
  typecheck are this revision's validation. No independent review or successful
  live test is claimed. Original approval and execution evidence remain intact.
