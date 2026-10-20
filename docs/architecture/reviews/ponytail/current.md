What this repo does: Fort is a single-owner native messenger with a modular Node
core. This review assumes one owner and the existing private ledger/runtime boundary.

Reviewed all changed source/tests and callers in the account routes, metadata
transaction, owner scope, Codex account validation, native reconciliation, editor,
compiled-host journey and runtime migration. The existing baseline supplies the
whole-tree map. No dependency, route, permission or provider fallback was added.

Must-fix findings were corrected: repeated confirmation discarded the prior choice;
legacy re-pairing lost that choice; a lost reply retained a stale selectable catalog.
Each has a public behavioral regression. Explicit new selection replaces one stored
preference atomically; preference alone cannot authenticate or authorize a request.
The owner scope remains exact, including after a new pairing device is issued.

Reviewed the durable live result/cleanup receipt separately from source tests.
Both requested 6.1 turns completed; effective model is unreported. Original history
is unchanged and local startup/pairing/provider credentials are absent. Package and
installed proof for this source update must be recorded separately after the commit.

Verdict: Source slice passes its behavioral gates; ready for the authorized signed,
disconnected app update. No further provider prompt or sign-in is required.
Not checked: independent effective-model attestation, new-source live reauthentication,
physical iPhone, TestFlight/G2, remote grants and unrelated historical donor modules.

```fort-review
{
  "format": 1,
  "reviewedCommit": "6f32f576d539ce1fd2824c0c2f2f850c3323da2b",
  "reviewer": "Codex primary implementation agent",
  "scope": "Exact-account preference reuse, legacy owner re-pairing, model catalog eligibility and completed installed 0.163.0 smoke/cleanup evidence; single owner",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Service public HTTP: 37 passed, 2 optional benchmark skips; observed exact-match and legacy-re-pair red/green.",
    "Portable final: 1417 passed, 16 optional skips. Strict TypeScript check passed.",
    "Native final: 345 passed, 16 optional skips, including compiled-core owner re-pairing and lost-receipt catalog invalidation. Release Mac build passed.",
    "Installed prior migration build: actual catalog and two selected gpt-6.1-sol completions; Quit/relaunch preserved first reply and pairing. 2/3 prompts used.",
    "Independent closed-ledger inspection: original messages/runs unchanged, two completed selected-model records, zero devices/enrollment/outbox. Startup/provider auth/client connection absent and owned host/runtime processes exited."
  ],
  "exceptions": [
    "Primary-agent semantic review, not an independent audit. Whole-tree baseline remains preserved.",
    "Two live turns used the prior installed d8d76bc3 migration build. Account-reuse repair has synthetic proof; disconnected signed update remains pending at this source commit.",
    "Effective serving model remains unreported. No new provider access is needed or authorized by this update.",
    "Physical iPhone, TestFlight, G2 and remote provider closure remain outside this check."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Reauthentication discarded an explicitly confirmed workspace and caused redundant selection. Persist one scoped choice; reuse only fresh exact account ID and label, leaving missing/ambiguous/changed/stale/canceled results unselected.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Legacy preference retention incorrectly required the old pairing device ID, losing the choice after owner re-pairing. Retain within the same installation/recovery/provider/host scope; fresh current-owner operation and runtime identity remain mandatory. Public red/green passes.",
      "disposition": "fixed"
    },
    {
      "id": "3",
      "detail": "A lost reauthentication reply left the previous catalog selectable. Account-status transitions now invalidate cached eligibility; native red/green passes. Refresh also exposes its prerequisite and unchecked saved models are labeled accurately.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "45155bc5f9f2a68ac55f6a7f46a639d0e71ceb5fb6e65b17db38289d08482b8d",
    "successor/APPLE-HOST-ACTIVATION.md": "79068b441223d25e5c5ae7a074a14b715a2b246596299a0cb8f63d12b577301c",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "6dbdc0dac83d99ed3a83896738194b7ea78944dbe20c5eaa54943f9a10bdf108",
    "successor/V1-ACCEPTANCE.md": "c05124ab93a39ac9d848e9a24cd84cb8842ed8b00fbb92199a91bb99df631cd8",
    "successor/apple/FortKit/Sources/FortKit/AgentSettings.swift": "7f131f71bae083195a930499b07981fbfa093a223af69848b3814a921fea533a",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "734fab58c612cb5fcd533c2a2c950b610b6e52a9ddceb1d80c5eaf053f40e229",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "f0fe462f781ee19d9b8a7fcc36f6cf16dfe32fcd3aa0627e4ca6b1d8379008d3",
    "successor/apple/FortKit/Tests/FortKitTests/AgentSettingsTests.swift": "7661638cf90841c7a7425f66ada01b0239331fd897a62274586395fd5ff8cf4b",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift": "62b4dd8452f6efdb8dcc3911ed61e686ab94b503a1ae1416a81396594cb84bee",
    "successor/evidence/2026-10-17-node-foundation/runtime-migration-0163.md": "468e3e9b93b568d9340a33739fc0867b0bafdc672459e45cbc58cc50185626cb",
    "successor/service/account-sign-in.test.ts": "8e9c7263448deb2aad05e440b4ea2f47d47e32ab4430664f600deb2eeaa825ee",
    "successor/service/account-sign-in.ts": "b094f2925d91febc7c8e877dac2cd9e43530a28ba43dd2fe87db6db5cf9e15cb",
    "successor/service/openai.ts": "80b5e5f0ac544c36cdcbe97e248600734d12b8c5291d0d31e7085e63e07c8211"
  }
}
```
