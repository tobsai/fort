What this repo does: Fort is a single-owner native messenger with a modular Node
service and optional runtimes. This review assumes one primary Mac and one phone,
with no shared hosting. The imported donor stays quarantined.

Reviewer: Codex primary implementation agent, October 18, 2026. Base:
`75d072a083169495730d423f598d2b2616cc4058`. This is a scoped semantic review,
not an independent-reviewer claim. The whole-tree baseline remains preserved.

Read the exact new Swift values, FortClient callers, shared identifier validation,
all new tests, spec revision and evidence. Compared the three request/response shapes
with contracts/v1.ts and service/companion-pairing.ts, and checked the existing HTTP
routes and authority boundary. Traced the recording transport and no-retry behavior.
Existing phone preparation still uses the same identifier algorithm. No new storage,
credential generation, listener, SDK, package, timer or lifecycle worker is added.
No code was declared unused; the new client entry points are the reviewed prerequisite
for the next Mac control slice. The pinned Ponytail skill was applied as text only.

## Must fix before v1 acceptance

1. **Ordinary Mac pairing management is not composed yet.**
   - What this is: Native public contracts now exist for overview, invitation and
     exact phone revocation, matching the service's primary-host endpoints.
   - Problem: No ordinary Mac control invokes them yet, and its optional secure
     listener is still uncomposed. These methods alone do not pair installed devices.
   - Fix: Add explicit Mac controls and reviewed listener lifecycle, retaining exact
     identity, lost-response behavior and required external authority.
   - If we skip it: The owner cannot complete ordinary two-device setup.
   - Disposition: Open in the v1 ledger; continue implementation, without activation.

2. **Real installed and Apple delivery gates remain unverified.**
   - What this is: Contract fixtures, full native tests and unsigned build evidence.
   - Problem: These do not prove actual Keychain/TLS/trust, network/startup permission,
     provider behavior, physical devices or TestFlight. Prior QA cold-window/alert
     interaction limits also remain recorded at their original revision.
   - Fix: Verify exact authorized installed artifacts and provider journeys through
     the required Apple/device gates; do not substitute fixtures or reuse old grants.
   - If we skip it: A source import or engineering build could be called a delivered v1.
   - Disposition: Open. Pending certificate approval remains unconsumed.

## Fixed during this slice

3. **Permissive values could misstate the selected companion.**
   - What this is: The first native overview/invitation implementation decoded plain
     strings and ignored state fields it did not understand.
   - Problem: Wrong/extra fields, impossible dates, invalid IDs, an inactive phone
     without expiry, or a phone claiming the primary ID could be accepted. Invalid
     request identifiers and codes also reached the injected transport.
   - Fix: Require exact overview/invitation keys, validated state values, matching
     installation/host/invitation, and the existing exact phone revocation proof.
     Validate input before I/O and reuse the phone identifier/token predicates.
   - If we skip it: Future controls can display an unverified identity or send a
     request that should have been rejected locally.
   - Disposition: Fixed. The 21 observed red issues now pass, with valid states,
     UTF-8 name boundaries, exact methods/auth/body, lost-response and no-retry controls.

Verdict: Commit the verified native contract boundary, then compose Mac controls.
This is source/build progress, not installed-product or v1 acceptance.

Not checked: actual invitation/revocation against a real host; credential, TLS/trust,
LAN or startup activation; real provider accounts; physical devices; ordinary UI or
simulator execution; signed archives, upload, Apple processing and TestFlight;
remote CI and provider closure. The service/provider implementation was not changed
or re-audited in full. Native full suite: 317 passed, 15 optional skips, zero failures.
All 11 focused functions and the p95 0.354 ms isolated contract budget passed.
Fresh unsigned Mac/iPhone builds and Release compilation passed. No ordinary app
was launched; prior exact QA screenshots remain tied to their original build.
See [evidence](../../../successor/evidence/2026-10-17-node-foundation/native-companion-management.md).

```fort-review
{
  "format": 1,
  "reviewedCommit": "75d072a083169495730d423f598d2b2616cc4058",
  "reviewer": "Codex primary implementation agent",
  "scope": "Native primary-host companion management contracts, exact public client requests/receipts, shared identifier checks and evidence; one Mac and one phone",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "317 native functions passed, 15 optional skips, zero failures (332 reported).",
    "11 focused functions passed including parameterized identity/state/input/date/UTF-8 cases and isolated budget; 21 red issues observed before correction.",
    "20 synthetic serial three-request journeys after five warmups: p50 0.350 ms, p95 0.354 ms, maximum 0.356 ms under 100 ms.",
    "Fresh unsigned Mac/iPhone Simulator and Release builds passed; exact file inventories recorded.",
    "Actual provider, credential, TLS/trust, listener, device and TestFlight checks unrun; no ordinary app launched."
  ],
  "exceptions": [
    "This bounded contract slice precedes ordinary Mac controls and listener composition.",
    "Portable service checks and UI screenshots were not repeated for these Swift contract-only changes; earlier evidence stays tied to its revision."
  ],
  "findings": [
    {
      "id": "PHONE-MAC-COMPOSITION",
      "detail": "Mac invitation/revocation controls and secure listener lifecycle remain uncomposed.",
      "disposition": "open"
    },
    {
      "id": "PHONE-INSTALLED-ACCEPTANCE",
      "detail": "Exact external authority, real device/provider and Apple delivery gates remain unverified; prior cold-window/alert limits retained.",
      "disposition": "open"
    },
    {
      "id": "COMPANION-MANAGEMENT-BOUNDARY",
      "detail": "Strict states and identity receipts, valid canonical dates/names and pre-I/O input checks fix the observed permissive boundary.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "95c2824a8efedc90cc435943760c154fb181874c9fc8c4ce8f363830c8a4e303",
    "successor/V1-ACCEPTANCE.md": "20d48e74eddca777a43b97fa073af2ae3513bce3e374f44746808b3b171078cb",
    "successor/apple/FortKit/Sources/FortKit/CompanionManagement.swift": "9534735bc68f2c78d890b569cbda8e58623a05ec6047bd2292105beacd6ddb27",
    "successor/apple/FortKit/Sources/FortKit/CompanionSetup.swift": "dcc5435cb802777827f3161116e0c4f78f1f64c93d341df1c628ebb1d81f7968",
    "successor/apple/FortKit/Sources/FortKit/FortClient.swift": "fcd58ee976737effb760ba79349a7b1030dea39f639f70f262880227cd606135",
    "successor/apple/FortKit/Tests/FortKitTests/CompanionManagementTests.swift": "e3a17b6d65d974846f9b98cb5e383355f343fa82b2cd1f1bca839aaf08ee5114",
    "successor/evidence/2026-10-17-node-foundation/native-companion-management.md": "b8390be1081e6616fff6a7a0db63cb374ad6d15c50b7f3b32e6cf39b32eb55c6"
  }
}
```
