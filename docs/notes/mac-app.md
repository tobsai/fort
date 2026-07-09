# Fort for Mac — build, sign, notarize, distribute (spec 032)

Fort ships a native macOS app (`FortMac`) that both **runs** Fort — a `fort
service` launchd daemon manager — and **drives** it — a windowed SwiftUI mirror
of the 031 dashboard (Define / Ready / In progress, gate approve-reject, live
per-run activity) over the existing HTTP/SSE control-plane contract.

The app has two scenes:
- a **menu-bar** item (`MenuBarExtra`) — the glanceable summary + gate inbox;
- a **window** (`Window("Fort")` → `FortWindow`) — the sidebar (service controls
  + machine roster) beside the dashboard.

Both share one `FortClient` (default `http://127.0.0.1:4087`). The window's
"Service" section is wired to `FortKit.ServiceController`, which shells out to the
bundled `fort` binary's `service install|start|stop|restart|status|uninstall`
subcommand (see `cmd/fort/service.go`).

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

FortMac globs `macOS/*.swift` (excluding `*.md`), so `FortWindow.swift` and
`FortMacApp.swift` are picked up automatically.

## Compile-verify (no signing, CI-safe)

```sh
cd ui/apple/FortKit && swift build
cd ui/apple && xcodegen generate
xcodebuild -project Fort.xcodeproj -scheme FortMac \
  -destination 'platform=macOS' CODE_SIGNING_ALLOWED=NO build
```

Expect `** BUILD SUCCEEDED **`. (`make apple-build` runs this for every Apple
target.)

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
make mac-dmg           # -> ./build/Fort.dmg
# override the notarytool profile name if you didn't use the default:
make mac-dmg NOTARY_PROFILE=my-profile
```

What it runs (see the `mac-dmg` target in the `Makefile`):

1. `make build` → `./bin/fort`.
2. `xcodebuild … -scheme FortMac … archive` → `build/FortMac.xcarchive`.
3. `cp bin/fort …/FortMac.app/Contents/Resources/fort` — bundle the daemon.
4. `xcodebuild -exportArchive … -exportOptionsPlist ExportOptions-mac.plist`
   → a Developer ID–signed `build/FortMac-export/FortMac.app`.
5. `hdiutil create … build/Fort.dmg` (swap in `create-dmg` for a styled DMG).
6. `xcrun notarytool submit build/Fort.dmg --keychain-profile fort-notary --wait`.
7. `xcrun stapler staple build/Fort.dmg`.

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

## Deferred (honest v1 boundary)

In-app Google sign-in for the 028 gateway is scaffolded only
(`FortKit.GatewayAccount` persists a gateway base URL + selected machine). The
full `ASWebAuthenticationSession` OAuth flow is a documented follow-on — it needs
the deployed 028 gateway. Local + mesh machines (the sidebar roster from
`/api/machines`) work fully today without any auth.
