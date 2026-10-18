# Mac core-host lifecycle composition

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
Engineering verification on October 18, 2026. All owner identities, device tokens,
runtimes and child data are synthetic. No real credential, provider, startup
registration, profile or persistent grant was used.

`MacCoreHost` owns one child transport incarnation. Its public launch requires an
explicit executable, arguments, directory, environment and parent-exit guardian.
A missing guardian fails without falling back to an unguarded child. The internal
initializer lets the existing packaging fixture retain its process diagnostics;
the ordinary application still has no production bootstrap or installed host.
Caller-supplied paths are not package/signature validation. Those checks remain
part of the future exact-bundle composition.

Ordinary commands, setup and cancellation use the existing bounded pipe. Recovery
shares one operation for an exact installation/device, blocks new ordinary work,
validates runtime stop and revocation, awaits clean child/group exit, and checks
`core.sock` with `lstat`. A remaining file, including a non-socket entry, is preserved
and prevents a successful receipt. After revocation the controller retains its
proof and retries only cleanup. It never asks an exited child to revoke again.
A foreign identity cannot reuse that retained proof. Receipts are local to this
controller incarnation; another host must obtain durable evidence from the core.

A caller canceled after recovery begins waits for the shared operation to finish.
Other callers can obtain its result. A canceled caller before dispatch makes no
recovery call. Ordinary shutdown waits for pending recovery and shares child cleanup;
shutdown alone preserves the pairing. Neither result proves helper-process absence,
startup unregistration, credential deletion or app removal. No automatic restart,
resume, socket deletion, timer or idle poll was added.

## Identity rejection and uncertain stop

The signed fixture's existing foreign-identity control exposed a needed distinction.
The private pipe previously collapsed both identity rejection and uncertain shutdown
into `unavailable`. Blocking all ordinary work after either outcome would disrupt the
valid pairing after a foreign request.

The core now throws `OwnerRecoveryIdentityRejected` only for a mismatched installation
or a device rejected before `stopOwner` writes its intent. The private channel emits
status 409 and only the requested installation/device plus `identityRejected`.
Swift requires the exact three fields, identity, outcome, status and <=1 KiB body
before recognizing the rejection. Only a previously serving controller can return
to serving on this proof. A prior uncertain recovery remains blocked, even when a
later foreign request is rejected. Runtime disposal failures cannot impersonate this
safe rejection: `stopOwner` translates them to uncertain shutdown first.

Other errors, timeouts and malformed/foreign rejection replies remain uncertain.
There is no HTTP recovery route or bearer-token authentication exception. The new
contract is included in the provider-free core artifact. No runtime implementation,
telemetry, control plane, permission expansion or package dependency was added.

## Test-first evidence and corrections

- Missing controller failed the initial native test: `/tmp/fort059-mac-core-host-red.log`.
  The first passing lifecycle journey is in `...-first-green.log`.
- A retained socket-path regression failed because cleanup retry tried to contact
  the exited child: `...-residue-red.log`. Retaining the exact revocation proof fixed
  it: `...-residue-green.log`. The test explicitly removes only its own synthetic
  marker; production code leaves the path untouched.
- The pipe rejection test failed on its generic error result: `...-rejection-red.log`.
  Core/pipe recovery tests then passed: `...-rejection-green.log`, 28 tests at that
  point. A later runtime-error impersonation test is included in the full suite.
- Native foreign recovery failed because valid ordinary traffic remained blocked:
  `...-foreign-red.log`. The exact rejection result fixed it: `...-foreign-green.log`.
- Host boundaries passed in `...-boundaries.log`, `...-launch.log` and
  `...-rejection-boundaries.log`. The last source-core targeted run excluded the
  guardian-specific function because its environment selector was absent. The full
  compiled-core run explicitly enabled and passed that function.
- The first full native run failed at the synthetic recovery marker's initial value:
  `...-swift.log`. The fixture published its filename before `writeFileSync` wrote
  contents; the assertion did not print the observed value, so this attribution is
  an inference from that publication window. The marker now writes a temporary file
  and publishes it by rename. The unchanged production behavior passed the rerun.

Final full native result: **166 test functions executed and passed; one existing
startup timing function explicitly excluded**. Swift Testing reports 167 registered
functions including that exclusion. `/tmp/fort059-mac-core-host-swift-pass.log`.
Ten new host functions include six malformed-reply parameter cases, exact guarded
launch, missing guardian, partial cleanup, stop failure, pre/post-dispatch caller
cancellation, overlap, foreign identity, unclean exit, retained pairing after ordinary
shutdown and repeated shutdown/recovery.

**893 portable tests passed with zero skips/failures**, including artifact removability,
private-pipe recovery, core identity rejection and disposal-error impersonation.
`/tmp/fort059-mac-core-host-portable.log`. Strict TypeScript checking passed in
`...-typecheck.log`. Tests use actual isolated children, local sockets and the real
compiled core; fixture runtimes do not prove an official provider integration.

Current compiled core:

