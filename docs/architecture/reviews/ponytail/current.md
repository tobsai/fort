What this repo does: Fort is a single-owner native messenger with a durable Node
core and optional runtimes. This slice exposes account operations to the primary
Mac owner; one optional phone has no account-management authority.

Codex self-review, October 18, 2026, base a522d2a. Read the complete new Swift account
DTO/storage/view/fixture modules and tests, the changed model/client/storage callers,
stop/disconnect/removal flow, existing owner file protections and service contract.
Traced data before/after awaits, retry deadlines, status recovery and code lifetime.
Reviewed final build/test logs, package input/output verification and all four exact
synthetic UI screenshots. The pinned whole-repository baseline is retained; this is
proportionate review, with no new dead-code or whole-tree assurance claim.

## Must fix

1. **Saved intent accepted fields outside the contract — fixed.**
   - What this is: The client restores an account operation after interruption.
   - Problem: Generic decoding ignored an extra field, weakening the exact saved-record boundary.
   - Fix: Strict key validation rejects additional/missing keys; the public model keeps the record and refuses login.
   - If skipped: A changed record could be silently adopted as the original request.

2. **Removal and re-pairing left conflicting account state — fixed.**
   - What this is: Confirmed disconnect and a later exact pairing share protected local storage.
   - Problem: Confirmed removal left the account intent; a historical confirmed receipt then blocked the new pairing.
   - Fix: Retire only the matching intent after confirmed cleanup, and distinguish the current connection from an older confirmed receipt. Keep unconfirmed/foreign records blocking.
   - If skipped: A user could disconnect successfully but be unable to sign in after pairing again.

3. **A late challenge returned after Stop — fixed.**
   - What this is: Starting sign-in and stopping local work can overlap across an await.
   - Problem: The held start response redisplayed its code after Stop completed; the public regression failed.
   - Fix: Stop/removal invalidate earlier account responses. Start/cancel recheck the captured generation before further effects; replies recheck before presentation. Status refresh does not run during lifecycle work.
   - If skipped: A canceled or uncertain operation could look active again after the owner stopped it.

4. **Actual provider and delivery acceptance remains open.**
   - What this is: Native, service, owned executor and optional protocol layers now exist.
   - Problem: Synthetic UI and unsigned packages do not prove the exact signed installed provider or TestFlight journey.
   - Fix: Verify Apple records/profiles and complete specifically authorized startup/network/provider actions, then actual installed and TestFlight acceptance.
   - If skipped: Engineering evidence would be mistaken for a usable delivered product.

## Should fix

5. **Concurrent suite load invalidated timing comparisons — resolved for this run.**
   - What this is: Existing functional journeys contain numeric performance assertions.
   - Problem: The ordinary parallel run exceeded two budgets although isolated reruns passed.
   - Fix: Run the full native suite with explicit --no-parallel and measure the account journey independently. All final assertions passed without changed limits.
   - If skipped: Shared CPU load could be mistaken for a functional regression or for a repeatable speed result.

The added fixture is DEBUG-only and has no network, Keychain, provider process or
startup composition. It uses the actual native controls with an explicitly synthetic
runtime and in-process transport. Its HTTP-shaped responses are not provider evidence.
No dependency, alternate launcher or credential store was added. Release exclusion is
checked separately. Mac/phone identity, provider and tool-sandbox boundaries stay exact.

Verdict: Native account behavior and exact synthetic UI are verified. Keep provider,
signed installed lifecycle, phone pairing and TestFlight acceptance open.
Not checked: Real accounts/credentials/provider calls, certificate changes, Apple remote
records/profiles, network/startup activation, physical devices, remote CI or TestFlight.
The current 200-file review package remains unsigned and unlaunched.

