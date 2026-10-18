What this repo does: Fort is a single-owner native messenger backed by a modular
Node service and optional runtimes. This review assumes one owning Mac and one
companion phone, with no shared hosting. The donor remains quarantined.

Reviewer: Codex primary implementation agent, October 18, 2026. Base:
`b0caff657c328b345c29d18456ef60f6948c6aab`. This is a proportionate semantic review,
not an independent reviewer claim. The whole-tree baseline is preserved.

Reviewed all staged source, tests, spec changes and evidence; traced ordinary
FortApp/MessengerView construction, pending setup through CompanionSetupCoordinator,
private storage and credential continuity, restore/refresh, removal, history clearing,
account/host guards and DEBUG-only composition. Checked existing native UI-test
callers before changing initialization. No code was declared unused. The unchanged
service, TLS adapter and provider implementation were not re-audited line by line.
The pinned Ponytail skill was read as text; no upstream script or plugin ran.

## Must fix before v1 acceptance

1. **The owning Mac cannot yet issue an invitation through ordinary controls.**
   - What this is: The phone form consumes the existing exact invitation contract.
   - Problem: A user still lacks composed Mac invitation and secure-listener controls;
     native phone source alone cannot connect the two installed products.
   - Fix: Implement that bounded Mac lifecycle next, with public behavior tests and
     explicit identity/trust/credential authority before activation.
   - If we skip it: The form exists but an ordinary owner cannot complete setup.
   - Disposition: Open, carried in the acceptance ledger; no listener was enabled.

2. **Actual device and distribution acceptance is still missing.**
   - What this is: Swift compile, synthetic Mac UI and in-process protocol tests.
   - Problem: They cannot prove actual iPhone Keychain, TLS trust, installed host,
     provider access or TestFlight availability. CLI cold QA launch needed a normal
     reopen event to show a window, and alert AX Cancel returned -25205.
   - Fix: Verify the exact authorized installed Mac/iPhone builds, ordinary launch,
     confirmation interactions, provider journey and Apple distribution separately.
   - If we skip it: We could label a tested source slice as a usable delivered app.
   - Disposition: Open. Mandatory external approvals remain pending; no workaround
     certificate, trust, account, network or startup grant was introduced.

## Fixed in this slice

3. **Expired phone access hid disconnect and offered misleading controls.**
   - What this is: PairingSettings previously placed disconnect inside ordinary
     management access; the first new QA build also retained an expiry editor.
   - Problem: An expired credential could leave the owner without visible phone-only
     removal, while the status text claimed a pending chat existed.
   - Fix: Expose exact leave separately, require verified access for expiry edits,
     and show a phone-specific unconfirmed-access message. Observed model regressions
     went red then green; final expired screenshot/AX shows the correction.
   - If we skip it: The owner cannot understand or act on the expired pairing safely.
   - Disposition: Fixed, including phone-only confirmation and no host/account UI.

4. **A lost join response needed explicit native recovery and removal choices.**
   - What this is: The durable coordinator existed without ordinary phone controls.
   - Problem: A generic enrollment form could not represent a prepared invitation,
     exact retry, local forgetting or the retained history of an earlier installation.
   - Fix: Use the coordinator in the ordinary iPhone model/form. Reopen reads pending
     state only; explicit recovery reuses its credential, clears stale draft display,
     and refuses a changed connection or stale forget target. Confirmed cancellation
     and unconfirmed local forgetting have different new-pairing availability.
   - If we skip it: Users can misread an uncertain result or carry a prior draft into
     a different installation.
   - Disposition: Fixed through public model tests and inspected synthetic controls.

5. **New ordinary setup would break the existing DEBUG enrollment UI tests.**
   - What this is: Existing NativeJourneyTests explicitly select a UUID UI-test session.
   - Problem: Routing every iPhone initialization to invitations hides those tests'
     existing loopback enrollment controls.
   - Fix: Retain an explicit DEBUG-only initializer constrained to UITests/UUID, used
     only by the existing test flag/session combination. Ordinary and Release paths
     retain companion setup. The path guard was tested red then green.
   - If we skip it: Future native test runs fail for a composition change unrelated
     to the behaviors those tests exercise.
   - Disposition: Fixed in source/build; actual simulator execution remains blocked
     by the existing CoreSimulator version mismatch.

Verdict: Commit the bounded phone-control slice. Continue Mac invitation/listener
composition; do not claim installed-product or v1 acceptance.

Not checked: actual accounts, provider runtime, Keychain, LAN/TLS/trust, background
registration, ordinary installed cold launch, actual alert Cancel activation, physical
Mac/iPhone acceptance, signed archives, upload/processing/TestFlight availability,
remote CI and provider closure. The final native suite passed 307 functions with 14
optional skips; this slice's isolated p95 is 25.541 ms under 100 ms. Fresh QA/Mac/iPhone
builds, Release compilation, QA signature, exact screenshots and QA absence passed.
Portable service tests were not repeated for this Swift-only slice. Full evidence:
[phone controls](../../../successor/evidence/2026-10-17-node-foundation/native-companion-controls.md).

