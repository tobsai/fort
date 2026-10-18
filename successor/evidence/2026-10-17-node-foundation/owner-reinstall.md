# Retained-history reinstall reconciliation

Source base: `dcdc013789333bc01f5af14a6231ee671f50726c`, followed by this local
slice on October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md)
and existing implementation authority govern it. The required
[Ponytail report](../../../docs/architecture/reviews/ponytail/current.md)
binds reviewed staged contents and modes.

## Behavior and limits

The ordinary Mac host view offers Set up Fort again when saved removal and local
cleanup records are complete. Inspection reads metadata only. The explicit action
rechecks the pinned app/helper/endpoint identity, exact recovery receipt, independent
helper exit, startup not registered, matching disconnected/confirmed-revocation
records, absent active/pending connection and absent exact credential.

The private owner-file lease covers the operation. Metadata is revalidated across
waits and immediately before deletion; cancellation keeps the completed record.
Only `host-removal.json` is removed, with directory sync. History, revocation and
disconnected markers remain. A retry after successful reconciliation is inert.
The action does not enable startup, create/remove credentials, start the helper,
pair, repeat revocation or delete history. Enabling and connecting remain separate
actions. Native status distinguishes recorded removal proof from current host status.

## Verification

| Check | Result and limit |
| --- | --- |
| Public red | Refusal-only reconciliation threw setupBlocked. Native model had no usable action; its three cases failed 15 expectations. `/tmp/fort059-reinstall-red.log`, `/tmp/fort059-reinstall-model-red.log`. |
| Focused behavior | Four functions passed, including 11 refusal variants, three native model cases, successful inert retry and cancellation/retry. `/tmp/fort059-reinstall-controls.log`. |
| Native regression | 236 functions passed. All budget, retained-signature and complete-host journey functions excluded from the parallel run. `/tmp/fort059-reinstall-native-full.log`. |
| Reconciliation timing | 20 isolated synthetic samples, nearest-rank p95 2.111875 ms, maximum 2.115667 ms; target 500 ms. `/tmp/fort059-reinstall-budget.log`. |
| Strict TypeScript | Passed. `/tmp/fort059-reinstall-typecheck.log`. Core source was unchanged by this slice. |
| Fresh native builds | Unsigned FortMac and FortiOS Simulator Debug builds passed. `/tmp/fort059-reinstall-mac-build.log`, `/tmp/fort059-reinstall-ios-build.log`. |
| Preservation | Foundation tree identical to quarantined fork checkpoint `ac2dd2ae55b3df4235d352f34937c33f1fe932c3`; primary checkout HEAD remains `5616faa2541b6ef2377f0ee222ec6aefff8a5feb` with unrelated dirty work preserved. |

Refusal variants cover requested/recovered progress, active connection, pending
setup, foreign disconnected identity, unconfirmed local cleanup, re-enabled startup,
live recorded helper, credential lookup failure, changed bundle and replaced progress.
The model rejects a retained credential or invalid bundle, permits retry, and proves
zero credential reads during inspection and zero registration/pairing during reconciliation.

The latest unchanged portable source passed 1,104 tests with five optional budgets
skipped at the preceding re-pair slice. Its complete compiled native journey passed
in 1,668.438 ms with all owned groups absent. Those checks were not rerun here;
this slice changes native removal/model/UI and their tests, not core/executor source.
The current 236 native functions include the affected removal and setup regressions.

Fresh unsigned executable hashes (not signatures or whole-bundle digests):

- `/tmp/fort059-reinstall-mac-build/Build/Products/Debug/FortMac.app/Contents/MacOS/FortMac`:
  `cc59418e92eaf813a7a741fa2c54c7e3fd84a5aace41c08cae9e784781ffefe0`.
- `/tmp/fort059-reinstall-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app/FortiOS`:
  `c7cbe6007db3074caa02728e387aa081ec3b8f89fadfeba1233c6ecc50943c24`.

## Timing conditions and samples

MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2, Swift 6.4. No concurrent task
tests/builds. Each sample reconciles one private confirmed record after its fixture
helper has exited, using authored bundle/startup/credential checks. Fixture setup,
real Keychain/signature verification, macOS startup callbacks, provider and installed
UI are excluded. The measured path includes record reads, process absence observation,
unlink and directory sync. Raw samples: `/tmp/fort059-reinstall-samples.json`.

```json
[0.892833,0.898625,0.964959,1.0755,1.203083,1.2265,1.369,1.356875,1.579833,1.543916,1.842916,1.833667,1.8285,1.849,1.835958,1.831625,1.829459,2.00925,2.111875,2.115667]
```

## Remaining gates

The local reinstall source gap is resolved; actual installation/removal/reinstall
still needs exact installed-build acceptance. History deletion stays disabled until
its exact local/host scope is settled. Runtime/package legal closure, current Apple
account/profile identity, provisioning and persistent activation approval, real
provider/Keychain, installed accessibility/screenshots, physical iPhone and TestFlight
remain unverified. Fresh tool discovery exposes no authenticated Apple account or
desktop UI automation tool here. No account connection, signing, provider execution,
startup registration, owner-file/credential mutation, deployment or cleanup occurred.
This is source, fixture and unsigned-build evidence, not v1 or release acceptance.
