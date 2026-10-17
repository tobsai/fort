# Complete-bundle storage and native review — October 17, 2026

Source `9d4aaef92630698b79a2a7a4885ea2dc8f46172f` follows the
[chat recovery milestone](../2026-10-17-mvp-recovery/README.md) on isolated branch
`codex/fort-openai-rebuild-20261016`. This increment adds inert complete-bundle
storage and native review; it does not complete runtime bundle execution or the MVP.

## Implemented behavior

Authenticated JSON import stores the full manifest and every regular file as
canonical base64 with computed byte counts and SHA-256 digests. The manifest records
source, license, version, instruction entry, supported runtimes, requested tools,
and optional script interpreter/resource declarations. A digest covers those
fields and all file contents/metadata. Imports execute nothing and write no bundle
file into a runtime workspace. Symlinks, traversal, absolute paths, colliding paths,
undeclared executables, unknown fields, forged digests/sizes, malformed content and
oversized input are rejected before staging. Limits are 64 files, 256 KiB per file,
512 KiB total content, and 16 KiB for the UTF-8 instruction entry; larger content is
rejected rather than truncated. This increment accepts JSON bundles; archive and
folder packaging are not implemented.

Each import creates a new immutable content revision requiring explicit review of
its exact digest. Stale enablement conflicts. Prior revisions remain exportable.
Accepted turns keep their exact reviewed contents after an edit. Disabling a pinned
revision before dispatch prevents its runtime call. The owner can also disable an
older revision without changing the current revision's review. Enablement records
owner review; it does not grant the bundle's requested runtime authority.

The shared native UI imports and exports complete bundle JSON, shows source/license,
file text or binary size/hash, changed paths, requested tools, runtimes and script
resource declarations, and provides explicit enable/disable actions. It retains
its loaded revision while review is open. Unreviewed, disabled and unknown review
states cannot be selected for a new native send. The service remains the authority
when a stale client submits. Complete-bundle imports cannot use the old instruction
edit endpoint to silently discard their assets.

The existing no-tool Codex adapter refuses unsupported bundle assets, scripts,
requested tools or runtime declarations before any protocol request. It can accept
an enabled single instruction-file bundle. General runtime packaging and execution
remain open. The older instruction-only editor/API is preserved; migrating that
historical path into the explicit complete-bundle workflow is also remaining work.

## Validation

- Complete TypeScript suite: **631 passed**, no failures or skips.
- FortKit: **26 passed**, including red-to-green rejection of unreviewed/disabled/
  unknown bundle states in the native send model.
- Strict TypeScript: passed, exit 0.
- Swift HTTP conformance: recovery journey still passes; native bundle import,
  binary asset preservation, review, changed-file display data, prior-revision
  lookup, stale-review rejection, older-revision disable and export/import passed.
  Re-import preserves content/digest and requires new review. The bundle phase
  added zero runtime executions. [Exact output](native-http.txt).
- New Mac and iPhone Simulator build-for-testing: both succeeded. Strict/deep
  signatures passed for both apps and both test runners using the host trust
  service. [Artifact inventory](builds.json).
- All six consumed attempts, the two-file native cache and the main checkout
  baseline remain unchanged. [Preservation check](preservation.json).

The first import test failed at the missing HTTP route; disabled-revision and
unsupported-runtime tests failed before their guards; the native selection test
failed for all three unsafe review states before implementation. Typechecking also
caught a literal type widened by a map and assertion-driven empty-array narrowing;
both were corrected before the full suite. [Log paths/hashes](checks.json).

## Fresh artifacts

- `/tmp/fort058-mvp-bundles-mac-20261017/Build/Products/Debug/FortMac.app`
- `/tmp/fort058-mvp-bundles-mac-20261017/Build/Products/FortMac_macosx27.0-arm64.xctestrun`
- `/tmp/fort058-mvp-bundles-ios-20261017/Build/Products/Debug-iphonesimulator/FortiOS.app`
- `/tmp/fort058-mvp-bundles-ios-20261017/Build/Products/FortiOS_iphonesimulator27.1-arm64-x86_64.xctestrun`

Earlier recovery and accepted one-turn build directories are preserved. No app,
Simulator or live provider was launched, no new live enrollment issued, and no
Keychain/provider credentials, settings, entitlements, trust configuration or
startup registrations changed during this work. No source was pushed, merged or
deployed. Native UI accessibility/screenshots and actual-provider acceptance of
these artifacts remain unrun.

## Current handoff

One consolidated request for the bounded temporary enrollment in
[MVP-20261017.md](../../MVP-20261017.md) was submitted and is still pending as of this
record. No timeout is treated as approval. The existing installed UI operator
remains `01a1451b-81d7-76cf-8e5c-a85d7164201c` through the parent. The new source and
artifact pins are independently reviewable without resetting any prior packet.

Native routines and durable schedules, full bundle runtime support, provider/
topology feasibility, physical iPhone/shared history, OS isolation, lifecycle,
backup/export/recovery and distribution gates remain open. The agent host still
has the Mac user's access. This record makes no complete-MVP or release claim.
