What this repo does: Fort is a single-owner native messenger with a Node host and
optional runtime adapters. This review covers account/model selection, removal
following a prior completed pairing, and the attended installed smoke test.

Reviewer: Codex primary implementation agent, not an independent reviewer. Reviewed
HEAD 04afae3b0bc535a3b361d5c4e5f3a3e9ef6b2b75 plus this exact pending slice, using
the pinned Ponytail skill and preserved whole-tree baseline. Read changed UI/model,
public account tests, full removal identity/progress/history boundaries and new
adapter regression. Traced runtime catalog and managed-login paths, version pins,
packaging controls and installed cleanup evidence. Assumed one owner and the
existing bounded request queue; no shared-hosting scope.

## Must fix

1. **Quit can fail after a Settings confirmation.**
   - What this is: Settings uses a native modal sheet with a window-local Quit policy.
   - Problem: Installed Quit and foreground Command-Q did not terminate after
     confirmation actions. Closing Settings first allowed normal Quit.
   - Fix: Reproduce the exact installed modal/navigation lifecycle before changing
     the window policy. Three simpler alert fixtures pass without a further fix.
   - If we skip it: Normal Quit can require an extra navigation step.
   - Disposition: Open v1 UI finding, assigned to native UI implementation. No
     ineffective regression or speculative global termination override added.

2. **The bundled runtime predates GPT-6.1 Sol catalog support.**
   - What this is: Fort pins its actual packaged Codex runtime to 0.157.1.
   - Problem: Official 0.159.1 added GPT-6.1 Sol; current stable 0.163.0 lists it.
     Fort's exposed catalog omits it. That catalog is not an entitlement test.
   - Fix: Prepare a coordinated runtime/adapter/legal/package update and run isolated
     startup controls. Preserve exact bindings; then verify authorized actual inference.
   - If we skip it: The user's requested model remains absent from the installed picker.
   - Disposition: Open compatibility/update slice. Preparation is ordinary scope;
     replaying the completed activation session or extracting managed tokens is not.

## Fixed in this slice

3. **Authentication left model selection behind a separate confirmation page.**
   - What this is: Model discovery requires explicit selection of the reported account.
   - Problem: The agent editor had an empty catalog and did not expose that control.
     An overlapping old discovery could also replace the newly selected account's list.
   - Fix: Reuse the existing account view in the editor, refresh after confirmed
     selection, and serialize account selection against model discovery in both directions.
   - If we skip it: The user must leave an unsaved form and manually discover the
     account-confirmation page; stale discovery can leave the catalog unavailable.
   - Disposition: Fixed. Three public-model regressions failed first and now pass.
     Exact-account checks, companion restrictions and saved-model preservation remain.

4. **Completed historical removal blocked removal of a new pairing.**
   - What this is: MacHostRemoval validates current connection and saved removal intent.
   - Problem: A confirmed intent from an earlier pairing was rejected as active foreign
     work, preventing normal cleanup of the exact new installed pairing.
   - Fix: Allow only confirmed foreign intent as completed history; retain exact
     current/disconnected identity, pending-setup and all later progress/exit proofs.
   - If we skip it: The owner's host can stay registered after a legitimate removal request.
   - Disposition: Fixed. Confirmed case failed first; requested/locally-forgotten foreign
     cases still reject before recovery, startup removal or credential cleanup. Normal
     installed cleanup and independent absence checks pass; history bytes are retained.

Correctness and safety: Account operations remain MainActor-serialized and retain
existing generation/identity checks. A successful explicit confirmation releases its
busy state before requesting exactly one guarded catalog refresh. No account/model
fallback was added. The removal exception is limited to completed history and does
not satisfy current connection, host receipt or final local-removal proofs by itself.
Private managed credentials stay inside the official runtime. Host access and
computer-tool sandbox restrictions remain separate; this UI slice does not change them.

Validation: Fresh complete native suite passes 341 with 16 optional skips; portable
suite passes 1,329 with 16 skips. Mac and iPhone Simulator Release builds pass.
Two client-only updates preserve helper/runtime/resources/permissions and old full
packages. Installed accessibility/screenshot, two exact-marker Astra requests,
Stop/Resume, relaunch/history and cleanup were inspected. Three of six prompt slots
are accounted for including an extra observed turn; no message bodies were copied.
The final 206-file manifest matches installed bytes. Client/host processes, startup,
auth.json and active connection are absent; history remains. No new live session began.

Verdict: Account selection and historical-pairing cleanup are fixed and verified.
Full smoke/v1 signoff remains blocked by the Settings Quit case and other recorded
release gates. The runtime update is assessed, not implemented or live-tested.
Not checked: New 0.163.0 execution or 6.1 inference, fresh sign-in reenrollment,
full OS isolation, physical iPhone, Simulator UI, optional signed/hardware benchmarks,
remote CI, TestFlight or G2 upload. Effective provider model was not reported by the
completed turn. This is a scoped review, not a repeated whole-repository audit.

