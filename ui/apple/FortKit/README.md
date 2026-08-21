# FortKit

FortKit is the shared Swift package for Fort's iPhone and macOS chat
experience. It owns the provider-neutral Agent Channel and nested Conversation
contracts, retains the typed Primary Channels rollback contract, and provides
the authenticated encrypted transport used by native clients.

- **Platforms:** iOS 16+, macOS 13+
- **Tools:** Swift tools 5.9
- **Dependencies:** Foundation, SwiftUI, Combine, and CryptoKit only

No dashboard, workflow-authoring, or raw orchestration surface is part of this
package.

## Shipping contract

- **`AgentChannels.swift`** defines provider-neutral Agent Channel, immutable
  binding, nested Conversation, target, and Needs You wire models.
- **`FortClient.swift`** calls the parent-qualified `/api/agent-channels`
  families and replacement SSE streams. It retains the Phase 1 Primary APIs
  solely for the explicit rollback presentation.
- **`AgentChannelsView.swift`** is the shared iPhone and macOS product surface:
  agent-first navigation, pinned/recent Conversations, exact identity and
  readiness disclosure, Needs You, and bounded recovery actions.
- **`PrimaryChannels.swift`** and **`PrimaryChannelsView.swift`** remain the
  closed rollback contract when the Agent Channels presentation flag is off.
  That path continues to use `/api/settings/primary-agent`, `/api/channels`,
  `/api/needs-you`, and `/api/schedules` without changing their meaning.
- **`PrimaryChannelsStyle.swift`** contains the shared palette and living Fort
  mark: restrained ambient motion, stronger Working energy, and non-spatial
  glow when Reduce Motion is enabled.
- **`GatewayAddress.swift`, `GatewayAccount.swift`, `GatewayRelay.swift`, and
  `SecureRelay.swift`** provide HTTPS gateway validation, renewable native
  sessions, pinned machine identity, request correlation, and Noise IK /
  ChaCha20-Poly1305 relay transport. The iPhone target keeps the bearer in its
  device-only Keychain store.
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
`ServiceController` to the Settings recovery surface. It does not expose a
remote-machine connection flow on Mac.

## Verification

From this directory:

```sh
swift run FortKitContractChecks
```

The executable contract checks pin Agent and Primary wire decoding, endpoint
paths, typed errors, idempotent client turn IDs, authoritative SSE replacement,
request IDs, gateway retry diagnostics, the cross-language Noise vector,
gateway-only iPhone Release behavior, and truthful living-mark motion.
