# CarPlay — entitlement & Info.plist wiring

CarPlay is **entitlement-gated by Apple**. A Fort CarPlay build will not run in
the CarPlay simulator or on hardware until the app has a CarPlay entitlement and
declares the CarPlay scene in its `Info.plist`. This file records exactly what to
add; nothing here is code the compiler sees — it's project configuration.

## 1. Request the CarPlay entitlement from Apple

CarPlay entitlements are granted per app category, by request, at
<https://developer.apple.com/contact/carplay/>. Fort's surface is a **driving
task / communication–style control app** — it shows waiting approvals and status
counts. Pick the CarPlay app category that best matches when you apply.

Once Apple grants it, enable the matching capability on your App ID and add the
entitlement key they assign to `Fort.entitlements`, e.g. one of:

```xml
<!-- Fort.entitlements — the EXACT key depends on the category Apple grants. -->
<key>com.apple.developer.carplay-communication</key>   <!-- example -->
<true/>
```

> You cannot self-issue this. Development builds still require the granted
> entitlement to be present in the provisioning profile.

## 2. Declare the CarPlay scene in Info.plist

CarPlay uses a **separate scene** from the phone UI. Add a second scene
configuration whose delegate is `FortCarPlaySceneDelegate` and whose role is the
CarPlay template-application role. Add this under `UIApplicationSceneManifest`
(alongside your existing phone `UIWindowSceneSessionRoleApplication` entry):

```xml
<key>UIApplicationSceneManifest</key>
<dict>
    <key>UIApplicationSupportsMultipleScenes</key>
    <true/>
    <key>UISceneConfigurations</key>
    <dict>
        <!-- Existing phone scene stays here:
             <key>UIWindowSceneSessionRoleApplication</key> ... -->

        <!-- CarPlay scene -->
        <key>CPTemplateApplicationSceneSessionRoleApplication</key>
        <array>
            <dict>
                <key>UISceneConfigurationName</key>
                <string>Fort-CarPlay</string>
                <key>UISceneDelegateClassName</key>
                <string>$(PRODUCT_MODULE_NAME).FortCarPlaySceneDelegate</string>
            </dict>
        </array>
    </dict>
</dict>
```

Notes:
- `UIApplicationSupportsMultipleScenes` **must** be `true` — CarPlay is a second
  scene living beside the phone scene.
- `UISceneDelegateClassName` uses the Swift `Module.Class` form. If the app
  target's module name isn't literally `Fort`, keep `$(PRODUCT_MODULE_NAME)` so
  it resolves correctly.
- There is **no** storyboard key for a CarPlay scene — the delegate builds the
  template stack in code (see `FortCarPlaySceneDelegate.swift`).

## 3. Local networking (device builds)

Fort's control plane defaults to `http://127.0.0.1:4087` (control-only default
in the docs is `:4091`). On a real iPhone that base URL points at the phone
itself, so the plane must be reachable from the device (or `client.baseURL`
repointed at the host that runs it). Two Info.plist consequences:

- **Plaintext HTTP** to a non-localhost host needs an App Transport Security
  exception. For localhost, iOS already permits cleartext, but if you repoint at
  a LAN host add an ATS exception for that host:

  ```xml
  <key>NSAppTransportSecurity</key>
  <dict>
      <key>NSExceptionDomains</key>
      <dict>
          <key>your-mac.local</key>
          <dict>
              <key>NSExceptionAllowsInsecureHTTPLoads</key>
              <true/>
          </dict>
      </dict>
  </dict>
  ```

- **Local Network** access on iOS 14+ prompts the user the first time the app
  touches the LAN. Add a usage string so the prompt reads sensibly:

  ```xml
  <key>NSLocalNetworkUsageDescription</key>
  <string>Fort connects to your Fort control plane to show approvals and status.</string>
  ```

## 4. Testing

- **Simulator:** run the app in an iOS Simulator, then Xcode ▸ *I/O ▸ External
  Displays ▸ CarPlay* opens the CarPlay window. The entitlement must be in the
  build for the CarPlay scene to attach.
- **Hardware:** requires the granted entitlement in the provisioning profile and
  a CarPlay head unit (or a compatible aftermarket unit).
