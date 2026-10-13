# UTM app inspection — passed, detached and cleaned

The official pinned UTM **4.7.5 (118)** app passes strict code-signature verification
and Gatekeeper accepts it as **Notarized Developer ID** on this arm64 Mac running
macOS 27.2 (26B5101f). The actual signer is **Developer ID Application: Turing
Software, LLC (`WDNLXAD4W8`)**, chaining through Developer ID Certification Authority
to Apple Root CA.

All **80 Mach-O files** (including the main executable), their **160 architecture
signatures**, and **78 code bundles** passed the recorded checks. Every inspected
architecture has that same signer and Team ID. These are static artifact results;
no app, helper, CLI, QEMU executable or VM was launched.

The exact read-only image was detached normally. All **five owned staging objects**
were removed, and a separate final script confirmed **24 paths absent**, no task
disk image attached, and no task/UTM app process. The earlier inspection and
proposal remain unchanged. Existing `reviews/` files were untouched.

## Authority and artifact

Owner approval `Sentinel_ea51da06ee408191a8b7ea5327e0f437` on 2026-10-13 followed
request `Sentinel_9e0f867178c481918604946ff36fbd87` and clarification
`Sentinel_16d91dfa308c8191ae23c21f5e29bbd0`. Only reference IDs and the delegated
scope summary were recorded, not transcript bodies. This changed only the stop
condition for the known outer-container primary-signature result. No macOS
security-policy exception was created.

Starting commit: `501dc395c4bf3f7d15d1eb9829a5ad316c844260`. The original proposal
is `ec6dd956bd6ebf45ec685331625ccc429e59b7f6`. Its seal and all prior-assessment
sealed files were verified unchanged before work and after cleanup.

