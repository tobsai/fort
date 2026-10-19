What this repo does: Fort is a single-owner native messenger backed by an owned
Node host and optional official runtimes. This review assumes one owner on this
Mac and covers the fresh-install startup seam, its tests, actual client UI and a
bounded proposed provider session.

Reviewer: Codex primary implementation agent, October 19, 2026; base
`2f33fbf8a2f0cf67860f7c77f320d09ae35183ae`. Used the pinned Ponytail skill and
preserved whole-tree baseline. This is a scoped semantic review, not independent
reviewer or v1 acceptance. Traced the changed controller/startup/view, model callers,
package verification, durable removal and credential gates; inspected exact build,
signature, installed AX/screenshot, process and registration evidence. The six
changed code/test files plus all evidence/spec/handoff files were reviewed.

## Fixed

1. **Fresh installed host could never be enabled.**
   - What this is: macOS returned notFound before this LaunchAgent had a record.
   - Problem: Startup/controller rejected it and the view separately hid Enable.
   - Fix: Allow explicit registration only after exact package inspection, keep
     raw state and post-registration confirmation, and use controller eligibility
     in the view. New public tests failed first; installed screenshot caught the
     view condition and the final screenshot proves the available action.
   - If we skip it: A valid signed fresh install cannot proceed to host setup.
   - Disposition: Fixed. Inspection makes no registration call, pairing still
     requires enabled, and unconfirmed removal remains fail-closed.

## Must fix before installed acceptance

2. **Unpaired client does not exit through normal quit.**
   - What this is: The installed client displayed its initial setup sheet.
   - Problem: NSRunningApplication termination and the exact enabled Quit Fort
     menu action returned success, but the client process remained alive.
   - Fix: Diagnose and repair ordinary setup-sheet quit, then inspect actual
     quit/reopen and installed host cleanup. Scope any artifact change explicitly.
   - If we skip it: The owner can be left with a running app after requesting quit.
   - Disposition: Open. Exact task-only SIGTERM cleanup and subsequent absence
     were verified; no helper was registered or launched. No normal-quit pass.

3. **The provider candidate remains an unexecuted permission proposal.**
   - What this is: A separately signed candidate adds only proposed helper outbound
     networking; every identity, profile, child entitlement and hash is pinned.
   - Problem: Static signatures do not establish shared-group/XPC/provider behavior
     or successful startup/credential cleanup, and those actions need approval.
   - Fix: Obtain the one exact bounded grant, then verify the pinned artifact with
     the owner's selected ChatGPT account and discovered gpt-6-astra model.
   - If we skip it: A prepared app could be presented as a working installed demo.
   - Disposition: Open. It is uninstalled/unlaunched; the installed client-only
     helper retains no outbound networking. G2 CSR upload remains held separately.

Verdict: The first-registration source/UI fix and exact inert artifacts are ready
for review. 66 focused functions passed, zero failed, two optional timing tests
skipped. Both Release builds and all 206-file/30-signature checks passed for each
candidate. Ordinary client quit, installed host/provider/removal and release
acceptance remain open. The bounded proposal does not grant its own authority.

Not checked: Actual registration/pairing, helper App Group/XPC, provider identity,
models/messages/retention, live stop/resume/unregistration/credential removal,
physical iPhone or TestFlight. Full portable/native suites were not repeated for
this narrow Swift seam. Earlier privacy disclosure gaps remain; no SDK-wide audit
or legal determination was performed. No G2 CSR upload or new certificate occurred.

