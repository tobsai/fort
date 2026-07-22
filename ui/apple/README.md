# Fort Apple clients

Native control-plane clients for Fort, all sharing one Swift package. They speak
the same HTTP/SSE contract as the web board (`docs/notes/event-contract.md`) and
adapt to control-only mode (`Summary.execution == false`).

```
ui/apple/
  FortKit/           shared Swift package — models + direct/pinned-relay client
  iOS/               iPhone app: five-tab Command Deck + Playbooks/Feed in More
  macOS/             windowed Command Deck + Playbooks + MenuBarExtra inbox
  watch/             watchOS app (glance + approve) + WidgetKit complication
  CarPlay/           CPListTemplate scene: gates + status (driving-safe)
  Support/           complication @main bundle + generated Info.plist
  project.yml        XcodeGen spec (all targets + FortKit dependency)
```

Every surface `import FortKit` — none redefine the models or client.

## Build / verify

```sh
make apple-build          # from repo root: generate project + compile all targets
# or manually:
cd ui/apple && xcodegen generate      # -> Fort.xcodeproj (git-ignored)
cd FortKit && swift build             # the shared package
xcodebuild -scheme Fort         -sdk iphonesimulator  -destination 'generic/platform=iOS Simulator'  build CODE_SIGNING_ALLOWED=NO
xcodebuild -scheme FortMac      -destination 'platform=macOS'                                          build CODE_SIGNING_ALLOWED=NO
xcodebuild -scheme FortWatch    -sdk watchsimulator   -destination 'generic/platform=watchOS Simulator' build CODE_SIGNING_ALLOWED=NO
xcodebuild -scheme FortComplication -sdk watchsimulator -destination 'generic/platform=watchOS Simulator' build CODE_SIGNING_ALLOWED=NO
```

All five compile against Xcode's iOS 26 / watchOS 26 / macOS SDKs. FortKit is
also verified against a live `fort control` server (decode round-trip).

## Deploy
See [`docs/notes/testflight.md`](../../docs/notes/testflight.md). Note the CarPlay
entitlement is category-gated by Apple and unlikely for a control-plane app — the
CarPlay code compiles and runs in the simulator but may not ship.

## Connect iOS

The iOS app starts in native connection setup, not against localhost. Set
`FORT_GATEWAY_URL` for the build or enter the deployed gateway origin in the
app, sign in with Google, choose a registered machine, and compare its
fingerprint with `fort relay join` output. Direct LAN/simulator mode is an
explicit fallback in Connection Settings.