- Source: [official UTM v4.7.5 DMG](https://github.com/utmapp/UTM/releases/download/v4.7.5/UTM.dmg).
- Exact size: **250,021,057 bytes**; SHA256:
  `a8435c93cfb5f8bbfeea4b134cfad1ac66b67632b75e438c63b1a8ae043bef0e`.
- Confirmed app ID: `com.utmapp.UTM`; version **4.7.5**, build **118**.
- Main arm64 CDHash: `19fc43aa858d42d2ec595ade708af75bfdf19572`.
- Main x86_64 CDHash: `7cc2bc96dc4a5c08684dbe8c37c231676ec40e9d`.
- Logical app bytes: **1,217,291,628**, below the 2 GiB app cap. No app copy was
  made. Conservatively counting the DMG, mounted app and evidence stayed below
  the 3 GiB task cap; the 32 GiB free-disk reserve was retained.

## Checks and supporting evidence

| Check | Result | Evidence |
| --- | --- | --- |
| Fresh paths, approved scope, disk reserve | Passed | [authority/preflight](authority-and-preflight.json) |
| Reacquired exact source, bytes and hash | Passed | [download](01-UTM.dmg.json), [downloader provenance](downloader-provenance.json) |
| DMG integrity and stapled ticket | Both exit 0 | [integrity](dmg-integrity.json), [stapler](dmg-stapler.json) |
| Read-only, non-browsing, no-autoopen mount | Passed; read-only flag verified | [mount identity](mounted-identity.json) |
| Main app deep strict signature | Exit 0 | [codesign](app-codesign-verify.json) |
| Main app Gatekeeper execute assessment | Exit 0, accepted / Notarized Developer ID | [Gatekeeper](app-gatekeeper.json) |
| Nested/all-architecture inspection | 318 codesign commands passed: 80 file verifications, 160 architecture displays, 78 bundle verifications | [inventory](nested-code-inventory.json), [raw receipts](nested-codesign-receipts.jsonl), [signatures/entitlements](nested-signatures-and-entitlements.json) |
| App contents unchanged before detach | All 1,180 manifest entries match: 518 regular files, 437 directories, 225 internal symlinks | [manifest](mounted-app-manifest.json), [pre-detach](pre-detach-validation.json) |
| Legal-file inspection | Filename/text inspection of matching bundled notices, retained hashes and public attribution excerpts | [legal inspection](legal-file-inspection.json) |
| Exact candidate state paths | All 17 absent; presence checks only | [path record](candidate-state-paths-before-cleanup.json) |
| Normal detach and owned cleanup | Passed | [detach](detach-result.json), [cleanup journal](cleanup-journal.jsonl), [cleanup result](cleanup-result.json) |
| Separate receipt/absence verification | Passed; 24 absent paths, no task image/process | [validation](validation.json) |

There were **no failed artifact checks this round**. The earlier DMG open /
primary-signature rejection remains preserved and was not repeated or rewritten.
The DMG staple plus the app's Gatekeeper notarization assessment were used; no
separate app-staple validation or individual Gatekeeper assessment of each nested
helper/library was performed. Product test suites were not rerun because no
product source changed. Python syntax and JSON/JSONL validation passed.

## Entitlements and boundary implications

The main app declares App Sandbox, the group
`WDNLXAD4W8.com.utmapp.UTM`, user-selected read/write files, microphone/USB,
network client/server, virtualization, VM device access and VM networking.
`utmctl` declares sandbox/group membership, a network client and a scripting
target for UTM VM access. QEMUHelper declares sandbox/group membership, JIT,
hypervisor, microphone, app-scoped bookmarks and network access. QEMULauncher
declares sandbox inheritance, JIT, hypervisor and VM networking. The arm64 and
x86_64 entitlement values agree for these four executables.

These are signed capabilities, not evidence that this task granted permissions
or exercised them. The distribution includes QEMU and its dependencies even if
a later experiment selects Apple Virtualization. No bundled converter or helper
was executed. The app and helper container IDs plus the group ID produced exact
candidate paths; preferences/cache/saved-state names are candidates, not a
complete observed first-launch footprint. No existing contents were opened.

Signing/notarization is not a security audit or proof of the proposed untrusted
tool boundary. UTM, Apple Virtualization, guest root and the planned broker remain
trusted components; confinement of a synthetic tool in the guest has not been
tested. No production runtime, provider credential boundary or T07 claim follows.

## Cleanup, remaining unknowns and next decision

The task root was
`/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`. Before detaching,
the exact image/device identities, read-only flag, unchanged DMG/app contents and
absence of open volume files or task processes were checked. Normal `hdiutil
detach` succeeded for `/dev/disk128`; no force option was used. The DMG and four
private directories were then deleted by verified ownership and no-follow
directory descriptors, with a durable journal. No mounted content was deleted.
Shared OS databases were not edited; ordinary Apple trust-cache effects are not
claimed absent. Only bounded public evidence remains in this worktree.

No additional package/guest download, downloaded-code launch, VM, key, account,
provider sign-in, permission/security/quarantine change, startup service,
LaunchServices invocation, push, merge or deployment occurred. Expected network
activity was the exact GitHub download and existing Apple trust checks; it was
not packet-captured. The separate final verification was another script by this
worker, not an independent agent review. No external agent was contacted.

App first-launch behavior, permission prompts, guest-image layout and bootability,
serial/NoCloud transport, crash cleanup and actual tool confinement remain unknown.
The legal-file scan does not establish licensing completeness or redistribution
rights for a modified distribution. Artifact signatures also do not prove
reproducibility from the reviewed upstream source.

**Next:** prepare and independently review a bounded Apple Virtualization/serial
synthetic-probe execution packet using this exact artifact and verified signer.
Before any guest download/verifier execution, app launch or VM session, obtain
the separate exact authority for that packet. This inspection authorizes none of
those actions and does not recommend bypassing any security control.

Branch: `codex/fort-core-openmaus-20261011`; worktree:
`/private/tmp/fort-core-openmaus-20261011`.
