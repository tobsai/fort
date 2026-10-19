What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional official runtimes. This review covers local native branding,
resource packaging, submission drafts and a read-only Apple readiness update.
It assumes the existing single-owner workload; no runtime or hosting change applies.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`10712e8419c0374f315fa7e315dda5c996c31371`. Scoped semantic review under the pinned
Ponytail skill and preserved baseline, not an independent-reviewer claim.

Read the project source and generated diff, shared catalog and PNG metadata,
original image provenance, actual compiled bundle metadata/resources, Mac assembly
copy/inventory guards, source-backed submission draft, current acceptance notes,
and the exact nonsecret Apple portal observations. All seven image hashes match
the historical Fort source; compiled macOS icon pixels were inspected. The three
client targets include the catalog, and the background helper remains unchanged.
QA keeps its explicit name and synthetic/no-network labels. No Swift/TypeScript
runtime code, dependency, signing default, bundle ID, entitlement or permission
changed. Existing legal attribution and older artifacts remain preserved.

## Must fix before release acceptance

1. **The exact Mac package still lacks provisioning and installed proof.**
   - What this is: The client/helper need exact identifiers, a shared App Group,
     matching profiles and signatures for their private host boundary.
   - Problem: The authenticated team has the existing Fort iOS record and this
     Mac's device registration, but no proposed Mac client/helper IDs, App Group
     or Mac profiles. The new review bundle remains unsigned and unlaunched.
   - Fix: Obtain the single bounded provisioning approval recorded in the activation
     note, reuse the existing certificate/device, and build the exact signed
     artifact before requesting its separate installed-operation grant.
   - If we skip it: Branding and successful assembly could be mistaken for a working
     installed host/provider or TestFlight delivery.
   - Disposition: Open. Program License Agreement confirmation is reconciled;
     Free Apps is Active. The Paid Apps new-version notice is separate, not proof
     that the intended beta is blocked. No portal record or agreement changed.

2. **Submission privacy and runtime declarations are not yet established.**
   - What this is: The native source and shipped Node/Codex payload must be reviewed
     together for privacy disclosures, required-reason APIs and encryption facts.
   - Problem: No native privacy manifest is present; source uses system uptime and
     file metadata, and a third-party runtime receives selected content/account
     context. A blanket no-data-collected or no-encryption answer is unsupported.
   - Fix: Complete the source/final-payload inventory and owner contact/URL and
     compliance decisions before finalizing submission answers. Keep the current
     worksheet local and explicitly incomplete.
   - If we skip it: Store metadata could promise privacy or compliance that the
     final application has not established.
   - Disposition: Open release gate. The draft names concrete data flows and limits;
     no empty manifest, exemption flag or fabricated contact URL was inserted.

## Fixed in this slice

3. **Ordinary native builds carried fixture branding and no app icon.**
   - What this is: The built app's public Info.plist and resource payload determine
     its displayed name and icon.
   - Problem: The old artifact said Fort Fixture and contained neither compiled
     assets nor an icon declaration. Existing ordinary builds could look like QA.
   - Fix: Set ordinary display names to Fort and compile the existing Fort artwork
     through one shared catalog. Preserve explicit QA and fixture evidence labels.
   - If we skip it: The delivered app would still have prototype display metadata
     and omit its existing brand asset.
   - Disposition: Fixed in source and compiled artifacts. The old artifact failed
     the name/icon check; Mac, helper, QA and iPhone builds now pass. Inert assembly
     preserves both icon files and every recorded output/input hash.

Verdict: Local branding, assets and submission drafts are complete for this bounded
slice. Signing/provisioning, final privacy/runtime review and installed/provider/
TestFlight acceptance remain open; no feature or release completion is claimed.

Not checked: Final signed launch, actual App Group/XPC, real provider/host operation,
physical-device UI, archive/upload/processing/tester access, remote CI or complete
SDK privacy audit. Full runtime suites were not rerun because their code is
unchanged. One build duration per scheme is not a performance benchmark. Apple
inventory used visible metadata only; no credentials or denied paths were read.
The separately approved G2 replacement was not executed or re-requested.

