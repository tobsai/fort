# Fort for Mac — build, sign, notarize, distribute (spec 032)

Fort ships a native macOS app (`FortMac`) that both **runs** Fort — a `fort
service` launchd daemon manager — and **drives** it — a windowed SwiftUI
Agent Channels chat client with nested Conversations, Needs You, and Settings
over the typed HTTP/SSE contract. Primary Channels remain the closed rollback
presentation.

The app has two scenes:
- a **menu-bar** item (`MenuBarExtra`) — open-Channel counts plus current
  recoverable Needs You rows;
- a **window** (`Window("Fort")` → `FortNativeChatView`) — the shared native
  agent-first surface, with daemon controls in Settings.

Both share one `FortClient` (default `http://127.0.0.1:4087`). The window's
"Service" section is wired to `FortKit.ServiceController`, which reads status
and invokes only Install, Start, or Restart recovery through the bundled `fort`
binary. Stop and Uninstall remain CLI-only administrative commands (see
`cmd/fort/service.go`).

> **Notarization is the operator's step.** Signing, notarization, and the
> Developer ID certificate require Apple credentials and cannot run headless / in
> CI. Everything up to that point (compile) is verified with
> `xcodebuild … CODE_SIGNING_ALLOWED=NO`.

## Prerequisites (once)

- **Xcode** (26+) and command-line tools; **XcodeGen** (`brew install xcodegen`).
- **Apple Developer account** on team **T3JB5MYZ93** (Maple Tree Enterprises LLC).
- A **"Developer ID Application"** signing certificate in your login keychain
  (Xcode → Settings → Accounts → Manage Certificates → +). This is the cert for
  distribution *outside* the Mac App Store — different from the App Store cert
  used for the iOS TestFlight build.
- A **notarytool keychain profile** holding an app-specific password
  (appleid.apple.com → Sign-In and Security → App-Specific Passwords):

  ```sh
  xcrun notarytool store-credentials fort-notary \
    --apple-id <YOUR_APPLE_ID> \
    --team-id T3JB5MYZ93 \
    --password <APP_SPECIFIC_PASSWORD>
  ```

  `fort-notary` is the default profile name the Makefile expects
  (`NOTARY_PROFILE`).

## Generate the project

The `.xcodeproj` is git-ignored and regenerated from `ui/apple/project.yml`:

```sh
cd ui/apple && xcodegen generate
```

The `FortMac` target uses an explicit source allowlist: `FortMacApp.swift`,
`MenuContent.swift`, and `Assets.xcassets`. Its local FortKit package supplies
the shared `PrimaryChannelsView`; adding an unrelated Swift file does not add
it to the shipping app.

## Compile-verify (no signing, CI-safe)

```sh
cd ui/apple/FortKit && swift build
cd ui/apple && xcodegen generate
xcodebuild -project Fort.xcodeproj -scheme FortMac \
  -destination 'platform=macOS' CODE_SIGNING_ALLOWED=NO build
```

Expect `** BUILD SUCCEEDED **`. (`make apple-build` runs this for every Apple
target.)

## Isolated DEBUG UI QA

FortMac DEBUG builds accept `FORT_DIRECT_HOST_URL` as a direct HTTP(S) client
base URL. This is intentionally compiled out of Release builds and changes only
the in-process `FortClient`; it never installs or restarts the launchd service.
Use an isolated fixture rather than port 4087:

```text
Xcode scheme → Run → Arguments → Environment Variables
FORT_DIRECT_HOST_URL = http://127.0.0.1:4187
```

The macOS UI test uses the same launch environment and defaults to port 4187.

## The bundled `fort` daemon

`FortMac` ships the `fort` binary inside the app at
`Contents/Resources/fort`. `ServiceController` prefers that co-located binary
(`Bundle.main.url(forResource:"fort", withExtension:nil)`) and falls back to
`/opt/homebrew/bin/fort` for a dev build off the menu-bar app. The DMG flow
copies the `make build` output into the archived app **before** signing so the
daemon is covered by the app signature:

```sh
make build   # -> ./bin/fort  (pure-Go, no cgo)
```

## Build the signed, notarized DMG

One target does archive → inject the daemon → export a Developer ID–signed
`.app` → DMG → notarize → staple:

