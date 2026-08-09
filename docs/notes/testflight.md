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

## 2 · Production gateway and direct development

Production iOS builds embed `https://fort-gateway.vercel.app`. The app signs in
there, selects a registered machine, verifies its fingerprint, and carries API
commands through the end-to-end encrypted relay. The Cloudflare worker address
is daemon-only and must not be entered in the app.

Physical iPhone and TestFlight builds expose Phase 1 only through the
authenticated encrypted gateway relay. Direct-host configuration, UI, and
actions are compiled out and never available on a physical iPhone, including
reachable LAN hosts. The DEBUG iOS Simulator alone retains
`FORT_DIRECT_HOST_URL` for an isolated local QA fixture. This boundary does not
change FortMac's local loopback client.

## 3 · Version numbers
`MARKETING_VERSION` + `CURRENT_PROJECT_VERSION` (build). **Every upload
needs a higher build number** — bump `CURRENT_PROJECT_VERSION` in `project.yml`
(or run `agvtool next-version -all`).

## 4 · Archive & upload
**GUI (simplest):** select the **Fort** scheme + **Any iOS Device (arm64)** ▸
**Product ▸ Archive** ▸ Organizer ▸ **Distribute App ▸ TestFlight & App Store ▸
Upload**. The Fort archive already embeds the watch app and complication.
Do not upload `FortWatch` separately. FortMac is a separate macOS distribution.

**CLI:**
```sh
cd ui/apple
xcodebuild -project Fort.xcodeproj -scheme Fort -destination 'generic/platform=iOS' \
  -configuration Release -archivePath build/Fort.xcarchive \
  -allowProvisioningUpdates archive DEVELOPMENT_TEAM=ABCDE12345
# NB: use -destination, NOT `-sdk iphoneos` — forcing the SDK poisons the
# embedded watch targets (iOS DTPlatformName/UIDeviceFamily/arch → App Store
# validation rejects the build).
xcodebuild -exportArchive -archivePath build/Fort.xcarchive \
  -exportPath build/Fort-upload -exportOptionsPlist ExportOptions.plist \
  -allowProvisioningUpdates
```
`ExportOptions.plist` has `method = app-store-connect` and
`destination=upload`, so a successful `-exportArchive` performs the upload.
It does not produce an IPA that should be uploaded a second time.

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

## Headless archive + upload (2026-07-12, verified working end-to-end)

The whole pipeline runs from the CLI with no Xcode GUI:

```sh
cd ui/apple
# 1. distribution identity — import once (the keychain had only "Apple Development").
#    The Apple Distribution cert+key were backed up in ~/.appstoreconnect/nimbus-dist-cert-backup/
security import ~/.appstoreconnect/nimbus-dist-cert-backup/P4B9AKSW76.p12 -P "" \
  -k ~/Library/Keychains/login.keychain-db -T /usr/bin/codesign -T /usr/bin/xcodebuild
security import ~/.appstoreconnect/nimbus-dist-cert-backup/P4B9AKSW76.cer \
  -k ~/Library/Keychains/login.keychain-db -T /usr/bin/codesign -T /usr/bin/xcodebuild
# 2. bump the version in project.yml, then regenerate
xcodegen generate
# 3. archive (embeds watch + complication). -allowProvisioningUpdates uses the
#    Xcode-signed-in tobias@mtree.io session to create App Store profiles.
xcodebuild -project Fort.xcodeproj -scheme Fort -destination 'generic/platform=iOS' \
  -archivePath build/Fort.xcarchive -allowProvisioningUpdates \
  CODE_SIGN_STYLE=Automatic DEVELOPMENT_TEAM=T3JB5MYZ93 archive
# 4. export and upload once through App Store Connect
xcodebuild -exportArchive -archivePath build/Fort.xcarchive \
  -exportOptionsPlist ExportOptions.plist -exportPath build/Fort-upload \
  -allowProvisioningUpdates   # ExportOptions.plist has destination=upload
```

**Version bump gotcha:** xcodegen writes `CFBundleShortVersionString`/`CFBundleVersion`
as the **literals** `1.0`/`1` into the generated Info.plists, which SHADOW the
`MARKETING_VERSION`/`CURRENT_PROJECT_VERSION` build settings. project.yml now sets
each target's `info.properties` to `$(MARKETING_VERSION)`/`$(CURRENT_PROJECT_VERSION)`
so a version bump actually reaches the build (phone + watch + complication all match).

## One upload, then independent verification

Before archiving, confirm that `CURRENT_PROJECT_VERSION` is higher than every
build already visible in App Store Connect/TestFlight. After a successful
`-exportArchive`, inspect the archive `Info.plist`: its distribution receipt
must record the intended `uploadedBuildNumber`, a successful upload event, and
no errors or warnings. Do not run altool, Transporter, or a second export for
that build after this receipt exists.

Apple processing is separate from upload acceptance. Completion requires an
independent signed-in TestFlight check showing the exact marketing version and
build number after processing. If Apple reports a new agreement gate, the
Account Holder must resolve it in App Store Connect before retrying; first check
the archive receipt so an already-successful upload is not duplicated.
