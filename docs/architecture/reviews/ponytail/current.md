What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional official runtimes. This continuation checks Mac package readiness
and the existing account-selection UI; no shared-hosting load or new feature applies.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`81938a49ffaf55bc39c09b1ef9a045a3e5e8bbe4`. Scoped semantic Ponytail review against the
pinned read-only skill and preserved baseline; no independent-reviewer claim.

Reviewed the exact package inventory and unchanged app/helper configuration, current
MacHostBundle signature/profile guards, project settings, AccountSignInView and its
model/client/decoder, the temporary QA responder's limited routes, normal-context
certificate/profile metadata, Apple primary documentation, observed accessibility
states/screenshots and the updated evidence/acceptance notes. The production view,
model, client, decoder and pairing view in the QA source copy are byte-identical to
the base. No product source, dependency, signing setting or permission changed.

## Must fix before installed acceptance

1. **The package has no matching Mac provisioning profiles.**
   - What this is: The exact current client/helper use an App Group and named XPC.
     Normal-context metadata lists valid development/distribution identities.
   - Problem: All 91 local profile files decoded, but zero macOS profiles for the
     selected team authorize this package or the proposed release pair/group. Both
     current bundles remain unsigned, with no embedded profiles.
   - Fix: Complete the existing secure Apple handoff, verify/reuse exact app/group
     records and compatible remote profiles, and approve only confirmed missing
     provisioning before preparing the final signed artifact. G2 repair is separate.
   - If we skip it: Certificate presence or a synthetic UI could be mistaken for a
     working installed App Group/host or TestFlight delivery.
   - Disposition: Open external gate. No new development key is indicated by this
     inventory; actual private-key use, profile/export signing and release remain
     untested. No credentials, legal acceptance or activation occurred.

2. **Cold-window launch remains unverified.**
   - What this is: The exact ad-hoc synthetic QA process launched without a window;
     a normal reopen action displayed the intended account controls.
   - Problem: Successful reopened-window accessibility and selection do not prove
     first launch of this QA app or the final signed installation.
   - Fix: Keep that limitation in acceptance and inspect the completed signed app's
     cold launch before signoff. This observation does not establish a production
     source cause or justify a speculative window/lifecycle change.
   - If we skip it: The Mac demo could be called launchable based only on reopen.
   - Disposition: Open installed-acceptance item. Reopened synthetic reported →
     selected controls passed; exact QA normal termination and absence were proven.

Verdict: Independent assembly, metadata and synthetic account-selection checks are
complete for this bounded continuation. Matching Mac profiles/records are the next
external dependency; no further independent packaging or existing UI check was found.
No production feature, credential, provider access or startup was added.

Not checked: Remote Apple records/role/agreement state, private-key use, final signed
profiles/App Group/XPC, actual account/model/provider, production cold launch,
installed cleanup, Simulator UI, archive/upload/TestFlight or provider teardown.
The previous complete test suites were not rerun for a documentation/evidence slice.
No global dead-code or dependency-removal claim is made. All historical evidence,
unrelated work and prior artifact bytes remain preserved.

```fort-review
{
  "format": 1,
  "reviewedCommit": "81938a49ffaf55bc39c09b1ef9a045a3e5e8bbe4",
  "reviewer": "Codex primary implementation agent",
  "scope": "Existing signing/provisioning metadata, unchanged Mac package, temporary synthetic account-selection UI, acceptance/evidence notes and screenshots",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "21 Mac assembly tests passed with zero skips/failures; exact 200-file output and 198-input inventory matched.",
    "Normal-context metadata: four valid signing identities; 91/91 profile files decoded, 50 UUIDs, zero compatible Mac profiles. Restricted-context false absence/decode failures are recorded.",
    "Fresh ad-hoc synthetic QA build and strict signature/entitlements passed. Actual AX reported-to-selected action and both visually inspected screenshots passed; normal termination and zero exact QA apps confirmed.",
    "First QA launch had no window; normal reopen worked. Initial cache/layout build failures were resolved. No production cold-launch pass is claimed.",
    "Full and implementation spec gates and whitespace check passed before staged review. Earlier portable/native/guarded suites were not repeated for unchanged source."
  ],
  "exceptions": [
    "No credential contents, key use, profile/account mutation, agreement acceptance, host/provider activation or installed/release claim. Temporary QA uses only authored in-process identity.",
    "Profile inventory covers the two standard local cache directories; remote records and current secure Apple session were not accessed."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Exact package lacks matching Mac profiles and final signatures; existing development/distribution identities do not establish installed or TestFlight readiness.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Synthetic first launch produced no window; reopen worked. Final installed cold launch and actual provider acceptance remain unverified.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "5318bb53fd1cd7600b4dc3b4d618b292ca478f28cf606628b1a36a46e500f91e",
    "successor/APPLE-HOST-ACTIVATION.md": "e6a1daa8da4a970b56456f956bf04646d84543fd0f7d38cb3145f4377bb254f3",
    "successor/V1-ACCEPTANCE.md": "d046d0d1ee90dc52bc40f24b78adf49fa7c264a3320e79d00e0a922c87539397",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "d5cdaba03a02407983ff565fcc2245c314e4a092e7a326d60a8ca894501d2c4c",
    "successor/evidence/2026-10-17-node-foundation/mac-review-package.md": "ed1118e858855f61035f92907a1b67ba6fc924fa5e2b5654cbe41aad4a7c6c1d",
    "successor/evidence/2026-10-17-node-foundation/account-selection-reported.png": "67b1b848bcca5728cb6c3cf8f4f399a9db8f5dc0f9eb275da8ba0fc90dd2e986",
    "successor/evidence/2026-10-17-node-foundation/account-selection-selected.png": "20b2709d0da827be64e09c15fa2ab4f8bc2b90649b092136ee50f3d24f558780"
  }
}
```
