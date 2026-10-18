What this repo does: Fort is a single-owner native messenger with a durable Node
core and optional runtimes. This slice prepares one phone companion for an exact
Mac host, with no shared hosting or automatic identity fallback.

Codex self-review, October 18, 2026, base 83effc5. Read the complete new preparation
coordinator and tests, versioned connection decoding, credential helpers, join/leave
client checks and changed model/owner/account/history callers. Traced the protected
journal before every effect and after awaits, interrupted token preparation, uncertain
join, exact recovery, cancellation/forgetting and partial cleanup. Reviewed the old
removal reconciliation and expired-phone authority against the service's companion
contract. The whole-repository baseline is preserved; no fresh whole-tree or dead-code
claim is made. The parked source was selectively restored without replacing newer work.

## Must fix

1. **Pending setup could overwrite unrelated recovery state — fixed.**
   - What this is: A new phone setup shares the protected local journal directory.
   - Problem: The parked code generated a credential and joined despite an older unconfirmed removal. A competing owner setup also contacted its host and published another connection.
   - Fix: Require exact confirmed historical removal or matching current cleanup; block competing owner setup before host contact and recheck after awaits. Both original regressions now pass.
   - If skipped: One flow could hide another unresolved grant or change the user's selected host.

2. **Other operations ignored a pending phone or account flow — fixed.**
   - What this is: Owner setup, account setup and saved-history removal share that directory.
   - Problem: New public regressions showed overlapping account/phone work and history deletion during phone preparation.
   - Fix: Reciprocal guards reject these operations before credentials, requests or deletion. Existing account, owner and deletion regressions remain green.
   - If skipped: Setup/removal truth could diverge between independently valid operations.

3. **Unknown phone journal fields were silently accepted — fixed.**
   - What this is: Lost replies restore the prepared journal and credential.
   - Problem: An added field was ignored and the changed record was adopted; the public test observed extra requests and a published connection.
   - Fix: Strict key decoding rejects changed schemas before recovery requests and preserves the record. Late-response byte checks protect already-running work.
   - If skipped: Changed recovery evidence could be treated as the original intent.

4. **An expired phone lost its leave authority — fixed.**
   - What this is: Ordinary status reads may be denied after pairing expiry.
   - Problem: The model could no longer identify the saved connection as a companion or request exact leave.
   - Fix: Schema 4 retains companion role only for HTTPS. The model permits only that phone's leave recovery and rejects contradictory role responses; host/account controls remain unavailable.
   - If skipped: Disconnect recovery would depend on a still-valid ordinary status read.

5. **Ordinary phone setup is not composed yet — open.**
   - What this is: The coordinator and public model recovery are now available.
   - Problem: The normal connection/settings UI and Mac invitation/secure-listener lifecycle still need composition.
   - Fix: Add the explicit import/retry/recovery/revoke controls and exact Mac invitation/lifecycle path, then inspect their synthetic UI before real device acceptance.
   - If skipped: Passing coordinator tests would be mistaken for usable phone setup.

6. **Actual device and delivery acceptance remains open.**
   - What this is: Real use needs exact Apple, Keychain, HTTPS trust and provider authority.
   - Problem: These builds/tests use disposable memory credentials, protocol fixtures and unsigned clients.
   - Fix: Continue authorized source composition, then perform only approved credential/trust/startup/provider actions and verify installed physical/TestFlight behavior.
   - If skipped: Fixture success would overstate privacy, lifecycle and delivery acceptance.

No new dependency, provider path, process launcher or credential store was introduced.
The existing guarded private-file lease owns atomic writes, mode/owner/ACL/link checks
and durability. Current preparation adds one bounded journal; no code or token is
written to it. Confirmed history is kept unless separately removed. No lean finding.

Verdict: Phone preparation and exact recovery are verified synthetically. Complete
ordinary UI/lifecycle composition and external acceptance before calling it usable.
Not checked: Real Keychain, TLS identity/trust, LAN, startup, provider/certificate/account
actions, current installed phone UI, physical devices, remote CI or TestFlight. The
prior Mac review package is preserved at 83effc5 and was not relabeled or activated.

