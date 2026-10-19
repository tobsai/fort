What this repo does: Fort is a single-owner native messenger with a Node host and
optional official runtimes. This review covers the installed-test evidence and
handoff only, with no product source or permission change.

Reviewer: Codex primary implementation agent. Read the constitution, spec workflow,
pinned Ponytail skill and baseline. Compared all seven changed evidence/spec files
with the exact candidate manifest, verification output, installed AX state, normal
Quit result, absence metadata, and both automatic approval rejections. Prior
implementation reviews remain in Git history.

## Fixed

1. **The handoff described stale installation and time status.** The candidate
   is now installed, and the owner extended test time as needed. The current notes
   preserve the original clock as history and report the actual activation-review
   block. They retain the six-prompt total, exact personal account/model choice,
   permission scope and cleanup requirement. No prompt was sent.

## Remaining gate

2. **Automatic review rejected persistent host activation.** It did not accept
   the recorded later approval as trusted authorization over the initial prohibition.
   The first action and reconsideration both stopped before execution. The requested
   explicit confirmation remains pending; no alternate execution path was attempted.
   All repaired startup/provider/paired lifecycle acceptance remains unrun. The app
   is installed and stopped, with independently confirmed job/process/storage absence.

Verdict: Evidence matches the observed inert installation and unresolved approval
block. Time and artifact changes are not presented as new approval requirements.
Not checked: Product tests/builds were not repeated for documentation. No helper
startup, pairing, sign-in, provider prompts, TestFlight or physical acceptance ran.
G2 CSR upload remains separately held.

```fort-review
{
  "format": 1,
  "reviewedCommit": "bfb79a1e32aef02c01c6b04b221df34a8277173b",
  "reviewer": "Codex primary implementation agent",
  "scope": "Extended test-time authority, verified repaired candidate installation, automatic activation rejection, normal Quit and absence evidence only; no product source changes",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Reverified 206 file records and 30 executable signatures/public certificate fingerprints, exact entitlements, strict nested signature and native full-package inspection for report b887864ad524a4cd312130b1f55cc2eb7abfec666a19dcaeb50ad29541fbecae.",
    "Installed exact repaired candidate, preserving cleanup-only app. AX showed host not enabled and Connect disabled. Enable rejected before execution; reconsideration also rejected. No workaround.",
    "Normal Quit passed. At 23:49:32 UTC exact launchd job, owned processes, host storage and provider credential path were absent; native storage only empty 0600 setup lock; all 206 installed records still matched. Zero pairings/sign-ins/prompts.",
    "Full structural spec check, accepted implementation gate and diff whitespace check pass. Product behavioral tests/builds not rerun because no product bytes changed."
  ],
  "exceptions": [
    "Scoped implementation-agent semantic evidence review, not independent review or v1 acceptance. Baseline and prior implementation reviews preserved.",
    "Trusted continuation reports the owner time extension and earlier approval references; source-thread read returned no turns. Automatic approval review rejected prior file/assistant records as authorization and remains binding pending explicit confirmation. No implied override.",
    "G2 CSR upload remains separately held; no provider authentication handoff, startup/XPC, real-provider or paired lifecycle acceptance occurred."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Current handoffs incorrectly continued to describe the candidate as uninstalled and time as the blocker. They now distinguish the owner time extension, actual inert installation, automatic activation rejection and verified absence. Historical clocks and source evidence remain intact.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Automatic approval review rejects persistent activation based on the initial startup-service prohibition and treats recorded later approvals as untrusted. Explicit installed-test confirmation was requested. No activation or provider acceptance can be claimed.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "b8a1e324bcafdf0127a19890a02b65896c8bfa5b3bdd3a658c276c0f61ea2d49",
    "successor/APPLE-HOST-ACTIVATION.md": "0fb8762870ed21accee655ec97ff5397dce99df41e99309cf37303ac4a36e52d",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "921d8456db61f1957da0805dece088cdeb5d248af7ae66c2dbc0be2f4e56198c",
    "successor/V1-ACCEPTANCE.md": "3e8701028c3e8e703c1c28c71e37bf9bee4a13ecf57d0141c6f59739e1228511",
    "successor/evidence/2026-10-17-node-foundation/helper-self-inspection.md": "3a062a71202a552e7880a5d7627be1699fb6d1935700fb0c2a8c349b880d8f49",
    "successor/evidence/2026-10-17-node-foundation/installed-host-proposal.md": "a230b2677cd2cdd0207fb72eb737f52a817d52fcc3da5267369cd104d7aaf00c",
    "successor/evidence/2026-10-17-node-foundation/installed-host-test.md": "05ea526de76cdc11bf38d620fe1e4042ff0deabe51637eee964057cbdce3d493"
  }
}
```
