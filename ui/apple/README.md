# Fort Apple clients

Native control-plane clients for Fort, all sharing one Swift package. They speak
the same typed Primary Channels HTTP/SSE contract as the local Web surface.

```
ui/apple/
  FortKit/           shared Swift package — models + direct/pinned-relay client
  iOS/               iPhone app: private Channels, Scheduled, Needs You, Settings
  macOS/             windowed Primary Channels + bounded MenuBarExtra glance
  watch/             watchOS app (glance + approve) + WidgetKit complication
  CarPlay/           CPListTemplate scene: gates + status (driving-safe)
  Support/           complication @main bundle + generated Info.plist
  project.yml        XcodeGen spec (all targets + FortKit dependency)
```

Every surface `import FortKit` — none redefine the models or client.

The shipping iPhone and Mac roots share `PrimaryChannelsView`. Watch and
CarPlay retain their separately bounded glance surfaces; they do not provide a
second Primary Channels implementation.

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
fingerprint with `fort relay join` output. Physical iPhone and TestFlight builds
expose Primary Channels only through this authenticated encrypted gateway relay.
Direct-host UI and actions are never available on a physical iPhone.

For the production app, enter the public web gateway origin
`https://fort-gateway.vercel.app`. The app accepts an accidental trailing
`/native` and canonicalizes it to the origin. Do not enter the daemon's
Cloudflare relay address: that endpoint carries the machine tunnel and is not
the iOS sign-in/API origin.

## Isolated native QA host

DEBUG builds of FortMac and the DEBUG iOS Simulator accept
`FORT_DIRECT_HOST_URL` so UI QA can target an isolated fixture without
installing, restarting, or reading from the real launchd service. Set it in the
Xcode scheme, for example to `http://127.0.0.1:4187`. Release iPhone builds
compile this override and every direct-host control out; FortMac Release keeps
its normal loopback behavior.
