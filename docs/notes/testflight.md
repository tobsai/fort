# Deploying the Fort clients via TestFlight

The Apple clients live in `ui/apple` (shared **FortKit** package + **iOS**,
**macOS**, **watchOS** apps, **CarPlay** scene, watch **complication**). All five
compile today (`make apple-build`). This is what you do to get them onto
TestFlight.

## What you need first
- **Apple Developer Program** membership ($99/yr) — required for TestFlight.
- Xcode signed into your Apple ID (**Xcode ▸ Settings ▸ Accounts**); note your **Team ID**.
- Access to **App Store Connect** (appstoreconnect.apple.com).

## 1 · Set your signing team
Open `ui/apple/project.yml` and set your team, then regenerate:
```yaml
settings:
  base:
    DEVELOPMENT_TEAM: ABCDE12345   # your Team ID
```
```sh
cd ui/apple && xcodegen generate && open Fort.xcodeproj
```
Or, in Xcode, enable **Automatically manage signing** per target and pick your
team — Xcode registers the bundle IDs (`io.mtree.fort`, `.mac`, `.watch`,
`.watch.complication`) for you. Change the `io.mtree` prefix if you don't own it.

## 2 · Point the app at a reachable server
The apps default to `http://127.0.0.1:4087`. **On a real device `127.0.0.1` is the
device itself** — it can't see your Mac. So:
- Run `fort control` (or `fort serve`) on a reachable host and set the app's
  base URL (iOS **Settings** screen) to that host — your Mac's LAN IP
  (`http://192.168.1.20:4091`), a Tailscale address, or a deployed box.
- Plain **HTTP** to a LAN IP needs an **App Transport Security** exception. Add to
  the app's Info.plist via `project.yml` (`info.properties`):
  ```yaml
  NSAppTransportSecurity:
    NSAllowsArbitraryLoads: true   # or a scoped NSExceptionDomains entry
  ```
  App Review prefers a **scoped** exception (or HTTPS) with a justification.

## 3 · Version numbers
`MARKETING_VERSION` (0.1.0) + `CURRENT_PROJECT_VERSION` (build). **Every upload
needs a higher build number** — bump `CURRENT_PROJECT_VERSION` in `project.yml`
(or run `agvtool next-version -all`).

## 4 · Archive & upload
**GUI (simplest):** select the **Fort** scheme + **Any iOS Device (arm64)** ▸
**Product ▸ Archive** ▸ Organizer ▸ **Distribute App ▸ TestFlight & App Store ▸
Upload**. Repeat with the **FortMac** scheme (**My Mac**) and **FortWatch**.

**CLI:**
```sh
cd ui/apple
xcodebuild -project Fort.xcodeproj -scheme Fort -sdk iphoneos -configuration Release \
  -archivePath build/Fort.xcarchive archive DEVELOPMENT_TEAM=ABCDE12345
xcodebuild -exportArchive -archivePath build/Fort.xcarchive \
  -exportPath build/export -exportOptionsPlist ExportOptions.plist
# upload with an App Store Connect API key (Users and Access ▸ Keys):
xcrun altool --upload-app -f build/export/Fort.ipa -t ios \
  --apiKey <KEY_ID> --apiIssuer <ISSUER_ID>
```
`ExportOptions.plist`: `method = app-store-connect`, `destination = upload`.

## 5 · In App Store Connect
- Create the app record (or it appears on first upload); set the bundle ID.
- **Export compliance:** the app uses only standard networking. Set
  `ITSAppUsesNonExemptEncryption = NO` (add
  `INFOPLIST_KEY_ITSAppUsesNonExemptEncryption: "NO"`) to skip the per-build prompt.
- After processing (a few minutes) the build shows in **TestFlight**. Add
  **internal** testers (up to 100, no review) or **external** testers (needs a
  one-time Beta App Review).

## Which surfaces ship where
| Surface | TestFlight path |
|---|---|
| **iOS** (`Fort`) | Standard iOS build. Primary. |
| **watchOS** (`FortWatch`) | Ship **embedded in the iOS app** (add `FortWatch` as a target dependency of `Fort` with `embed: true`, and embed `FortComplication` in `FortWatch`) so it installs with the phone app. A standalone watch app is also allowed but needs more setup. |
| **macOS** (`FortMac`) | A **separate** App Store Connect macOS record; TestFlight supports macOS. Archive with **My Mac**, then Distribute ▸ TestFlight. |

## CarPlay — read this before you count on it
The CarPlay scene (`ui/apple/CarPlay`) **compiles** and runs in the CarPlay
Simulator during development, but shipping it is gated by Apple:
- The CarPlay app entitlement (`com.apple.developer.carplay-*`) is granted **only
  for specific app categories** — navigation, audio, communication (VoIP), EV
  charging, parking, quick-food ordering, fueling, driving-task. **A dev/ops
  control-plane dashboard doesn't fit an approved category**, so Apple is unlikely
  to grant it. Request at developer.apple.com/contact/request/carplay.
- **Without the entitlement the CarPlay scene simply won't activate** — the iOS
  app still ships to TestFlight fine. Treat CarPlay as a **dev/simulator-only**
  surface unless Fort is reframed into an approved category and Apple approves it.

**Local CarPlay testing:** add the CarPlay scene manifest to the iOS Info.plist
(a `CPTemplateApplicationSceneSessionRoleApplication` with delegate
`FortCarPlaySceneDelegate`) + the carplay entitlement to a dev profile, then use
the CarPlay Simulator (Simulator ▸ I/O ▸ External Displays ▸ CarPlay). See
`ui/apple/CarPlay/Info-CarPlay-notes.md`.

## The one environment fix on this Mac
`xcodebuild` reported *"CoreSimulator is out of date"* — that only disables
**running** in the simulator (device enumeration); compilation against the SDKs
works. **Reboot the Mac** (or finish the Xcode update) to restore the simulators
before doing a simulator run.
