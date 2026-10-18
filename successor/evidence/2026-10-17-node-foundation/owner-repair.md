# Explicit pairing after disconnect

Source base: `b2b005c2b7fd896c434fe06e8e98a00bb3a22f15`, followed by this local
slice on October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md)
and the existing owner implementation decision govern it. The
[Ponytail report](../../../docs/architecture/reviews/ponytail/current.md)
binds reviewed staged content and modes.

## Behavior

After confirmed Disconnect, explicit local setup can offer a new pairing while
dispatch remains paused. Completing the exact offer prepares the selected runtime,
revalidates the offer and new device, and atomically saves pairing and running intent.
Only successful completion resumes dispatch. Begin, inspection, cancellation and
reopen do not. Old credentials remain revoked and conversation history stays intact.

The existing trusted setup channel carries this operation; no HTTP route, XPC
command, bootstrap option or permission was added. A normal Stop with retained
pairing still requires authenticated resume. Pending cancellation/revocation,
unsupported runtime preparation, failed cleanup and overlapping work remain barriers.
Offer expiry during preparation or concurrent close prevents credential commit.
Lost replies can retry the same prepared identity and token.

Setup completion is now asynchronous. All source callers and test fixtures were
migrated; the private pipe already awaits its callback. Authenticated resume and
new pairing share the existing bounded runtime preparation and close coordination.
The host process and its computer-tool sandbox remain separate boundaries.

## Verification

| Check | Result |
| --- | --- |
| Public red | Same-core and reopened setup both failed with hostUnavailable before implementation. `/tmp/fort059-repair-red.log`. |
| Initial green | Same-core/reopened re-pair plus existing resume regressions passed. `/tmp/fort059-repair-green.log`. |
| Setup/control regression | 54 passed, including seven re-pair tests, migrated setup, resume and private-channel behavior. `/tmp/fort059-repair-controls-final.log`. |
| Full portable suite | 1,104 passed, zero failed, five optional budgets skipped. Node 26.5.0. `/tmp/fort059-repair-portable.log`. |
| Native regression | 232 functions passed. All Budget-named, retained-signature and complete-journey functions were excluded from this parallel run. `/tmp/fort059-repair-native-full.log`. |
| Compiled native journey | Passed separately in 1,668.438 ms against 5,000 ms, with owned groups absent. `/tmp/fort059-repair-journey-final.log`. |
| Strict TypeScript | Passed. `/tmp/fort059-repair-typecheck.log`. |
| Resource artifact | 87 payloads, 365,144 bytes; every manifest size/hash verified. `/tmp/fort059-repair-host-resources`. |

The native count differs from the prior 235-function run because this run also
excludes three unchanged timing functions: setup inspection, setup preparation
durability and setup recovery durability. Their prior evidence is preserved.
The complete journey covers same-core resume, paused reopen, exact replay, ordinary
disconnect, new pairing, retained history, another send, revocation of the new
pairing, full host shutdown and all recorded process/group absence checks.

Journey conditions: MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2, Swift 6.4;
selected Homebrew Node 26.5.0, fresh compiled resources, retained synthetic guardian.
One isolated sample, no concurrent task tests/builds. Authored signature, container,
listener and Codex fixtures; build/provider time excluded. This is not a percentile
benchmark or installed/official-provider evidence.

Resource manifest SHA256:
`8a66baac0749239f052c7201fffb7d3cb946f2b12c7ece72c26a68aa17ca96ee`.
Native product source did not change in this slice. The fresh unsigned Mac/iPhone
builds from [owner resume](owner-resume.md) remain the client artifacts; no fresh
client build, signing or installation was claimed here. The changed native test
and core resources were rebuilt and executed.

## Failed attempts

The first caller migration left an await inside a synchronous test callback in the
corrupt-state test. Node rejected that file before its tests ran. Converting that
callback to the asynchronous rejection assertion fixed it; all 54 focused checks
then passed. `/tmp/fort059-repair-controls.log` preserves the failed invocation.

The first expanded native journey used full host recovery to simulate Disconnect.
That operation deliberately closes the core, so its next setup call was unavailable.
Using the ordinary client's Disconnect request matches the actual app action.
Full host recovery remains at final cleanup, with all absence assertions retained.
The initial failure is `/tmp/fort059-repair-journey.log`.

## Remaining gates

The reviewed re-pairing source gap is resolved. Completed uninstall/reinstall
reconciliation and the exact history-deletion scope remain separate work; this
change cannot clear removal records. The production removal path keeps history.
Standalone runtime/package legal closure, Apple identity/profile/account setup,
persistent startup approval, actual provider and Keychain use, installed UI,
physical iPhone and TestFlight acceptance remain unverified. No real credentials,
account connections, provider starts, signing, registration, deployment or cleanup
occurred. Source/fixture acceptance is not v1 or installed-product signoff.
