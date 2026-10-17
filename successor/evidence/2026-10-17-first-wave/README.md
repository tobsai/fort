# Native first-wave refinements — October 17, 2026

Source and fixture work is complete for agent identity, model selection, bundled
animated avatars, settings, local image display, and the pairing/removal contract.
This is build and fixture evidence, not installed-product or persistent-host signoff.

Source revisions on `codex/fort-openai-rebuild-20261016`:

- `60675b5fd8b4cc6c9e86cca188ee684628c84e0a`: native settings, agent metadata,
  runtime model discovery and bounded device-local image previews.
- `1a3e5bdc4902ba7fdf4c936730174754f6a6b2e6`: explicit owner pairing policy,
  guarded service shutdown/revocation and native removal flow.

The owner's first-wave request is recorded as
`Sentinel_c5a65efc2f1881919230ff550ff5221a`. The later correction
`Sentinel_102d9824b0e48191be308694aeb42dc3` removes the proposed default pairing
limit: no expiry by default, optional expiry, straightforward disconnect/uninstall.
The coordinator authorized preparation and fixtures, not persistent activation.
Image rendering is implemented as local PNG/JPEG display; generation, remote image
fetching and provider image input are not implemented by this slice.

## Implemented boundaries

The TypeScript service remains the shared backend. Swift owns settings, animation,
image display and device credential storage. Versioned HTTP contracts allow another
client or a later backend without sharing native UI code. Existing identities and
conversation IDs remain stable; configuration edits create new binding revisions.
Old agents with no avatar continue to decode. Already accepted turns retain their
instructions/model. Model discovery preserves exact runtime slugs and unavailable
saved selections; it never substitutes a model. The old demo proxy still limits
its catalog to its authorized model. Discovery does not attest the effective model.

`ownerPairingPolicy` is an explicit service-construction option, currently used
only by fixtures. It requires a fresh installation and cannot adopt an existing
legacy or bounded-demo ledger. Its no-expiry value is persisted; optional expiry
is 1–365 days from the owner's save. Restart does not reset it. One owner-local
device can be paired at a time. Ordinary service construction and all historical
demo deadlines keep their previous behavior. No launcher activates this new policy.

Authenticated stop/disconnect routes stop scheduling and dispatch, abort active
work and await runtime disposal. Disconnect then revokes the pairing and pending
enrollment codes. Agents, messages and events remain. The response confirms runtime
shutdown; it does not prove the enclosing host process exited. The listener closes
after the response. Failed runtime shutdown returns a controlled error and retains
pairing for recovery. Browser-origin requests are rejected.

Native removal targets the exact installation/device Keychain account. It preserves
cached history and local image previews. A persisted disconnected marker prevents
reconnection while local credential removal is incomplete and exposes a retry that
only finishes local cleanup. It does not restore revoked access. Lost shutdown
responses or an unreachable host still need the future owned-host recovery path.

`OwnerLifecycle` requires both owned-host exit and revocation proof before local
credential removal and app removal. It keeps history by default; deleting history
is a separate confirmation path. `OwnedMacInstallation` has fixture implementations
only. There is no real installation/removal adapter, no installed permanent host,
and no enabled uninstall action in ordinary app construction. These boundaries are
prepared code, not a claim that this Mac can already uninstall a managed host.

## Validation

| Check | Result |
| --- | --- |
| Affected service/runtime fixtures | 119 passed, 0 failed |
| Native FortKit fixtures | 50 passed, 0 failed |
| Strict TypeScript checking | Passed |
| Git whitespace check | Passed |
| Mac Debug build | Passed, ad-hoc signed |
| iPhone Simulator Debug build | Passed, ad-hoc signed |
| Strict deep signature verification, both apps | Passed |

Service tests cover optional expiry persistence, no-expiry restart, expired access,
rejection of legacy/demo adoption, one-device enrollment, browser-origin denial,
shutdown failure and delayed shutdown proof, revoked-token rejection and unchanged
message history after explicit fixture reenrollment. Native tests cover exact scoped
credential removal, retained history without reconnecting, partial-cleanup recovery,
false shutdown receipts, uninstall ordering and default history retention. Existing
agent, model catalog, avatar, image, routine and recovery fixtures also pass.

Commands used existing tools and dependencies:

```sh
/opt/homebrew/opt/node@26/bin/node --test successor/service/openai.test.ts successor/service/agent-settings.test.ts successor/service/owner-pairing.test.ts successor/runner/codex-chat.test.ts successor/prototype/mvp-admission.test.ts
/opt/homebrew/opt/node@26/bin/node successor/node_modules/typescript/bin/tsc -p successor/tsconfig.json
/usr/bin/xcrun swift test --package-path successor/apple/FortKit
```

Build commands are preserved at the start of the build logs. App paths and per-file
hashes are in [checks.json](checks.json). No app was launched or installed here.

- Mac: `/tmp/fort058-first-wave-mac-20261017/Build/Products/Debug/FortMac.app`
- iPhone Simulator: `/tmp/fort058-first-wave-ios-20261017/Build/Products/Debug-iphonesimulator/FortiOS.app`

The Mac signature contains sandbox, outbound-network and Debug task-access
entitlements. It contains no broad filesystem exception and no user-selected-file
entitlement. Actual file-picker access therefore remains an explicit acceptance
gate; no entitlement or security setting was changed. Both builds reported the
existing older CoreSimulator service. Builds passed; Simulator UI was not launched.
AppIntents metadata extraction was skipped because the app has no such dependency.

The service lifecycle red tests failed on missing behavior as expected. A later
fixture run was blocked by the command sandbox's loopback restriction; supported
escalation succeeded. Native red tests failed on absent types, followed by a corrected
Swift throwing-expression error. Final results above have no failing checks.

## Remaining acceptance

Inspect these exact builds through installed accessibility and screenshots, including
settings navigation, avatar Reduce Motion, the file picker and inline image layout.
Confirm the real runtime catalog and exact model dispatch under separately approved
access. Prepare the exact owned local host launcher/removal adapter, installation
identity and process-exit proof before offering persistent setup. Ask once for its
concrete access scope; no default expiry, optional expiry and default history retention
are already decided. Do not request a thirty-day limit again.

No provider call, sign-in, real credential, live session, startup service, security
configuration, app install/removal, public publication or provider cleanup occurred.
Fixtures use temporary synthetic credentials. The host still has the Mac user's
access; computer-tool controls are not a host sandbox. Physical iPhone and broader
MVP gates remain in MVP-20261017.md. The owner reported the earlier demo worked;
that is basic chat acceptance, not signoff for this revision. This work does not add
independent cleanup verification for that demo.

Historical source pins, demo ledgers, approval packets and the quarantined donor
remain preserved. None were reset, extended or resealed. The primary checkout and
unrelated `.build/` and approval-file work were not edited.