- Path: `successor/.build/mac-core-host-059-20261018`.
- Manifest: 28 files, 188,021 payload bytes.
- Manifest SHA-256: `33295af8f2a0917d60d89dcec80bfc3d4df7b3644481b6124f69685a9714c7a1`.
- New files are the compiled owner-recovery contract and declaration.

Reproduce the native run from the isolated worktree:

```sh
xcrun swiftc -swift-version 6 successor/host/mac-process-guardian.swift \
  -o /tmp/fort059-mac-core-host-guardian
FORT_TEST_GUARDIAN=/tmp/fort059-mac-core-host-guardian \
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node \
FORT_TEST_CORE_ARTIFACT="$PWD/successor/.build/mac-core-host-059-20261018" \
swift test --package-path successor/apple/FortKit \
  --scratch-path /tmp/fort059-owner-setup-xpc-native
```

## Synthetic recovery measurement

MacBookPro18,2, M1 Max, 64 GiB, 10 CPU; macOS 27.2 (26B5112a), Node 26.5.0,
Xcode 27.1 (27A9275), Swift 6.4. Twenty serial runs use a fresh private ledger and
synthetic pairing each time, with warm tool/binary caches and no cache purge.
Timing starts immediately before recovery and ends after its confirmed receipt;
launch, initial pairing and directory creation are excluded. No active turn,
provider, network, XPC, ServiceManagement, Keychain or installed UI cost is included.
This timing uses the isolated direct child fixture; the guarded factory and signed
XPC path are separate behavior checks. No other tests/builds ran concurrently.
p50 and p95 are the tenth and nineteenth sorted samples. Target <=2,000 ms.

Measured p95: **56.867083 ms**. Raw log and sample file:
`/tmp/fort059-mac-core-host-budget.log` and `...-budget.json`.

```json
{
  "samplesMs": [
    55.487416,
    56.654083,
    57.270708,
    56.867083,
    53.192542,
    53.180833,
    53.126042,
    56.391583,
    55.807041,
    54.767042,
    55.300375,
    54.55825,
    56.011792,
    55.509208,
    56.832125,
    51.926959,
    56.042,
    56.249,
    53.254542,
    53.208084
  ],
  "p50Ms": 55.487416,
  "p95Ms": 56.867083,
  "budgetMs": 2000
}
```

## Signed artifact and cleanup

All four signed sandbox scenarios passed, with zero skips, using the recorded
existing development identity and unchanged entitlements.
`/tmp/fort059-mac-core-host-signed.log`. The XPC fixture now composes `MacCoreHost`;
its synthetic bootstrap/restart/crash controls remain fixture-only. Existing exact
peer rejection, foreign recovery with pairing preserved, cancellation, retry,
crash recovery, private-core sandbox denials and cleanup assertions are unchanged.

A separate CLI run retained the artifact for independent inspection:
`successor/.build/mac-core-host-xpc-059-20261018/FortSandboxProbe.app`.
Its identifier is `io.fort.foundation.probe.f9ec6a6a-f5d5-4685-b6d8-7cd59aa7ae22`.
The [redacted evidence](mac-core-host.json) includes the result, source hashes,
manifest hashes, entitlement values and owned identities. The verifier is
`/tmp/fort059-verify-mac-core-host.py`; CLI output is
`/tmp/fort059-mac-core-host-retained.json`.

Independent checks passed: deep strict signature, all seven unchanged entitlement
targets, both 28-file core copies matching the current manifest, 13 PIDs and five
process groups absent, no process using the owned executables, and four synthetic
containers absent. The fixture reports no forced cleanup. This is temporary
embedded-XPC evidence, not registered named-agent or installed-product acceptance.

The retained run measured crash cleanup at 14.832125 ms, below its three-second
bound; crash restart at 10,158.907292 ms, below its separate twelve-second bound;
and parent work at 11,778.191542 ms, below fifteen seconds. Those existing fixture
bounds include launchd's restart throttle and are distinct from the direct recovery
measurement above. Provider use and startup registration are both false.

## Native builds

Fresh unsigned Debug engineering builds passed with `CODE_SIGNING_ALLOWED=NO`:

- `/tmp/fort059-mac-core-host-mac-build/Build/Products/Debug/FortMac.app`.
- `/tmp/fort059-mac-core-host-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.

The commands used `successor/apple/FortSpec058.xcodeproj`, the `FortMac` / `FortiOS`
schemes, generic macOS / iOS Simulator destinations and separate fresh derived-data
roots. Logs are `/tmp/fort059-mac-core-host-{mac,ios}-build.log`. Neither app was
installed or launched. The existing CoreSimulator 1171.7.0 versus required 1174.9.2
mismatch remains; no OS or security setting was changed. Full and implementation
spec gates and whitespace checks passed. Exact staged coverage is required at commit.

## Remaining acceptance

Actual named-agent IPC, bundle/profile validation,
canonical host storage/bootstrap, startup registration, installation removal,
Keychain, official providers, reachable iPhone transport and installed UI remain
separate gates. The agent host and computer-tool sandbox remain separate boundaries.
No TestFlight availability, Apple acceptance or complete v1 is claimed.
