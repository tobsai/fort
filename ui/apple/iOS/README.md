# Fort — iPhone client

The iPhone app is Fort's native agent-first chat surface. Agent Channels are
the top-level destinations; each owns durable, independently pinnable
Conversations. Exact identity/readiness and bounded Needs You recovery are
exposed through FortKit's typed contract. Primary Channels remain available as
the closed rollback presentation.

- **Platform:** iOS 16+
- **Shipping bundle:** `io.mtree.fort`
- **Production transport:** authenticated HTTPS gateway plus the pinned,
  end-to-end encrypted machine relay
- **Shared presentation:** `FortNativeChatView`, also used by the Mac app

## Shipping sources

The XcodeGen target has an explicit source allowlist in `../project.yml`:

| File | Role |
|---|---|
| `FortApp.swift` | App entry point, fail-closed client construction, connection setup, and the closed native chat root. |
| `GatewayCoordinator.swift` | Web-auth handoff, persisted gateway session, machine discovery, fingerprint confirmation, and startup restoration. |
| `GatewaySessionTokenStore.swift` | Device-only Keychain storage for the renewable native session credential. |
| `Assets.xcassets` | The canonical Fort app icon and orbital agent artwork. |

Adding another presentation file requires an intentional project manifest
change; dropping a Swift file into this directory does not compile it into the
shipping app.

## Connection boundary

Physical-device and TestFlight builds start with `FortClient.gatewayOnly()`.
They remain inert until the user authenticates, selects a registered machine,
and accepts its fingerprint. Fort then carries typed API requests through that
machine's encrypted relay. A valid session renews on app restoration, the
trusted machine reconnects automatically, and Fort restores the last valid
Agent Channel and open Conversation or shows the Agent Channel list. Signing
out or losing authorization clears the transport rather than falling back to
another host.

Source defaults `FORT_AGENT_CHANNELS` to `off`. An Agent Channels release must
set the exact build setting to `primary`; missing or invalid values retain the
Primary Channels rollback surface.

The configured production origin is:

```text
https://fort-gateway.vercel.app
```

The Cloudflare worker address belongs to the daemon tunnel and is not an app
sign-in address. Release builds declare no local-network usage and no cleartext
transport exception.

DEBUG Simulator builds may use `FORT_DIRECT_HOST_URL` for an isolated QA fixture.
That compile-time seam is unavailable in physical-device Release builds.

## Generate and verify

```sh
cd ui/apple
xcodegen generate
xcodebuild -project Fort.xcodeproj -scheme Fort \
  -destination 'generic/platform=iOS Simulator' \
  CODE_SIGNING_ALLOWED=NO build
```

Archive and TestFlight upload instructions live in
[`../../../docs/notes/testflight.md`](../../../docs/notes/testflight.md).
