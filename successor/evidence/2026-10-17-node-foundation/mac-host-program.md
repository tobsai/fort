# Fixed helper program and engineering target

October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md) and
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md)
govern this source/build preparation. No helper executable, App Group, named
listener, startup registration, provider or installed client was activated.

## Program boundary

`MacHostProgram.inspect()` uses only the running bundle's URL. It derives the
containing app, performs the existing complete parent/helper inspection, and
requires the inspected helper URL to match. Callers cannot supply another app,
runtime path, argument list or environment through this public entry.

Fixed paths inside the helper are `Contents/MacOS/node`,
`Contents/MacOS/FortProcessGuardian` and `Contents/Resources/host/main.mjs`.
Canonical/no-symlink, regular/single-link, nonempty and non-writable-to-group/world
checks precede use. Limits are 512 MiB for Node, 8 MiB for the guardian and
256 KiB for the bootstrap. Native tools also require executable permission.

The tools require Apple-anchored signatures under the inspected helper's team,
with identifiers `<serviceIdentifier>.node` and `<serviceIdentifier>.guardian`.
Each must declare exactly App Sandbox plus inheritance, both true, and no App
Group. The existing metadata reader now exposes the entitlement key set; parent
and helper entitlement requirements remain unchanged. Whole-bundle signatures
cover resources, but this inspection is not an atomic update lock. Embedded
profile presence remains explicitly unverified rather than account eligibility.

The inspected program constructs `MacAgentService` with fixed Node flags
`--jitless --no-experimental-strip-types --no-warnings`, the bundled bootstrap,
an empty environment and exact guardian. `FortHost` handles SIGTERM by canceling
pending startup and awaiting that lifecycle's shared shutdown. Failure prints a
generic diagnostic and exits nonzero. It does not register itself or choose a
fallback runtime. Actual signal delivery to this target is unrun.

## Engineering target

XcodeGen 2.45.4 added only the `FortHost` target, its source group/product and
shared scheme. Existing targets/schemes were preserved. The target uses local
engineering identifier **`io.fort.spec058.fixture.mac.host`**, App Sandbox enabled,
outgoing networking disabled and `LSBackgroundOnly=true`. This is not adoption
or registration of the proposed production helper identifier or App Group.
The target is not automatically embedded into the Mac client.

The fresh unsigned helper's generated Info.plist was independently inspected:
identifier `io.fort.spec058.fixture.mac.host`, executable `FortHost`, background-only
true. Its profile, `fort-host.json`, Node, guardian and bootstrap are absent.
It therefore cannot pass program validation or act as a complete host package.
No dummy runtime was installed to disguise those missing resources.

## Verification and preserved failures

Logs share `/tmp/fort059-host-program-`.

- Initial tests failed to compile with the missing program type and entitlement
  key field (`red.log`). After implementation, a nested integer expression in
  the oversized-file test exceeded Swift's typechecking limit
  (`first-green.log`). Replacing it with an explicitly typed limits array fixed
  the test. No compiler setting or security check was weakened.
- Five program test functions passed, then negative code-authority cases were
  expanded to both tools. Final coverage: **37 cases** across exact layout/code
  requirements, invalid inherited sandbox/group/extra keys/signature, missing/
  empty/directory/symlink/hardlink/writable/oversized files, native execute bits,
  and public rejection of the non-helper test runner (`final-behavior.log`).
- Full native regression: **227 functions passed**, zero failures
  (`native-final.log`). Product source was final; the later test-only expansion
  to both tool negatives passed separately. Isolated budgets and real-signature
  probe were excluded from the combined run.
- Strict TypeScript and structural validation passed (`types.log`). No TypeScript
  product source changed; the preceding 1015-test portable run is separate evidence.
- A read-only Security-framework probe on the retained signed synthetic app
  verified its actual entitlement keys, correct identity/team, rejection of wrong
  identities and rejection of a tampered temporary copy. The original remained
  unchanged and the temporary copy was removed (`signature.log`, one function
  passed). This did not execute Codex or the app and created no new signature.
  It does not validate a complete signed production helper or the new derived
  Node/guardian identifiers, which remain synthetic test evidence.
- Fresh unsigned Debug builds passed for helper, Mac and iPhone Simulator
  (`helper-build.log`, `mac-build.log`, `ios-build.log`). Artifacts:
  `/tmp/fort059-host-program-helper-build/Build/Products/Debug/FortHost.app`,
  `/tmp/fort059-host-program-mac-build/Build/Products/Debug/FortMac.app`, and
  `/tmp/fort059-host-program-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
- CoreSimulator remains 1171.7.0 versus Xcode's required 1174.9.2. Simulator device
  execution is unavailable; build success does not repair that gate.
- Full structural, accepted-implementation, exact staged-path and whitespace
  checks passed before committing. Generated project changes were reviewed.

## Isolated metadata measurement

Recorded host/toolchain: M1 Max, 10 cores, 64 GiB, macOS 27.2/26B5112a, Swift 6.4,
Xcode 27.1/27A9275. Hardware is the prior inventory. Twenty serial inspections of
one freshly created synthetic app/helper layout used warm filesystem caches and
fake signature results. Configuration/plist/profile-presence/file metadata checks
are included. Setup/removal, real signature work, process launch, provider, network
and installed App Group/launchd work are excluded. No other builds/tests ran.

p50 **0.730 ms**, p95 **0.778 ms**, maximum **1.637 ms**; metadata target p95 <=50 ms.
The complete signed-program target of <=2 seconds remains unmeasured.
Samples/log: `/tmp/fort059-host-program-budget.json` and `.log`.
Command selects only `hostProgramMetadataInspectionBudget` with
`FORT_HOST_PROGRAM_BENCHMARK` set to the new output path.
Rounded display samples, milliseconds:

```text
1.637 0.777 0.754 0.734 0.737 0.726 0.720 0.730 0.726 0.725
0.729 0.743 0.747 0.766 0.778 0.730 0.715 0.719 0.718 0.724
```

## Remaining

The runtime bootstrap/configuration and complete package still need composition;
there is no working installed helper yet. Exact release identities, App Group,
profiles, actual runtime/library/legal closure and provider/account access remain
behind their recorded gates. No new signing, credentials, registration, upload,
provider cleanup, installed accessibility/screenshots or TestFlight result is claimed.
