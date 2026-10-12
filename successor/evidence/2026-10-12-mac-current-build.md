# Current Mac artifacts ready for attended tests — 2026-10-12

**Build passed; Mac installed acceptance remains pending local owner authentication.**
No app, XCTest runner or UI test was launched. Automation Mode was neither queried
nor requested or enabled during this build-only follow-up.

Built source: `986afd30f2967287764c5e10f11282a0e4f33a22`, branch
`codex/fort-core-openmaus-20261011`, worktree `/tmp/fort-core-openmaus-20261011`.
This includes the `d044da9` closure-during-refresh fix and `b99d89e` installed
journeys. The coordinating review reports this source has no new actionable
findings, with 19 Swift tests, strict TypeScript and the iPhone/race evidence
confirmed. This follow-up does not rerun those tests or claim Mac UI acceptance.

## Prepared artifacts

All are in the fresh `/tmp/fort058-mac-current-986afd3-build` directory:

| Artifact | Path relative to that directory |
| --- | --- |
| Mac app | `Build/Products/Debug/FortMac.app` |
| UI test runner | `Build/Products/Debug/FortMacUITests-Runner.app` |
| UI test bundle | `Build/Products/Debug/FortMacUITests-Runner.app/Contents/PlugIns/FortMacUITests.xctest` |
| Test configuration | `Build/Products/FortMac_macosx27.0-arm64.xctestrun` |

Full build log: `/tmp/fort058-mac-current-986afd3-build.log`.
[Hash and signature record](native-2026-10-12/mac-current-build-hashes.json)
contains the exact source SHA, executable/debug-dylib/resource hashes, test
configuration hash, log hash, verified signing metadata and entitlements.

`xcodebuild build-for-testing` returned zero and `TEST BUILD SUCCEEDED` under
Xcode 27.0 (27A266a), arm64 macOS, Debug. The project has only local FortKit as a
package dependency and no custom shell-script build phases or package plugins.
No package, source, project setting, entitlement or signing configuration changed.
The two build warnings only report skipped AppIntents metadata extraction because
neither target depends on AppIntents.framework. There were no build errors.

Strict/deep code-signature verification passed for the app, runner and test bundle.
All use the same existing Apple Development identity
`58F40D1AB480565B89109B853235AC3A7B2E7AD1`, team `T3JB5MYZ93`.
Their entitlements exactly match the prior successful development-signed products.
The generated test configuration points to these new products and contains no
fixture credential values. Normal signing completed without a new permission
prompt, certificate, key, profile, account grant or provisioning update.

All 11 previously recorded hashes still match across the older Mac product,
older iPhone product and latest iPhone acceptance product. Those build directories,
signed artifacts, test results and the Mac authentication diagnosis are preserved.

## Owner-attended next command — not executed

Once the owner is present at the logged-in Mac, start one normal recovery test:

```sh
cd /tmp/fort-core-openmaus-20261011
/opt/homebrew/opt/node@26/bin/node successor/apple/native-ui-check.ts \
  /tmp/fort058-mac-current-986afd3-build/Build/Products/FortMac_macosx27.0-arm64.xctestrun \
  'platform=macOS,arch=arm64' \
  /tmp/fort058-mac-986afd3-owner-recovery-1.xcresult recovery
```

The result path is unused as of this record; use a fresh result path on any later
attempt. The harness creates an ephemeral authenticated loopback fixture and
runs the already built artifacts, so no build step is needed. This is the current
source replacement for the older-product command in the
[Mac authorization diagnosis](2026-10-12-mac-authorization-diagnosis.md).

The owner must inspect and resolve any actual normal macOS UI Automation
session-authentication prompt locally. The exact prompt has not been captured.
If it asks for a different permission or scope, stop and retain the prompt for
review. Do not change security policies or use an authentication bypass. The
prior diagnosis and its stop condition remain in force. Approval/cancellation,
unknown closure and all current-artifact Mac installed checks still require their
own attended execution and evidence; compilation alone does not pass them.

## Reproduce the build only

The successful command used the existing signing workflow unchanged, with only
the output directory changed. Choose another fresh directory before rebuilding:

```sh
xcodebuild build-for-testing \
  -project successor/apple/FortSpec058.xcodeproj -scheme FortMac \
  -destination 'platform=macOS,arch=arm64' \
  -derivedDataPath /tmp/fort058-mac-current-986afd3-build \
  CODE_SIGNING_ALLOWED=YES CODE_SIGNING_REQUIRED=YES CODE_SIGN_STYLE=Manual \
  CODE_SIGN_IDENTITY=58F40D1AB480565B89109B853235AC3A7B2E7AD1 \
  DEVELOPMENT_TEAM=T3JB5MYZ93 PROVISIONING_PROFILE_SPECIFIER= PROVISIONING_PROFILE=
```

No provider login, deployment, push, startup service or external account
connection occurred. The donor quarantine and all broader Spec 058 gates remain
unchanged. This record establishes prepared signed artifacts only.