```fort-review
{
  "format": 1,
  "reviewedCommit": "a522d2a266efcd21366b82313fa2f6a5895e5c19",
  "reviewer": "Codex",
  "scope": "Native primary-owner account DTOs, protected intent, explicit start/status/cancel/retry, lifecycle response invalidation, DEBUG offline UI composition, tests, spec and evidence.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Final full native suite: 284 passed, 12 optional skipped, zero failed; Swift Testing reports 296 including skips. Explicit --no-parallel.",
    "Account focus: 13 passed and one budget skip; two factory tests passed after final QA wrapper. Initial public red cases fixed.",
    "20 isolated start/cancel samples after five warmups: p95 8.004708 ms under 100 ms; maximum 8.372083 ms.",
    "Fresh QA/Mac/iPhone Simulator/helper Debug and FortKit Release builds passed.",
    "Exact sandboxed QA AX/screenshots verified idle, challenge, canceled and cold-reopen unconfirmed. Link not followed. Exact app stopped; zero remaining processes.",
    "200 package outputs and 198 input hashes/modes/membership independently verified; zero profiles; exact public unsigned-activation refusal passed."
  ],
  "exceptions": [
    "Proportionate review; retained pinned baseline, no dead-code or fresh whole-tree claim.",
    "Initial full parallel run had two timing failures; isolated and explicit nonparallel full runs passed without threshold changes.",
    "Initial QA build lacked full signing; launch command erroneously continued after failed verification. Exact app stopped without account interaction; repaired ad-hoc sandbox build verified before subsequent launches.",
    "Actual provider, certificate/account changes, network/startup grants, remote CI, physical devices and TestFlight not performed."
  ],
  "findings": [
    {
      "id": "NATIVE-ACCOUNT-INTENT",
      "detail": "Additional intent fields now fail strict decoding and retain the protected record; observed public regression fixed.",
      "disposition": "fixed"
    },
    {
      "id": "NATIVE-ACCOUNT-REMOVAL",
      "detail": "Confirmed disconnect retires only its exact account intent; confirmed historical removal permits a later pairing while unconfirmed state blocks it.",
      "disposition": "fixed"
    },
    {
      "id": "NATIVE-ACCOUNT-LATE-STOP",
      "detail": "Delayed start challenge previously reappeared after Stop. Lifecycle generation invalidation and async rechecks now reject it; public red-to-green regression passed.",
      "disposition": "fixed"
    },
    {
      "id": "ACCOUNT-ACTIVATION",
      "detail": "Exact Apple records/profiles, authorized network/startup/provider actions, signed installed behavior and TestFlight remain unverified; ledger retains blockers.",
      "disposition": "open"
    },
    {
      "id": "NATIVE-SUITE-TIMING",
      "detail": "Parallel load exceeded two performance assertions. Isolated and explicitly nonparallel suites passed without altered limits; measurement context is recorded.",
      "disposition": "accepted-exception"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "2a9af51925af6d838e34d9a43da76c0664e7674d8a3f36e7e01797fc817f3d9f",
    "successor/V1-ACCEPTANCE.md": "fd8da64c00cbefac6a82c0b7f5a817f4458effb378dc6375bdda1adcf03dea42",
    "successor/apple/Apps/FortApp.swift": "5b961e02b0da9d720a82ecdc5746f8072ff8c089122c9428f4936bdbfefe07d2",
    "successor/apple/FortKit/Sources/FortKit/AccountSignIn.swift": "f1113cd0e8f63eb3a5be400cdf74cee9b205ece9e94aa8f64bd8436af0e5fab8",
    "successor/apple/FortKit/Sources/FortKit/AccountSignInFixture.swift": "3f95f4295b151eeaf58b045d5dcc1081be00c867b91888433716c0b609e1b228",
    "successor/apple/FortKit/Sources/FortKit/AccountSignInView.swift": "20e96b7dad49ae8e944f2e3c9d25af14b84f24b873343ff397cf14db6077592b",
    "successor/apple/FortKit/Sources/FortKit/FortClient.swift": "e67729a52565983dbe5a3ae2fd8eda94483819eab3a558985cc2df446d492a11",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "29f4baf9f677d5c052264bb7c15393aab3cc699070cd76a3e7959ffaf420094d",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupFiles.swift": "2d8c0a5742197ebe79272e0ed45e6ddbff8ba9cb98c778e9bdbbc2dc46517a5d",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "6319966a7facd4d68e0a572b92d1a973b9be578c85de445dda5b4ec9cd587243",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInFixtureTests.swift": "bd2f3572b12bef027a658d53ace4007318d85d0e49a154941d3d1723a3c0e34a",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "9b3b18efc4648047ed2d3a5eddd83022411d6d2fc47f3e3b336eff6c29851e40",
    "successor/evidence/2026-10-17-node-foundation/account-controls-canceled.png": "d24261078296304df4070052f9507d9d71262e76c1ceb9132ebbe931df30b7bb",
    "successor/evidence/2026-10-17-node-foundation/account-controls-challenge.png": "a510c05390103ca2ffc244ba796fba53b409877a5cd6493d567e018dd65f15ad",
    "successor/evidence/2026-10-17-node-foundation/account-controls-idle.png": "c3b8e69319324ad4d88bde444878a854e6a91e5eaa18d482e24b28ef3263cdb8",
    "successor/evidence/2026-10-17-node-foundation/account-controls-reopened.png": "7f5cd85b28bb9cb5ddcaaae38e0a8cc362cae32ed409eb234a37707b0e90d293",
    "successor/evidence/2026-10-17-node-foundation/mac-review-package.md": "4a21eb8ce0fab9f7b78c4218dea71119ef4c822127a9ed9b35d54ce73c479a06",
    "successor/evidence/2026-10-17-node-foundation/native-account-sign-in.md": "c7525fc8c5a48f0a4e475688b89517b09311f12c1cc06a18b503fa13c6d4f9ff"
  }
}
```