```fort-review
{
  "format": 1,
  "reviewedCommit": "b0caff657c328b345c29d18456ef60f6948c6aab",
  "reviewer": "Codex primary implementation agent",
  "scope": "Ordinary native phone setup, exact recovery/removal, expired access controls, DEBUG-only fixture and reviewed evidence; single owner and one phone",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Final serial native suite: 307 functions passed, 14 optional skips, zero failures (321 reported).",
    "Seven phone/legacy focus functions passed, one performance skip; 14 affected lifecycle functions passed after expired UI correction.",
    "20 isolated samples after five warmups: p50 23.772 ms, p95 25.541 ms, maximum 26.238 ms, under 100 ms.",
    "Fresh QA/Mac/iPhone builds and Release compilation passed; QA strict signature and sandbox entitlements verified.",
    "Exact final synthetic AX/screenshots for preparation, reopen, recovery and expired leave; exact QA process count zero.",
    "Actual provider/Keychain/LAN/trust/device/TestFlight and simulator execution unrun; alert AX Cancel failed with -25205; normal QA termination succeeded."
  ],
  "exceptions": [
    "Synthetic in-process responses and memory credentials establish source/UI behavior only.",
    "Preserve the prior account-revision inert review package; no new archive, deployment, provider cleanup or certificate action."
  ],
  "findings": [
    {
      "id": "PHONE-MAC-COMPOSITION",
      "detail": "Ordinary Mac invitation and secure-listener lifecycle remain uncomposed.",
      "disposition": "open"
    },
    {
      "id": "PHONE-INSTALLED-ACCEPTANCE",
      "detail": "Actual trust/Keychain/provider/device/TestFlight, ordinary cold-window and alert Cancel acceptance remain unverified.",
      "disposition": "open"
    },
    {
      "id": "PHONE-EXPIRED-CONTROLS",
      "detail": "Expired leave is visible independently; expiry editing and messages require verified access.",
      "disposition": "fixed"
    },
    {
      "id": "PHONE-RECOVERY-CONTROLS",
      "detail": "Explicit pending recovery, captured confirmation, credential continuity and history/draft isolation are composed and tested.",
      "disposition": "fixed"
    },
    {
      "id": "PHONE-LEGACY-UI-TESTS",
      "detail": "Preserved explicitly scoped DEBUG-only loopback UI-test initialization.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "662d35afbe8e6d35b026b79ac7f56dedda7322f2e6ea5437b1d223903027b166",
    "successor/V1-ACCEPTANCE.md": "c8de06efe67eebfa6fd3ce7801fed7f05092a45cddfdc72a609b252073e2f733",
    "successor/apple/Apps/FortApp.swift": "3e7f87ae99f388b56a1cf478c3c969dd24426dfd922855f60337fa9cbdf72722",
    "successor/apple/FortKit/Sources/FortKit/CompanionSetup.swift": "bd4bf30bd9446637a584a74898e31e4755184e3c57dfd8bb12879a7466d5a6ee",
    "successor/apple/FortKit/Sources/FortKit/CompanionSetupFixture.swift": "c79a57b441114c8b01596ea9c3b25ef89df77462c6df90f5f13019e6be202f03",
    "successor/apple/FortKit/Sources/FortKit/CompanionSetupView.swift": "bc981739156b68fb25a1867eb2058eb4e2502e0f50478abfd25e30c84faa8cb1",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "caa1eb587ddf12fd2695536d2e6c54199e8b4cd3cde33f33a975be51a1e3c6b4",
    "successor/apple/FortKit/Sources/FortKit/MessengerView.swift": "9ea305852bd8792187d80b09f5b73efa306f3901d17d04515e9b4667a5de35e6",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "35ae83a2d4a8a41d26ae3e36dc06421d3f2381fd71d232037999799e79f48c17",
    "successor/apple/FortKit/Tests/FortKitTests/CompanionSetupModelTests.swift": "1f057f441a63f17ad891154eb74c744a06b701564a55db5f10d27aebad61fe08",
    "successor/evidence/2026-10-17-node-foundation/native-companion-controls.md": "f4eca5c9e03d7e65d07476acd7e5fc4a34277401be82c3b02174d49ae7d628f6",
    "successor/evidence/2026-10-17-node-foundation/phone-controls-expired.png": "91fddc939e91abd8ef709fda8efaa64fdf0002b5e7076489d563bf5bcb5b1bfc",
    "successor/evidence/2026-10-17-node-foundation/phone-controls-paired.png": "c675e2aa4607239e843be0a7ff0cdebf98f77187f8e4e98586461fa61c03f8a3",
    "successor/evidence/2026-10-17-node-foundation/phone-controls-prepared.png": "dca0ecdbffabec6a95c09196df09031558369ff3043746d107d0c9ee4297e170",
    "successor/evidence/2026-10-17-node-foundation/phone-controls-reopened.png": "ac3c429fc41ed6bc97d649cbb9fbb1eef3a2fbddb0b5720de3724dfadd961458"
  }
}
```
