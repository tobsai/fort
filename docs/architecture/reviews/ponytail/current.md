What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional runtimes. This scoped review covers the exact current Mac demo
package and its evidence, assuming one owner and a per-user Mac host.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`1f3cda5dc473b4960c397b4a9a20915d2a262d64`. Scoped semantic review, not an
independent-reviewer claim. Applied the pinned Ponytail skill as read-only text;
the whole-repository baseline and earlier implementation reviews remain in history.

Read the package builder/configuration, current client/helper targets, ordinary
bootstrap, MacHostBundle/Program/Control and the exact refusal test; reviewed all
output/input manifest membership, modes and hashes. Compared the ledger and Apple
handoff against observed build/authentication state. This slice changes evidence
only. It introduces no API, dependency, entitlement, runtime startup or account grant.
No code was declared unused. Existing native provider-account controls were traced
as implemented source with real-provider acceptance still absent.

## Must fix before the Mac demo

1. **The review artifact cannot pass installed-host activation.**
   - What this is: A fresh unsigned 200-file Mac app/helper/runtime assembly.
   - Problem: It has zero embedded profiles and no verified production identity or
     final signature; its public inspector correctly refuses activation.
   - Fix: Verify existing Apple app/helper/group records, obtain exact approved
     profiles/signing and separately review outbound network/provider/startup scope.
   - If we skip it: The native app cannot establish the real owned host journey.
   - Disposition: Open. No profile is fabricated and no permission is bypassed.

2. **Real-provider and TestFlight acceptance remain unrun.**
   - What this is: Portable/native fixtures and compiled Release packages.
   - Problem: They do not prove actual account access, installed chat/restart/
     reconnect, TestFlight availability or physical acceptance.
   - Fix: Complete the authorized installed Mac journey once the exact activation
     package is ready, then the separate Apple distribution gates.
   - If we skip it: Source/build success could be mistaken for demoable delivery.
   - Disposition: Open. Phone UI is deferred behind the Mac demo priority.

Verdict: The package and evidence are consistent and reviewable. Its exact blockers
are Apple identity/provisioning/signing and per-action host/provider grants, followed
by installed acceptance. This is not a working installed demo or TestFlight build.

Not checked: Actual signed/provisioned host, production provider/network/startup,
installed UI screenshots/accessibility, phone, archive/upload/processing/TestFlight,
remote CI or provider cleanup. The optional retained signed probe was not repeated.
Full native tests and fresh client/helper builds passed; package inspection and
all input/output hashes passed. No source implementation changed in this slice.

```fort-review
{
  "format": 1,
  "reviewedCommit": "1f3cda5dc473b4960c397b4a9a20915d2a262d64",
  "reviewer": "Codex primary implementation agent",
  "scope": "Current Mac demo engineering package and truthful activation/delivery evidence; no source or permission change",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Existing source: 1308 portable passes / 15 skips and strict TypeScript passed.",
    "Fresh native run against current compiled resources: 317 passed / 15 skips, 332 reported; exact package refusal probe then passed separately.",
    "Fresh unsigned Release Mac client and helper builds passed.",
    "200 output files, exact membership and all 198 input hashes/modes independently verified; zero profiles; public signature refusal and synthetic profile refusal confirmed."
  ],
  "exceptions": [
    "No installed/provider/Apple/TestFlight execution or permission change; no new product implementation in this evidence-only slice."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Exact production identity/profiles/signing and network/provider/startup grants remain before activation.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Actual installed Mac launch/chat/restart/reconnect and TestFlight acceptance remain unrun.",
      "disposition": "open"
    }
  ],
  "files": {
    "successor/evidence/2026-10-17-node-foundation/mac-review-package.md": "5336f92731e9bc4054935f8f1acdd86114d7c3715a78db0ad5b1e472cdd5ed4a",
    "successor/APPLE-HOST-ACTIVATION.md": "83bca34df909eaefcc434c0d362dfb0ea3d6f431d5b29a766089d8a66bdfd82b",
    "successor/V1-ACCEPTANCE.md": "256f839a8f23bab2e0cd5417d5325d74700c2571e703083b172757625094848e"
  }
}
```
