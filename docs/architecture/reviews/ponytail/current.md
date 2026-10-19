What this repo does: Fort is a single-owner native messenger with a Node host and
optional official runtimes. This review covers helper startup inspection after the
approved installed test failed and its startup registration was removed.

Reviewer: Codex primary implementation agent. Reviewed full changed inspector and
manifest parser, program child validation, full-client registration caller, history
maintenance caller, signed-client XPC requirement, native tests, artifact delta and
all changed spec/evidence/handoffs. No optional provider or domain behavior changed.
The whole-tree baseline and prior consumed-test evidence remain preserved.

## Fixed

1. **The sandboxed helper tried to inspect its enclosing client bundle.**
   Its startup called full client inspection, but the real sandbox denied parent
   manifest/startup/profile reads. The canonical signed probe reproduced unsafeFile
   and EPERM before storage/runtime creation. Helper self-inspection now verifies
   its own signed strict manifest, identity/team/group/profile/layout and child
   requirements. Client activation still checks both complete bundles and matching
   manifests; history maintenance retains full validation and identity comparison.
   Existing exact signed-client XPC checks remain. No entitlement was broadened.
   Behavioral and real signed read-only red/green checks pass.

## Remaining acceptance gate

2. **Inspection success does not prove a working provider session.**
   The new 206-file signed package is uninstalled and unlaunched. Its 30 signatures,
   exact metadata and full client inspection pass, but helper startup after inspection,
   XPC, account selection, effective runtime controls, Astra requests and paired
   removal still need a new bounded installed test. The prior grant is consumed;
   the personal workspace selection persists. TestFlight and physical gates remain.

Verdict: The observed inspection boundary defect is fixed without broader authority.
The next exact candidate is concrete and inert; no live-application pass is claimed.

Not checked: Repaired installed startup or provider/lifecycle execution, iPhone or
older-macOS runtime, TestFlight, privacy declaration acceptance and broad portable
tests. All performed tests and the 11 optional skips are recorded below.

```fort-review
{
  "format": 1,
  "reviewedCommit": "4ca08324a851b37cfab0dcc89bba105ac755649b",
  "reviewer": "Codex primary implementation agent",
  "scope": "Helper-only signed bundle validation, retained full-client and exact XPC/child requirements, native regressions, signed read-only sandbox proof and inert replacement candidate",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Original signed canonical-path read-only helper fails unsafeFile; file probes show parent client files return EPERM while own helper files open.",
    "New behavioral regression fails unsafeFile before fix. Final affected suite passes 133 functions with 11 optional skips; 144 reported including skips. Full-client negative controls, helper/child signatures, storage, setup, service and paired/fresh cleanup regressions pass.",
    "Repaired signed sandboxed read-only helper passes MacHostProgram.inspect. Existing storage check reports ENOENT and creates nothing. Probe has no network entitlement or service/runtime launch.",
    "Release FortMac/FortHost build in 27.496/3.005 seconds; iPhone Simulator Release compilation passes. Simulator execution not run.",
    "New inert candidate: every 206-file inventory/hash/mode/size, all 30 strict signatures and public certificate fingerprints, exact entitlements and full native MacHostBundle.inspect pass. Only client/helper executables and outer resource seal differ from original signed payload.",
    "Installed cleanup app remains stopped with exact report 9e401469e60006e14f063718be077c4aed14f0719c13e3b7d145a2c68b215453; original consumed test and cleanup evidence preserved."
  ],
  "exceptions": [
    "Scoped implementation-agent semantic review, not independent review or whole-v1 acceptance. Prior baseline and review history preserved.",
    "The initial /tmp diagnostic had path normalization differences; decisive red and green probes use canonical Fort-local paths.",
    "Inert runtime candidate is uninstalled/unlaunched. The read-only probe validates inspection only, not full startup, XPC, provider requests or lifecycle. No new credential, service registration, G2 upload or provider cleanup occurred.",
    "CoreSimulator service version warning remains; iPhone code compiled but simulator and physical execution are unrun. Broad portable suite not repeated for Swift-only validation change."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Helper startup wrongly used enclosing-app inspection despite sandbox-inaccessible parent resources. The helper now checks its own signed configuration/profile/executables; the client retains full package checks and exact XPC peer requirements. Real signed read-only red/green proof passes.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Full installed startup/XPC and selected personal ChatGPT/Astra behavior remain untested after repair. New exact candidate is inert and needs a new bounded grant; paired lifecycle, privacy declarations, TestFlight and physical acceptance remain open.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "0b6fd823b1a3d6e727e707e51a2088e9495b2fda30894a2eb4198cb135718e48",
    "successor/APPLE-HOST-ACTIVATION.md": "67d442d8780e05ef562413245d43df10b5014e081f41d68920c1d49513ad6bd7",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "1664e16cf58b88a6bf16db2d8a97b397cde878cf9ddefe5eb4e8f618bf93fd73",
    "successor/V1-ACCEPTANCE.md": "5a80232495e4c0b0849486ff0b0e69e6777980867a9c568c20d9fe0f3c77c574",
    "successor/apple/FortKit/Sources/FortKit/MacHostBundle.swift": "f31ca7b1e450cdecd994e1c7f9c7c84c72fbad44a6476a2f7982b2439850521c",
    "successor/apple/FortKit/Sources/FortKit/MacHostProgram.swift": "c46a3b76f89aa0bea9a79080da65cb30b421132ad4dee3cf87fb9916d9e53716",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostProgramTests.swift": "91b453d8ae791dd66c9be4a222e93b82010ed36a91574a9c79332bc44870fe13",
    "successor/evidence/2026-10-17-node-foundation/installed-host-proposal.md": "f684d2b1a8b6b0899247cdc2715c1ba1c48e47b49b3351670354a8d71deb01a4",
    "successor/evidence/2026-10-17-node-foundation/installed-host-test.md": "2279897142e3303445bca02107abc1d6c812570e1d512edd0b133786c2b3f803",
    "successor/evidence/2026-10-17-node-foundation/helper-self-inspection.md": "ee3292ff078b88a033f996ee5b55331ca7b1b3f45a27a3e82488d5cb013b06ff"
  }
}
```
