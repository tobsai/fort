# Durable native companion preparation — October 18, 2026

[Spec 059](../../../specs/059-modular-node-v1.md), implementation base
`83effc5435a29abd6c910078f1558989a4960add`. The earlier parked preparation was restored
selectively from stash `7b190e38e2d55d43c2bf368aa28ed2e922032a11`; the stash remains
intact. New Mac account changes and unrelated files were preserved.

The public CompanionSetupCoordinator prepares an exact HTTPS phone pairing before
join. Construction is inert. One protected journal stores the selected endpoint,
installation/device/invitation, device name and code/token digests. The actual token
uses the existing credential seam; tests use memory only. No plaintext code or token
is written to the journal. A missing/changed ready token never generates a replacement.

Lost join replies can recover using the same prepared credential and authenticated
installation/pairing reads, without the invitation code, token regeneration or another
join. Every awaited response is checked against current journal bytes, connection,
credential, installation and companion role. Foreign responses preserve uncertainty.
Explicit pending disconnect targets only the exact phone. Confirmed remote removal
survives local credential-cleanup failure; retry then performs only local cleanup.
Local forgetting records revocation as unconfirmed and does not contact the service.

Connection schema 4 persists companion role only for ordinary HTTPS network endpoints.
Older primary/demo schemas retain their meaning. A restored expired phone keeps only
its exact leave-recovery authority; it cannot stop/restart the Mac or manage accounts.
This public model behavior is prepared for ordinary UI composition, not a physical
phone acceptance claim.

## Observed failures and fixes

- The parked tests reproduced credential creation/join despite an unconfirmed older
  removal, and a competing owner setup contacting the host/publishing a connection.
  Exact removal reconciliation and reciprocal owner-setup guards now prevent both.
- New public regressions observed account preparation overlapping pending phone setup,
  phone preparation ignoring a pending account operation, owner setup ignoring account/
  history work, and saved-history deletion continuing during pending phone preparation.
  All now refuse before effects. Owner setup also rechecks after its awaited steps.
- A changed journal with an additional key was previously adopted during recovery.
  Strict decoding now rejects it before requests and preserves its bytes.
- Expired-phone restore initially lost its companion role and could not revoke itself.
  Persisted role now enables only exact leave recovery. Public tests verify no host,
  restart or account request is sent, even when ordinary installation reads fail.
- Late join replies after journal changes, foreign installation/device/role, missing/
  changed credentials, wrong receipts, symlink replacement, old confirmed receipts,
  lost leave replies and partial local cleanup retain the correct records and scope.

## Verification

Full native: **300 functions passed, 13 optional functions skipped, zero failed**.
Swift Testing reports 313 total including skips. Command:
`swift test --no-parallel --package-path successor/apple/FortKit`, with the existing
explicit Node26, guardian, core and host-resource inputs used in
[native account verification](native-account-sign-in.md#verification).
Log: `/tmp/fort059-phone-preparation-full.log`.
Focused preparation/schema run: **15 passed, one optional benchmark skipped**,
including parameterized refusal cases. Expired-phone/account/companion regressions
also passed together. Observed red logs and corrected runs are retained under
`/tmp/fort059-phone-preparation-*` and `/tmp/fort059-phone-expired-{red,green}.log`.

Isolated public prepare → lost reply → recover: 20 serial samples after five warmups,
**p50 22.186 ms, p95 30.544 ms, maximum 31.052 ms**, budget 100 ms. Each sample uses
one new private temporary directory, durable journal, memory credentials and in-process
transport. Directory/fixture construction is outside the sample; actual network and
Keychain latency are excluded. No build/test ran concurrently. MacBookPro18,2,
macOS 27.2, Xcode 27.1 / Swift 6.4, existing inventory 10 CPUs / 64 GiB.
`/tmp/fort059-phone-preparation-samples.json` contains all samples.

The full-suite skips are eleven separate opt-in performance measurements, the exact
review-package probe and retained-signature probe. Phone preparation performance ran
separately. Those other opt-in checks and portable service tests were not rerun for
this Swift-only change; their prior checkpoint results remain distinct.

Fresh unsigned Mac and iPhone Simulator Debug builds and FortKit Release compilation
passed. Build logs: `/tmp/fort059-phone-preparation-{mac-build,ios-build,release}.log`.
The existing CoreSimulator 1171.7.0 versus required 1174.9.2 incompatibility still
prevents simulator execution. No OS update, signing/profile, real credential, TLS
identity/trust, LAN listener, startup registration or provider action was performed.

`/tmp/fort059-phone-preparation-artifacts.json` records every file's size/mode/SHA256.
These hashes cover sorted-key compact inventories plus newline:

| Unsigned artifact | Files | Inventory SHA256 |
| --- | --- | --- |
| mac | 7 | `c790b6337b85802e38ea3e0331e2755d48787b115af29a39b68c701df50a6021` |
| ios | 7 | `c036db664f8ede76fae06e8c044717e9aff7165c49d03adad4c31dac14a195ce` |

Mac path: `/tmp/fort059-phone-preparation-mac/Build/Products/Debug/FortMac.app`.
iPhone path: `/tmp/fort059-phone-preparation-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
Neither was launched. The preserved account milestone package remains at source
83effc5; this slice does not silently relabel that artifact as a newer build.

## Remaining composition and external gates

Next ordinary UI must import the exact invitation, expose prepared retry/recovery/
cancellation and expired-phone leave, and connect the Mac invitation/secure-listener
lifecycle. Current public methods and fixtures do not establish usable phone setup.
Real Keychain/HTTPS/trust, simultaneous Mac/phone, signed installed lifecycle, physical
accessibility/screenshots/provider and TestFlight remain separate gates. Certificate
replacement approval remains with the parent; no certificate action was taken.
