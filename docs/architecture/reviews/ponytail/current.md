What this repo does: Fort is a single-owner native messenger with a durable Node
core and optional runtime adapters. This slice carries explicit device sign-in
through the existing owned process lifecycle, for one owner and one active executor.

Codex self-review, October 18, 2026, base d69cfb3. Read the changed contracts,
request/progress/result parser, shared exchange, worker, Codex adapter and fixtures;
traced exclusivity, cancellation, output bounds, cleanup and completion races.
Constructor validates the pinned Codex version; direct chat rejects busy state.
Read every new behavioral test and final validation logs, checked artifact exports
and independently verified resource hashes. Retained the whole-repository baseline.
Applied the pinned Ponytail method as a report; fixes were separate test-first work.

## Must fix

1. **Worker consumed its cleanup reserve — fixed.**
   - What this is: Account work and process cleanup have separate deadlines.
   - Problem: A 300 ms pending fixture initially took 5,310 ms and lost confirmed cancellation.
   - Fix: Abort runtime work at its own deadline, retaining five seconds for cleanup.
   - If skipped: Ordinary expiration can force termination and lose the truthful result.

2. **Caller mutation could redirect the challenge — fixed.**
   - What this is: The operation accepts a display callback.
   - Problem: Changing the supplied options after start replaced the code recipient.
   - Fix: Capture the original callback at both public operation boundaries; regressions pass.
   - If skipped: An ongoing request could display its code through a changed destination.

3. **Usable native sign-in remains incomplete — open.**
   - What this is: The owned runtime capability now works through authored processes.
   - Problem: Primary-owner service start/status/cancel and native controls are not composed yet.
   - Fix: Implement those seams with exact identity, durable lost-response recovery and tests.
   - If skipped: A packaged capability would be mistaken for an accessible product flow.

4. **Real activation and delivery remain unverified — open.**
   - What this is: Signed Mac operation requires exact Apple and provider prerequisites.
   - Problem: Profiles, helper network authority, real account operation and TestFlight are unverified.
   - Fix: Continue independent source work; verify existing access and obtain only required concrete grants.
   - If skipped: Fixture acceptance would be mislabeled as provider or installed acceptance.

The shared executor slot and cleanup mechanism avoid a second launcher. Progress is
bounded, correlated, copied and presented once; no challenge survives cancellation
or terminal completion. Forced/uncertain process cleanup cannot claim success.
No dependencies, browser opening, credential storage or persistence schema were added.

Verdict: Owned account protocol/composition passes synthetic regression; finish the
primary-owner/native flow before proposing actual account activation.
Not checked: Real provider/account, new credentials, network entitlement, startup,
signed installation, Apple records, remote CI or TestFlight. Native artifact unchanged.

```fort-review
{
  "format": 1,
  "reviewedCommit": "d69cfb3e72247c1a7ccd3ff976f3f9cbbc3f4d8b",
  "reviewer": "Codex",
  "scope": "Owned optional account operation, worker protocol and guarded Codex composition; spec, evidence and ledger.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "1259 portable passed, zero failed, 11 documented skips.",
    "129 focused passed, four skips; 35 guarded Mac passed, three skips. Strict TypeScript passed.",
    "20 isolated process journeys, five warmups: p95 93.669167 ms under 2000 ms.",
    "102 resource files independently hash/size/membership verified.",
    "Missing capability/composition, work deadline and mutated callback regressions observed red and fixed."
  ],
  "exceptions": [
    "Proportionate review; whole-tree baseline preserved. No dead-code claim.",
    "Synthetic authored processes only; no real provider, credentials, installed UI or TestFlight.",
    "Core has historical service/database names; optional provider implementation remains outside it.",
    "Native review app unchanged; new resources are separate artifact."
  ],
  "findings": [
    {
      "id": "OWNED-ACCOUNT-DEADLINE",
      "detail": "Worker initially waited through cleanup reserve; work-deadline abort now permits correlated cancellation and clean exit.",
      "disposition": "fixed"
    },
    {
      "id": "OWNED-ACCOUNT-CALLBACK",
      "detail": "Mutating callback options could redirect challenge; original callbacks captured at both public boundaries.",
      "disposition": "fixed"
    },
    {
      "id": "ACCOUNT-SERVICE-NATIVE",
      "detail": "Primary-owner service action, durable lost-response status and native controls remain required for usable sign-in.",
      "disposition": "open"
    },
    {
      "id": "ACCOUNT-ACTIVATION",
      "detail": "Apple profiles, exact network/provider/startup authority, installed behavior and TestFlight remain unverified.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "89a139c714801206c5de8d431f78c0e44d5cf258e6c6ff5a97e5a162e238866d",
    "successor/contracts/v1.ts": "c7530f2f1a8d7f4d3afdb3b1bbc2551d770c4155a1c61ff1a009a0f318e93fcc",
    "successor/runner/chat-runtime.ts": "0377046b395338212b4aa73fe613075d8c832ff6ce61e577cc029449a9acf5a5",
    "successor/runner/codex-account.test.ts": "e24416e43fab8bff480268e7245d7ff40a026dc5064e7cd955b365642b583370",
    "successor/runner/codex-account.ts": "c567718e6f68701fb6e6f68ad1889dfb1b6020966b8dd150d29317b2af6fe91e",
    "successor/runner/codex-chat.ts": "d266b2c572ec792933425daa2cd3d28d3b40e646650fd7b4b0759acc84f4e6b9",
    "successor/runner/codex-executor.test.ts": "6e8573d04f410efabcb70f24f7fa9670d249d15b7143d557b0ee051e1c9b2e4e",
    "successor/runner/codex-executor.ts": "e91f76118a5951fec32a57ef884c0f4efe6907b56dd070b12dbb065b5950bf5c",
    "successor/runner/executor-process.ts": "1c18babbba36152358258849a11c3bf0f078589555398c3327eba25bf6693e7b",
    "successor/runner/executor-worker.ts": "5a873564c855620411a13d21be69eaee55957686fc79eb2ddf57e834c4ce4f5c",
    "successor/runner/fixtures/codex-executor-runtime.mjs": "aca2e714657ecc5cfef92ff9774dded5cda8e0f35b5cd88656447827e044b2ac",
    "successor/runner/executor-account-wire.ts": "588ffbe5c276d3cb2a4ed8b448d91141bdf6d86cbc6caf35cbec47f28c70f6cf",
    "successor/runner/executor-account.test.ts": "30c71296d1f2629ac922a5693561bcc9a9f4f8d4abea53c96902ecec3148899b",
    "successor/runner/fixtures/executor-account-worker.mjs": "dc26d733e2a67ce8e497f8dcbafb37fc4c9ab0b4869d3fef624c7cfef8c2a064",
    "successor/evidence/2026-10-17-node-foundation/owned-provider-sign-in.md": "5c58892e71b3f6b4b63944cbdc2b0d3dd8336c639a5ec11889d27abb2be10d38",
    "successor/V1-ACCEPTANCE.md": "40fd8517674db60577f902c3c96d3ccba35d089722d3f16b540d8c4b066f0833"
  }
}
```
