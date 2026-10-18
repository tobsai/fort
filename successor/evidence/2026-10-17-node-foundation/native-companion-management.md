# Native companion management contracts — October 18, 2026

[Spec 059](../../../specs/059-modular-node-v1.md), base
`75d072a083169495730d423f598d2b2616cc4058`, under unchanged-scope
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
This is a native contract slice before Mac UI/listener composition, not a working
installed two-device pairing flow.

FortClient now exposes the existing service's companion overview, invitation and
exact phone revocation endpoints. Overview requires the selected installation and
host device. Invitation receipts require the exact installation/invitation. Revocation
requires the exact installation/device, revoked=true and companion-only scope.
The service still enforces primary-host authority; these client methods confer none.

Available, invited and paired are distinct Swift values. Overview and invitation
decoders reject unknown or
contradictory fields, malformed IDs, impossible/noncanonical dates, invalid name byte
limits and a phone claiming the host's identity. They preserve valid Unicode names.
A phone without an expiry cannot be marked inactive. Expired dated phones remain
representable; the service owns the active status, not the client's clock.
Invalid installation/host/invitation/device IDs and invitation codes fail before
transport. Identifier/token validation is shared with existing phone preparation.
No code is generated, stored, logged or retried here. Each explicit call sends one
request through the existing authenticated transport and preserves lost-reply errors.

## Verification

- Initial public-interface compilation failed, then the first exact overview/invite/
  revoke request and receipt test passed.
- The initial permissive decoders/input boundary failed with **21 observed issues**:
  invalid fields/dates/identities could be accepted or reach transport. All pass after
  strict decoding and pre-I/O validation. Wrong revocation scope/identity and lost
  response cases also pass; lost responses produced exactly one call per operation.
- **11 focused functions passed**, including parameterized cases and the opt-in
  budget. `/tmp/fort059-companion-management-final-focus.log`.
- **317 native functions passed, 15 optional checks skipped, zero failed**; Swift
  Testing reports 332 total including skips, 52.918 seconds.
  `/tmp/fort059-companion-management-full.log`.
  Command: `swift test --no-parallel --package-path successor/apple/FortKit`, with
  FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node,
  FORT_TEST_GUARDIAN=/tmp/fort059-assembly-guardian,
  FORT_TEST_CORE_ARTIFACT=/tmp/fort059-owner-account-host-resources/core and
  FORT_TEST_HOST_RESOURCES=/tmp/fort059-owner-account-host-resources.
- Isolated serial benchmark: 20 overview/invite/revoke journeys after five warmups,
  **p50 0.350 ms, p95 0.354 ms, maximum 0.356 ms**, budget 100 ms. Injected in-process
  transport, synthetic inputs, no network/Keychain/provider/user time and no concurrent
  build or QA app. Samples: `/tmp/fort059-companion-management-samples.json`.
  Existing hardware inventory: MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2,
  Xcode 27.1 / Swift 6.4. The benchmark ran serially after other focused cases.
- Fresh unsigned ordinary Mac and iPhone Simulator builds passed. FortKit Release
  compilation passed. Logs: `/tmp/fort059-companion-management-{mac,ios}-build.log`
  and `/tmp/fort059-companion-management-release.log`.

The 15 full-suite skips comprise 13 isolated budgets and the review-package and
retained-signed-probe checks. This slice's budget ran separately. Portable service
checks were not repeated for these Swift-only changes. Existing server authority
checks and the phone UI source were not changed. Prior screenshots remain evidence
of the exact 75d072a QA build, not of these newly compiled, unlaunched ordinary apps.
iPhone Simulator execution remains unavailable because installed CoreSimulator
1171.7.0 is older than the required 1174.9.2. No system update occurred.

## Exact compiled artifacts

`/tmp/fort059-companion-management-artifacts.json` records sorted relative paths,
modes, byte lengths and file SHA256 values. Inventory hashes below bind compact,
sorted-key JSON file lists.

| Artifact | Files | Inventory SHA256 |
| --- | ---: | --- |
| Mac | 7 | 8aae426c56a52426ed051ebc5b74d7ed869f70f5f7353d15fd81965f76c9a953 |
| iPhone Simulator | 7 | 23fd3606b6ec4fb4350f8e675be6bb4ecba7c41825a3d41e71b5b2cc364cfadf |

Mac: `/tmp/fort059-companion-management-mac/Build/Products/Debug/FortMac.app`.
iPhone: `/tmp/fort059-companion-management-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
Neither was launched. Existing inert packages and their evidence remain preserved.

## Remaining work

Compose explicit Mac invitation/revocation controls and the secure listener lifecycle.
Actual TLS identity/trust, protected credentials, network/startup access, installed
provider behavior, physical devices and TestFlight remain unverified. This slice
performed no real invitation, revocation, account, certificate, network-listener,
provider, startup, push/deployment or provider-cleanup action. Pending certificate
approval remains unconsumed. The agent-host and computer-tool sandbox boundaries
stay separate; the client contracts add no computer-tool authority.
