# Native Mac XPC transport

The production `MacXPCTransport` implements the shared Fort request/response
contract and passed the signed sandbox probe. The probe now compiles that actual
source and protocol; its previous echo-only protocol was removed. The embedded
service still supplies synthetic responses, so this is transport evidence, not
an XPC-to-core bridge or installed host. Governing design:
[Spec 059](../../../specs/059-modular-node-v1.md). Source base: `4adfe4d`.

Each request selects one exact embedded service and signing team, validates a
relative versioned route and deadline, and owns one XPC connection. Completion,
error, cancellation and timeout settle once on a private serial queue and
invalidate that connection. There is no retry, fallback carrier or polling.
Cancellation ends the transport request; it does not establish cancellation of
an already accepted domain Run. That still requires the correlated cancel API.

The adapter accepts at most 2 MiB encoded request, 12 MiB encoded response and
8 MiB decoded response body, with a maximum 30-second deadline. These are accepted
payload limits; Foundation receives the XPC object before the response-size
check, so they are not a bound on hostile-peer allocation. Peer signatures,
malformed response decoding and status validation remain separate checks.

Verification:

- Red: the signed public packaging test failed because cancellation evidence was
  absent. The implementation replaced the probe-only protocol with
  `FortHostChannel` and the actual native transport.
- All four signed sandbox scenarios passed, zero skips. The XPC scenario checks
  exact Unicode/whitespace and installation identity, mismatched client identity
  (4097), mismatched service signing team (4102), cancellation, timeout, malformed
  JSON, invalid status, oversized request, and oversized encoded/decoded reply.
  The compiled-core, filesystem/TCP denial and hung-child controls also passed.
- All 68 Swift package tests passed with Swift 6 language checking. New native
  cases reject malformed peer configuration, foreign/nonrelative/traversing
  routes and invalid deadlines. Strict TypeScript passed for the harness.
- A fresh signed artifact reproduced every control. Strict/deep signature
  verification passed; extracted entitlements remained App Sandbox only for the
  app and XPC helpers, and App Sandbox plus inheritance for Node. The known helper
  PID and owned app/Node processes were absent; no forced XPC cleanup was needed.
  Generated synthetic containers were then removed. No startup was registered.
- Fresh unsigned Debug Mac and generic iPhone Simulator builds passed. The latter
  excludes the Mac-only adapter. Products are under
  `/tmp/fort059-native-xpc-mac/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-native-xpc-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
- Structural and accepted/staged spec gates and whitespace checks passed.
  Portable Node and compiled HTTP journeys were not repeated for this adapter
  slice; the immediately preceding transport slice's HTTP/redirect results remain
  recorded separately. No provider, installed UI or physical device was tested.

The [retained result](native-xpc.json) records source hashes, byte limits, process
IDs, exact artifact and signed entitlements. Bundle:
`successor/.build/native-xpc-059-20261018-a/FortSandboxProbe.app`.
The [earlier XPC proof](xpc.md) records the SDK/API basis and original failures.
The current helper has the same eight-second fixture lifetime and peer pinning.

The ordinary app has not been switched to XPC. Core forwarding, owner setup,
device-storage migration, host process supervision, startup/recovery/removal,
official runtime containment and TestFlight/physical acceptance remain open.
This transport creates no account, credential, workspace grant or service
registration. The agent-host and computer-tool sandbox remain distinct.

Logs: `/tmp/fort059-native-xpc-red.log`, `/tmp/fort059-native-xpc-green.log`,
`/tmp/fort059-native-xpc-swift.log`, `/tmp/fort059-native-xpc-typecheck.log`,
`/tmp/fort059-native-xpc-artifact.log`, `/tmp/fort059-native-xpc-mac-build.log`
and `/tmp/fort059-native-xpc-ios-build.log`.