```fort-review
{
  "format": 1,
  "reviewedCommit": "10712e8419c0374f315fa7e315dda5c996c31371",
  "reviewer": "Codex primary implementation agent",
  "scope": "Native display names and existing artwork catalog, generated Xcode project, actual bundle/assembly evidence, local submission drafts and read-only Apple provisioning reconciliation",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Old compiled Mac artifact failed display-name/Assets.car/icon checks. Fresh Mac Release, helper Release, QA Debug and generic-device iPhone Release builds passed unsigned.",
    "Compiled Mac/iPhone ordinary names and icon payloads passed; QA retains Fort QA First Wave. Seven exact PNG dimensions/opacity/provenance checks and byte-identical XcodeGen 2.45.4 regeneration passed.",
    "New inert assembly contains 202 files, 407452451 bytes and 200 verified input hashes/modes; all output membership/hash/length/mode checks and assembled name/icon checks passed.",
    "Apple read-only inspection confirmed team T3JB5MYZ93/Account Holder, existing Fort app 6786748810/io.mtree.fort/fort-ios, registered exact Mac, absent proposed Mac IDs/group/profiles, Active Free Apps and Active Paid Apps with New Agreement Available. Program License Agreement email confirmation was supplied by the coordinator, not fetched independently.",
    "Full runtime/native suites not repeated for unchanged code. Signed installed/provider/physical/TestFlight checks remain unrun."
  ],
  "exceptions": [
    "No account/profile/certificate creation, agreement acceptance, credential read, entitlement change, host/provider/startup activation, upload or publication.",
    "Historical Fort artwork only; no legacy implementation or donor trademark imported. Existing engineering IDs/version values are not release adoption.",
    "Source privacy inventory and compliance worksheet are explicit draft evidence; final declarations and SDK audit remain open."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Exact Mac provisioning and signed installed/provider/TestFlight proof remain missing; current authenticated facts support one bounded provisioning request using existing certificate/device.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Final privacy/required-reason API, bundled-runtime and encryption declarations need review; contact/URL decisions remain open. Drafts do not claim completed compliance.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Ordinary native name/icon omission fixed in actual compiled bundles and inert assembly, preserving explicit QA and fixture labels.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "9f7aa663d6c1eb791ce35c284d7bc87c8f96bba5bdc7afa3c16fc724b42bf0e0",
    "successor/apple/project.yml": "37b11551c2244c3736088e8529724bbf3b207c3d45e05eb996f9b4bd673c0e9b",
    "successor/apple/FortSpec058.xcodeproj/project.pbxproj": "7adabc1dedea9a1d4d24bd5beb04fd93018d79e36b9ed5d59a881a9d3ca67895",
    "successor/APPLE-HOST-ACTIVATION.md": "baeb1a23ba42b68f085974193a89289f8b29b4a66584e81b4eda6180e8317486",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "0dc4cc8185f4658d77c2c7c6cb5cc66577ba0038d25c071d8481654b0b273b4a",
    "successor/V1-ACCEPTANCE.md": "00f88632006077461beec109d3637870fe51a978f78c8997075042bbfef5c7f0",
    "successor/release/SUBMISSION-DRAFT.md": "ab6192cbb4eb7bc045d91ae69e00397b5ee781de9e89a7a6e1e08e7ddf300a8d",
    "successor/evidence/2026-10-17-node-foundation/release-assets.md": "139c9edcc50bc754fc5534304ee69760a5dea96b795f85ca43be479c7cbe4563",
    "successor/evidence/2026-10-17-node-foundation/release-assets.json": "6c9ee56de5738a091334e5f26d9dafd4a4dce10f45dcd288539269d8209f12de",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/Contents.json": "3942388134f8a0eb97307673baa009c9726ae99a06a794eea9d1faa4dd8d87c6",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/icon-1024.png": "7369ab000608a53d1f7c8ad0381e23da39a780644ef6728a88c893fc88f19cbd",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/icon-128.png": "bc233b9fbfba3f1902e52bdd815145e88c0ecec512ff909b6ed81e3a6cdea985",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/icon-16.png": "8ff2d959724b11d28abc494beb0ec560c7d63efed5bd01f243c2380d46ab881f",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/icon-256.png": "72a2617214ec8f6719d7c6cdd788c072749da8e84bdb690153532555d1063950",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/icon-32.png": "c3f33c9d89632e06a7ef6926e8e4371b05180e2fb98fb5a37100df663596bd1e",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/icon-512.png": "c8a6f876cd26dd1df6da40058f0b574f1dcd3fde99dc665385dc7140b1ff4c11",
    "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset/icon-64.png": "b9eb7f24f2298a269cbdd8176d674097177b38f07bae6d804055a50b15e26c8d",
    "successor/apple/Apps/Assets.xcassets/Contents.json": "84333ed75db436b52dab380ddd676b7a026fa8d50f414950c8c421e401d31065"
  }
}
```
