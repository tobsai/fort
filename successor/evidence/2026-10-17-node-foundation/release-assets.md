# Native release assets, October 19, 2026

Base `10712e8419c0374f315fa7e315dda5c996c31371`, isolated branch
`codex/fort-openai-rebuild-20261016`, under [Spec 059](../../../specs/059-modular-node-v1.md).
The delegated scope is local branding, icons and truthful submission drafts.
No source publication, account/security setting, signing or service activation is
part of these artifact changes.

## Changes and provenance

Ordinary Mac/iPhone `CFBundleDisplayName` is now Fort. The exact engineering bundle
IDs and executables remain unchanged. FortQA still says Fort QA First Wave and
retains its synthetic/no-network labels. The existing runtime fixture-evidence
labels describe actual synthetic results and were retained. No Swift/TypeScript
runtime source changed.

A shared `Apps/Assets.xcassets/AppIcon.appiconset` supplies ten macOS slots and
one iOS 1024-pixel slot from seven unique PNG resolutions. Images are byte-identical
to Fort's native assets at commit `0960a39b294e2231614c47efb03ac2072f8ef5b0`.
The earlier brand refresh is `98d0dcd05944edb0f15bedebda565c0cb4022e61`.
Only the image bytes were reused, with no legacy code or donor artwork.

Each PNG has the expected square dimensions, 16 through 1024 pixels, and no alpha.
The catalog names AppIcon in ordinary and QA targets. XcodeGen 2.45.4 regeneration
is byte-for-byte reproducible. The original artwork and decoded compiled macOS
icon were visually inspected. [Exact image hashes, commands, output inventories
and results](release-assets.json) retain provenance and compiled evidence.

## Verification

The public check reads actual built Info.plist/icon resources. The old ordinary
review app failed all three requirements: it said Fort Fixture, lacked Assets.car,
and declared no icon. The fresh builds passed. The QA check also requires its
explicit QA display name. The checker is `/tmp/fort059-release-branding-check.py`;
its SHA256 is recorded in the JSON evidence.

| Build | Configuration and destination | Result | Seconds |
| --- | --- | --- | --- |
| FortMac | Release, macOS | Passed, name Fort and compiled AppIcon.icns/Assets.car | 53.244 |
| FortHost | Release, macOS | Passed, helper name Fort Host unchanged | 3.442 |
| FortQA | Debug, macOS | Passed, explicit QA name and compiled icon | 24.332 |
| FortiOS | Release, generic iOS device | Passed, name Fort and primary AppIcon metadata/resources | 27.038 |

All used `CODE_SIGNING_ALLOWED=NO`; none was installed or launched. Output roots
are `/tmp/fort059-release-assets-build-{mac,qa,ios}`. The helper reused the Mac
DerivedData; the other roots were fresh. These are one observed duration per build,
with existing shared OS/compiler caches, not a performance benchmark or percentile.
They fall within the existing 120-second native clean-build target but do not
establish production startup or runtime performance. Logs are
`/tmp/fort059-release-assets-{FortMac,FortHost,FortQA,FortiOS}.log`.

Fresh inert assembly at `/tmp/fort059-release-assets-review-20261019/Fort.app`
preserves the compiled name and icon. It contains 202 files and 407,452,451 bytes;
all output hashes/modes/lengths and all 200 input hashes/modes were independently
verified. The package report SHA256 is
`8e85bc4cfd50b767f27ff4255216b54a5b49b7daee44059eb180f55f6ffd59c5`.
It is unsigned, unlaunched and has zero embedded provisioning profiles. Existing
runtime/attribution inputs and earlier review packages remain preserved. Source
resource cost and exact payload membership are recorded in the JSON inventory.

Full runtime/native behavior suites were not repeated for unchanged runtime code.
This slice establishes build/resource correctness, not installed UI/provider,
physical-device, signing, archive, upload or TestFlight acceptance. Existing
CoreSimulator component warnings were not repaired or bypassed; the iPhone build
uses the generic device destination and makes no Simulator execution claim.

## Drafts and remaining work

[SUBMISSION-DRAFT.md](../../release/SUBMISSION-DRAFT.md) contains bounded beta text,
a reviewer checklist, contact/URL decisions, a data-flow worksheet and unresolved
privacy/export answers. Nothing was submitted. It does not claim that all data
stays on-device, that every credential is in Keychain, or that a fixture screenshot
is a store screenshot. No empty privacy manifest or unreviewed encryption exemption
was inserted to make the package appear ready.

The acceptance audit found two independent release omissions beyond these assets:
a complete required-reason API/bundled-runtime privacy inventory, and final support/
privacy contact and published URL decisions. The draft now names concrete source
facts for that review, including elapsed-time use of system uptime and the bundled
Node/OpenSSL/runtime payload. Phone listener/management UI composition remains a
separate implementation item deferred behind the owner's Mac-first priority.
Remote CI, signing, actual host/provider, physical devices and TestFlight gates
remain distinct. No repeated fixture run substitutes for these missing results.

Apple access/provisioning changed independently during this work. The current
[activation record](../../APPLE-HOST-ACTIVATION.md) records the authenticated exact
team, supplied Program License Agreement confirmation, existing Fort iOS record,
missing proposed Mac identifiers/group/profiles and the separately observed
Paid Applications Schedule banner. No portal record or agreement was changed.
