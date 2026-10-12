# Fort Spec 058 successor

This is the implemented Stage 1 **fixture increment**: a fresh authenticated
TypeScript/SQLite service, constrained deterministic fixture worker, FortKit,
and native SwiftUI iPhone and Mac apps. It does not enable official providers.
Stage 2 now has a Codex turn codec and an authenticated synthetic runner-to-ledger
bridge. Only explicit temporary fixture transports exercise it; no official
runtime is launched and Stage 2's live-provider exit remains open.

- [Implementation decisions and remaining gates](IMPLEMENTATION.md)
- [HTTP contract and shared fixtures](contracts/README.md)
- [Service/runner validation and trust boundary](service/README.md)
- [Initial validation record](evidence/2026-10-12-stage1.md)
- [Review fixes and historical validation](evidence/2026-10-12-review-followup.md)
- [Spec 058 acceptance map](evidence/2026-10-12-spec058-matrix.md)
- [Latest Mac fixture and regression evidence](evidence/2026-10-12-mac-and-contracts.md)
- [Codex protocol preparation and open boundaries](runner/README.md)
- [Codex protocol validation](evidence/2026-10-12-codex-protocol.md)
- [Synthetic bridge, crash and replay evidence](evidence/2026-10-12-synthetic-runner-bridge.md)

The runtime and behavior tests need no dependency installation. Run from this worktree's root:

```sh
/opt/homebrew/opt/node@26/bin/node --test successor/service/service.test.ts successor/service/synthetic-runner.test.ts successor/runner/fixture.test.ts successor/runner/codex.test.ts successor/runner/synthetic-transport.test.ts
swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort058-swift-build
```

The checked-in `successor/apple/FortSpec058.xcodeproj` contains `FortiOS` and
`FortMac` schemes; `project.yml` reproduces it using installed XcodeGen 2.45.4.
Simulator builds use local ad-hoc signing for Keychain identity. Other default builds
disable signing. Mac XCTest needs a trusted development identity. This host's
ad-hoc runner was rejected; the owner-authorized existing Apple Development
identity subsequently allowed normal launch. Signing was selected per build,
without changing project defaults or macOS security settings. See the latest
evidence record for exact identity, artifacts and journey results. The apps show fixture status and require
explicit enrollment. The local harness API documented in `service/README.md`
issues short-lived single-use codes; no service starts on login or app launch.

The compiled `FortNativeCheck` product drives the real HTTP boundary using two
synthetic native devices, then closes/reopens the service and verifies retry,
replay and read state. Pass its built executable explicitly:

```sh
/opt/homebrew/opt/node@26/bin/node successor/apple/native-http-check.ts /tmp/fort058-swift-build/out/Products/Debug/FortNativeCheck
```

Build paths depend on the installed Swift build system. This script creates and
cleans its own test data and loopback listener. Do not point fixture tests at
existing Fort/provider data. A physical iPhone requires a trusted private HTTPS
endpoint, which this loopback-only increment does not provision.

Strict static checks use the pinned development dependencies in `package-lock.json`;
install only with reviewed configuration and lifecycle scripts disabled, then run
`node successor/node_modules/typescript/bin/tsc -p successor/tsconfig.json` with Node
26.5.0. The runtime never imports these development packages.

Additional native checks (pass the built `FortNativeCheck` path explicitly):

```sh
/opt/homebrew/opt/node@26/bin/node successor/apple/native-http-check.ts /tmp/fort058-swift-build/debug/FortNativeCheck controls
/opt/homebrew/opt/node@26/bin/node successor/apple/native-redirect-check.ts /tmp/fort058-swift-build/debug/FortNativeCheck
```

`apple/native-ui-check.ts` accepts a built `.xctestrun`, an Xcode destination and a
fresh `.xcresult` path (and optional `approval` scenario). It starts a temporary fixture ledger, supplies synthetic
one-use enrollment to the UI test, and cleans the listener/configuration afterward.
Recovery checks include an explicit loopback fault proxy, offline restart and a dropped
acceptance response, then compare exact canonical message counts. The test uses real SwiftUI accessibility controls; DEBUG-only app setup selects a
unique test storage directory and prefills the enrollment form. It does not bypass
enrollment, authentication or Keychain. Installed UI results and launch blockers are
recorded separately from Swift-library HTTP checks.