```fort-review
{
  "format": 1,
  "reviewedCommit": "83effc5435a29abd6c910078f1558989a4960add",
  "reviewer": "Codex",
  "scope": "Durable native companion preparation and exact recovery, role-preserving connection schema, reciprocal owner/account/history isolation, expired-phone leave authority, tests, spec and evidence.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Final native suite: 300 passed, 13 optional skips, zero failed; Swift Testing reports 313 including skips. Explicit --no-parallel and current compiled resource inputs.",
    "Prepared-phone/schema focus: 15 passed, one benchmark skip; expired-phone/account/companion regressions also passed together.",
    "Observed public red cases: unconfirmed older removal, competing owner/account/history work, changed journal fields and expired-phone leave. Corrected runs passed.",
    "20 serial prepare/lost-reply/recover samples after five warmups: p95 30.543625 ms, maximum 31.051708 ms, under 100 ms.",
    "Fresh unsigned Mac/iPhone Simulator Debug builds and FortKit Release compilation passed; complete file inventories recorded."
  ],
  "exceptions": [
    "Proportionate review with the existing pinned baseline retained; no fresh whole-tree or dead-code assurance.",
    "No actual credential, trust, LAN, startup, provider, certificate, physical device or TestFlight action. Simulator execution blocked by existing CoreSimulator version mismatch.",
    "Public coordinator/model foundation only; ordinary pairing UI and Mac invitation/secure-listener composition remain open. Previous source/fixture/package milestones remain distinct."
  ],
  "findings": [
    {
      "id": "PHONE-OLDER-REMOVAL",
      "detail": "Unconfirmed historical removal and competing primary setup initially allowed effects. Exact receipt reconciliation and owner guards now reject before host/credential effects.",
      "disposition": "fixed"
    },
    {
      "id": "PHONE-CROSS-OPERATION",
      "detail": "Pending phone/account/history operations were not reciprocally isolated. New public regressions pass with guarded shared records.",
      "disposition": "fixed"
    },
    {
      "id": "PHONE-JOURNAL",
      "detail": "Unknown saved-plan fields were silently adopted; strict decoding now rejects before recovery requests and retains evidence.",
      "disposition": "fixed"
    },
    {
      "id": "PHONE-EXPIRED-LEAVE",
      "detail": "Expired phone restore lost companion identity. Versioned role now permits only exact leave recovery and excludes host/account authority.",
      "disposition": "fixed"
    },
    {
      "id": "PHONE-COMPOSITION",
      "detail": "Ordinary invitation/preparation/recovery UI and exact secure-listener lifecycle remain to compose; acceptance ledger records this next step.",
      "disposition": "open"
    },
    {
      "id": "PHONE-EXTERNAL-ACCEPTANCE",
      "detail": "Actual Keychain/TLS/trust/device/installed-provider/TestFlight verification remains separate and unperformed.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "7556f775e054c2b5afdaeb4a8c4d063c9a3c8ef1637d6ea855dbef1285b4b562",
    "successor/V1-ACCEPTANCE.md": "513d75661ca72b4392d4e10a5cd53de7e524c3934bfe9876842fc2ffd707db4c",
    "successor/apple/FortKit/Sources/FortKit/AccountSignIn.swift": "44fc7cba2ed8a995a9d7567efa59862c0da5429634c31f28ed07b56dcea6c883",
    "successor/apple/FortKit/Sources/FortKit/CompanionSetup.swift": "829c9a1ef7afa30ec81b29d031ce4ec9d7f9144754d4e822b126240fabe4335e",
    "successor/apple/FortKit/Sources/FortKit/DeviceConnection.swift": "beb283860a0768dab035951ed2ecb4b141bfc9daebf310bb2809a3d0cfbacc79",
    "successor/apple/FortKit/Sources/FortKit/DeviceStorage.swift": "c3866f0e512b4867504e87252a3eb8ee8696a830ca2ac3502485e43925a72c33",
    "successor/apple/FortKit/Sources/FortKit/FortClient.swift": "ec62260d1e1ee471f83b83240a810f4969fd8c39d4d970775575a65baa47ae2d",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "5cf4e517319e53de94826627d675b480ffe3f382a2b7930b0b75c6d487ce4d93",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupFiles.swift": "1d8571079e5c21f228426d4b58e552d14c7ba21888f8a2f46fe327499f57d74d",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupPreparation.swift": "d648f94478e09d0603db8d56222e4aa5e8a6a4270b88a02556127a089c918465",
    "successor/apple/FortKit/Sources/FortKit/SavedHistoryRemoval.swift": "d03183e71305f1627c817937fd45571d43f0dd31ba3b06bedc12abb8b813f4de",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "0d2ee2ee249c8365c0f7e02f8f425ced0e2995cd833a3c02d546be6151c8bf4b",
    "successor/apple/FortKit/Tests/FortKitTests/CompanionLifecycleTests.swift": "d80e10ec2af82efa83f494d9340bd383a5060458663e43b2ffc140b88f7472b9",
    "successor/apple/FortKit/Tests/FortKitTests/CompanionSetupTests.swift": "028c25045f0b05f6de0553993c5403886ec465944389594fe9f60f9028d3c18b",
    "successor/evidence/2026-10-17-node-foundation/native-companion-setup.md": "e273cc3d584ae322f3011e1ee5f64a1661cd3e4372e7210f3829ddeda84d2376"
  }
}
```
