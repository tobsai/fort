# Signed XPC-to-core bridge

The signed fixture now runs the actual Mac XPC client, production pinned listener,
Swift process transport, compiled pipe relay and core together. Native relative
requests cross XPC, owned pipes and the fixed private Unix socket. The production
listener accepts one bounded request per connection and cancels its task when
the connection closes. It creates no enrollment or provider authority. Governing
design: [Spec 059](../../../specs/059-modular-node-v1.md). Source base: `fe21058`;
the [record](xpc-core.json) pins all relevant inputs.

The XPC service contains its own Node executable, linked libraries, licenses,
compiled core and fixture composition. App and XPC entitlements remain App
Sandbox only; both Node executables have App Sandbox plus inheritance. The probe
keeps a separate outer-app runtime for its earlier controls. That duplication is
fixture packaging, not the intended installed product. Strict/deep signature
verification and all six entitlement inspections passed. The service's compiled
core has 22 files totaling 161,592 bytes; every manifest entry was independently
hashed. The manifest matches [the portable relay artifact](private-channel.md).

Final retained bundle:
`successor/.build/xpc-core-059-20261018-c/FortSandboxProbe.app`.

Verified in that exact bundle:

- Unauthorized and foreign-installation requests fail; explicit fixture
  enrollment permits exact Unicode/whitespace text, durable messages and replay.
- Cancellation reaches the server and its owned process transport. The core
  restarts in the same fixture directory, retains installation/receipt/history,
  and does not execute the duplicate request. This is a Node-core restart within
  one helper, not recovery from an XPC-helper restart.
- Both XPC-owned Node launches refuse outside-marker reads/writes and TCP
  listening. The helper refuses execution of the outer app's Node (`EPERM`).
  It can execute its own bundled Node and use its own private socket/directory.
- The production listener rejects the wrong client identifier (4097); the real
  client rejects a wrong service signing team (4102). Existing cancellation,
  timeout, malformed/status and encoded/decoded size controls pass. Accepted
  payload bounds are checked after Foundation receives XPC objects; they do not
  claim hostile-peer allocation containment.
- Clean stop removes the socket. The harness independently observes both core
  PIDs/groups, helper paths and outer probe processes absent. No forced XPC
  cleanup was needed. All generated containers were then removed.

All 75 Swift package tests passed, including the production host's single-call,
payload-bound and malformed-input behavior. Strict TypeScript passed. Fresh
unsigned Mac and iPhone Simulator builds passed at
`/tmp/fort059-xpc-core-mac/Build/Products/Debug/FortMac.app` and
`/tmp/fort059-xpc-core-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
The source-core native journey passed again after the fixture-only denial
controls were added. The preceding standalone four-scenario signed suite passed
4/4 with zero skips; after strengthening the production-listener and helper-Node
controls, the XPC artifact was rebuilt and every required result passed. The
other three unchanged scenarios were not repeated. Structural, accepted/staged
spec and whitespace checks passed before commit.

Failures and corrections remain evidence:

1. Initial red lacked XPC-to-core authentication evidence. A fixture collector
   compile error was corrected before execution.
2. Launching Node from the outer app failed with a recorded sandbox
   `process-exec` denial. Each XPC service has a distinct sandbox and its own
   bundle resources ([Apple's XPC guide](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingXPCServices.html)).
   Packaging the same signed closure inside the service resolved execution
   without adding an entitlement; the denied path remains a negative control.
3. The next helper launched the core, then crashed because async `main` called
   `NSXPCListener.resume()` from a main-queue block. The crash report identified
   `dispatch_main` misuse. Listener startup is now synchronous; a transport task
   awaits core readiness. Failed artifacts are retained, and their three generated
   container sets were removed only after independent process absence checks.
4. One four-scenario run overlapped another signed probe and native builds. Its
   XPC scenario hit the unchanged 15-second harness deadline; the other three
   passed. The cause is not established. The unchanged suite passed 4/4 when run
   alone. The failed artifact and separate absence reconciliation are recorded;
   no timeout was raised to turn this failure into a pass.

Bootstrap, restart and stop control routes live only in the packaging fixture's
transport. They are absent from the core and ordinary app composition. No account,
real provider, startup registration, installed UI or physical device was used.
The ordinary app still needs owned setup, canonical persistent storage, startup/
recovery/removal, runtime packaging and reachable iPhone transport. TestFlight
and actual-provider acceptance remain open. Process ownership and these host
sandbox checks do not establish containment of a future computer-tool executor.

Logs use `/tmp/fort059-xpc-core-`: `red.log`, `first.log`, `diagnostic.log`,
`contained.log`, `green.log`, `all-signed.log`, `all-signed-serial.log`,
`denials-red.log`, `artifact-final.log`, `swift-final.log`,
`native-fixture-final.log`, `typecheck-final.log`, `mac-build.log` and
`ios-build.log`. Redacted failure/container reconciliation is included in the
JSON record; original failed artifacts remain under their recorded temporary paths.
