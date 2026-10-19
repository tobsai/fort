What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional official runtimes. This review follows one Mac owner's removal
and account-status paths, assuming one owned host and private App Group storage.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`41b650a227ad6b968e7df325466398bc4aa0a953`. Scoped semantic review, not an
independent-reviewer claim. The pinned Ponytail text and whole-repository baseline
were read; earlier reports remain in Git history.

Reviewed all changed source/tests/spec/evidence, the removal callers and journals,
App Group and file metadata checks, generic lifecycle proof and native status copy.
Compared pinned public Codex auth storage/account/routing source with Fort's private
launch plan. Checked preservation/refusal tests, failed-then-passed results, complete
native execution, isolated timing, unsigned builds and the exact review package.
No code is declared unused; no unrelated architecture expansion is proposed.

## Must fix before installed acceptance

1. **Selected-account correlation remains unresolved.**
   - What this is: Codex has nonsecret optional email and experimental workspace
     account ID fields, while Fort's current result contains only a sign-in outcome.
   - Problem: It cannot distinguish the owner-selected account from another
     ChatGPT account. An Apple email or successful login result supplies no proof.
   - Fix: Carry validated nonsecret account/workspace evidence through the existing
     flow, correlate it with the owner's selection and reject missing/changed
     identity before dispatch/restart acceptance. Native copy now states the limit.
   - If we skip it: Real-provider evidence could be attributed to the wrong account.
   - Disposition: Open implementation/account decision; no live account was read.
     `refreshToken:false` is not an offline guarantee in this pinned implementation.

2. **The final signed installation remains unverified.**
   - What this is: Fresh unsigned Mac/helper/iPhone builds and a verified 200-file
     inert package with synthetic cleanup tests.
   - Problem: No profiles or final signature, no live cleanup/account, no installed
     screenshot/accessibility and no TestFlight acceptance exist for this artifact.
   - Fix: Complete the pending secure Apple handoff, exact records/profiles and
     approved final package/grants; then inspect real installed behavior and cleanup.
   - If we skip it: Source tests would be presented as installed-product evidence.
   - Disposition: Open external/acceptance gate. G2 approval remains valid and
     unexecuted; agreement acceptance remains unapproved.

## Fixed findings

3. **Uninstall preparation retained Fort's provider credential.**
   - What this is: Earlier removal deleted only the pairing Keychain token.
   - Problem: The public synthetic removal journey left `Runtime/state/auth.json`
     after confirmed host exit and unregistration.
   - Fix: A required callback removes only the exact private file using metadata
     operations after lifecycle proof; a required receipt gates device-token cleanup
     and app handoff. History and unrelated account markers remain unchanged.
   - If we skip it: Removal could falsely promise that Fort's saved login was gone.
   - Disposition: Fixed in source/synthetic tests. Retry, unsafe paths/files, links,
     ACLs, replaced roots and absent proof are covered. Local absence does not revoke
     remote provider grants. Real signed-installation cleanup remains finding 2.

4. **Intermediate checks broke refusals or inert setup.**
   - What this is: An initial permission mask accepted executable auth files;
     later eager storage inspection ran while constructing removal controls.
   - Problem: A malformed file was deleted, and existing setup/reinstall metadata
     tests failed before an explicit removal action.
   - Fix: Reject execute/special/shared bits; resolve App Group storage only at
     confirmed removal. Keep the existing metadata-only setup/reinstall behavior.
   - If we skip it: The cleanup rule would be weaker than specified and setup UI
     would require unnecessary runtime storage access.
   - Disposition: Fixed. New refusal tests and existing complete regressions pass;
     intermediate failures are retained in the evidence.

Verdict: The local credential-removal defect is fixed with synthetic proof and a
fresh inert package. Account correlation and exact installed acceptance remain
open. No provider cleanup or Apple activation was performed.

Not checked: Real secrets/accounts, final profiles/signatures, installed UI and
provider lifecycle, remote revocation, phone connectivity, archive/upload/TestFlight
or provider teardown. No credentials were copied. Unchanged TypeScript suites and
unrelated opt-in timing/signed probes were not rerun. The final file unlink is not
claimed to defeat an adversarial same-user race; ordinary owned processes are
stopped and the existing owner-operation lease is held. No history deletion, broad
recursive cleanup, permission repair, dependency or network entitlement was added.

