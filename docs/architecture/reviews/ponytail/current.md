What this repo does: Fort is a single-owner native messenger with an optional
Codex executor behind its Node host. This slice fixes private runtime startup and
records the installed authentication handoff, with unchanged runtime permissions.

Reviewer: Codex primary implementation agent. Read the pinned skill and baseline;
reviewed the exact four source/test changes, private-path guards, Codex effective
configuration/inventory checks, both callers of shared settings, account/executor
flow, compiled artifact and all changed evidence. Prior implementation reviews
remain in Git history.

## Fixed

1. **Fresh Codex startup rejected transportless MCP entries.** Disabled named
   servers still require valid transport definitions. The private launch now asks
   for an empty map instead. Shared validation accepts an empty map while still
   rejecting servers without explicit disabled state and unexpected/active inventory.
   Two public red tests, 561 green focused tests and the exact official sandboxed
   initialization/control probe support the change. No tool or entitlement expanded.

## Must fix before v1 signoff

2. **Settings blocks normal Quit while its sheet is open.** The installed attempt
   stayed running after Quit; Back/Done followed by Quit succeeded. The host had
   already completed normal removal. A separate native behavior regression and
   repair are required, rather than claiming this installed lifecycle fully passed.

3. **Real-provider acceptance still awaits human authentication.** The repaired
   installed build reached the official challenge and opened its verification page.
   Personal workspace verification and explicit account selection must precede Astra
   discovery and prompts. Zero of six prompts were used. Current host/pairing remain
   active for that handoff, so final cleanup is explicitly pending.

Verdict: The narrow startup defect is fixed with fixture and official sandbox proof.
The authentication handoff is real; provider and v1 acceptance are incomplete.
Not checked: Real replies, cancellation, completed-account Stop/Resume and final
cleanup, TestFlight or physical-device acceptance. No broader portable suite or new
native build was needed for this TypeScript-only repair. G2 remains separately held.

```fort-review
{
  "format": 1,
  "reviewedCommit": "6c848f7d3f0a352a20acdde14b57831f93f2c1fb",
  "reviewer": "Codex primary implementation agent",
  "scope": "Private Codex empty MCP startup repair, public behavior regressions, official signed sandbox startup proof, admitted installed activation/paired cleanup and authentication handoff; no entitlement or permission change",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "New public empty MCP runtime and private-plan assertions fail before the fix. 561 focused account/startup/plan/chat/executor/service tests pass with five optional benchmark skips. Initial restricted run has 18 local IPC errors; normal IPC rerun passes.",
    "Strict TypeScript checking and compiled executor artifact build pass. Fourteen additional historical-caller/artifact fixture tests pass with one optional benchmark skip; no old packet was activated.",
    "Exact official Codex in a signed no-network fresh-state diagnostic first rejects invalid transport. Empty private MCP configuration then passes initialize and all effective controls, with no login/account/discovery/prompt request.",
    "Replacement report 0ab05d939bcbdc7eb1d0905156790a42a1db4375b5bb1ac63d6106cb95d563fc passes all 206 file checks, all 30 signatures/public leaf fingerprints, exact unchanged entitlements, strict nested signature and native full-package inspection.",
    "Explicit override admits Enable; first helper startup and pairing pass. Normal paired removal confirms shutdown/revocation/unregistration/credential removal and independent absence at 23:57:29. Settings sheet blocks Quit until Back/Done; then normal Quit passes.",
    "Installed replacement setup-again retains history, pairs and reaches official device challenge; verification page opened 00:06:29. Zero of six prompts; user authentication and final cleanup pending."
  ],
  "exceptions": [
    "Scoped implementation-agent semantic review, not independent review or whole-v1 acceptance. Baseline and earlier source/clock/rejection evidence preserved.",
    "Two no-network diagnostic harness issues were reconciled before relying on the decisive parser failure: missing process group, then EPERM signaling an already exiting process. Exact diagnostic process absence checked; no credentials or prompts involved.",
    "Shared CODEX_SETTINGS is also used by the guarded historical launch plan. Its fixture tests pass; active-server configuration remains refused by effective attestation. No historical packet or ambient credential was used.",
    "Current installed host/pairing remain active for user authentication. Device code was omitted from tools/evidence; account identity/selection, Astra catalog, real replies, cancellation and final cleanup are not complete. No G2 upload."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Transportless disabled MCP definitions reject fresh private Codex startup before initialization. Remove those named requirements and request an empty private MCP map. Retain refusal of active/unspecified servers and full inventory checks. Public red/green and official signed sandbox proof pass.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Installed Settings sheet prevents normal Quit until closed using Back/Done. Cleanup succeeds after closing it. Add an installed public regression and repair in a separate native slice.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Exact official authentication handoff reached, but personal account verification/selection, Astra discovery, six-prompt tests and final paired cleanup await the user step. Do not claim complete provider acceptance.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "e1e9c903feb7dc5ea73e096eaf56a895d66593e47d17889763d4a2e4fbbabf9d",
    "successor/APPLE-HOST-ACTIVATION.md": "52d3f5464bba6e96021f01d152029ab0102d8b7dcc1b1bf4db3467a5597bbc2f",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "511725613fb8533fbb4b337a190fd8656fa2e8f2fd899f3d6c0192ac49fbe222",
    "successor/V1-ACCEPTANCE.md": "9d9926f5677b465b3db2566e5966b0b9f2b4dbca31dacdd37c9250c983123718",
    "successor/evidence/2026-10-17-node-foundation/helper-self-inspection.md": "be044385c2b10e7cb0d82a97890a5a01d635d4e90416634749d1e577cd75e702",
    "successor/evidence/2026-10-17-node-foundation/installed-host-proposal.md": "e74d736f3bc08f25eaa489e3fe9aa430a894030131c784f7fda62430b98a4bfd",
    "successor/evidence/2026-10-17-node-foundation/installed-host-test.md": "5f418c6e7c586e4f3fa940f3b73ed6900fabd694a530769ae8ebe8507b857b27",
    "successor/runner/codex-controls.ts": "bb3fe03a5c2fcb87830b22e183e00622547a5c2e8debd514f7dc37784af96d06",
    "successor/runner/codex-startup-contract.test.ts": "9b928123b1c796f83a9538bb07ccddb7891fea670d5a9ed2ed22332aa01e7663",
    "successor/runner/private-codex-plan.test.ts": "1cba06249cd7b0aec5f337af324244fcefb1ad78032b70523d61b6f7ebb8cf7d",
    "successor/runner/private-codex-plan.ts": "55a0d05b3d5bb7e2eb40acfa0c4639eaca3dc8ded6746784a96be7e83f64d49e"
  }
}
```
