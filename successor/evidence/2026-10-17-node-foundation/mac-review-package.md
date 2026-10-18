# Inert Mac review package — October 18, 2026

Governing [Spec 059](../../../specs/059-modular-node-v1.md), decision 059.
This is a composed engineering source artifact. It was not signed as a finished
application, installed, launched, registered, uploaded or accepted through TestFlight.
No owner data, credentials, profiles, account or external agent was accessed.

## Deliverable and boundaries

`/tmp/fort059-mac-review-20261018-0/Fort.app` contains the fresh unsigned Release
client and nested Release `FortHost.app`, compiled guardian, 87-file host resource
artifact (plus its manifest), relocated Node closure with 39 notices, and the
exact selected Codex 0.157.1 executable with root LICENSE/NOTICE. No separate
voice/code-mode payload, provider login or startup grant is included.

The public `packaging/mac-application.ts` CLI requires explicit input paths and an
exact client/helper/team/group configuration. It checks the source bundle IDs,
resource/runtime manifests and files, rejects linked/unsafe/unexpected inputs and
existing output destinations, and retains all source hashes. It writes matching
client/helper `fort-host.json` and the exact three-key bundle-local LaunchAgent
plist required by native inspection. It neither installs that plist nor calls
SMAppService. Apple metadata/load-command tools run with an empty environment.
Node load commands are relocated to the helper's Frameworks directory; existing
code signatures are not final authority after assembly. All final roles require
their own reviewed identities and entitlements in the later signing stage.

The review configuration uses existing engineering bundle identifiers
`io.fort.spec058.fixture.mac` and `io.fort.spec058.fixture.mac.host`, team identifier
`T3JB5MYZ93`, and unprovisioned engineering group `group.io.fort.spec058.fixture.mac`.
These are explicit review inputs, not assertions of registration or release IDs.
No embedded profile was invented or copied. The source app bundles were unchanged.

Independent read-only inventory checked all **171 output files**, file modes and
hashes, all **169 input hashes**, exact output membership and zero embedded profiles.
Total payload: **391,456,677 bytes**. Report:
`/tmp/fort059-mac-review-20261018-0/package-report.json`, SHA-256
`51d5d043d4f47c0c75f0fb281a3670904eccbe0519406754e82720606b7c7f9a`.
Sorted-key compact JSON of its `files` array plus newline hashes to
`93c85349429a6819f493cfe5a55415cf136e47acbe99c6189f8fe8a75c70ea7b`.
The report describes this artifact as `review-only`, `signed:false`, `started:false`.
It records each source/target hash and the pinned Codex source commit.

## Checks, failures and timing

- `/tmp/fort059-assembly-red.log`: positive CLI test failed at the inert stub.
- `/tmp/fort059-assembly-focused.log`: **15 passed, zero skipped**. Client/helper
  fixtures are inert; Node and Codex bytes come from explicitly selected local
  inputs and are copied/inspected, never executed. Exact layout/notices, preexisting
  destination, identity/group/hash/path/unexpected-payload/link/mode/license/executable
  refusals and owned failed-output cleanup are covered.
- `/tmp/fort059-assembly-inventory-tests.log`: existing test partition assertion
  failed because it did not yet include the new Mac-only test. Migrated it along
  with discovery; `...-inventory-pass.log` passes one function. Ordinary CI can run
  the refusal tests without accounts. Positive assembly requires explicit retained
  Node/Codex inputs; it is not silently downloaded or enabled.
- `/tmp/fort059-assembly-typecheck-final.log`: strict TypeScript passed.
- `/tmp/fort059-assembly-helper-build.log`, `...-client-build.log` and
  `...-guardian-build.log`: fresh unsigned Release helper/client and guardian compile
  passed. Host resources built in `/tmp/fort059-assembly-host-resources`; the empty
  environment Git license fallback emitted temporary-directory warnings but succeeded.
- `/tmp/fort059-assembly-native-inspection.log`: one read-only native function passed.
  An explicitly synthetic signature callback lets layout inspection reach the real
  missing-profile guard; the normal public inspector refuses the unsigned code
  signature. No profile bytes or product process were created for this check.

Three warm-cache assemblies with actual local payloads, fresh destination per run,
no concurrent task test/build, MacBookPro18,2 / 10 CPUs / 64 GiB / macOS 27.2,
Node 26.5.0 and Xcode 27.1. Measurements include input hashing, copying, relocation,
metadata verification and output hashing; exclude compilation, signing and providers.
P95 uses nearest rank (maximum for three samples): **1880.101542 ms <=15,000 ms**.
Raw `/tmp/fort059-assembly-samples.json`; all three review artifacts are retained.

```json
[{"milliseconds": 1880.101542, "exit": 0}, {"milliseconds": 1865.771542, "exit": 0}, {"milliseconds": 1874.425958, "exit": 0}]
```

## Concrete remaining work

Review found that the production core and executor-worker launch paths do not yet
supply the fixed Node-specific OpenSSL configuration used by the successful sandbox
probe. The [recorded sandbox failure](sandbox.md) already demonstrated the unwanted
Homebrew global-configuration read. Carry that tested fixed configuration into both
produced artifacts/launch paths before any installed runtime acceptance. This is
an implementation gap, not a request to relax the sandbox or change TLS policy.

Complete Codex's statically linked third-party notice inventory, full host-ledger
and iPhone cache deletion, and final signing/provisioning preparation. The exact
external Apple lookup remains described in [distribution requirements](distribution-requirements.md).
Actual provider authority, helper network/account permission, accessible iPhone
transport and installed/TestFlight acceptance remain unproven. Do not describe
this review-only composition or the synthetic signature callback as a working
installed application. No current code/runtime permission was broadened.