```fort-review
{
  "format": 1,
  "reviewedCommit": "41b650a227ad6b968e7df325466398bc4aa0a953",
  "reviewer": "Codex primary implementation agent",
  "scope": "Exact offline provider credential removal, lifecycle proof, unresolved account-identity label and current engineering package",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Observed failing public synthetic cleanup, unsafe executable-file and account-label tests before fixes.",
    "Final serial native: 326 passed / 16 skips, 342 reported, zero issues; focused 32 passed / 1 skip; isolated cleanup p95 0.403667 ms after five warmups and twenty samples.",
    "Fresh unsigned Release Mac/helper and Debug iPhone Simulator builds passed. Exact 200-file package and all 198 inputs independently verified; public unsigned/profile refusal probe passed.",
    "Full and accepted-spec gates passed; exact 15-path staged spec/review and whitespace gates passed, with 14 file hashes and two explicitly open findings."
  ],
  "exceptions": [
    "No real credential, provider runtime, Apple mutation, startup activation, installed UI or release acceptance; selected-account correlation remains explicitly unresolved."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Supported nonsecret identity fields are not yet correlated with the owner selection through the end-to-end Fort flow; native status explicitly labels this unresolved.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Signed/provisioned installed cleanup, actual account/UI and TestFlight acceptance remain unrun.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Exact local provider credential cleanup now follows lifecycle proof, preserves history, retries safely and gates app handoff.",
      "disposition": "fixed"
    },
    {
      "id": "4",
      "detail": "Corrected unsafe executable-file acceptance and eager storage inspection; refusal and inert setup/reinstall regressions pass.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "4894cbcde2030bddba671a153d284310485859de37349f257ac889a891439e73",
    "successor/APPLE-HOST-ACTIVATION.md": "6e79323b34d5f5e1d5da7d9f8c90d678fc50df174b8455e0493f69c7b8608cbf",
    "successor/V1-ACCEPTANCE.md": "2909dea9b72476a976974fcb3f243eb34d65fbbbda40efcdb9ec09021113e267",
    "successor/apple/FortKit/Sources/FortKit/MacAgentStorage.swift": "02c0cea9656e805e723be244da28747bae449c19f75a60ed8a92b84701ff52c5",
    "successor/apple/FortKit/Sources/FortKit/MacHostControl.swift": "a0f4efc00128cdd82bb245959790cb0524306e357cd1fa6b38f8b34d16649c0b",
    "successor/apple/FortKit/Sources/FortKit/MacHostRemoval.swift": "d8a00677d1414f45057ce46687c8df6ccd002c86f7cb513147beb86d78da7b20",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "2a396f49dc98fe18eb99bd16c7bc2011629d6363fcb633028c38e66f0872434a",
    "successor/apple/FortKit/Sources/FortKit/OwnerLifecycle.swift": "7bf485ac43a8be6a2f1797a6353ab927ab2a6e79ffd91fd23cd54eca1ab59f50",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "da6c6782f25715632e0abad18c87678290abbcca10bbce445ad23dac4a9bb8ba",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "d3f452a2ec097f6dc725fcd701da8556d6d0f43cb47761512b8c968cd2ee98c5",
    "successor/apple/FortKit/Tests/FortKitTests/MacAgentStorageTests.swift": "30360a975b9a2d0876bef9f5108f34594a98e5d90094bf073d1f7095d6ca13bc",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostRemovalTests.swift": "c1668c9922d90c5f3ca73dc0614a4e50b481f1eaa7d0f3407eca0476dd5c269c",
    "successor/apple/FortKit/Tests/FortKitTests/OwnerLifecycleTests.swift": "3d3d61637efcadae89b34b807fc18e09d8469444c13152bfe9299de1f5b5ec7a",
    "successor/evidence/2026-10-17-node-foundation/provider-session-removal.md": "ba08e13d4d3f6efbb7396491491f25f25aafc6c67d8195495b22c68e5cb7163f"
  }
}
```
