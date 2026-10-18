# Inert Mac review package — October 18, 2026

## Current Mac test milestone

Source: `1b08c224a497974f09c39b3af9c1f99c4be39cc0`. The current review bundle is
`/private/tmp/fort059-mac-milestone-20261018/Fort.app`. It combines the fresh unsigned
Debug Mac client and helper with the current 98-file host resources, retained Node
closure, guardian and pinned Codex 0.157.1. No product process was launched.

Independent verification matched all **190 output files / 415,407,945 bytes**,
their modes and hashes, all **188 input hashes**, exact output membership and zero
embedded profiles. `package-report.json` SHA256:
`2c08189a78565fa58655e5755cc750ad021e0148093efdb392b4637e74f2dceb`.
Canonical sorted-key compact `files` JSON plus newline SHA256:
`2c3431da9be1e7bfdd0e0f9a2293ea4b4e047c37153b5ccf2e7a275aa09c2ec8`.
Report flags remain `review-only`, `signed:false`, `started:false`.
The configuration still uses engineering identifiers and an unprovisioned group;
it does not claim the proposed release identities are registered.

Current inputs:

- Client: `/tmp/fort059-native-companion-mac/Build/Products/Debug/FortMac.app`.
- Helper: `/tmp/fort059-native-companion-helper/Build/Products/Debug/FortHost.app`.
- Resources: `/tmp/fort059-native-companion-host-resources`.
- Node: `/tmp/fort059-assembly-node-input`; guardian: `/tmp/fort059-assembly-guardian`.
- Codex: `/opt/homebrew/Caskroom/codex/0.157.1/bin/codex`, inspected/copied only.
- Configuration: `/tmp/fort059-assembly-configuration.json`.

The existing explicit `packaging/mac-application.ts` CLI assembled these inputs
into a fresh destination. Xcode reported the new helper build successful; the client
and resources were already built and verified at this exact source checkpoint.
All **21 packaging tests passed, zero skipped** with explicit Node/Codex inputs.
The native `hostBundleReviewAssemblyReachesProvisioningGateAndRefusesPublicActivation`
test passed against this exact bundle: real public inspection refuses its unsigned
signature; an explicitly synthetic signature callback reaches the missing-profile
guard. The latter is layout proof only, not signature or provisioning evidence.

Logs: `/tmp/fort059-native-companion-helper-build.log`,
`/tmp/fort059-mac-milestone-{assembly,packaging,inspection}.log`.
Independent inventory: `/tmp/fort059-mac-milestone-inventory.json`.
Full source regressions remain the preceding slice's **1,191 portable passed / nine
skipped** and **269 native functions passed / eleven skipped**; this documentation
and packaging checkpoint did not rerun or relabel those suites. Existing source
and compiled synthetic journey evidence remains separate from this unlaunched app.

### What can be launched today

The isolated engineering UI at
`/tmp/fort059-safe-ui-20261018/Build/Products/Debug/FortQA.app` can be launched with
its synthetic session. Its cold-restart transcript fix passed accessibility,
screenshots and synthetic send/reply inspection: the newest reply's visible fraction
changed from 0 to 1. See [exact UI evidence](apple-access-ui.md). That QA app predates
the newest companion controls; their installed UI inspection remains open.
The current composed review app above cannot pass normal activation checks.
Neither artifact is a real-provider demonstration or TestFlight delivery.

### Shortest remaining path to real-provider Mac chat

1. Finish the native provider-account flow. Current `private-codex-plan.ts` selects
   a private `Runtime/state` and does not adopt ambient credentials. `CodexChat`
   requires an existing ChatGPT account through official `account/read`, while no
   current host/service/native caller implements `account/login/start`. A fresh
   installation therefore cannot reach model discovery yet. This is implementation
   work, not merely an approval or a request to copy credentials.
2. Complete read-only verification of team `T3JB5MYZ93`, client `io.mtree.fort.mac`,
   helper `io.mtree.fort.mac.host` and group `group.io.mtree.fort.mac`, then use the
   verified IDs/profiles to prepare the exact signed package. Existing authorized
   access is allowed. New IDs/capabilities/profiles, credentials or agreements need
   their specific approval. No absence is inferred from unavailable access.
3. Review the exact helper's provider network authority and native authentication
   action before activation. The current helper has outgoing networking disabled;
   signing alone cannot make provider chat work. Keep host access distinct from
   computer-tool restrictions, with no tool or account fallback.
4. With the exact package ready and the required grants approved, enable the owned
   helper through the supported Apple flow, sign in to the selected existing
   provider account through its official flow, select the exact model and verify
   native send/reply, quit/reopen, retained history, stop and cleanup. Expired demo
   grants do not authorize this run.
5. Complete Mac archive, TestFlight upload/processing/tester availability and inspect
   that exact installed build. A prior engineering/provider check would remain
   separate evidence; user delivery still requires TestFlight.

Read-only Apple recheck: the existing App Store Connect window still has sign-in
text and an email-entry field. No field value, credential or account record was read
and no control was pressed. Team/role/client/helper/group/profile/agreements remain
unverified. Log: `/tmp/fort059-mac-milestone-apple-readonly.log`. The smallest current
prerequisite is owner sign-in to that existing account; the earlier question remains
pending. Independent account-flow code and tests can proceed without that answer.

Phone preparation is preserved in local stash
`7b190e38e2d55d43c2bf368aa28ed2e922032a11`; two known isolation regressions remain
red there. It is excluded from this source/build milestone and is not a completion
claim. It need not delay the Mac chat path, but remains required for full v1.

## Earlier assembly checkpoint

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