```fort-review
{
  "format": 1,
  "reviewedCommit": "2f33fbf8a2f0cf67860f7c77f320d09ae35183ae",
  "reviewer": "Codex primary implementation agent",
  "scope": "Fresh installed startup notFound handling, model/view eligibility, focused synthetic tests, exact signed candidates and client-only installed evidence, consolidated unapproved provider-test proposal",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Three new public test functions failed with seven issues before the fix. Final focused native run: 66 passing functions, zero failures, two optional timing skips. Explicit compiled synthetic core/Node/guardian inputs; real registration/provider backends not exercised.",
    "Final Release builds passed: FortMac 27.572 seconds and FortHost 2.479 seconds. Both final packages pass all 206 hashes/modes/sizes, 30 strict all-architecture signatures and approved public certificate fingerprints, exact entitlements/profiles and native bundle inspection.",
    "Actual installed client AX/screenshot confirms Enable available and Connect disabled; neither pressed. Exact client process and service absent after scoped cleanup; only zero-byte client setup lock and no host storage.",
    "Full spec and accepted implementation gates passed before edits; exact staged implementation/review and diff checks required before commit."
  ],
  "exceptions": [
    "This is scoped implementation-agent semantic review, not independent reviewer signoff. Prior whole-tree baseline remains unchanged.",
    "The first broader restricted run had three synthetic core connection issues; configured normal-IPC rerun passed. One intervening clang-stat-cache build stalled and was stopped; final tests reused the already rebuilt executable. Restricted trust verification refused; normal read-only trust access passed without settings changes.",
    "Ordinary quit failed on the unpaired setup screen; exact-process SIGTERM was required. This limits installed acceptance; the open issue is not waived by the proposal.",
    "Only client-only installed checks ran. The separately signed network-enabled proposal was neither installed nor launched; no new credentials, provider traffic, service registration, G2 upload or release occurred."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Fresh notFound status blocked explicit registration and a second raw-status view condition hid Enable. Exact-bundle eligibility and explicit-only startup handling are now tested and the final installed action is visible; pairing/removal remain fail-closed.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Normal quit did not exit the installed client while the initial setup sheet was open. Exact-task termination and subsequent absence are verified; ordinary quit/reopen still needs a fix and actual acceptance.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Exact signed provider proposal remains uninstalled/unlaunched pending the consolidated startup/network/credential grant. Shared-group/XPC, live provider/lifecycle, complete privacy disclosures and TestFlight remain unverified; G2 upload is held separately.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "4e8f7cb03dd7a1e0464b86b88e74c9d4e901b91968e89cc6e77a2fb136dd1c4d",
    "successor/APPLE-HOST-ACTIVATION.md": "40d610d410f092cb2c6051ecfef32edb19616adb2ff4925add0a34c22359ef76",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "f886ec3f60a2cc1dcb8e4ddae3ee3e59aa9fa7331edf17c8a83418e88f919afe",
    "successor/V1-ACCEPTANCE.md": "72695a032023907cc93f97bf7882e8198d5cf10b78d8fa11a1fb16ec9c9f85c5",
    "successor/apple/FortKit/Sources/FortKit/MacHostStartup.swift": "7ddbb3c2acf071861b00b7bb16f49a7808073d6c83e9fb35a871aa53f74a5e44",
    "successor/apple/FortKit/Sources/FortKit/MacHostControl.swift": "c3eab929eccb2c1e9ceeaca3f959558204696fd088577181781d1c72a7e755fa",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupView.swift": "090b5fb1718d9bc23e386d0a38cc78084ef9f9d612313147e3b82b41500ab83e",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostStartupTests.swift": "ac53cd20accf9d34397898d0ac9edff09f24b963247c03512587ace342d4d8a9",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostControlTests.swift": "6d2047595cab065da2a1362e3eb605470da8116547b1ca619f5b0e9a8656399e",
    "successor/apple/FortKit/Tests/FortKitTests/OwnerSetupModelTests.swift": "dead956a26585ac57f3ddedb2327dc368f7912f812d995206d0b23f2115497cf",
    "successor/evidence/2026-10-17-node-foundation/installed-host-preflight.md": "e81fb3887b5d6495eb2ac78bbf94bd2649d8905cdc285d4e50c6a777648e5e91",
    "successor/evidence/2026-10-17-node-foundation/installed-host-proposal.md": "784c98a10c7903879cd75cd3186e54981e0c6eeff9c34d1a2714875272e5a02a",
    "successor/evidence/2026-10-17-node-foundation/installed-host-enable.png": "095483fd68ac560978b88a8ad6e4f725991f6bdc3fe14fc6d00020e00cbb4e99"
  }
}
```
