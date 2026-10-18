What this repo does: Fort is a single-owner native messenger with a local Node
ledger and optional runtime adapters. This slice exposes offline history maintenance
only through a separately launched, owned private process. Expected load is one owner,
one bounded command and the existing capped local ledger.

## Semantic review

Codex self-review, October 18, 2026, base c28720d. Read the pinned Ponytail skill
and preserved baseline. Reviewed every changed source/test/configuration, the ledger
transaction and format guards, native framing/cancellation/process ownership callers,
compiled entry and resource manifest construction. Traced exact identity, parent EOF,
malformed controls, lost replies, retry, recovery epoch and process-group absence.
Checked that ordinary HTTP/XPC/core dispatch receives no deletion route. Reviewed
final staged evidence against raw test logs and the fresh artifact manifest.

## Must fix

1. **Owner-facing host history deletion remains incomplete — open.**
   - What this is: The maintenance command can now delete a disconnected synthetic ledger.
   - Problem: Production has no durable captured confirmation linked to completed
     startup/host/executor/credential absence and client cache cleanup.
   - Fix: Compose and test that native coordinator before enabling the control.
   - If skipped: Passing a private command test would be misrepresented as product deletion.

Verdict: The bounded source and compiled-fixture slice passes; continue native
composition. No new dependency or alternative process manager was added. The final
full portable rerun includes the prior ledger's two last format guards.
Not checked: actual owner history or accounts, App Group/profile eligibility, installed
UI, provider behavior, physical iPhone, signed apps, TestFlight or remote CI. New optional
compiled maintenance was explicitly enabled locally. No release exception is granted.

```fort-review
{
  "format": 1,
  "reviewedCommit": "c28720dec6ae89390fad8e274f618c22161d3bae",
  "reviewer": "Codex",
  "scope": "Bounded offline history-maintenance entry, exact Swift request/receipt and private guarded-process integration, packaging graph and final portable/native regression evidence.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "NATIVE-HOST-DELETION",
      "disposition": "open",
      "detail": "Private compiled maintenance now passes, but durable native confirmation and completed lifecycle/credential/cache composition remain before enabling production deletion."
    }
  ],
  "tests": [
    "18 controls/packaging passed; one optional measurement skipped.",
    "Strict TypeScript and Swift package build passed.",
    "Compiled native synthetic deletion/retry/reopen/old-epoch denial passed; PID/group absent; 125.127333 ms within 5000 ms bound.",
    "Final portable: 1158 passed, six optional skips. Final native: 253 executed functions passed, ten optional skips.",
    "94 payload files and manifest hashes verified; no new application assembled."
  ],
  "exceptions": [
    "No real data, credential, startup, provider, network or installed-product invocation.",
    "New optional compiled journey was explicitly enabled locally; remote CI execution and release configuration unverified.",
    "Single integration timing is not a percentile benchmark; earlier isolated ledger benchmark remains separate."
  ],
  "files": {
    "specs/059-modular-node-v1.md": "0cbead9b2f2a54b53ea067d7c360466a8baf4476430364f5b075dcffef8ebf90",
    "successor/V1-ACCEPTANCE.md": "667475d87f77ff48855fe34e20afc82233fc9ab2d3800cb0157c12e154d88116",
    "successor/apple/FortKit/Sources/FortKit/MacCoreProcessTransport.swift": "91a895171a1ee45d126a22c5a45f146e3add6547e082085d6abc66711a99c2d3",
    "successor/apple/FortKit/Sources/FortKit/MacHistoryDeletion.swift": "699751d28374cd169fa38df2568967df2804246638dc279f77f1eeaf2abe1fbd",
    "successor/apple/FortKit/Tests/FortKitTests/MacHistoryDeletionTests.swift": "62e47f9e8cf6dfec64de0ba784b17ae05df15aeceeadc73a81947fa41fe07e37",
    "successor/evidence/2026-10-17-node-foundation/history-maintenance.md": "1ccc1e3fc0288ae05671344164b9c26d11d4be8b8c4b30a29656467f43e03719",
    "successor/host/history-maintenance.mjs": "a12434cebc87aca7e64ca6ff7c5f9c19190ce844c65bae4e313c3d906d97397b",
    "successor/host/history-maintenance.test.ts": "fc9cfd73fa9bfcfb15cfded2376653c46d0cdecd302329152db0535c22c1acd6",
    "successor/host/history-maintenance.ts": "98ea24d6700df634f8f3c3a411d4cacb6a8758a52c0f79811455fcc67f186c2a",
    "successor/packaging/core-artifact.ts": "ad29c5a9610f15e2034c602f5fde127552c1119c250620085ebdcf084b272465",
    "successor/packaging/host-resources.test.ts": "fd6ee2fe1ec379313a6dd099f413643c52d584e97a4248f563df6ed56f41e639",
    "successor/packaging/host-resources.ts": "9e7d7e1c4f1d16f8ec64b0ea79970efde4a219dbb711b409cafa8423aab71cc1",
    "successor/tsconfig.core.json": "a9f429083cccc1da962adaabe1f1d3bf978c5d1f177bb7a06d5eb6b39d18853f"
  }
}
```
