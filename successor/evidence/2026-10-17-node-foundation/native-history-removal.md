# Native retained host-history removal

October 18, 2026; Spec 059, base cb53e4c. The ordinary Mac model now composes
separate explicit host-history deletion after completed removal. Normal removal
keeps history. This is source, synthetic fixture and unsigned-build evidence;
no owner history, credential, provider, startup registration or installed app was used.

## Behavior and boundaries

The confirmation captures installation, device, recovery epoch and a unique deletion
identity. The coordinator holds the existing private metadata lease and verifies
confirmed exact host removal, disconnected/confirmed pairing metadata, no pending
setup, absent credential, bundle identity, old helper absence and startup absence.
It persists history-removal.json before maintenance. Exact retries retain that request
across lost replies, cancellation and interrupted cache cleanup. Foreign, changed,
malformed and future progress refuse. Rechecks surround asynchronous steps and the
credential inspection itself. Receipt persistence precedes guarded client-cache
cleanup; final completion requires renewed absence and unchanged-record checks.

The production controller selects only the inspected bundled maintenance entry,
Node, fixed OpenSSL configuration and guardian, with an empty environment and
existing inspected App Group storage. It creates no runtime directory and invokes
neither the ordinary host entry nor Codex. Completion requires clean worker exit,
owned process-group absence and unchanged storage. Core ledger guards still require
paused schedules, no devices/pending enrollment, exact owner disconnect and the
matching epoch. Notes, settings, other installations/devices and provider history
remain outside deletion. The agent-host and computer-tool sandbox boundaries are
unchanged; a tool sandbox is not a claim of host confinement.

The UI offers deletion only after ordinary removal/credential cleanup, captures the
request before confirmation and clears its presentation only after both deletion
scopes complete. Failure remains explicitly unconfirmed; an already committed
erasure cannot be undone. Explicit reinstall can retire the saved retry after
cleanup proof; it does not undo an erasure or enable startup. The legacy generic
combined deletion method stays unsupported by the production installation.

Initial deletion confirmation requires a known saved recovery epoch. If the client
cache was independently erased before any host-deletion request was captured, this
action refuses until a fresh connection establishes the current scope; it does not
guess an epoch. A pending deletion journal remains retryable after cache removal.

## Observed checks

- Behavioral red: the coordinator's initial stub refused deletion; the model's
  initial missing target failed. A test-only throwing-macro compile error was
  corrected before recording the model's behavioral failure.
- Mutation regression: changing the journal during credential inspection originally
  reached maintenance. Rechecking metadata after that inspection fixed it.
- Fixed-entry regression: seven unsafe maintenance-entry cases originally passed
  native inspection. The same bounded file validation now applies to both entries.
- Packaging regression: a manifest-consistent missing maintenance entry originally
  assembled successfully. The assembler now requires it before output. All 21
  assembly tests pass, with zero skips and explicit reviewed runtime inputs.
- Eight focused native functions pass, including 15 pre-maintenance refusal cases,
  five interrupted/concurrent cases and three worker exit/storage cases. The timing
  function's no-variable return in that run is not a measurement.
- Isolated 20-retry benchmark: p50 21.053834 ms, p95 23.065958 ms, maximum 23.188791 ms,
  below the 500 ms target. Synthetic private metadata plus fake OS/credential/ledger
  actions; no real code-signature, Keychain or provider latency is represented.
- The first full native run: 271 discovered, 261 passed, ten optional skips,
  zero failures, 52.294 seconds. It included compiled maintenance and worker absence.
- CI selection regression: using only the native job's existing fixture inputs skipped
  compiled maintenance. The test now shares FORT_TEST_HOST_RESOURCES; no workflow or
  permission change. Final rerun with only existing job inputs: 271 discovered,
  261 passed, ten optional skips, zero failures, 52.038 seconds. The compiled
  deletion journey executed and passed. No production source changed after builds.
