# Shared iPhone saved-history deletion

October 18, 2026; Spec 059, base bb2086d. Source, synthetic macOS execution and
unsigned iOS/Mac build evidence only. No real owner records, Keychain items,
provider, endpoint request, network listener, startup setting or account was used.

The existing explicit saved-copy action is now available on iPhone. Confirmation
captures the exact connection, and deletion requires confirmed disconnection,
absent credentials, no active/pending setup and unchanged private metadata. The
bounded traversal, no-follow/single-link checks, directory/file identities, ACL
checks and recovery records are shared unchanged. iOS SDK compilation confirms
the Darwin ACL symbols are available; no check was removed to make it compile.
Mac owner-host/startup actions remain platform-specific. Provider/host history
and other installations are retained. The model clears displayed data only after
successful local deletion; stale selection, unsafe state and interruption refuse
or retain a safe retry path. Same-local-network scope remains a planning/test limit;
this change opens no connection and makes no transport/security setting change.

## Tests and builds

The first new network-endpoint model expectation failed because the status said
“this Mac.” The action now uses device-neutral text. Four behavioral functions
passed, including 15 refusal and three changed/interrupted-state cases. The fifth
function reports success without measurement unless its benchmark variable is set;
it was run separately with that variable and all 20 samples were collected.

Shared deletion benchmark: 100 additional snapshots per disposable fixture,
20 serial warm samples, p50 43.374333 ms, p95 46.304375 ms, maximum 50.751584 ms,
500 ms budget. No concurrent build/test at measurement time. This is macOS timing,
not an iPhone performance claim. MacBookPro18,2, 10 CPU cores, 64 GiB, macOS 27.2,
Xcode 27.1/Swift 6.4. Full final native run: 253 executed functions passed and ten
optional measurement skips, 263 discovered, 50.353 seconds. Compiled core and
maintenance journeys were enabled with the existing explicit synthetic inputs.

Fresh unsigned Debug FortMac and iPhone Simulator FortiOS builds passed using
CODE_SIGNING_ALLOWED=NO. The package's FortKit-Package build-for-testing also
compiled/linked FortKitTests for arm64 and x86_64 iOS Simulator. An initial FortKit
scheme build succeeded but had no test configurations; it is not test-build proof.
No test was executed on iOS. Xcode reports CoreSimulator 1171.7.0 is older than its
required 1174.9.2 and disables simulator device support. Updating system tools is
outside this slice; no update or simulator startup was attempted.

Artifacts:
- /tmp/fort059-iphone-cache-ios/Build/Products/Debug-iphonesimulator/FortiOS.app:
  seven files, 16,873,262 bytes.
- /tmp/fort059-iphone-cache-mac/Build/Products/Debug/FortMac.app:
  seven files, 20,591,162 bytes.
- /tmp/fort059-iphone-cache-artifacts.json records all files, including Debug dylibs;
  SHA256 c127999a9c3964f2ffecef3b0b888a9feb60e6abf07dda08add9ff20015a74c7.
- /tmp/fort059-iphone-cache-testbuild/Build/Products/Debug-iphonesimulator/FortKitTests.xctest
  is a compiled synthetic test bundle, not an executed test result.

Commands: xcrun swift test --package-path successor/apple/FortKit --scratch-path
/tmp/fort059-maintenance-swift --filter savedHistory; benchmark uses
FORT_HISTORY_BENCHMARK=/tmp/fort059-iphone-cache-samples.json and the dedicated
savedHistoryDeletionBudget filter. Full native command/inputs match the preceding
history-maintenance.md evidence. xcodebuild used successor/apple/FortSpec058.xcodeproj,
FortiOS/FortMac schemes and generic/platform=iOS Simulator or macOS destinations;
package test build used FortKit-Package from its package directory.

## Remaining acceptance

Physical iPhone Data Protection/Keychain behavior, installed accessibility/screenshots,
provider inspection and TestFlight are unrun. The cache action does not delete host
history; durable owner-host deletion composition remains separate work. Portable
TypeScript is unchanged, so the preceding final 1,158-pass/six-skip run remains the
applicable portable evidence and was not repeated for a Swift-only change.

## Raw evidence integrity

- `/tmp/fort059-iphone-cache-red.log`: SHA256 `7250d476209571103b7c982bdb9262f61072ffc5b2988c8589118a46cc8d32e8`.
- `/tmp/fort059-iphone-cache-green.log`: SHA256 `53cb92dc120d14022d0e94e51d59cc8078efad6e75c9f2ecb19dfdc37e7afd62`.
- `/tmp/fort059-iphone-cache-budget.log`: SHA256 `f744a34d372be71b5c0240bc1146fa6e03d2c3145669b7a7ededc0ab614c51a3`.
- `/tmp/fort059-iphone-cache-native-full.log`: SHA256 `dafefd930d0ab09c25b4929aa1457dfc16067dd70b16b105a71eb6f7be755cd7`.
- `/tmp/fort059-iphone-cache-ios-build.log`: SHA256 `f0986d03ae465346bfee7d3fd7d0c1e619ffef60057e2ddfc821c38b745d84b3`.
- `/tmp/fort059-iphone-cache-mac-build.log`: SHA256 `093e53a1b23675eeb94d8b0cadf767288b1981c28a3f3d1e0a8231a13db5cd73`.
- `/tmp/fort059-iphone-cache-testbuild.log`: SHA256 `70cb21c35e2ee02dcf081f65542ff527f89b1d90ea3405f03452c72f252a1b9f`.
- `/tmp/fort059-iphone-cache-package-tests-build.log`: SHA256 `b743d8f51a67dc72fc7e56631c7f77fa24b3e19e178de25fc2d036bfab2d97d9`.
- `/tmp/fort059-iphone-cache-samples.json`: SHA256 `ba76cc1d0c470c467eef25f867fb9c09f08e0b2d14f6855406649cae353e26d8`.
