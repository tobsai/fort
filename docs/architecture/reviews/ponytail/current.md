What this repo does: Fort is a single-owner native messenger with a Node host and
optional runtime adapters. This review covers the Mac Settings sheet's effect on
normal Quit while preserving the active installed authentication handoff.

Reviewer: Codex primary implementation agent. Read the pinned Ponytail skill and
baseline, both changed views, the new rendering test, Settings navigation/account
callers, model/storage construction and all changed evidence. Whole tracked-tree
search finds the window bridge only in the two intended views, with no application
termination override. Prior reviews remain in Git history. No dependency or policy
boundary changes; ordinary source-repair authority under decision 059 applies.

## Fixed

1. **Settings retained the modal window default that blocks Quit.**
   - What this is: Connect to Fort and Settings are separate SwiftUI sheets.
   - Problem: The earlier setup repair attached its window bridge only to Connect
     to Fort. Installed Quit with Settings open remained blocked until Back/Done.
   - Fix: Reuse the bridge in Settings, keeping its property assignment local to
     the containing window. The actual view-rendering regression fails before the
     change and passes afterward, preserving its draft and empty temporary storage.
   - If we skip it: Users must close Settings before ordinary application Quit.
   - Disposition: Source fixed; changed-build installed Quit remains unverified.

## Must complete before v1 signoff

2. **Changed-build installed Quit and provider acceptance are pending.** The live
   installed app was deliberately left untouched while the owner can authenticate.
   No new code was generated and no credential inspected. The last observed exact
   host/pairing remain active; zero of six prompts used and final cleanup pending.
   Confirm personal account state, then complete the authorized provider and removal
   checks. Install/test this source repair only when the handoff can safely pause.

## Should fix

3. **The removal timing test measures other tests waiting on the same actor.**
   - What this is: ordinaryMacRemovalCompositionMetadataBudget measures an async
     MainActor operation against a 50 ms p95 budget during parallel Swift tests.
   - Problem: Both parallel runs have one timing failure, at 52.55 and 65.75 ms;
     all 24 functional functions pass. The new view initialization also runs on
     MainActor. The second failure occurs after native builds finish, so compilation
     contention alone does not explain it.
   - Fix: Keep performance measurement isolated from concurrent MainActor tests.
     The explicit sequential focused run passes all 25 functions, with 0.668 ms p95.
     A dedicated measurement invocation in CI is a follow-up; no budget was relaxed.
   - If we skip it: The default concurrent native CI run can report timing failures
     that do not isolate host composition cost. No fresh remote CI pass is claimed.
   - Disposition: Open validation concern recorded in the acceptance ledger.

Verdict: The bounded Settings source repair has red/green and native compile proof.
It is not installed acceptance. Preserve the authentication handoff until the owner
confirms the personal-account step, then finish the approved tests and cleanup.
Not checked: Changed-build installed AX/screenshot/Quit, further modal sheets,
real replies/cancellation, final active-host cleanup, fresh full portable/native CI,
TestFlight or physical-device execution. The Simulator compile succeeds despite its
service version warning; no Simulator execution is claimed. G2 upload stays held.

```fort-review
{
  "format": 1,
  "reviewedCommit": "6ffca1e0f3047396e8ae7884b391da55e369044d",
  "reviewer": "Codex primary implementation agent",
  "scope": "Settings modal termination repair using the existing setup bridge, hidden real-view red/green regression, native builds and exact authentication-handoff evidence; no installed application or account changes",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "New real Settings view test fails on default modal termination behavior before source repair; same test passes afterward with invisible window, unchanged draft and empty isolated storage.",
    "Sequential focused Swift run passes 25 test functions with zero skips/failures. Two concurrent runs each pass 24 and fail the existing removal composition timing assertion at 52.55/65.75 ms versus 50 ms. Sequential p95 is 0.668 ms; no budget/code change.",
    "Fresh unsigned Release FortMac and FortiOS Simulator builds pass in separate derived-data directories. CoreSimulator service version warning remains; neither artifact was installed or launched.",
    "Full spec, accepted implementation, exact staged spec/review coverage and whitespace checks pass. The review explicitly retains two open acceptance/validation findings.",
    "No installed UI interaction, code generation, account/credential inspection, provider prompt or cleanup action in this source slice. Zero of six prompts; authentication confirmation and final cleanup pending."
  ],
  "exceptions": [
    "Scoped implementation-agent semantic review, not independent review or whole-v1 acceptance. Earlier source, review, approval and installed test evidence preserved.",
    "A hidden NSWindow regression verifies the public termination-blocking property; it does not run application Quit or prove process-exit preservation. Actual installed checks deferred to protect the current user authentication step.",
    "Default concurrent timing check remains unreliable in the tested workload. The reported all-pass result explicitly uses --no-parallel; no fresh full CI or broader native/portable suite is claimed.",
    "Source change creates no new global termination behavior. Other sheets, host lifecycle, provider controls and account permissions are outside this bounded change; G2 upload remains held."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Settings uses a different modal window from Connect to Fort and retained its default termination restriction. Reuse the existing window-local bridge and verify the rendered view through a red/green regression. Source fixed; installed acceptance pending.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Owner authentication confirmation, personal account selection, exact Astra discovery, provider tests, final cleanup and changed-build installed Quit are incomplete. Preserve the running sign-in UI and six-prompt total.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "The existing async MainActor removal timing test fails in both concurrent focused runs but passes sequentially. Isolate performance measurement in the validation lane; keep the 50 ms budget. Recorded in acceptance ledger.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "be803df14f294fa3b7fd6f642148eb53a6af29f9676b0666b7e7098f103dd211",
    "successor/APPLE-HOST-ACTIVATION.md": "4c6622d88e6fb63fa9417b1b4a0193c31a0fdcb7652aafa12b752a8030b02765",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "8257c42278107b7bbc0003be8264520792ff325bd5585438c49c4111d0985a06",
    "successor/V1-ACCEPTANCE.md": "86e35381022f6c11a69a2b65783c10e5dfd54cfdafc16bbd4d43dcf0344d4e3e",
    "successor/apple/FortKit/Sources/FortKit/AgentSettings.swift": "7227d48877a12ef697a9d16e0cb3d1877d88d8d31e7ec9ca3b343ce8ca304458",
    "successor/apple/FortKit/Sources/FortKit/MessengerView.swift": "81253acdcbe3394a0ee7cae58ce7ce4e4102ea680e5ea59cf949c62b01725439",
    "successor/apple/FortKit/Tests/FortKitTests/AgentSettingsTests.swift": "dbc9d69c3dda3be68271ad1d700b48613f800783fb1dfd8c666969fca88eef22",
    "successor/evidence/2026-10-17-node-foundation/installed-host-test.md": "6b82f066545a61801eac024b503e3c4e402cf8ad834a154b501b5956434d3556",
    "successor/evidence/2026-10-17-node-foundation/installed-setup-quit.md": "699c928d882da157528f6e6aa1175578c73fdb8cf708960cf3da966c982d978b"
  }
}
```
