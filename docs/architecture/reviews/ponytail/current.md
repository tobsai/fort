What this repo does: Fort is a single-owner native messenger with a Node host and
optional runtime adapters. This review covers the Pairing settings Quit repair and
its disconnected installed verification.

Reviewed HEAD 43bd03279df93a1ead6c1fbbd694c9d4aa6d87ee plus this exact slice. Reviewer is the primary
implementation agent. Used the pinned Ponytail skill and preserved whole-repository
baseline. Read PairingSettingsView, FortSettingsView, MessengerView's sheet bridge
and workspace navigation, the new regression and evidence. Traced Stop/Cancel to
confirm this change adds no host operation. Reviewed candidate/install scripts,
rollback, exact manifest and absence checks. Assumed one owner and normal SwiftUI
navigation. No shared host or global application termination change.

## Fixed

1. **Entering Pairing blocks normal Quit.** The same sheet becomes termination-blocking
   when NavigationStack replaces its root. The earlier alert-only fixtures missed
   that transition. Reusing the existing NonBlockingSheetWindow on the destination
   fixes the actual flow with three production lines. The native regression failed
   first, and the faithful synthetic navigation plus confirmation/Cancel/Quit passes.
   Without this change, users must leave Pairing before quitting.

## Remaining acceptance

2. **The disconnected installed app cannot reach Pairing.** It has no workspace and
   therefore does not expose Settings. The actual Pairing flow is verified with real
   FortKit views and synthetic transport, while the installed check covers setup,
   screenshot/accessibility, normal Quit, exact bytes and no host activation. Repeating
   the paired production journey requires separately authorized activation. Do not
   create a diagnostic bypass or infer live acceptance from synthetic data.

Correctness: The reused view sets only its containing window's termination flag
when attached. It has no event polling, delegate override, lifecycle side effect or
state write. The actual alert remains blocking until dismissal. Draft and connection
bytes survive the regression. macOS conditional compilation preserves iPhone behavior.
The three ledgers now lead with current evidence and retain earlier observations.

Validation: Native 342 pass and 16 optional skips; Mac/iPhone Simulator Release
builds pass. Candidate has 206 verified files and 30 verified Mach-O signatures.
Only the installed client executable changed; the full prior app is retained.
Installed disconnected setup inspection and normal foreground Quit pass. Final
03:40:12 UTC checks confirm no client/host process, job, auth file or connection.

Verdict: The reproducible navigation defect is fixed. The exact paired installed
flow remains an acceptance gap; no live session was restarted.
Not checked: Portable tests for this Swift-only slice, new Codex execution or live
6.1 inference, production paired flow, physical iPhone, TestFlight or G2 upload.

```fort-review
{
  "format": 1,
  "reviewedCommit": "43bd03279df93a1ead6c1fbbd694c9d4aa6d87ee",
  "reviewer": "Codex primary implementation agent",
  "scope": "Pairing destination Quit policy; rendered-view regression; faithful synthetic Settings flow and disconnected installed client-only update",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Rendered PairingSettingsView regression failed before fix; both destination and root policy tests pass, preserving draft/connection bytes.",
    "Faithful synthetic app with actual MessengerView/Settings views reproduces navigation resetting the flag and blocking normal Quit. Fixed Pairing/Stop/Cancel flow exits via normal foreground Command-Q.",
    "Complete native suite: 342 pass, 16 optional skips, 358 total, zero failures in 59.766 seconds. Mac and iPhone Simulator Release builds pass.",
    "206-file candidate, 30 Mach-O signatures, exact entitlements/certificates and native inspection pass. Installed update changes only client executable, preserving full rollback app and helper inode.",
    "Installed disconnected setup accessibility/screenshot and foreground Command-Q pass. Final 03:40:12 UTC check confirms exact manifest and no client/host processes, startup, provider credential path or active connection.",
    "Portable suite not rerun for Swift-only slice; prior 1329-pass result belongs to 43bd032. Spec/staged review/whitespace gates required before commit."
  ],
  "exceptions": [
    "Primary-agent scoped semantic review, not independent or repeated whole-repository audit.",
    "Production Pairing route is unavailable while disconnected. Exact installed paired route remains unrun; no host activation, pairing or provider access performed.",
    "Codex runtime remains 0.157.1; separate offline 0.163.0 preparation has not yet produced an accepted package.",
    "Physical iPhone, TestFlight, live 6.1 inference and G2 upload remain unrun/held."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Pairing NavigationStack destination resets sheet termination blocking. Reuse existing local NSView bridge on the destination; failing native regression and faithful synthetic normal-Quit flow now pass.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Production installed Pairing/confirmation flow cannot be reached after verified cleanup without authorized host activation and pairing. Preserve this acceptance gap rather than reenroll.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "42c6239c4250ed38abb1da17c9df6b20f7c3a210aff8fce519e1999bcb28a469",
    "successor/APPLE-HOST-ACTIVATION.md": "9733953ddd0a3298bb02b6febb76fec4cba1e1b70c99b6fca0d5bf6e4853a43c",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "84754579c1ddaea8a416bcc5d483d2fae776242a4d049467e01272883d494a6f",
    "successor/V1-ACCEPTANCE.md": "2547731b706a81a170c64a7aae9a8e458b5f6433804475a13bd085876cdcb489",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "ffc92771a47beb93cb8b62504570ae85bba9547ca8ddd7a1976c93b4d5921dde",
    "successor/apple/FortKit/Tests/FortKitTests/AgentSettingsTests.swift": "fb21e597f780cc77b03cad98c7d4e9f965ca5e29b7a3cbdc919bf9fc017d07f0",
    "successor/evidence/2026-10-17-node-foundation/model-selection-recovery.md": "2abc2cc2a98ff4d3acceb473ca24cf762ae9b1e1326ddd7bfc25ff3518fd7532"
  }
}
```
