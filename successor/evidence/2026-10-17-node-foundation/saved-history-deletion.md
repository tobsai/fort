# Saved client-history deletion — October 18, 2026

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md), reusing
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
The coordinator explicitly authorized deletion implementation/tests with disposable
synthetic fixtures. No real owner history, Keychain entry or provider data was read
or deleted. This is a client-cache feature, not complete v1 or host-ledger erasure.

## Behavior and boundaries

The Mac pairing view offers a separate confirmation to delete the saved copy for
one exact installation. It captures that connection when showing confirmation and
rechecks it at the action. Confirmed revocation, no active/pending pairing and absent
exact credential are required. Stop/uninstall still keep history by default.

Storage removes that installation's navigation/conversation snapshots, pending
submissions and local image previews. Other installations and arbitrary files stay.
The host ledger, settings, provider records, backups, app files, empty directories,
disconnection/revocation records and host-removal progress stay. The UI says what
is retained. Full host-history deletion and iPhone cache deletion remain separate
implementation work; unsupported full removal deletion stays disabled.

An existing exclusive owner-file lease excludes cooperating setup operations.
Private directory descriptors pin reads/unlinks; no-follow/nonblocking opens,
owner/mode/ACL/link-count checks and exact format/path hashes reject unsafe inputs.
The whole pass is bounded to 4096 entries and 256 MiB decoded, with 16 MiB per file.
Preflight verifies selected file identity/content after credential rechecking, then
rechecks each file before unlink and syncs its directory. Final rescan confirms
absence. This is not a transaction across multiple files: interruption can leave
partial deletion, which reports incomplete and permits an idempotent retry. It is
not secure overwriting or erasure of filesystem/provider backups. Same-user hostile
code is not confined by a cooperative lease; the agent-host boundary is unchanged.

The model clears its visible/cache projections only after success. A failed attempt
keeps the displayed data and an incomplete result. No network, reconnection, startup
or deletion is performed while inspecting availability.

## Verification

- `/tmp/fort059-history-red.log`: public storage test failed at the refusal stub.
- `/tmp/fort059-history-green.log`: first implementation compile failed for missing
  Swift `try` annotations in the compound guard; corrected before execution.
- `/tmp/fort059-history-model-red.log`: storage positive/refusals passed; native
  action failed its expected visible-state assertions while still an inert stub.
- `/tmp/fort059-history-focused-final.log`: mutation test exposed validation order
  after the credential check. The test was narrowed to the selected installation's
  file, and full verification moved after that check.
- `/tmp/fort059-history-identity-final.log`: four focused functions passed, including
  11 refusal cases, three mutation/interruption cases, exact retained data, retry,
  stale confirmation identity and native success/failure projection.
- `/tmp/fort059-history-native-full.log`: 240 native functions passed before the
  final confirmation-capture refinement; final native/build results follow below.
- `/tmp/fort059-history-typecheck.log`: strict TypeScript passed; no TS behavior
  changed. The preceding 1104 portable tests remain separate prior evidence.

## Isolated timing

Twenty fresh fixtures, each with 100 added snapshots plus navigation, one pending
submission and one image record for the selected installation; another installation
and unrelated sentinel are retained. Each measurement covers verification, unlink,
sync and final absence check; fixture creation and removal are excluded. The fake
credential provider is in memory, so this does not establish real Keychain latency.
MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2; Xcode 27.1 / Swift 6.4. Warm compiler
cache, fresh private data per sample, no concurrent task test/build during timing.

`/tmp/fort059-history-budget.log` and `/tmp/fort059-history-samples.json`:
p95 **43.269041 ms**, maximum **43.652875 ms**, against <=500 ms.

```json
[43.652875, 42.656083, 42.752125, 42.662375, 42.425333, 42.400917, 42.422334, 42.465083, 42.471541, 42.474792, 42.451083, 42.398209, 42.276458, 42.5515, 42.831625, 42.866875, 42.446875, 43.028, 43.269041, 42.52725]
```

Actual installed accessibility/screenshots, action-time owner authorization for
real deletion, Keychain/startup, provider behavior and TestFlight remain unrun.

## Final source and builds

- `/tmp/fort059-history-reopen-red.log`: reopening after deletion incorrectly
  claimed saved history remained. The new model regression failed, then passed
  after restore derived its wording from the saved navigation actually present.
- `/tmp/fort059-history-native-final.log`: **240 native functions passed** after
  the final production change, with budgets/signed probes/journey excluded.
- `/tmp/fort059-history-final-focused.log`: **four focused functions passed**;
  the refusal function now covers 15 cases, including file-size/entry limits,
  missing revocation and a mismatched content/path identity. The mutation function
  covers three cases. No production source changed after the full native run.
- `/tmp/fort059-history-mac-final.log` and `...-ios-final.log`: final unsigned
  Mac and iPhone Simulator builds passed. CoreSimulator remains older than this
  Xcode requires; these are builds, not launched/installed UI evidence.

Exact artifacts:

| Artifact | Debug code SHA-256 | Complete file/mode manifest SHA-256 |
| --- | --- | --- |
| `/tmp/fort059-history-mac-build/Build/Products/Debug/FortMac.app` | `440ae568348c2b74107b08da018cdd715822fc90ba0d5069af8d7dd9510e0bcc` | `70dca109524283a4e2289009d06c3ee14bf39867fff51d63493da50b026cceb3` |
| `/tmp/fort059-history-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app` | `d9b11448734b957030ce68a9ae01665485779b7190aaabdea56809ad01095f83` | `a3a4875a4694b107f5f7e58bbda4a43720422141707bd120ac31a137894acbab` |

Xcode's Debug executable is a small launcher; compiled product code is in its
`.debug.dylib`. Those code hashes and the seven-file manifests identify these
artifacts more completely than the launcher hash alone. Manifests are
`/tmp/fort059-history-FortMac-manifest.json` and `...-FortiOS-manifest.json`.
Each hashes sorted relative-path/mode/file-content entries serialized as compact
sorted-key JSON plus a newline. Source paths in the build logs point to this worktree.
No helper/runtime payload is embedded in these client artifacts yet.

[Distribution research and retained-runtime audit](distribution-requirements.md)
separate license obligations, package engineering and actual Apple/provider access.
The donor tree still matches quarantined commit `ac2dd2a` and the primary checkout
was untouched by this work; its unrelated dirty-file count changed externally.
