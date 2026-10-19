# Static privacy and runtime inventory

October 19, 2026. Evidence refinement under [Spec 059](../../../specs/059-modular-node-v1.md).
Product source is `60b144706a4f58852a8a5634c12a2ba758b54f0e`; inventory base is
`ce3f2936a645a9bc528caf6bec98dd7f33a73a10`. No product source, entitlement,
privacy manifest or remote declaration changed. This is a completed static
inventory, not a complete runtime privacy audit or release acceptance.

## Exact artifacts and method

- Signed, unlaunched Mac: `/tmp/fort059-development-signed-20261019/Fort.app`.
  Signed-package report SHA-256:
  `20ed08cc2df56549234a63f09c3da9b1854cc05a08958cbdf18301cdaa03b559`.
- Unsigned iPhone device build:
  `/tmp/fort059-release-assets-build-ios/Build/Products/Release-iphoneos/FortiOS.app`.
  This is a generic-device build, not physical iPhone execution.
- [Machine-readable inventory](privacy-runtime-inventory.json) records every
  binary's relative path, SHA-256, bytes and matches, plus native source hashes.

The read-only script `/tmp/fort059-privacy-inventory.py` walks regular files,
recognizes Mach-O/fat headers, hashes each file and runs `/usr/bin/nm -u` on it.
It found 30 Mac objects and one iPhone object; all 31 symbol reads exited zero.
No `PrivacyInfo.xcprivacy` exists in either actual artifact. The source search
covers production Swift under FortKit/Sources and Apps, excluding test code.
C symbols match exact names after removing the leading underscore and Darwin
suffix. The earlier broad substring scratch scan was discarded because it matched
unrelated names such as RAND_status. The final JSON separates CFPreferences,
mach_continuous_time and possible Swift names as additional review signals.

Undefined symbols indicate linked imports, not executed behavior. Static linking,
inlining, framework calls, selectors and dynamic lookup can hide API usage; absence
from this list does not prove absence. No shipped app, helper, Node or Codex process
was executed by the inventory. The guardian has no match in the selected symbol
set; that is not an all-API or security certification.

## Source and payload findings

| Component | Verified static observation | Interpretation and remaining work |
| --- | --- | --- |
| Native iPhone binary | fstat, lstat imports | Shared native file safeguards need a justified declaration; binary linkage alone cannot identify every caller |
| LocalChatImages.swift:50 | fstat checks regular-file type and size before bounded image/cache reads | Trace covers the explicit file importer and private cache; it does not read source file timestamps or send images to a provider |
| OwnerSetupFiles.swift:24,36,52; SavedHistoryRemoval.swift:24,31,39,94 | Ownership, mode, link count, size and inode checks on private setup/history files | Metadata access still falls within the listed API family even without reading a timestamp field |
| MacAgentStorage.swift:145–184 | App Group storage metadata; ctime comparison protects exact auth-file removal | Mac-only code; does not read auth contents for this check |
| MacHostBundle.swift:116–136 | Bundle directories, manifest and profile reads checked against type, size and mtime/ctime changes | Mac-only; installed app-bundle location and any applicable reason still require assessment |
| MacCoreHost.swift:97; MacCoreProcessTransport.swift:186 | Private socket absence and private-directory checks | Mac-only metadata access |
| MacCoreProcessTransport.swift:366,379 | systemUptime measures shutdown elapsed time and 2/4/6-second deadlines | Mac-only local timer use; no boot-time export in these callers |
| Node/libnode | fstat, stat, mach_absolute_time imports | Review exact Node 26.5.0 uses and shipped library responsibilities |
| libuv | fstat, lstat, stat, statfs; separate mach_continuous_time signal | File/disk metadata imports need source-to-behavior review |
| SQLite | fstat, lstat, stat, fstatfs, statfs | Determine exact bundled version's reachable uses before assigning reasons |
| OpenSSL crypto/SSL, c-ares, ICU | stat and/or fstat imports | Part of the bundled dependency closure; not covered by Fort's native-source conclusions |
| Codex | fstat/fstatat/lstat/stat and fstatfs/statfs/statvfs; CFPreferences signals | Exact CLI is 0.157.1; preferences signals are not automatically classified as Apple's UserDefaults API |

The ordinary app constructs its private Application Support directory in
`Apps/FortApp.swift:62`; Mac runtime state resolves the exact App Group container
through `MacAgentStorage.swift`. Local image selection runs the normal SwiftUI
file importer, then requests security-scoped access in `MessengerModel.swift:1020`.
Installed picker/container behavior remains unverified. Source searches found no
explicit production UserDefaults, available-disk-space or active-keyboard call in
these Swift directories. That negative search has the same indirect-use limits.

## Declaration decisions

Apple's [required-reason overview](https://developer.apple.com/documentation/bundleresources/describing-use-of-required-reason-api)
explicitly names iOS, iPadOS, tvOS, visionOS and watchOS. Its key documentation also
lists macOS availability. This inventory does not assume identical Mac enforcement
from the mobile overview; confirm the final distribution requirements.

From Apple's [API category and reason list](https://developer.apple.com/documentation/bundleresources/app-privacy-configuration/nsprivacyaccessedapitypes/nsprivacyaccessedapitype):
file-metadata reasons C617.1 and 3B52.1 distinguish private containers from files
explicitly chosen by the user. They are candidates for the traced native paths,
not declarations already made. Timer reason 35F9.1 is relevant to local elapsed
time. Do not apply a disk-space or preferences reason solely because a runtime
imports a related symbol. Review actual use, permitted scope and off-device limits.

Before shipping, resolve native target/platform coverage and each bundled runtime's
manifest responsibility, inspect reachable SDK uses, then add only substantiated
entries to the appropriate bundle and inspect the resulting archive/privacy report.
No empty manifest, blanket Data Not Collected answer, tracking declaration or
export-encryption exemption was added. Provider retention and final Apple answers
remain outside this static proof.

## Runtime controls and validation

`runner/private-codex-plan.ts` supplies private home/state/workspace/tmp paths,
requests disabled analytics/startup updates, disables OpenTelemetry exporters and
prompt logging, and pins the authentication/storage class. `codex-controls.ts`
checks effective configuration and feature inventories before execution. These
specific controls do not prove provider retention or an absence of every outbound
runtime behavior. The agent host remains distinct from its computer-tool sandbox.

Focused existing behavior tests ran with Node 26.5.0:
`node --test runner/private-codex-plan.test.ts runner/codex-startup-contract.test.ts`
from successor. Result: **407 passed, zero failed, one optional budget test skipped**.
They use synthetic files/protocol responses; no provider sign-in or runtime launch.
Full application suites/builds were not repeated for unchanged product source.

The Codex third-party attribution inventory remains intentionally conservative:
its lock graph includes development/build/other-platform dependencies. It is not a
binary SBOM or a substitute for the remaining source/payload privacy audit. The
[submission worksheet](../../release/SUBMISSION-DRAFT.md) retains contact, URL,
provider-disclosure and encryption decisions. Signed installed and TestFlight
acceptance remain separate.
