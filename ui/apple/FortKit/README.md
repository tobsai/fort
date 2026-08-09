# FortKit

FortKit is the shared Swift package for Fort's Phase 1 iPhone and macOS
experience. It owns the typed Primary Agent, private Channel, Needs You, and
Scheduled contracts plus the authenticated encrypted transport used by native
clients.

- **Platforms:** iOS 16+, macOS 13+
- **Tools:** Swift tools 5.9
- **Dependencies:** Foundation, SwiftUI, Combine, and CryptoKit only

No deferred product surfaces or legacy endpoint families are part of this
package.

## Shipping contract

- **`PrimaryChannels.swift`** defines the Codable Primary Agent, Channel,
  target, Needs You, schedule, and occurrence models plus deterministic local
  reducers.
- **`FortClient.swift`** calls only the Phase 1 API families:
  `/api/settings/primary-agent`, `/api/channels`, `/api/needs-you`, and
  `/api/schedules`. Channel live updates use replacement snapshots from
  `/api/channels/{id}/events`.
- **`PrimaryChannelsView.swift`** is the shared iPhone and macOS product
  surface: Channels, Scheduled, Needs You, Settings, truthful model disclosure,
  and closed recovery actions.
- **`PrimaryChannelsStyle.swift`** contains the shared palette and approved
  Working-only Fort orb motion. Reduce Motion disables spatial animation.
- **`GatewayAddress.swift`, `GatewayAccount.swift`, `GatewayRelay.swift`, and
  `SecureRelay.swift`** provide HTTPS gateway validation, persisted native
  session state, pinned machine identity, request correlation, and Noise IK /
  ChaCha20-Poly1305 relay transport.
- **`ServiceController.swift`** is macOS-only recovery support for reading
  daemon state and invoking Install, Start, or Restart. Stop and Uninstall stay
  in the Fort CLI rather than the product UI.

## Transport boundaries

Physical iPhone Release builds start disconnected with
`FortClient.gatewayOnly()`. After native gateway authentication and machine-key
verification, call `useGateway(account:machine:)`; signing out calls
`disconnectGateway()` and returns the client to its inert state. A direct host
is available only to the macOS app and DEBUG iPhone Simulator QA.

The macOS app uses `FortClient()` for its same-host Fort daemon and provides a
`ServiceController` to the Settings recovery surface. Phase 1 does not expose a
remote-machine connection flow on Mac.

## Verification

From this directory:

```sh
swift run FortKitContractChecks
```

The executable contract checks pin Primary wire decoding, endpoint paths,
typed errors, idempotent client turn IDs, authoritative SSE replacement,
request IDs, gateway retry diagnostics, the cross-language Noise vector,
gateway-only iPhone Release behavior, and truthful orb motion.
