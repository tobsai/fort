What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional official runtimes. This scoped review follows one Mac owner's
sign-in, chat, restart and removal flow to describe the minimum activation grant.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`7d528baeb176d7827f60a7fa61fedb9c620b853c`. Scoped semantic review, not an
independent-reviewer claim. Read the pinned Ponytail skill and retained baseline;
earlier implementation/package reviews remain in Git history.

Read current bootstrap/owned core, runtime launch plan and controls, account and
chat account checks, native account UI/storage, helper startup/storage/program,
bundle inspection, owner removal and history deletion. Searched production paths
for provider logout/credential deletion and distinguished them from pairing-token
deletion. No code was declared unused. The documentation changes only describe
observed behavior and proposed bounded approval scope; they grant no access.

## Must fix before claiming a complete reversible Mac demo

1. **The final package and account-specific grant are not ready.**
   - What this is: The current review package is unsigned and has zero profiles;
     its IDs are engineering values. Apple sign-in is awaiting the owner's secure
     password/device handoff.
   - Problem: Proposed app/helper/group names, current role, exact profiles, final
     artifact/path and the intended ChatGPT identity cannot yet be verified.
   - Fix: Finish the existing secure handoff and read-only Apple checks, prepare
     the exact signed package, and resolve the provider identity before presenting
     the consolidated entitlement, registration and session approval.
   - If we skip it: Approval would refer to incomplete or assumed identities.
   - Disposition: Open external/owner gate. Existing G2 approval stays valid;
     the Program License Agreement stays unapproved. No activation occurred.

2. **Pairing cleanup leaves the provider credential behind.**
   - What this is: `private-codex-plan.ts` selects file credentials under
     `Runtime/state/auth.json`; `MessengerModel.swift` supplies
     `DeviceStorage.removePairing` to `OwnerLifecycle` during removal.
   - Problem: That callback deletes only the Fort device Keychain token. Current
     production source has no provider logout or deletion of that auth file, and
     saved-history deletion preserves runtime sessions. A signed-in installation
     could complete the current removal preparation while retaining provider auth.
   - Fix: Prepare and behavior-test one exact private-session sign-out/local
     credential-removal operation, preserving history and requiring process
     absence. Separately identify any provider-side grant revocation still needed.
   - If we skip it: A promise that removal clears all Fort-created credentials
     would be false, even if pairing revocation and startup removal succeed.
   - Disposition: Open bounded implementation/acceptance work for the implementation
     owner. This review records the limit; it does not delete credentials or add
     permission to use an existing account. Apple access does not block synthetic
     cleanup implementation.

3. **Native signed-in status does not establish the selected human account.**
   - What this is: `codex-account.ts` and `codex-chat.ts` validate ChatGPT account
     type and authentication requirement; account identity is deliberately omitted
     from the public result under the current account protocol spec.
   - Problem: A signed-in result does not distinguish two different ChatGPT
     accounts. The selected Apple email cannot fill this evidence gap.
   - Fix: Resolve the intended ChatGPT identity with the owner and establish an
     exact, privacy-preserving correlation through the official authorization flow
     before dispatch. If that cannot be established through the supported flow,
     review a bounded identity check instead of assuming a match.
   - If we skip it: The acceptance evidence could attribute a real request to an
     account that Fort has not actually verified.
   - Disposition: Open account/evidence decision; no credential contents were read.
     This is a limit of the deliberately scoped contract, not a claim that its
     existing fixture tests fail or that the current spec authorized email exposure.

## Fixed evidence issue

4. **The delivery matrix described an obsolete package.**
   - What this is: The ledger's latest row recorded the new 200-file Release
     package and source account controls, but its summary matrix still said
     190 files and an incomplete provider-account flow.
   - Problem: Readers could choose the wrong artifact or overlook the actual
     remaining account identity and cleanup limits.
   - Fix: Align the matrix with the current package and label the remaining
     source/account/installed gates explicitly.
   - If we skip it: The summary and supporting evidence contradict each other.
   - Disposition: Fixed in this evidence-only slice; earlier checkpoint rows kept.

Verdict: The minimum grant is documented, but it is not ready for approval or
activation. Complete exact identity/package verification and the provider-session
cleanup path before promising a reversible installed demo. This is not a claim
that all remaining useful implementation is blocked on Apple.

Not checked: Live Apple state during the owner's password handoff, actual provider
credentials/account, final signed/provisioned host, production network/startup,
installed UI accessibility/screenshots, physical phone, TestFlight or provider
cleanup. Product tests/builds were not repeated because this slice changes only
evidence. The previous 1,308 portable passes/15 skips, 317 native passes/15 skips
and unsigned package checks remain attributed to their existing source checkpoint.
Full and accepted-spec structural gates passed before editing. The staged spec,
review-coverage and whitespace gates passed for the three-file documentation slice.

```fort-review
{
  "format": 1,
  "reviewedCommit": "7d528baeb176d7827f60a7fa61fedb9c620b853c",
  "reviewer": "Codex primary implementation agent",
  "scope": "Minimum Mac host/provider activation scope, exact identity and credential-removal limits, and current acceptance evidence; documentation only",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Read-only semantic trace of source account checks, private runtime storage, helper entitlements, startup registration and removal callbacks; no product runtime executed.",
    "Full spec, accepted Spec 059, exact staged-path, review-coverage and whitespace gates passed for this documentation slice.",
    "No product tests/builds repeated for documentation-only changes. Earlier 1308 portable passes/15 skips, 317 native passes/15 skips and exact unsigned package checks remain historical evidence for the unchanged source."
  ],
  "exceptions": [
    "No provider, Apple, credential, entitlement, startup or installed-product action; complete activation approval remains blocked on explicit unresolved facts and cleanup work."
  ],
  "findings": [
    {"id":"1","detail":"Final signed package, Apple records/profiles and exact provider-account grant are not yet verified.","disposition":"open"},
    {"id":"2","detail":"Native removal deletes the pairing token but retains private Codex auth; exact provider-session cleanup still needs implementation and acceptance.","disposition":"open"},
    {"id":"3","detail":"Native signed-in status proves ChatGPT account type, not the selected human account; exact correlation remains unresolved.","disposition":"open"},
    {"id":"4","detail":"Updated the obsolete delivery matrix to the current 200-file unsigned package and precise remaining gates.","disposition":"fixed"}
  ],
  "files": {
    "successor/APPLE-HOST-ACTIVATION.md": "53c78a813f165bc0216e038b4484f33c2c669e7ab3ce91b38db6bb3c18fc5050",
    "successor/V1-ACCEPTANCE.md": "279bf0a81c24237e36ade444f7e376b8e9c96e6792530e3102e0ca1cb9d81a65"
  }
}
```