```fort-review
{
  "format": 1,
  "reviewedCommit": "04afae3b0bc535a3b361d5c4e5f3a3e9ef6b2b75",
  "reviewer": "Codex primary implementation agent",
  "scope": "Account confirmation/model discovery race repair; completed-historical-pairing removal repair; exact installed smoke/cleanup and official runtime-update assessment",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Three public model regressions failed before the fix: explicit account confirmation refreshes exact catalog; selection refuses pending discovery; discovery refuses pending selection. No sign-in or prompt is invoked by these fixtures.",
    "Public MacHostRemoval regression: confirmed foreign prior intent initially fails; requested and locallyForgotten foreign intents refuse before effects. Final all three states pass with unchanged connection/history/prior intent bytes.",
    "Fresh sequential full native suite: 341 pass, 16 optional skips, zero failures (357 total), 59.552 seconds. Portable full suite: 1329 pass, 16 skips, zero failures (1345 total), 42.134 seconds.",
    "Fresh Mac and iPhone Simulator Release builds pass. Final candidate: 206 files, 30 Mach-O signatures, exact certificates/entitlements and native package inspection pass. Installed bytes match report b418cf5fb43b631339a9b1031ab162d3f0e8f774756e39b9352bfb96b50af67f.",
    "Installed account/model persistence, accessibility/screenshot, two synthetic Astra marker replies, stop/resume, window close/reopen/history and removal pass. Three of six slots conservatively accounted. Effective model not reported.",
    "Independent cleanup at 03:04:57 UTC; final disconnected verification at 03:17:18 UTC confirms no client/host process, launchd job, provider credential path or active connection; history retained.",
    "Installed Quit after confirmation fails; Back/Done then Command-Q passes. Plain AppKit and two SwiftUI alert diagnostics pass without product changes and are explicitly nonreproductions.",
    "Full and accepted implementation spec checks pass; exact staged spec/review and whitespace gates required before commit."
  ],
  "exceptions": [
    "Implementation-agent semantic review; no independent reviewer claimed. Existing baseline and prior evidence remain in history.",
    "New runtime is source-assessed only. Model/list can be bundled/cached; no credential extraction, direct token-sharing request or entitlement claim.",
    "Completed attended authority is not replayed. Local cleanup retains app/history and provider-side grants; G2 upload stays held.",
    "Settings confirmation Quit remains open. Diagnostic tests are preserved outside product source because they do not reproduce the installed defect."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Installed Quit after a Settings confirmation can fail until Settings is closed. Root cause not reproduced by simpler alert fixtures; retain as native UI acceptance blocker.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Installed Codex 0.157.1 predates 6.1 Sol bundled support. Prepare coordinated 0.163.0 compatibility/package update; account inference remains unverified.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Expose explicit account confirmation within agent editor and refresh exact model catalog afterward while preventing both discovery/confirmation operation races. Public regressions and installed selection pass.",
      "disposition": "fixed"
    },
    {
      "id": "4",
      "detail": "Only confirmed foreign prior pairing-removal intent is completed history. Current exact identity and cleanup proofs remain mandatory; unfinished foreign states refuse. Regression and installed cleanup pass.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "f893426b219196ccfb4ec81a52aab05f4ba2f810b1027cf666e932b03c4cf6a8",
    "successor/APPLE-HOST-ACTIVATION.md": "fe0aa18c260870d1e0d637bfea2e9a959fca79b2976b32c212a106275d1634fc",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "8dd1e1258e5406e510ef5e7ce03b40fe6c7bf15bf9671e57666ab690c7ac6f75",
    "successor/V1-ACCEPTANCE.md": "227b252f4e841d2cba2d5accb96b15e86382d63c934fe8f12508a2881a611805",
    "successor/apple/FortKit/Sources/FortKit/AccountSignInView.swift": "e6883f0ec7d819c2a526eb94bf5f60b962df4eb5ab7c3086e4df4feaab298a23",
    "successor/apple/FortKit/Sources/FortKit/AgentSettings.swift": "33cbcbdb587f2cfe5b0a0c8ac914569a11c5dbd31b3ff31a7c8ce1f747af30e3",
    "successor/apple/FortKit/Sources/FortKit/MacHostRemoval.swift": "5ced8fe9b5bbe11f67729237c96933841dac6d1d0ba738224588a59ca90e819b",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "9dc2b9be79d05c1328f7ae21f3e150c1a553f96148b13169279a15a507df764e",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "7f0f432c4ea1519a0c3ddb2f2de06db898605a6f8268a541f84b08140bad6bed",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostRemovalTests.swift": "f7a20d2884460b6310e0fc95fb9f25a0ab67e1d021be6861ae412e111dde74ca",
    "successor/evidence/2026-10-17-node-foundation/model-selection-recovery.md": "d53811773e0fbb9299d50314918f35699ef2de1b44a0b0e08bdc13ddc641e635"
  }
}
```
