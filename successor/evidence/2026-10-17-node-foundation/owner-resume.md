# Explicit owner resume

Source base: `3cc5fcfce2532b8328bd1c16bef1266d3db076af`, followed by this local
slice on October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md)
and the existing owner decision govern it. The required
[Ponytail report](../../../docs/architecture/reviews/ponytail/current.md)
records semantic review and binds the staged contents and modes.

## Behavior and boundaries

Stop pauses dispatch and schedules durably and disposes the selected runtime.
The local control channel stays available for history, status and recovery.
Native settings and conversation status offer an explicit Resume local work
action. Reopening preserves pause; an exact authorized request to the existing
runtime-restart route resumes work without reenrollment or startup registration.

Resume requires the current installation, recovery epoch and latest owner pairing.
Failed cleanup, pending disconnect/cancellation, overlapping control operations,
expired credentials and temporary sessions cannot use it. Authorization is checked
again after asynchronous runtime restart, before committing running intent. Failed
resume disposes the runtime and retains pause. Close waits for that owned operation.

The guarded stateless executor re-enables dispatch only after confirmed shutdown
and process absence. Resume itself launches no child or changes any pinned runtime,
model, arguments, environment, tool permissions or account. Native lost replies are
reconciled against authoritative readiness. History and stored credentials are kept.
Agent-host access remains distinct from the computer-tool sandbox.

## Verification

| Check | Result and limit |
| --- | --- |
| Public red tests | Service control disappeared after Stop; executor did not advertise resume; native restore lost control or allowed sending while paused. Logs: `/tmp/fort059-owner-resume-red.log`, `/tmp/fort059-executor-resume-red.log`, `/tmp/fort059-native-resume-red.log`. |
| Review regression | Expiry during a held resume returned 200 before the fix; after rechecking the same credential it returns 503 and disposes the runtime. `/tmp/fort059-resume-expiry-red.log`, `/tmp/fort059-resume-expiry-green.log`. These focused commands used shell Node 22; the final pinned suite below includes the same test. |
| Final portable suite | Node 26.5.0: 1,097 passed, zero failed, 5 optional budgets skipped. `/tmp/fort059-resume-portable-pinned.log`. |
| Native regressions | 235 functions passed, with timing, signed probes and the complete host journey excluded for separate execution. `/tmp/fort059-resume-native-final.log`. |
| Affected Mac process checks | 27 passed, 3 optional budgets skipped. `/tmp/fort059-resume-guarded-mac.log`. |
| Final compiled native journey | 1,496.871292 ms, below 5,000 ms. Stop/read/resume, paused reopen, exact receipt replay, new send, revoke, and all six owned process groups absent. `/tmp/fort059-resume-journey-reviewed.log`. |
| Strict TypeScript | Passed on Node 26.5.0 with TypeScript 6.0.3. `/tmp/fort059-resume-typecheck-pinned.log`. |
| Native engineering builds | Fresh unsigned FortMac and FortiOS Simulator Debug builds passed. `/tmp/fort059-resume-mac-build.log`, `/tmp/fort059-resume-ios-build.log`. |
| Architecture gates | Full structural and accepted implementation checks passed; exact staged spec/review gates run before commit. |

The isolated journey used MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2, Swift 6.4
and the selected Homebrew Node 26.5.0. No other task builds/tests ran concurrently.
This is one complete synthetic journey, not a percentile benchmark. Signature,
App Group, listener and Codex behavior are authored fixtures; provider time and
build time are excluded. It does not establish real sandbox, provider or UI acceptance.

Fresh resources: `/tmp/fort059-resume-reviewed-resources`, 87 files and 364,332
payload bytes; every size/hash verified. Manifest SHA256:
`d86676a3568209eeebc401ce7a1b6403ffb77f58aeeb695e30cc91e43192377c`.
The retained guardian is `/tmp/fort059-review-guardian`.

Unsigned executable hashes (not signatures or complete bundle hashes):

- `/tmp/fort059-resume-mac-build/Build/Products/Debug/FortMac.app/Contents/MacOS/FortMac`:
  `9b0693770172e2eab7b8769da32eed0b3c6cd83b512a90d0288586107690ce71`.
- `/tmp/fort059-resume-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app/FortiOS`:
  `ff501f08ec4fb3726ff1c8c4b3852c8011844a174f88c815ae7484dd2f2db255`.

## Failed attempts and corrections

The first portable regression retained two old `unavailable` assertions after the
public paused state became `stopped`; updating those expectations preserved the
dispatch-denial checks. A parallel native run exposed the existing setup fixture's
10 ms deadline preceding its durable commit. The fixture now holds its response;
the test observes commit before timeout and releases the response before exact retry.
Production deadlines are unchanged. Those logs remain at
`/tmp/fort059-resume-portable.log` and `/tmp/fort059-resume-native-full.log`.

The expanded complete journey initially attempted pairing while paused. Phase
diagnostics identified the failing step. Moving its existing paired-receipt check
after explicit resume fixed the test without weakening the setup guard. The failure
is retained in `/tmp/fort059-resume-native-journey-diagnostic.log`. An intermediate
Swift diagnostic used a String instead of Testing.Comment; interpolation fixed it.

The first final-suite/resource commands used shell Node 22.22.3 and were refused
by the existing version checks before tests or output creation. The corrected
commands selected `/opt/homebrew/opt/node@26/bin/node`; the portable suite, strict
typecheck, resource build and final journey then passed. The refused suite log is
`/tmp/fort059-resume-portable-reviewed.log`.

## Remaining acceptance

Baseline finding PT1 is resolved in source, behavior tests and engineering builds.
Installed accessibility/screenshots, real provider, Keychain/startup, exact Apple
profiles, physical iPhone and TestFlight remain unrun. No new credential, account,
persistent grant, provider process, signing, deployment or cleanup was performed.

Re-pairing after a completed disconnect still encounters the persisted pause and
requires a separate tested local lifecycle change. History deletion remains disabled
pending its exact scope. Concrete standalone runtime packaging and Apple/account
gates remain open. This is not a working installed product or v1 completion.
