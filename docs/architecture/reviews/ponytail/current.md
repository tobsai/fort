What this repo does: Fort is a single-owner native messenger with a Node host and
optional official runtimes. This review covers one unpaired Mac client's setup
presentation, normal termination and an inactive proposal for a later provider test.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`7a0713642acb921ae54b98bc5273e98095d8c133`. Applied the pinned Ponytail skill and
preserved whole-tree baseline. This is scoped semantic review by the implementation
agent, not independent review or v1 signoff. Reviewed the product diff, complete
installed regression runner, model presentation/restore callers, native setup/
recovery gates, packaging and all changed spec/evidence/proposal/handoff files.

## Fixed

1. **Setup prevented normal Quit and had no unpaired Close action.**
   - What this is: SwiftUI presented the connection form as a modal sheet.
   - Problem: Its window inherited AppKit's termination-blocking default, and the
     old form only offered dismissal after an installation existed. Normal Quit
     left the unpaired process alive. A minimal sheet reproduced this independently
     of Fort's model/runtime; the client main thread was not blocked.
   - Fix: A private view sets the documented AppKit property only on the setup
     window, with no global exit override. Close setup changes presentation state
     only. The maintained installed regression failed first, then three final
     cycles passed six normal Quits, nine setup close/reopens and three window
     close/reopens. Actual screenshot/AX confirms available Close/Enable and
     disabled Connect. No activation action was pressed.
   - If we skip it: A fresh owner cannot normally leave or quit setup.
   - Disposition: Fixed for the exact installed client-only artifact on macOS 27.2.
     The Mac 14 deployment target still compiles; older-OS runtime is untested.

## Remaining acceptance gate

2. **Signed provider bytes still need authorized installed execution.**
   - What this is: The corrected proposal candidate retains the one proposed helper
     outgoing-network entitlement and unchanged identities/profiles/child sandbox.
   - Problem: Signing/client UI checks do not establish actual helper App Group/XPC,
     provider behavior, host shutdown/unregistration or credential cleanup.
   - Fix: Use the refreshed exact artifact and one consolidated action-specific
     grant for the selected account/model, bounded test and scoped cleanup.
   - If we skip it: A prepared artifact could be mistaken for an accepted live app.
   - Disposition: Open. Candidate remains uninstalled/unlaunched. G2 upload is held
     separately. Privacy declarations, TestFlight and physical gates remain open.

Verdict: The confirmed setup Quit defect is fixed and verified at its real installed
interface. Final clients all exited normally; no host/service/credential activation
occurred. The repinned proposal preserves its one-time 30-minute/six-request scope
and the product's no-default-expiry pairing policy. No new in-scope defect found.

Not checked: Real host/provider/XPC/removal, older-macOS runtime, physical iPhone,
TestFlight, runtime privacy declarations or full portable/native regression suites.
The focused native run passed 66 functions with two optional timing skips; Mac,
helper and iPhone Simulator Release builds passed. All 206 files and 30 signatures
per exact candidate passed. Initial test-only startup/toolbar assumptions were
corrected and are documented. No final repaired client required force termination.

```fort-review
{
  "format": 1,
  "reviewedCommit": "7a0713642acb921ae54b98bc5273e98095d8c133",
  "reviewer": "Codex primary implementation agent",
  "scope": "Setup-sheet modal termination/Close fix, maintained installed no-activation UI regression, exact signed candidate verification and refreshed one-time authorization proposal",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Actual installed baseline and checked-in runner fail at normal Quit with setup open. Minimal independent SwiftUI sheet reproduces; closing it or applying the documented termination property makes normal Quit succeed. Main thread waits normally.",
    "Final signed client-only three-cycle run passes six normal Quits in 0.052\u20130.337 seconds, nine setup close/reopens and three same-process window close/reopens. Final AX/screenshot and separate normal Quit pass. No activation or force-kill action in the runner.",
    "Focused native suite: 66 passing functions, zero failures, two optional timing skips. Release FortMac/FortHost builds pass in 28.651/3.124 seconds; iPhone Simulator Release build passes.",
    "Both exact final packages pass every 206-file membership/hash/mode/size check, all 30 strict all-architecture signatures and exact approved public certificate fingerprints, profiles/entitlements and native bundle inspection. Independent final process/service/host-storage absence and installed hashes pass.",
    "Full spec/accepted implementation gates passed before product edits; exact staged spec, review and diff checks required before commit."
  ],
  "exceptions": [
    "Scoped implementation-agent semantic review, not independent review or v1 signoff. Whole-tree baseline is preserved.",
    "Initial minimal-fixture compile lacked compiler-plugin sandbox access; supported normal build access passed. The first fixed-package UI check exceeded a five-second launch callback allowance. Startup now allows thirty seconds while Quit retains five. A later check missed Connection in toolbar overflow; the runner now follows that observed menu path. Signed product bytes did not change for these test corrections.",
    "Baseline failing processes required exact-task SIGTERM cleanup. Every repaired final client used normal Quit, with independent absence checks. Temporary minimal sources/app are stopped and retained in /tmp/fort059-quit-diagnosis.",
    "No helper/provider/network activation, pairing or credential configuration, G2 upload, trust change, new certificate or remote cleanup occurred. Existing one-time test proposal remains unapproved."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "The setup sheet inherited modal termination blocking and lacked an unpaired Close action. The scoped AppKit property and Close presentation action now pass actual installed normal Quit plus repeated setup/window flows without activation.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Refreshed exact provider candidate remains uninstalled/unlaunched pending the existing action-specific approval. Actual helper/XPC/provider/removal, privacy declarations, TestFlight and physical acceptance remain open; G2 upload is held separately.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "ef4cb06326481c6d9ae1ac578f655e3dfd6dbe2586f5d720fcfd06e4e052bdd4",
    "successor/apple/FortKit/Sources/FortKit/MessengerView.swift": "145aaeef2d93b24c5f61120d0e5abfe288554d40886e6704425040c3a42a7df4",
    "successor/apple/check-installed-setup.swift": "24ed423113609d740e06bacd7527c5abb86e200de39f3c8df3c9a0f2c191d43e",
    "successor/APPLE-HOST-ACTIVATION.md": "e2e62d77a02c20fac02eceab70fa063288db52b4a027b8f88035c1ecf9c4ae54",
    "successor/V1-ACCEPTANCE.md": "9c3ad74e0e08a89fa5b952b7a8bbf28bd81de4ac6b0e336f622d4196a8c5360b",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "8fa9075b5c1b4ec1aa0fcec4d83ca61df9d75602471cda32837bcc4f3c250684",
    "successor/evidence/2026-10-17-node-foundation/installed-host-proposal.md": "bccad78cd9e6c83dbfc28835adf5ca52230b375952c5be273597972079cc802c",
    "successor/evidence/2026-10-17-node-foundation/installed-host-preflight.md": "0acba3d860eff77dcb1a5e0e561c9d31dd80f06fcb0dc0d9a71d940aa0c0db26",
    "successor/evidence/2026-10-17-node-foundation/installed-setup-quit.md": "7d3361da0e913cb929dc698023c7d947e7e80d6e09c401b078380241238948dc",
    "successor/evidence/2026-10-17-node-foundation/installed-setup-close.png": "13ba362c903d7a45d3c28e3cb94f651f20676d3538beeeb10170d4329a77930e"
  }
}
```
