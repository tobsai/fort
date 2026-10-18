# Explicit companion pairing preparation

Date: 2026-10-18. Base: `3739ac93c1d0673cf0fa63d6a28d44bb977b650e`.
Authority: [Spec 059](../../../specs/059-modular-node-v1.md), explicit companion
refinement under [decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
This is a source and synthetic HTTP result. Native phone setup, real credentials,
LAN activation, production TLS trust and installed acceptance remain unperformed.

## Behavior and boundaries

One primary Mac retains its control/recovery identity and may explicitly invite one
phone. The client prepares a new invitation ID/code and a separate private device
ID/token before joining. Core stores digests, commits membership atomically, and
returns the same receipt for an exact retry after lost responses, restart or an
expired invitation. A completed pairing can confirm its receipt while the host is
stopped; it cannot grant new access or resume work. Changed identities/tokens fail.

The phone shares ordinary messaging and controls its own expiry. It cannot invite
another device, replace the runtime or stop/disconnect the owning host. Phone leave
revokes only that phone and keeps history and accepted work. Its exact revoked token
can recover only the same leave receipt; normal requests remain unauthorized. Mac
revocation also works while stopped. Whole-host removal retains both credentials
until runtime stop is confirmed, then revokes both and clears the invitation.

One bounded metadata record and existing device rows own this state; no schema or
dependency was added. Effective changes emit workspace invalidations in the same
transaction. Exact retries and unchanged no-expiry selection add no wakeups. The
existing temporary/demo path cannot issue companion grants. Local setup never
mistakes an active phone for an expired Mac. Replacing two expired devices clears
the retired slot. Offline copy preserves membership; confirmed history deletion
follows whole-host revocation. No loopback address confers authority.

## Checks and observed failures

Commands ran from `successor/` with `/opt/homebrew/opt/node@26/bin/node` 26.5.0,
TypeScript 6.0.3 and @types/node 26.6.5. All runtime/provider behavior was synthetic.

- Final `node --test service/companion-pairing.test.ts`: **21 passed**, one isolated
  benchmark excluded, no failures. Final log `/tmp/fort059-companion-complete-green-authorized.log`.
- Final `node verification/test-suite.mjs --suite portable`: **1,191 passed**, nine
  explicit skips, zero failures/cancellations; 1,200 total, 33.793 seconds. Eight
  skips are isolated budgets (the affected companion budget passed separately);
  one is the unavailable 127.0.0.2 source interface. No interface was configured.
- Strict `node node_modules/typescript/bin/tsc -p tsconfig.json`: passed.
- Fresh compiled core `/tmp/fort059-companion-complete-core`: **38 payload files,
  238,291 bytes**; every manifest length/hash independently verified. The portable
  suite also passes the independent compiled runtime contracts. No provider/test
  identity implementation was added to core.
- Initial public invitation returned 404; lifecycle tests then exposed missing
  phone leave/revoke, wrongly permitted phone runtime controls, incomplete whole-host
  revocation and expired-Mac identity selection. Later observed regressions covered
  invitation-code reuse as a persistent token, retired-slot replacement, missing
  replay events and stopped-host exact receipt recovery. All now pass.
- A first corruption fixture tried reading a ledger while its exclusive writer
  was open and failed with a lock error. The fixture was corrected to close before
  inspection; this was not evidence of a product defect. Node type stripping also
  rejected an initial constructor parameter property, which was replaced with
  explicit fields before the final checks.
- The final suite's first sandboxed attempt could not bind disposable loopback
  sockets (`EPERM`). The authorized rerun passed. No automatic approval rejection
  occurred. The builder's Git fallback printed a temporary-directory warning but
  completed; payload verification passed.

## Isolated performance

MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2. One Node process, temporary private
SQLite ledger, synthetic runtime, three local HTTP requests per cycle: invite,
join, revoke. Five warmups followed by 20 sequential samples, no concurrent build
or test workload. IDs/tokens are generated outside timing. Provider, TLS, Keychain
and native UI cost are excluded. No idle polling is added.

`FORT_COMPANION_BENCHMARK=/tmp/fort059-companion-complete-benchmark.json node --test --test-name-pattern='twenty isolated companion' service/companion-pairing.test.ts`

p50 **5.600 ms**, p95 **5.920 ms**, maximum **6.347 ms**; specified p95 budget 25 ms.
This measures the synthetic local seam, not physical phone performance.

## Evidence integrity and remaining gates

| Artifact | SHA256 |
| --- | --- |
| Core manifest | `13136a7b201a45532b24e590a47592f2e7aac5a5306074e1d041d7dedcad9882` |
| Final portable log | `32cbd37594d2ae1e53d774e8dd0efabe75983a80682feed17b434bc93e1cc026` |
| Final focused log | `f5faa885a87b7a7b12a9f00e59daec7c94463361c70907e2cabd5e852dfb817e` |
| Isolated timing JSON | `0f653d5262de1dbcc4fd3e3de0e21fdc61062ae6295ea58acb0cf68951647c33` |

Native durable prepared credentials, phone/host settings and removal presentation,
explicit secure-listener lifecycle and actual HTTPS trust are next integration
gates. Apple account records are unverified pending existing-account sign-in.
Actual Keychain, App Group, startup, provider, physical phone/Mac and TestFlight
acceptance remain open. No real device token, certificate, startup grant or provider
session was created or used. No network listener was enabled outside test loopback.
The agent host and computer-tool sandbox remain distinct authority boundaries.
