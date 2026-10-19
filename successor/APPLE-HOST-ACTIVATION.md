# Apple host activation gate

## Current Mac demo handoff — October 19, 2026

Current source `1f3cda5dc473b4960c397b4a9a20915d2a262d64`; prioritize installed
Mac launch/actual-provider chat/restart/reconnect, then TestFlight, ahead of phone UI.
[Fresh exact package and checks](evidence/2026-10-17-node-foundation/mac-review-package.md#mac-demo-readiness--source-1f3cda5)
record 1,308 portable passes, 317 native passes, fresh Release client/helper builds
and a 200-file unsigned review app. No embedded profile, startup activation or
actual provider session exists for that package. Public signature inspection refuses
it; the synthetic signature check reaches the missing-profile gate.

Read-only Apple access previously confirmed Maple Tree Enterprises LLC, team
`T3JB5MYZ93`, role Admin, with Account Holder Tobias Gunn. Developer ID Application
creation was disabled. The owner selected `tobias@mtree.io` in source message
`Sentinel_abde69899a008191b033d31fb1194848` (October 19, 00:38:56 UTC), relayed by the
parent. The existing Admin browser session was signed out using ordinary Sign out,
without Trust and sign out. The selected account identifier was verified exactly.
The existing unlocked 1Password item was selected and secure Autofill invoked, but
Apple's password-step Sign In remains disabled. The secure sign-in/device-verification
handoff is pending. No password was read, revealed, copied or transmitted by tools.
The selected account's current role is **not yet verified**.

The separate approved G2 Developer ID Application action remains pending, preserving
all existing certificates and keeping its private key on the signing Mac. No key,
CSR or certificate has been created/installed/exported/revoked. The earlier account
banner reports an updated Program License Agreement; it remains unaccepted and needs
its verified link plus separate approval before acceptance. Xcode's system-component
Install prompt remains untouched. G2 direct distribution is separate from TestFlight.

Next: finish secure Apple sign-in, verify the exact existing Mac app/helper/group
records, then request only any confirmed missing provisioning changes. The proposed
release IDs below remain unadopted until that lookup. After signing/provisioning,
present exact helper outbound-network, provider session and per-user startup scope
for approval before installed execution. Existing certificate approval does not
authorize those grants. The current helper still has outbound networking disabled.
The source provider-account/native controls are implemented; the older "not
implemented" statements below describe their historical checkpoints only.


October 18, 2026: **not provisioned, registered or activated**. Governing design:
[Spec 059](../specs/059-modular-node-v1.md) and its
[owner decision](../docs/architecture/decisions/059-node-foundation.md).
Source checkpoint: `9c3d17d06e9010147c2448be1a25c6962de0efbd` on
`codex/fort-openai-rebuild-20261016`. This records the missing external steps;
it does not authorize them. Independent implementation continues. The current
source/package checkpoint is `1b08c224a497974f09c39b3af9c1f99c4be39cc0`; see the
[190-file review package and Mac test path](evidence/2026-10-17-node-foundation/mac-review-package.md#current-mac-test-milestone).
Its public inspector correctly refuses unsigned code. No profile was fabricated.

Read-only recheck on October 18: the existing App Store Connect window still shows
sign-in text and an email-entry field. Existing accessibility permission was used
to read labels only; no field value was read or control pressed. Current team,
role, app/helper/group records, profiles and agreements remain unverified. The owner
sign-in question is pending; this is an access prerequisite, not evidence that the
records are missing. No credentials, new access or Apple settings were changed.

## Exact identities and missing profiles

Current clients use engineering IDs `io.fort.spec058.fixture.mac` and
`io.fort.spec058.fixture.ios`; QA uses `io.fort.qa.firstwave.20261017`.
None is verified as the intended App Store Connect release identity. Existing
signing metadata verifies team **`T3JB5MYZ93`**, not its current account role,
app records, enabled capabilities or agreements.

The historical delivery note references the two client IDs below. This is a
**proposed identity set for account verification only**. These values have not been
adopted in release configuration, registered, granted or remotely verified.

| Component | Exact proposed identifier | Evidence / missing item |
| --- | --- | --- |
| iPhone client | `io.mtree.fort` | Historical reference and local iOS Store profile; current app record/build/testers unverified |
| Mac client | `io.mtree.fort.mac` | Historical reference; current Mac app record and matching Mac profile unverified |
| Nested Mac agent | `io.mtree.fort.mac.host` | New proposal; no registration or matching local Mac profile verified |
| Shared Mac App Group | `group.io.mtree.fort.mac` | New proposal; no matching local entitlement/profile verified |
| Derived Mach service | `group.io.mtree.fort.mac.fort.host` | Derived from that group; no service registered or connected |
| Derived LaunchAgent label/plist | `io.mtree.fort.mac.host` / `io.mtree.fort.mac.host.plist` | Proposed bundle-local configuration; no job or plist installed |

The October 18 [local inventory](evidence/2026-10-17-node-foundation/mac-agent-boundary.md)
decoded 91 profiles. Fort matches were the historical iOS app, Watch app and Watch
widget, all without App Groups. No Fort Mac/agent profile was found. Local absence
does not establish remote absence. If compatible helper/group records already exist,
record their exact values and review the identity set before configuration; do not
create duplicates or silently adopt a historical identity.

## Required package agreement

Both Mac client and agent need exact code identities under the verified team and
matching profiles authorizing the same `com.apple.security.application-groups` value.
For this proposal, profiles must authorize application identifiers
`T3JB5MYZ93.io.mtree.fort.mac` and `T3JB5MYZ93.io.mtree.fort.mac.host`, using the
actual macOS entitlement keys, with group `group.io.mtree.fort.mac`. Inspect team,
identifier, group, purpose, expiry and signing match in the actual profiles.

The standalone per-user agent needs its **own** App Sandbox and App Group entitlements;
it cannot inherit the foreground client's sandbox just because it is bundled there.
Node and the guardian launched by that agent retain only App Sandbox plus sandbox
inheritance. Prevent debugger/base-entitlement injection into those inherited tools.
Provider/network and additional file-access capabilities remain separate reviews.

Proposed layout:

- Helper: `Fort.app/Contents/Helpers/FortHost.app`, with its own
  `Contents/embedded.provisionprofile`; the parent app also needs its own profile.
- LaunchAgent plist:
  `Fort.app/Contents/Library/LaunchAgents/io.mtree.fort.mac.host.plist`.
- Bundle-relative program: `Contents/Helpers/FortHost.app/Contents/MacOS/FortHost`.
- History: `<Apple's URL for group.io.mtree.fort.mac>/FortHost`, resolved through the
  system API and storage preflight. A returned URL alone is not entitlement evidence.

This is a per-user agent, not a root daemon or a separately published helper product.
The background helper, native lifecycle and runtime bootstrap now compile and the
complete unsigned payload is assembled. Signed package/profile integration and
actual runtime acceptance remain. The strict no-extended-ACL storage policy
needs compatibility evidence from the actual provisioned container; do not repair
unknown ACLs or delete retained history. The iPhone needs no Mac App Group merely
to pair over its future network transport; this group grants no cross-device access.

Sources: [Apple's named-XPC/profile guidance](https://developer.apple.com/forums/thread/835003),
[App Groups](https://developer.apple.com/forums/thread/721701),
[bundle locations](https://developer.apple.com/documentation/bundleresources/placing-content-in-a-bundle),
[container API](https://developer.apple.com/documentation/foundation/filemanager/containerurl(forsecurityapplicationgroupidentifier:))
and the installed SDK's `SMAppService.h`. These describe platform requirements,
not this account's authorization or approval of the proposed names.

## Smallest external action, in order

1. **Read-only account check.** An owner/admin with existing authorized Apple access
   verifies team `T3JB5MYZ93`, the two historical client records, and whether the exact
   helper/group proposals already exist. Retain only identifiers, platform, capability
   membership, role and profile purpose/expiry. The existing browser remains at
   sign-in and no cached authorized CLI session is available. The owner must sign in
   through the existing account before the permitted read-only check can continue.
   No API key is needed for that UI check; do not send credentials in chat.
2. **Conditional provisioning approval.** Reuse matching existing records/profiles
   when available. Otherwise, the smallest proposed mutation is to register only the
   missing verified Mac client/helper IDs and shared group, associate the group with
   those two IDs, and generate matching development profiles using existing signing
   material for the test Mac. The new proposed values are `io.mtree.fort.mac.host`
   and `group.io.mtree.fort.mac`; client reuse must be verified first. This has not
   been approved or performed. Role changes, keys/certificates, agreements and
   distribution changes require separate scope.
3. **Local activation approval, later.** First finish and verify the actual signed
   package, profiles, hashes, entitlements, program path and cleanup behavior. Then
   present one bounded per-user
   `SMAppService.agent(plistName: "io.mtree.fort.mac.host.plist").register()` action
   for that exact installation. If macOS reports `requiresApproval`, the owner must
   approve its background item in system UI. Do not bypass that gate with launchctl,
   manual LaunchAgents files or permission changes. Registration status does not prove
   helper health, pairing revocation or descendant absence.

The per-action security/persistent-access requirement comes from
[CLAUDE.md](../CLAUDE.md). **No approval request is pending here:** the production
package is not yet ready for activation review. No provisioning/account mutation or
registration occurred. These Apple actions would not authorize provider sign-in,
credential copying, public ingress, another device's access, deployment or upload.

## Local code and verified checkpoint

Earlier local preparation added a background-only engineering helper target,
`io.fort.spec058.fixture.mac.host`. It is not a release identity or a registered
capability. That initial standalone build lacked the runtime payload. The current
composed review app now contains manifest, Node, guardian and bootstrap, but still
has no embedded profile or valid final signature. No activation request is ready.
The helper still has outgoing networking disabled and native provider sign-in is
not implemented; these are separate prerequisites for real-provider chat.
Historical source/tests/build evidence is in
[helper program preparation](evidence/2026-10-17-node-foundation/mac-host-program.md)
and the [v1 ledger](V1-ACCEPTANCE.md); the initial checkpoint table below remains
historical evidence rather than the latest build inventory.

Independent work includes wrapper/configuration and signature validation, exact
owned-installation cleanup, provider adapter containment, synthetic recovery tests
and engineering builds. Actual named-agent/App Group execution waits for matching
identities/profiles; persistent registration waits for its bounded approval. Provider
acceptance, reachable iPhone transport and TestFlight delivery remain separate gates.

Client evidence is from `51197365210f643d45a5eb73519c11dcb4eac7f8`; client product
source is unchanged by `9c3d17d`:

| Check | Actual result |
| --- | --- |
| Native regression | 173 functions executed and passed; 2 timing functions excluded, new storage timing passed separately |
| Fresh unsigned Mac Debug build | Passed: `/tmp/fort059-agent-storage-mac-build/Build/Products/Debug/FortMac.app` |
| Fresh unsigned iPhone Simulator Debug build | Passed: `/tmp/fort059-agent-storage-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app` |
| Simulator UI | Unrun; CoreSimulator 1171.7.0 is older than required 1174.9.2 |
| Latest packaging slice | 6 affected checks and strict TypeScript passed; retained Codex version-only artifact independently verified |
| Installed activation, accessibility/screenshots, real provider and TestFlight | Unrun or unverified; no completion claim |

See [storage evidence](evidence/2026-10-17-node-foundation/mac-agent-storage.md),
[Codex packaging evidence](evidence/2026-10-17-node-foundation/codex-bundle-version.md)
and the [v1 ledger](V1-ACCEPTANCE.md). Historical specs, unrelated work, consumed
demo grants and the unresolved full-provider-closure status remain preserved.
