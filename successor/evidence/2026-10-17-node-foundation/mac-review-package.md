# Inert Mac review package — October 18, 2026

## Mac demo readiness — source 1f3cda5

October 19, 2026. Current source:
`1f3cda5dc473b4960c397b4a9a20915d2a262d64`, branch
`codex/fort-openai-rebuild-20261016`, under decision 059 and Spec 059.
The latest owner steering prioritizes installed Mac launch, actual-provider chat,
restart and reconnect, then TestFlight, before further phone UI. This section
supersedes checkpoint-specific next-work lists below; their history is retained.

The current review artifact is
`/private/tmp/fort059-mac-demo-review-20261019/Fort.app`.
It is **unsigned, unlaunched and not a working installed demo**. It contains fresh
Release Mac client/helper binaries, fresh compiled host resources, the preserved
self-contained Node closure/guardian, and exact Codex 0.157.1 with retained legal
payload. The source Mac provider-account controls and guarded host composition are
implemented; actual account/host activation and provider behavior remain unverified.
Optional phone networking is available in source but the ordinary bootstrap does
not enable it, and phone UI does not block this Mac demo milestone.

Verified results:

- Full portable suite at this source: **1,308 passed, 15 skips, zero failures**;
  strict TypeScript passed. See [owner-network evidence](owner-network.md).
- Fresh host resources at `/tmp/fort059-mac-demo-host-resources-20261019`:
  **112 manifest payload files, 113 including manifest, 484,231 total bytes**.
  Each size/hash and exact membership matched independently. Manifest SHA256:
  `2e7463a21456f569a243de770a1f77bee75a5babbdeee6449b8cce672bfff1a5`.
- Full serial native suite against those exact resources: **317 passed, 15 skipped,
  332 reported**, 53.081 seconds. Skips are 13 opt-in budgets and two retained
  package/signature probes without their optional inputs. The package probe was
  then supplied this exact new artifact and passed separately. The retained signed
  probe was not repeated. `/tmp/fort059-mac-demo-native.log` and
  `/tmp/fort059-mac-demo-package-inspection.log`.
- Fresh unsigned Release Mac client and helper builds passed in
  `/tmp/fort059-mac-demo-build-20261019`. No source setting, entitlement, identity or
  signing configuration was changed. Xcode performed ordinary LaunchServices build
  metadata registration; no SMAppService startup registration or product launch ran.
  Logs `/tmp/fort059-mac-demo-client-build.log` and
  `/tmp/fort059-mac-demo-helper-build.log`.
- Assembled **200 files / 404,145,780 bytes**, independently verified every output
  hash/mode/length, exact output membership and all **198 input hashes/modes**.
  **Zero embedded profiles.** Package report SHA256:
  `0ef15409b282973c248dce875f307214c61ba1b4c4b79a83e0c6987b4d4e50e2`.
  Report `/tmp/fort059-mac-demo-review-20261019/package-report.json`; assembly log
  `/tmp/fort059-mac-demo-assembly.log`. All prior artifacts remain intact.
- Actual public `MacHostBundle.inspect` refuses this unsigned artifact. An explicitly
  synthetic signature reader reaches the real missing-profile refusal. This proves
  the preflight gate, not a valid signature/profile or installed operation.

The exact review identities remain `io.fort.spec058.fixture.mac`, helper
`io.fort.spec058.fixture.mac.host`, engineering group
`group.io.fort.spec058.fixture.mac`, team `T3JB5MYZ93`. They were not adopted as
release IDs, provisioned or represented as registered records.

Minimum next actions, in dependency order:

1. Complete the selected `tobias@mtree.io` Apple sign-in through secure 1Password/UI
   and any device verification. Verify its role and the exact current team, app,
   helper and group records. The earlier Admin session verified the team but did
   not allow Developer ID Application creation. It was signed out without adding
   browser trust. On the selected account's password step, secure Autofill remained
   unconfirmed and Apple kept Sign In disabled; the owner handoff is pending.
2. Reuse verified existing app/helper/group records and compatible profiles. If
   absent, present only the exact missing registrations/group associations and
   development profiles for approval. The proposed release set remains
   `io.mtree.fort.mac`, `io.mtree.fort.mac.host`, `group.io.mtree.fort.mac`, pending
   remote verification. No replacement identity or access grant is inferred.
3. Prepare the final signed/profiled package with exact entitlement inspection.
   Actual provider operation needs the helper's separately approved outbound
   network capability (currently disabled), a scoped new Fort provider session,
   and an explicit per-user host startup grant. Present activation only after the
   exact signed artifact and cleanup path are reviewable. No owner credential is
   copied from another installation; agent-host and computer-tool permissions stay
   separate. The selected official runtime is the bundled Codex 0.157.1.
4. Execute installed native launch/pair/sign-in/chat/quit/relaunch/reconnect/stop
   acceptance through the real provider, recording exact build, accessibility,
   screenshots and cleanup. Then prepare the verified Mac App Store/TestFlight
   identity, archive/upload, Apple processing and tester availability separately.
   Physical iPhone acceptance remains required for full v1 after the Mac milestone.

G2 Developer ID Application creation is already approved for team T3JB5MYZ93, with
its key retained on this Mac and existing certificates preserved. It has not been
performed and is not a TestFlight prerequisite. Apple's updated Program License
Agreement remains unaccepted; provide its verified link and request separate owner
approval if that action is needed. No new key/CSR/certificate, provisioning, trust
change, provider sign-in, startup activation, archive or upload occurred here.
No actual provider/LAN/physical acceptance or new installed-product screenshot is
claimed. Safari inspection is Apple workflow evidence, not Fort UI acceptance.


## Native account checkpoint

Current native source: a522d2a plus the reviewed native-account slice. New inert bundle:
`/private/tmp/fort059-account-milestone-20261018/Fort.app`. Current unsigned Mac/helper
builds are `/tmp/fort059-account-{mac,helper}-20261018/Build/Products/Debug/` and host
resources are `/tmp/fort059-owner-account-host-resources` (108 verified files).
The retained Node, guardian, pinned Codex 0.157.1 and engineering configuration inputs
below are unchanged. The application/runtime was not launched or signed.

Independent verification matched **200 output files / 395,097,724 bytes**, all
**198 input hashes**, modes, exact output membership and zero profiles.
Report SHA256: `e4c1b3a4e9c35056184454503e9492f9bbf25b3e5af26ca9cb352fc453f30aa8`.
Canonical output inventory SHA256: `09c599d4c333428c3a344332fb6582d8ff2ac059397b333456f1893dcbdc37e5`.
Full verification: `/tmp/fort059-account-package-inventory.json`.
Normal public inspection rejects its unsigned signature; the synthetic signature
callback reaches the missing-profile gate. That exact native check passed.

[Native account evidence](native-account-sign-in.md) records fresh builds and actual
synthetic Settings inspection. These controls now exist through native/service/owned
executor/protocol layers. Real provider and signed installed acceptance still require
exact Apple records/profiles and authorized network/startup/provider actions.
The launchable artifact remains the separately labeled offline QA app. Neither artifact
establishes TestFlight availability. The earlier package below remains preserved.

## Earlier Mac test milestone — source 1b08c224

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