```sh
make mac-dmg VERSION=<release-version> AGENT_CHANNELS_MODE=primary
                                            # -> ./build/Fort.dmg
# override the notarytool profile name if you didn't use the default:
make mac-dmg VERSION=<release-version> NOTARY_PROFILE=my-profile
# if the keychain contains duplicate certificate names, select one by SHA-1:
make mac-dmg VERSION=<release-version> DEVELOPER_ID=<CERTIFICATE_SHA1>
```

Do not omit `VERSION`: the Makefile's development default is not a release
identity, and that value is embedded in the bundled daemon. Source and the
Makefile default Agent Channels to `off`; the explicit `primary` value above is
embedded in the signed app archive. Verify the exported app's Info.plist before
distribution.

What it runs (see the `mac-dmg` target in the `Makefile`):

1. `make build` → `./bin/fort`.
2. `xcodebuild … -scheme FortMac … archive` → `build/FortMac.xcarchive`.
3. `cp bin/fort …/FortMac.app/Contents/Resources/fort` — bundle the daemon.
4. `xcodebuild -exportArchive … -exportOptionsPlist ExportOptions-mac.plist`
   → a Developer ID–signed `build/FortMac-export/FortMac.app`.
5. Harden and Developer ID–sign the bundled daemon, then reseal and verify the
   outer app.
6. `hdiutil create … build/Fort.dmg` (swap in `create-dmg` for a styled DMG).
7. Developer ID–sign and verify the DMG container.
8. `xcrun notarytool submit build/Fort.dmg --keychain-profile fort-notary --wait`.
9. `xcrun stapler staple build/Fort.dmg`.

`ui/apple/ExportOptions-mac.plist` selects `method: developer-id` (distribution
outside the App Store). The sibling `ExportOptions.plist` is for the iOS App
Store upload — do not confuse the two.

## Verify a downloaded DMG

```sh
spctl -a -vvv -t install /Volumes/Fort/FortMac.app   # Gatekeeper acceptance
xcrun stapler validate build/Fort.dmg                # ticket is stapled
```

## Why this can't run in CI

Steps 4–7 need the operator's Developer ID certificate and Apple ID. The
notarytool submission talks to Apple's notary service with credentials that must
not live in CI. This runbook is intentionally a Makefile target the operator runs
locally; the compile gate (`CODE_SIGNING_ALLOWED=NO`) is the part CI verifies.

## Run locally without notarization (own Mac, no Developer ID)

Notarization (above) is only needed to distribute to *other* Macs. To run FortMac
on the machine that builds it, an **Apple Development** cert is enough — no
Developer ID, no notarytool. Verified 2026-07-12 on Xcode 26.6:

```sh
make build                                             # -> ./bin/fort
cd ui/apple/FortKit && swift build && cd ..
xcodegen generate
xcodebuild -project Fort.xcodeproj -scheme FortMac -configuration Release \
  -destination 'platform=macOS' -derivedDataPath build/mac-dd \
  CODE_SIGN_STYLE=Automatic DEVELOPMENT_TEAM=T3JB5MYZ93 build

APP=build/mac-dd/Build/Products/Release/FortMac.app
ID=$(security find-identity -v -p codesigning | awk '/Apple Development/{print $2; exit}')
mkdir -p "$APP/Contents/Resources" && cp ../../bin/fort "$APP/Contents/Resources/fort"
codesign --force --options runtime --timestamp=none --sign "$ID" "$APP/Contents/Resources/fort"
codesign --force --timestamp=none \
  --entitlements <(codesign -d --entitlements :- "$APP" 2>/dev/null) --sign "$ID" "$APP"
cp -R "$APP" /Applications/FortMac.app && open /Applications/FortMac.app
```

The daemon must be injected into `Contents/Resources/fort` and signed **before**
the outer bundle is re-signed (adding a file invalidates the signature). FortMac
is a windowed app with a Dock icon and also supplies its MenuBarExtra glance.

## Deferred trust boundary

FortMac connects only to the same-host daemon over loopback. It does not expose
a gateway sign-in, remote-machine picker, mesh roster, Command Deck, or legacy
admin surface. A remote Mac connection experience requires its own approved
product and trust contract; the iPhone's authenticated gateway flow does not
silently broaden the Mac UI.