- Strict TypeScript passes. Portable rerun: 1,158 passed, six optional skips,
  zero failures, 34.823 seconds. The first restricted run failed fixture startup and
  stalled; its log is retained. Only its verified process group 69049 was stopped.
  The same suite passed with permission for disposable local sockets/processes.
- Fresh unsigned Debug Mac client, helper and iPhone Simulator client builds pass.
  No simulator execution: installed CoreSimulator 1171.7.0 does not match required
  1174.9.2. The preceding iPhone test bundle build remains separate from execution.

Environment: MacBookPro18,2, ten CPUs, 64 GiB; macOS 27.2, Xcode 27.1, Swift 6.4,
Node 26.5.0, TypeScript 6.0.3. Benchmark ran without builds/tests in parallel.

## Artifacts and commands

Unsigned products: /tmp/fort059-native-history-mac/Build/Products/Debug/FortMac.app
(7 files, 20,870,426 bytes), adjacent FortHost.app (7 files, 20,802,564 bytes), and
/tmp/fort059-native-history-ios/Build/Products/Debug-iphonesimulator/FortiOS.app
(7 files, 16,873,406 bytes). Full file/mode/hash inventory, including Debug dylibs:
/tmp/fort059-native-history-artifacts.json, SHA256
6482e32a7a10df945560bb2634187adc6bcbd042cb4964ccaa113ebe94096066.

Fresh review assembly: /tmp/fort059-native-history-review-20261018/Fort.app,
186 files, 414,957,609 bytes. Every report hash and size independently checked.
package-report.json SHA256
6e892261fc34f205cd87b4f9b9ca4630c76d1b2a36b64cbd19231a23646fe3d9.
The same pinned 94-file maintenance resources and six Codex legal resources remain.
The artifact is unsigned, uninstalled, unlaunched and unregistered.

Native command: xcrun swift test --package-path successor/apple/FortKit
--scratch-path /tmp/fort059-maintenance-swift --no-parallel, with FORT_TEST_NODE set
to /opt/homebrew/opt/node@26/bin/node, FORT_TEST_GUARDIAN to
/tmp/fort059-assembly-guardian, FORT_TEST_HOST_RESOURCES to
/tmp/fort059-maintenance-host-resources and FORT_TEST_CORE_ARTIFACT to its core child.
The initial run also supplied the now-removed duplicate history resource variable.
The benchmark additionally selected retainedHistoryDeletionCoordinatorRetryBudget
with FORT_HISTORY_REMOVAL_BENCHMARK=/tmp/fort059-native-history-samples.json.
Builds used FortSpec058.xcodeproj, FortMac/FortHost/FortiOS schemes, generic macOS
or iOS Simulator destinations, Debug and CODE_SIGNING_ALLOWED=NO. Assembly used the
same reviewed explicit Node/Codex/guardian/configuration inputs as the preceding
attribution package, with these fresh native products.

## Remaining acceptance

Same-local-network scope remains the immediate iPhone test boundary; transport stays
modular for a later owner-managed VPN. No network/security setting or access was
activated. The HTTPS client adapter exists, but a secure reachable host transport is
still independent implementation and acceptance work; private core IPC is not it.

Read-only Apple account verification is unavailable through the currently callable
tools. An owner/admin with existing access must verify team T3JB5MYZ93 and exact
client io.mtree.fort.mac / io.mtree.fort records, helper io.mtree.fort.mac.host and
group group.io.mtree.fort.mac before any missing-resource provisioning proposal.
Local profile absence does not establish remote absence. No login, account mutation,
profile creation, signing or agreement acceptance occurred. No new legal agreement
was identified by this slice; actual account/provider terms still need verification.

Installed accessibility/screenshots, actual App Group/Keychain/startup behavior,
authorized real-provider checks, physical iPhone, TestFlight archive/upload/processing/
tester availability and remote CI remain unrun or unverified. This commit does not
establish v1 or release completion. Exact existing grants and consumed demos cannot
be reused to fill these gates.

## Raw local evidence hashes

- `/tmp/fort059-native-history-red.log`: SHA256 `755f28b5d29b4878568e01f40dcd18878b5279bf2135f6006cc843d0852c9426`.

