What this repo does: Fort is a single-owner native messenger with a Node core and
optional official runtime adapters. This review covers composing ChatGPT plan
sign-in end to end in source, assuming one owner and no shared hosting.

Reviewed the new challenge kind against every consumer (executor wire, account
service, Swift status decoder and view), the sign-in method against the merged
account module, the executor module against the Codex executor pattern, and the
removal change against the existing Codex credential checks. Searched for other
AccountDeviceCode or removeCodexCredential callers; all were updated.

Must fix (resolved): the listener could stay open after a presentation failure
until the deadline; sign-in now aborts it in a finally block, with a test. The
first authorization-link limit (4096) could not fit the 4096-byte account frame;
it is now 2048.

Should fix (open): the helper network scope awaits the owner; temp-file leftovers
on crash; refresh form fields are undocumented; names are not forwarded.

Verdict: ready to commit as fixture-tested source with no entitlement change.
Not checked: Swift compilation (left to CI), real OpenAI, packaging, host
bootstrap selection, installed UI and TestFlight.

```fort-review
{
  "format": 1,
  "reviewedCommit": "a4152e82f0870b4c3ab6652d58aed0e10cd16cd1",
  "reviewer": "Claude Code (orchestration architecture thread)",
  "scope": "ChatGPT plan sign-in composition: browserAuthorization challenge (core and Swift), OpenAIPlanChat.signIn with fetch token transport and discovered signing keys, owned openai-plan executor, sign-in view text, and Mac removal of openai-plan.json. No entitlement, project, bootstrap or packaging change. Single owner; no provider contact.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Red first: contracts/account.test.ts failed on the browser challenge, then 12 of 12 passed.",
    "Red first: four sign-in tests in openai-plan-chat.test.ts failed on the missing method, then 12 of 12 passed.",
    "openai-plan-account.test.ts 7 of 7 and openai-plan-executor.test.ts 2 of 2 pass; the executor test was written after its module.",
    "Portable suite: 1448 passed, 15 skipped, 5 failed; the 5 are pre-existing synthetic-process and owner-demo tests that fail only in this root container and pass in CI.",
    "Strict typecheck passes under Node 26.5.0; spec structural and implementation gates pass.",
    "Not run here: Swift tests and builds (no toolchain; CI macOS job), real provider sign-in or turns, installed checks."
  ],
  "exceptions": [],
  "findings": [
    {
      "id": "PT-OAI-8",
      "detail": "Loopback listener chosen; the helper entitlement scope is corrected and awaits the owner, so no entitlement is applied.",
      "disposition": "open"
    },
    {
      "id": "PT-OAI-3",
      "detail": "Mac removal now deletes openai-plan.json beside auth.json under the same checks.",
      "disposition": "fixed"
    },
    {
      "id": "PT-OAI-5",
      "detail": "Signing keys now come from OIDC discovery restricted to the issuer origin; refresh form fields remain assumed standard OAuth.",
      "disposition": "open"
    },
    {
      "id": "PT-OAI-10",
      "detail": "A crash between temp write and rename can leave .openai-plan.json.<uuid>; removal does not delete it yet.",
      "disposition": "open"
    },
    {
      "id": "PT-OAI-11",
      "detail": "The account frame bounds sign-in to about 115 seconds of browser time.",
      "disposition": "open"
    },
    {
      "id": "PT-OAI-9",
      "detail": "Agent and recipient names are not forwarded in history items.",
      "disposition": "open"
    }
  ],
  "files": {
    "docs/architecture/decisions/059-node-foundation.md": "f5c887952a44cafdf37aa6ba72049febb31ec6e91bfcafaa35f428dcb2d53e15",
    "specs/059-modular-node-v1.md": "239cb39c5c3f044cc5d43afe8055bdbadce176db218920904ec0905971adc343",
    "successor/apple/FortKit/Sources/FortKit/AccountSignIn.swift": "9189596af9aeca9dfc0e28d03b392ce43caedc42a847a8073bf54778f4cc5296",
    "successor/apple/FortKit/Sources/FortKit/AccountSignInView.swift": "ea6cf178081604d87eb87771226b802bc6e74a135ce8542b8e7f30a484fe8658",
    "successor/apple/FortKit/Sources/FortKit/MacAgentStorage.swift": "3c715dbc2c15ac80d3f7bba0cd2e18c9cb256618aabaaaefacd9f4f18e2d8c5a",
    "successor/apple/FortKit/Sources/FortKit/MacHostControl.swift": "7efba423bccd8aa2b35e2a1d1ee4a6c162aa61a623728298dfdf41749b53b509",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "177b3020d9c6e233bce2a59afff39b1990ebd2861871cd4e53f98fb3b505db17",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "fb3ad694afaebf302b00d41c31ffebd9ca1df5f3eddf8a1a132b1b7921aca97e",
    "successor/apple/FortKit/Tests/FortKitTests/MacAgentStorageTests.swift": "b191e14e04f57998f5e1df080589df4a9492360d53151c04175b5cdf112f8e37",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostRemovalTests.swift": "ae47d6a98c6a82a820880e6a152776ad47b7de5232b0809f28cbb1c4ee3ca9be",
    "successor/contracts/account.test.ts": "95d7da4c6a91402c58e77e5885ae90f0928752ab874e5ac2bc8628161cb21f87",
    "successor/contracts/account.ts": "ba03ed02ad009546d0908215d23b6a687502dc128e301d53fee012a5a8fdb31b",
    "successor/contracts/v1.ts": "cdee302bf50b2b12d127d33a40f31dd8b4d925888cd5210aa3a61998f8a9799f",
    "successor/runner/fixtures/openai-plan-executor-worker.mjs": "f5fdab1562d94a4f9e5916c8944c89a8eebca49d2004b8196936ff986b4a2074",
    "successor/runner/openai-plan-account.test.ts": "913ac6b58fa4da04edf0b20880da43e5742ef4a1e97260a24f1360e8865d2af8",
    "successor/runner/openai-plan-account.ts": "005558f727e3adc416e8eccbc56cd288ce45d9a0668acaf5effac00a4dc2cf8e",
    "successor/runner/openai-plan-chat.test.ts": "34ab6517e7c97e042b9b19ae6e6f706c33378530cbcdcdcad4e8b645b1c0ead7",
    "successor/runner/openai-plan-chat.ts": "7da0614b0a195f155df44df68ffa82593518ecb05eb04364b45df59478a88d2d",
    "successor/runner/openai-plan-executor.test.ts": "43d8e648622c9b558a6812bf5bd00c29dd69d483e18c4f4572d96a3708e41581",
    "successor/runner/openai-plan-executor.ts": "a4d53e660751493d1d85894f5a9d065212666b64233fad18e0450ee9881d20b0"
  }
}
```
