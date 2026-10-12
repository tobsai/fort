# Fort Spec 058 successor

This is the implemented Stage 1 **fixture increment**: a fresh authenticated
TypeScript/SQLite service, constrained deterministic fixture worker, FortKit,
and native SwiftUI iPhone and Mac apps. It does not enable official providers.

- [Implementation decisions and remaining gates](IMPLEMENTATION.md)
- [HTTP contract and shared fixtures](contracts/README.md)
- [Service/runner validation and trust boundary](service/README.md)
- [Exact local validation record](evidence/2026-10-12-stage1.md)

No dependency installation is needed. Run from this worktree's root:

```sh
/opt/homebrew/opt/node@26/bin/node --test successor/service/service.test.ts successor/runner/fixture.test.ts
swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort058-swift-build
```

The checked-in `successor/apple/FortSpec058.xcodeproj` contains `FortiOS` and
`FortMac` schemes; `project.yml` reproduces it using installed XcodeGen 2.45.4.
Builds intentionally disable signing. The apps show fixture status and require
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
