What this repo does: Fort is a single-owner native AI messenger with a Node core
and optional official runtimes. This scoped review covers approved Mac development
provisioning, the resulting inert signed artifact and evidence updates. It assumes
the existing one-owner workload; no product architecture or runtime code changed.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`60b144706a4f58852a8a5634c12a2ba758b54f0e`. Semantic review uses the pinned Ponytail
skill and preserved baseline; this is not an independent-reviewer claim.

Read the actual source-backed approval, portal confirmations and downloaded-profile
metadata; traced the assembly input/hash guards, native bundle/program signature and
entitlement checks, helper bootstrap and separate activation boundary. Reviewed
all staged documentation and the actual build/signing commands/results. Exact team,
app/group IDs, one existing certificate and one registered Mac match the approval.
All original source/artifacts and unrelated Apple records remain preserved. No
private key was read/exported, new key created or existing certificate revoked.

## Must fix before installed or release acceptance

1. **Installed host/provider operation is still unverified.**
   - What this is: Code signatures and profile metadata permit the next exact
     runtime test; they do not establish effective App Group or named XPC access.
   - Problem: The package has never run. Its helper still has no outgoing-network
     entitlement, so this artifact must not be represented as provider-ready.
   - Fix: Prepare the concrete separate installed/network/provider scope against
     these exact identities and artifact, then run the approved native lifecycle,
     account/provider, accessibility and screenshot checks.
   - If we skip it: A signed development app could be mistaken for a working or
     accepted installed product.
   - Disposition: Open. No startup registration, provider login or app launch ran.

2. **Submission/runtime disclosures and TestFlight proof remain open.**
   - What this is: The shipped payload needs accurate privacy/API/encryption
     declarations and final Apple distribution/physical acceptance.
   - Problem: Development profiles and local signing do not answer those questions
     or make a build available to testers. The privacy inventory remains partial.
   - Fix: Finish the independent source/payload review and owner contact/URL and
     compliance decisions, then the separate distribution and installed gates.
   - If we skip it: The release record would claim unverified compliance or delivery.
   - Disposition: Open; the local submission draft and previous evidence remain.

## Fixed in this slice

3. **The exact Mac package lacked registered identities and provisioning.**
   - What this is: Both client and helper need their own profile and code identity
     with the same explicitly approved App Group.
   - Problem: Prior artifacts were unsigned and had no compatible profile.
   - Fix: After the exact owner approval, create only the missing two IDs/group and
     two development profiles, then build/sign a separate unchanged-source package.
   - If we skip it: The native bundle verifier rejects the package before setup.
   - Disposition: Fixed for local development signing. Profile/certificate/device
     checks, 30 exact signing certificates, strict nested verification, 206 file
     hashes, exact entitlements and the real native bundle inspector pass.

Verdict: Approved provisioning and inert development signing are complete. Actual
installed operation and release acceptance remain open. No source fix was needed
for the probe's /private/tmp alias refusal or corrected codesign option syntax.

Not checked: Effective OS profile acceptance, real App Group/XPC, service activation,
provider login/messages, installed UI/accessibility/screenshots, physical devices,
archive/upload/processing/tester access, complete SDK privacy audit or remote CI.
Full runtime suites were not repeated for unchanged code. G2 was not executed.

```fort-review
{
  "format": 1,
  "reviewedCommit": "60b144706a4f58852a8a5634c12a2ba758b54f0e",
  "reviewer": "Codex primary implementation agent",
  "scope": "Approved Apple Mac identifiers/group/development profiles, unchanged-source inert signed package, exact signature/profile/entitlement validation and evidence-only handoff updates",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Fresh unchanged FortMac and FortHost Release builds passed under explicit approved ID overrides, 53.339 and 3.367 seconds. No benchmark claim.",
    "Both downloaded profiles match exact OSX/team/app ID/shared group, existing certificate fingerprint and only the registered Mac. Original public certificate independently matched local identity.",
    "Unsigned strict signature gate failed as expected; signed strict nested and 30 all-architecture code signatures passed. All 30 extracted public leaf certificates match the exact approved fingerprint. All 206 output hashes/modes/sizes and client/helper/child entitlements verified.",
    "Existing public MacHostBundle.inspect passed with 49 byte-identical FortKit library files. Initial /private/tmp spelling failed its path check; correct Foundation-normalized /tmp spelling passed with unchanged product source. No app/helper/runtime launch.",
    "Signed Fort name and compiled icon checks passed. Existing runtime/fixture suites were not repeated for unchanged source. Full spec structural gate passed; exact staged spec/review/diff gates required before commit."
  ],
  "exceptions": [
    "No new certificate/key, key export, credential read, agreement acceptance, unrelated record mutation, provider or startup activation, app installation or publication. Raw profiles/public certificate downloads remain outside Git.",
    "Mac app records are cross-platform in Apple UI; generated profiles are OSX. Apple supplies profile wildcard allowances, but app signatures request only the exact approved group. Non-editable default In-App Purchase checkbox does not create a purchase product.",
    "Source and signing evidence do not prove actual OS profile eligibility, App Group/XPC access, provider functionality, privacy compliance or TestFlight delivery. G2 remains separately approved and unexecuted."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Actual signed installed App Group/XPC/provider/UI acceptance remains unrun. The helper intentionally retains no network entitlement; request the concrete separate installed/network/provider scope before activation.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Privacy/runtime inventory, contact/URL and export declarations plus TestFlight distribution/processing/physical acceptance remain open. Development signing does not close them.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Missing Mac development provisioning and unsigned-package gap closed under exact owner approval; both profiles and signed identities/entitlements are independently verified.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "fe3f2c905d1641df8b288b16d76229b93974bec75275d0b46dc7d7f576445c76",
    "successor/APPLE-HOST-ACTIVATION.md": "62180bda677b4353b4ff5fde6b6d6fee2885ab9b860f2cd6f79184d18274b590",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "f3fcc66715e317d9f91d46b21b275d61a32d6a8956bc5296da968e1f7990c78f",
    "successor/V1-ACCEPTANCE.md": "485bf6165350f21ad4609f176ed72c346f0039bc8f637b3c4020afe77c087ea9",
    "successor/evidence/2026-10-17-node-foundation/mac-development-provisioning.md": "105e002b2311629b756e3c59af1b393ae0b8156d062d795e3ae27fe78598c0fc",
    "successor/evidence/2026-10-17-node-foundation/mac-development-provisioning.json": "d1304c7a3c1119161f5780070c873a3397806a02ca47f24b960bce01aab379ef"
  }
}
```
