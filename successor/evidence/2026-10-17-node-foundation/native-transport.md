# Native transport boundary

The shared native client now sends a Fort-owned relative request through a
required transport. A local transport needs no network address. The HTTP adapter
owns URL construction, endpoint validation, headers, URLSession and redirect
refusal; the client still owns service-error decoding and installation/replay
checks. This follows [Spec 059](../../../specs/059-modular-node-v1.md).
Source base: `8a62fef`.

`FortRequest` carries method, encoded relative path/query, installation identity,
optional device token, body bytes and timeout. `FortResponse` carries status and
body. Both are Codable; transports must protect the token and body as private
request content. There is no new telemetry, dependency or credential creation.
The interactive fixture implements the same transport without a synthetic HTTPS
origin. The former optional `FortHTTPTransport` path and all active callers were
removed; ordinary network construction still uses the same public initializer.

Verification:

- Red: new public local-transport tests could not compile because that contract
  and the address-free initializer were absent. The first implementation exposed
  a nested Swift Testing macro error in the new test; splitting its body decode
  corrected the test expression.
- All 67 Swift package tests passed. New cases cover exact Unicode/whitespace
  submission, request and binding identities, token/body encoding round trip,
  service-error propagation, replay query escaping, 30-second long-poll timeout,
  cancellation, foreign installation rejection and HTTP base-path/authentication.
  Existing interactive fixture and native recovery tests passed through the new
  boundary.
- The rebuilt `FortNativeCheck` passed real loopback HTTP against the synthetic
  core: ten executions, six restarts, two lost acceptance replies and one lost
  cancellation reply, no duplicate or canceled dispatch, and an empty pending
  queue. The separate fixture journey passed enrollment, exact submission,
  two-client snapshots, restart/retry, replay and shared read state.
- Redirect checks passed for 301, 302, 303, 307 and 308, for enrollment and
  authenticated reads. No redirect destination received the fixture authority.
- Fresh unsigned Debug Mac and generic iPhone Simulator builds passed at
  `/tmp/fort059-transport-mac/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-transport-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  These are uninstalled builds; the existing CoreSimulator-version mismatch
  still prevents any claim of Simulator UI acceptance.
- Structural and accepted/staged spec gates and whitespace checks passed.
  Portable Node, signed sandbox and real-provider suites were not rerun for this
  Swift-only implementation slice.

This prepares the local client boundary. There is no production XPC transport
implementation, XPC-to-core bridge or local device-storage migration yet. The
ordinary native app still uses its existing network connection setup. Installed
UI, physical devices, provider and TestFlight acceptance remain open.

Logs: `/tmp/fort059-native-transport-red.log`,
`/tmp/fort059-native-transport-green.log`,
`/tmp/fort059-native-transport-client-build.log`,
`/tmp/fort059-native-transport-http.log`,
`/tmp/fort059-native-transport-fixture-http.log`,
`/tmp/fort059-native-transport-redirect.log`,
`/tmp/fort059-native-transport-mac-build.log`,
`/tmp/fort059-native-transport-ios-build.log`.