- `/tmp/fort059-native-history-race-red.log`: SHA256 `c0c437c61d590723aa736607bd7d4f62870e9061f7a2694def917ed1ece491d4`.

- `/tmp/fort059-native-history-race-green.log`: SHA256 `e33b95ee93591bec7a5fd4acd1c4e160a2db77a77079a149c52fe6e2f0bd2a5d`.

- `/tmp/fort059-native-history-model-red-compile.log`: SHA256 `827c31f105da5fb06ce0f9aa9678188a3a42b9eb7c8849241aaf33073b27b717`.

- `/tmp/fort059-native-history-model-red.log`: SHA256 `0377a9ac6765794bf53b93b0c326e568dd21222fd6a0691ceb697cf3a1c912ac`.

- `/tmp/fort059-native-history-model-green.log`: SHA256 `cb59c5c9c31cc3922a4ce084e9b907fb8ec4d2aa403038761dcb94775e1f30bc`.

- `/tmp/fort059-native-history-controls.log`: SHA256 `d120e39c058f61320dcd420e347adcc91e1709b7a62d489a8ed2f79c619fb103`.

- `/tmp/fort059-native-history-budget.log`: SHA256 `cfd341a7ad500fb2357e2cf03fb727d23425cb1e306aebd468850200a2967440`.

- `/tmp/fort059-native-history-samples.json`: SHA256 `c3d0bdb7e8b5f037a5073db786111ca1e1d877f68cdf6459443251b6b1bfb596`.

- `/tmp/fort059-native-maintenance-program-red.log`: SHA256 `9f78661f96f08387252e7f370223832f5c9a398f71b10c34f4a5704ed77686b1`.

- `/tmp/fort059-native-maintenance-program-green.log`: SHA256 `8903074f051a2be11739991cc4e0846f322f9bed9d341e01a75b880f5651f903`.

- `/tmp/fort059-native-history-packaging-red.log`: SHA256 `aedadd1fc07577c1d2de204c811910e9ec08067c0ed6865d0dad00a7e21d8198`.

- `/tmp/fort059-native-history-packaging-green.log`: SHA256 `577021027b5d0d9c376f0c0f5689d51bb3fa265cf70ed8517c8297a181b15067`.

- `/tmp/fort059-native-history-ci-skip.log`: SHA256 `431d2170ca640836aea43deff92e8deca80d5372f419d2aa1c1e512f82f285da`.

- `/tmp/fort059-native-history-full.log`: SHA256 `0ae183fde682e7d52ae8a01863d79a879ac1612ae1b317d2a5140e1ba5be2023`.

- `/tmp/fort059-native-history-typecheck.log`: SHA256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

- `/tmp/fort059-native-history-portable-sandbox-failure.log`: SHA256 `1f8ccdd51276bdb4904b794ca58cf2e1e517341200e40060052f2ee5a47cb696`.

- `/tmp/fort059-native-history-portable.log`: SHA256 `3b6dd3992e63a0b6036c3a3a14b221dd33031dab8720b303844e4755c2c9a167`.

- `/tmp/fort059-native-history-mac-build.log`: SHA256 `f30cff8f802dcbe2f51f7bf00953aa9641ce9e81a6f05201e3465285b39c57fd`.

- `/tmp/fort059-native-history-helper-build.log`: SHA256 `76cd3753c8db5a5689f3173b93609f414a0d53041752d3ac768e255ff9fb0726`.

- `/tmp/fort059-native-history-ios-build.log`: SHA256 `cb6361cbcc35126e172af243481400f29a1d4b80e90251ca5af285ea3bfc36ba`.

- `/tmp/fort059-native-history-app.log`: SHA256 `44f3a4bf1b5c328198a3d7b405022dd6dadca427eecae7d706861a7d2fefaadd`.

- `/tmp/fort059-native-history-final-native.log`: SHA256 `6c210eacfe4272a7ffda738d43643f792c468d8602a80d82fe7a9010a2ddb826`.
