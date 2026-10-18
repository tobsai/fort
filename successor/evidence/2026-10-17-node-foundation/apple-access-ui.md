# Read-only Apple access and exact synthetic UI inspection

October 18, 2026, under [Spec 059](../../../specs/059-modular-node-v1.md), base
`09a7d6c`. Existing accessibility and screen-capture permission were verified with
read-only preflight calls. No setting, credential, enrollment, provider connection,
agreement, startup service or network grant was created or changed.

## Apple account prerequisite

No Apple connector was exposed. Inspection of the installed fastlane session path
metadata found zero cached session files; no session environment or CLI user was
selected. No session/credential contents were opened and no login tool was run.
Safari opened App Store Connect's Apps URL and showed its sign-in page, with an
empty Email or Phone Number field. The account did not authenticate automatically.
No sign-in button was pressed, credential entered or agreement accepted.

The smallest prerequisite is owner sign-in to App Store Connect in Safari using
the existing account, then read-only inspection can continue. This was requested
once while independent work continued. No response had arrived at this checkpoint.
Remote team `T3JB5MYZ93`, clients `io.mtree.fort.mac` and `io.mtree.fort`, proposed
helper `io.mtree.fort.mac.host`, and group `group.io.mtree.fort.mac` remain unverified.
Local absence is not evidence that a remote record is absent. No new agreement was
identified or accepted. Incidental browser menu labels from the initial dump were
removed from the retained account-page evidence and are not copied here.

## Exact running engineering app

Built `FortQA` with unique identifier `io.fort.qa.transport.20261018`, ad-hoc signing,
and no development team. Its compile-time fixture path excludes ordinary owner
setup and uses only the synthetic session
`4c07902a-0c9c-4b8a-93bd-56514aef2f4f`. Strict signature verification passed.
Entitlements contain App Sandbox and debug get-task-allow only: no App Group or
outgoing network permission. The exact bundle was launched by absolute path:
`/tmp/fort059-safe-ui-20261018/Build/Products/Debug/FortQA.app`.

Actual accessibility interaction typed and submitted synthetic text, observed a
saved simulated reply, opened Settings and verified those messages after normal
termination and cold restart. The first saved reply existed in the accessibility
tree but was entirely below the transcript viewport. The new bounded read-only
`apple/check-fixture-viewport.swift` check reproduced that defect:

| Exact synthetic reply | Visible fraction | Result |
| --- | --- | --- |
| `message-4ab50819-f6b2-42fa-a934-9e536ffe2810-reply`, before | 0 | Failed |
| Same reply after rebuilding and cold restart | 1 | Passed |
| New reply `message-20fe21dd-c844-43ce-b15b-a63769d7aba5-reply` | 1 | Passed |

The fix is one initial scroll anchor in the shared MessengerView. Explicit earlier
history retains its top anchor. There is no timer, provider/transport change or
new dependency. The assertion identifies one exact running fixture bundle/window
and compares its transcript/reply accessibility bounds; it does not press controls
or request new accessibility permission. Inspections completed in under one second.

Screenshots were captured by verified QA window ID only and visually inspected:
[before](qa-transcript-before.png), [after](qa-transcript-after.png), and
[settings](qa-settings.png). They contain labeled synthetic fixture content only.
Settings exposes the fixture catalog, agents and pairing/local-host entry. No
production removal, enrollment or account control was activated. A first text-field
AX value assignment did not update SwiftUI state; targeted keyboard input to the
exact QA PID corrected the inspection technique. A first termination identity check
refused because `/tmp` was not canonicalized; resolving the known bundle path then
allowed normal termination. No other application was closed. Final process check:
zero running apps with this unique QA identifier.

## Builds and artifact identity

Fresh QA and unsigned ordinary Mac/iPhone Simulator Debug builds passed with Xcode
27.1 / Swift 6.4. The unsigned ordinary clients were not launched. No simulator
execution was attempted: installed CoreSimulator 1171.7.0 remains incompatible
with Xcode's required 1174.9.2. No OS installation or update was initiated.

`/tmp/fort059-secure-ui-artifacts.json` records every file's path, mode, size and
SHA256. Manifest SHA256: `6ab4f68a9f147171ee36313f55632482fd99a406cbdf11015d7c6ed9260291f7`.
Per-app canonical inventory hashes:

| Exact bundle | Files | Inventory SHA256 |
| --- | --- | --- |
| `/tmp/fort059-safe-ui-20261018/Build/Products/Debug/FortQA.app` | 12 | `769c677c92903bb242de13006a996685f35f609ba1ea8eb0915383be97f1af06` |
| `/tmp/fort059-secure-ui-mac/Build/Products/Debug/FortMac.app` | 7 | `26cf1642617307990ee8569a73ca7b9411307313a38f14bb66b31fe497c97edd` |
| `/tmp/fort059-secure-ui-ios/Build/Products/Debug-iphonesimulator/FortiOS.app` | 7 | `dd8070e2eff914a2d60520de30417f61ea119f1e9287fcb0617c97f571a1d82b` |

The QA executable SHA256 is
`18b589b6337f9d7770250341994731e56829d46f16b05135c1c543ba59f8b7a7`;
its actual Debug implementation library SHA256 is
`15969b005ba6505be96937e9ab57e43875dcbdab67bb3a6e98cf4e8ee50fd9be`.
The first inventory attempt selected nonexistent generic Fort.app names and stopped;
the corrected inventory uses the actual target output names above.

## Evidence limits

This verifies an exact running synthetic engineering UI, not the production host,
Apple signing/account eligibility, Keychain/App Group/startup behavior, real
provider, physical iPhone or TestFlight. Those acceptance gates remain open. The
existing owner account must authenticate before its proposed records can be checked.
Network/TLS activation still needs its concrete per-action authority. No approval
was requested merely to fix source, run fixtures or record this checkpoint.

Retained logs `/tmp/fort059-safe-ui-viewport-red.log` and `...-green.log` have SHA256
`e50ddc36131704af56e03ee88e789cd0c59f62f56c9b45acdf1bbcb3ead7bd9e` and
`4d91d66fc4092643f671020da44e39f04833e59d7546c683189db06867968282` respectively.
Screenshots have SHA256 `d6f530177cb6abf7fb9cb950134e83f1e90ec0bca0832b99c8b6ad11f303a38b`
(before), `2595c07c73470f890333dd4a2bee6db1841876e46ee2bd39a5b277cd0ca1bbc7`
(after), and `14113599eb5099f1392fdbe719a652db20d6f940038f54956de29c5ffd637f11` (settings).
