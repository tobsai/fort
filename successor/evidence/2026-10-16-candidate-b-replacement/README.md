# Candidate B — prepared replacement attempt

Source **`f7d7c9f1f8943f6a49708a30ef1208748a25b8d9`**, branch `codex/fort-openai-rebuild-20261016`, worktree
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
[Exact pins](pins.json), [approval](approval.json), [verification](verification.json)
and [UI launch instructions](../../ops/candidate-b-replacement-20261016/MANUAL.md).

The owner approved one separate five-minute replacement at 19:34:56 UTC on
October 16. The original failed attempt remains blocked and is not reset or
relabelled. The designated UI task executes the replacement; this preparation
has started no controller, official runtime, native app, enrollment or provider
turn. Its fixed directory does not yet exist and is created only by the controller.

## Controller change

The manual entry selects a dedicated directory and persisted replacement identity.
The sole start is consumed before native freshness inspection. A failed freshness
check, runtime preflight, cancellation, crash or any persisted state prevents a
second start, including after constructing a new controller. The replacement
cannot adopt or clean up an original packet, even one marked ready for restart.
The original two-session behavior is retained only behind the historical policy;
the live entry selects the one-start replacement.

The same five literal messages, exact model, Agent/binding identities, instructions,
skill and conversation checks run within one session. Sample 5 checks same-session
recall only. There is no restart or restoration claim. All runtime launch settings,
strict preflight predicates, per-turn controls, stop diagnostics, native cleanup
implementation, service implementation and Apple source/app remain unchanged.

Work stops at 292 seconds, with eight seconds reserved for cleanup inside the
five-minute total. Native freshness consumes that same budget. The credential
expires no later than 300 seconds from start and is never retained for restart.
Success also requires cleanup to finish by that deadline; simulated late cleanup
is blocked. Timer supervision remains subject to the previously disclosed host
limitations and is not hard-real-time containment. Same-user startup effects
remain possible before configuration can be checked.

## Tests and exact preservation

**36 affected nonlive tests passed, zero failed**, plus strict TypeScript exit 0
and `git diff --check`. [Test output](affected-tests.log), [typecheck](typecheck.log).
Public session/HTTP behavior covers the five allowed samples, sixth rejection,
one start after reopen, expired/failed preflight, cancellation, native freshness
refusal, deadline and cleanup. Inert manual-entry checks confirm the new root,
policy and pins without launching the provider. No real credentials are read.

The red cases capture the original longer credential window ([session](session-red.log)),
late cleanup incorrectly reporting success ([cleanup](cleanup-red.log)), old manual
root ([wiring](manual-red.log)), and repeatable native freshness refusal
([freshness](freshness-red.log)). All corresponding final checks pass. Red logs
are expected failures before their corrections, not unresolved final failures.

[Original preservation](original-preservation.json) verifies both original files
byte-for-byte; only file hashing was used, never a database query. State SHA-256:
`19be731c00c39548b57717256780a48532029cc20443e6900e68b8f6917abafe`.
Ledger SHA-256:
`418125696b3f6ebcf34e1e3defb5f2fb3e3e531804d8f8d91062dc03c6ef93d0`.
Original approval, packet and evidence are unchanged. Main checkout status matches
the recovery baseline. The only unrelated worktree residue remains `.build/`.

Pins verify **457 source files and 5 unchanged app files**.
Source-list SHA-256: `7b87c8cd3f2cccc042014cb623ea4b591c700e0c8d51c4c44bf6930724fb543d`.
App-list SHA-256: `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`.
The app remains the existing build from `de9e482bac99920d52a03b6904542a78226c5ba2`;
no rebuild was needed or claimed.

Live provider startup/turns, native pairing/Keychain behavior, installed UI
accessibility/screenshots, Swift/HTTP reruns and independent review of this delta
are **unrun**. The previous failure's exact cause is still unknown; strict
preflight may stop the replacement and will now retain its redacted check.
No configuration bypass, extra start or automatic retry follows that result.
