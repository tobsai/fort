What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional official runtimes. This review covers one synthetic Mac launch
diagnosis and its evidence; no shared-hosting or provider workload applies.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`c906b00f4f3876f344d77d0ccf506bb3f686367c`. Scoped semantic Ponytail review against
the pinned read-only skill and preserved whole-repository baseline; this is not
an independent reviewer claim.

Read the production App/WindowGroup/task/QA argument branches, the temporary
account responder and its in-process transport, QA project/build settings, exact
path/ID AX+CG probe, launch/close/reopen/termination runner, capture helper, final
raw observations/inventory and each changed document. Verified all 109 tracked
Apple files exist in the copy; only the described authored responder differs.
The restored application source is byte-identical to the base. Temporary app
instrumentation is absent. No source, dependency, entitlement, credential or
authority change belongs to this slice. Prior baseline findings remain tracked
in the acceptance ledger; this is not a fresh whole-repository audit.

## Must fix before installed acceptance

1. **The final package still needs matching provisioning and installed proof.**
   - What this is: The current production client/helper use an App Group and named
     XPC. The separate ad-hoc QA build uses only an in-process synthetic responder.
   - Problem: The production package remains unsigned and unlaunched. The previous
     local profile inventory found no matching Mac profile; this diagnosis did not
     access Apple accounts or verify remote records.
   - Fix: Complete the pending secure handoff, verify/reuse exact records/profiles,
     resolve only missing provisioning under its required authority, then test
     cold launch, host/account/provider behavior and cleanup on the signed package.
   - If we skip it: Passing a synthetic window check could be mistaken for installed
     host operation or TestFlight availability.
   - Disposition: Open external/installed gate. No credential retry, agreement,
     activation or new permission occurred. G2 and TestFlight remain separate.

## Fixed in this slice

2. **The QA launch command suppressed its initial window.**
   - What this is: The diagnostic launcher passed `--qa-account --qa-session UUID`
     after `open --args` to the dedicated synthetic app.
   - Problem: The exact running app had no AX/visible CG window; a subsequent
     ordinary reopen created it. Reordering arguments or giving the flag an
     explicit value removed the failure. The deeper OS parser cause is unresolved.
   - Fix: Use `--qa-account YES --qa-session UUID`. With restored source, the old
     form still failed; five corrected cold launches and five close/reopens passed,
     as did two no-argument launches and a separate cold AX/screenshot inspection.
   - If we skip it: A harness artifact is treated as a production lifecycle defect
     or masked by silently reopening before checking cold launch.
   - Disposition: Fixed in the harness invocation and recorded evidence. No product
     lifecycle patch was justified. Every trial ended with exact-app absence.

Verdict: The reported synthetic launch failure is isolated and the corrected
invocation is verified. Preserve unchanged production lifecycle code; final signed
installed/provider acceptance remains open.

Not checked: Final signed launch, App Group/XPC, real host/provider, Apple account
role/records/agreements, private-key use, physical devices, TestFlight or provider
closure. Complete portable/native suites were not rerun for unchanged production
source. Diagnostic visibility times are not a production benchmark. The earlier
temporary QA output was rebuilt during diagnosis; its old inventory is historical,
while the final separate output has a new exact inventory. Production review
package and prior evidence remain preserved. No dead-code/removal claim is made.

```fort-review
{
  "format": 1,
  "reviewedCommit": "c906b00f4f3876f344d77d0ccf506bb3f686367c",
  "reviewer": "Codex primary implementation agent",
  "scope": "Synthetic Mac cold-window launch diagnosis, unchanged-source cold/close/reopen regression, exact artifact/AX/screenshot evidence and acceptance notes",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Original launch failed in three initial trials and one final uninstrumented trial; ordinary reopen recovered. Launcher form, title/content and twenty-second timing probes did not remove it.",
    "Final explicit-value launch: five cold launches and five AX close/reopens passed. Two no-argument synthetic launches passed. Separate cold AX/screenshot inspection passed. Normal termination and independent absence passed after every final trial.",
    "Fresh final QA build and strict ad-hoc signature passed. Only App Sandbox/get-task-allow entitlements. Exact 12-file artifact inventory retained; all 109 copied Apple files present and only authored synthetic responder differs.",
    "Argument-order and explicit-value isolation passed three trials each on diagnostic app. Argument dump initially failed on renamed Swift API; corrected scratch probe compiled. Deeper OS mechanism unresolved.",
    "Production source unchanged; full portable/native suites, final signed installed launch and real provider tests were not repeated/run."
  ],
  "exceptions": [
    "Synthetic QA only. No account sign-in, new credentials, provider/host/startup activation, security setting or agreement action.",
    "Spec update records evidence and exact evidence paths under existing decision 059; no new product requirement or architecture is adopted.",
    "Temporary harness source paths/hashes retained in JSON; final production package remains untouched, unsigned and unlaunched."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Matching Mac provisioning and final signed installed cold-launch/host/provider/TestFlight acceptance remain open. Synthetic UI does not establish them.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Windowless synthetic launch follows argument shape. Explicit --qa-account YES fixes the harness; final cold and close/reopen checks pass without production source changes.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "bfb0921070edd0887e28000283982e5cfde6137e8b949d12f87ddae2066c4471",
    "successor/APPLE-HOST-ACTIVATION.md": "f1abe040b7a73db6ced436ac30757623784f74618966d8a3d492929410d2c30d",
    "successor/V1-ACCEPTANCE.md": "b086074b9ac0eaf139ddc0230431793d8463899a59feb40bf700bf1bf34a653e",
    "successor/evidence/2026-10-17-node-foundation/mac-review-package.md": "20889cd011ef8285781b49ee504b190ca1c010c7a8ca18f693f29e5b339afc0a",
    "successor/evidence/2026-10-17-node-foundation/cold-window-regression.json": "243b3b38cd70a48f7b5b966d3756873e9d0c5d85d55e9470752678acfc8574f9",
    "successor/evidence/2026-10-17-node-foundation/account-selection-cold.png": "67b1b848bcca5728cb6c3cf8f4f399a9db8f5dc0f9eb275da8ba0fc90dd2e986"
  }
}
```
