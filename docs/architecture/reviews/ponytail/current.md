What this repo does: Fort is a single-owner native messenger with a durable local
ledger and replaceable runtime. This slice permits one explicitly invited companion
phone while preserving the primary Mac's control identity. Review assumes one owner
and two devices, with no shared hosting or enabled network service.

## Semantic review

Codex self-review, October 18, 2026, base 3739ac9. Read the pinned Ponytail method and
preserved baseline. Reviewed the complete companion module and public HTTP tests,
changed contracts/routes, authentication, transaction/event ordering, owner setup,
expiry, runtime stop/restart, offline copy/deletion and compiled artifact callers.
Inspected final source and test diffs and independently verified all core payloads.
The existing owner decision covers this bounded preparation; the spec explicitly
keeps actual credentials, network/startup and provider activation separate.

## Must fix

1. **A phone could inherit primary host controls — fixed.**
   - What this is: Existing authenticated owner routes assumed a single device.
   - Problem: A second device could restart/stop the host, or be selected as the
     primary identity when the Mac expired. Disconnect initially left it authorized.
   - Fix: Preserve the exact primary identity, limit companion authority, require
     local recovery for an expired primary, and revoke both only after confirmed stop.
   - If skipped: Phone actions would change Mac ownership or falsely claim all access revoked.

2. **Disconnect and replacement lacked exact retry boundaries — fixed.**
   - What this is: Join/leave requests can commit before their response is received.
   - Problem: Missing receipt recovery, invitation-as-device-token reuse and a retired
     expired slot either weakened credential separation or stranded replacement.
   - Fix: Retain one digest-only record; exact retries recover only the same result.
     Separate invitation and device secrets; clear retired records during new primary setup.
   - If skipped: Lost responses could cause credential replacement or ambiguous removal.

3. **Pairing state changed without notifying clients — fixed.**
   - What this is: Clients use durable replay invalidations to refresh saved projections.
   - Problem: Invitation/membership/expiry changes left the event cursor unchanged.
   - Fix: Append existing workspace events within the same transaction; exact retries
     and unchanged no-expiry selection emit none. The observed regressions pass.
   - If skipped: Other devices could show stale pairing state until an unrelated refresh.

4. **Stopped-host recovery rejected a completed join — fixed.**
   - What this is: A phone retries its exact prepared identity after losing a reply.
   - Problem: Host availability was checked before recognizing an already committed pairing.
   - Fix: Validate the exact existing pairing first; keep availability required for
     new grants. The stopped/expired-host regression now returns the same receipt.
   - If skipped: A successfully paired phone could not finish local recovery after host stop.

5. **Native integration and external acceptance remain open.**
   - What this is: The service now supports the companion contract; ordinary native
     enrollment still uses the earlier single-device flow.
   - Problem: No durable phone setup/removal composition or activated trusted LAN path exists.
   - Fix: Continue native prepared-credential recovery, exact receipts and explicit TLS
     lifecycle. Verify existing Apple records after owner sign-in and retain activation gates.
   - If skipped: Synthetic service tests could be mistaken for a usable delivered product.

Verdict: The bounded service slice passes its source, synthetic HTTP and package
checks. Native two-device integration and TestFlight v1 acceptance remain unfinished.
No new dependency or fallback was added. No live access was enabled.
Not checked: actual credentials/Keychain, LAN/TLS trust, helper/App Group/startup,
real provider, physical phone, TestFlight, Apple account records or remote CI.
Prior baseline findings retain their checkpoint dispositions in Git and the v1 ledger.

```fort-review
{
  "format": 1,
  "reviewedCommit": "3739ac93c1d0673cf0fa63d6a28d44bb977b650e",
  "reviewer": "Codex",
  "scope": "Single-owner companion pairing: exact primary identity, digest-only prepared join, per-device expiry, narrow leave retries, atomic replay changes and stop/copy/delete regressions. No native or live access activation.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "21 focused behavior tests passed; one budget excluded then passed alone.",
    "Final portable: 1191 passed, 9 explicit skips, zero failures; 33.793 seconds.",
    "Strict TypeScript passed; all 38 emitted core payload hashes verified.",
    "20 isolated synthetic invite/join/revoke cycles: p95 5.920 ms against 25 ms budget.",
    "Observed public behavior red-to-green tests retained. Initial locked corruption fixture and sandbox EPERM are identified separately in evidence.",
    "Native composition, real provider, credentials, LAN, installed product, Apple and TestFlight remain unrun."
  ],
  "exceptions": [
    "No claim of real device or network activation; one primary and one companion is the explicit bounded scope.",
    "Unaffected isolated performance checks were not repeated; the unavailable 127.0.0.2 source test is explicitly skipped."
  ],
  "findings": [
    {
      "id": "companion-primary-authority",
      "detail": "Phone host controls, expired-primary selection and incomplete whole-host revocation corrected through public behavior tests.",
      "disposition": "fixed"
    },
    {
      "id": "companion-retry-boundaries",
      "detail": "Exact receipt recovery, invitation/device secret separation and retired-slot replacement verified.",
      "disposition": "fixed"
    },
    {
      "id": "companion-replay",
      "detail": "Effective pairing and expiry changes append transaction-scoped invalidations; exact retries emit none.",
      "disposition": "fixed"
    },
    {
      "id": "companion-stopped-retry",
      "detail": "Completed exact join may recover its receipt while stopped; new grants remain denied.",
      "disposition": "fixed"
    },
    {
      "id": "native-and-external-acceptance",
      "detail": "Native pairing, secure lifecycle, actual Apple/provider and physical TestFlight acceptance remain v1 gates.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "fd42680468f7d0d7c93d9d5bc3666bde46ef5363ecd25cfe28d8e9e0609abef7",
    "successor/V1-ACCEPTANCE.md": "0a23d44483f6d09aea57168ae52df947301fcf8074fbda1bda386a5c7733c7f1",
    "successor/contracts/v1.ts": "287f1ef925b0feaad9ee4b698da5bb09483b080a7c58d9537254c9152e8f24da",
    "successor/evidence/2026-10-17-node-foundation/companion-pairing.md": "165eccfb1cf08c907040a317f6c2d4809fb600f76d2896d0783a0c8f49c01d46",
    "successor/service/companion-pairing.test.ts": "566e652b6cea87a6313b985616a36656d8919e6cfec4b564633385f6d28359f0",
    "successor/service/companion-pairing.ts": "9d7f2f89c1a7849fce297b24d875319580e914249edad577b8faf284a2957bdb",
    "successor/service/openai.ts": "ac14429698e5425fee4c9c5a28130dec712b9d3f7e74a713ee5cd4779963758c"
  }
}
```
