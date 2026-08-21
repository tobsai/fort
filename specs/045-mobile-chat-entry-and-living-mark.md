# 045 — Durable mobile chat entry and living Fort mark

**Status:** implemented and production-deployed with the combined Agent
Channels release on 2026-08-21.
**Governed by:** [028-remote-gateway](028-remote-gateway.md) · [044-private-primary-channels-phase-1](044-private-primary-channels-phase-1.md).

## Goal

On iPhone, sign in once, remain signed in while Fort is actively used, and open
directly into an available private Channel. Keep the Fort orbital mark visibly
alive throughout the app instead of freezing it whenever the agent is idle.

## Decisions

1. A native gateway bearer lasts 30 days. A valid bearer may renew itself for
   another 30 days through an authenticated native-session endpoint, so normal
   app use keeps the owner signed in. Sign out, allowlist removal, secret
   rotation, or 30 days of inactivity still fail closed and require sign-in.
2. iOS stores the native bearer in the device-only Keychain, not UserDefaults.
   Existing UserDefaults credentials are migrated once; gateway URL, trusted
   machine pins, and selected machine remain non-secret persisted metadata.
3. Reauthentication preserves the selected trusted machine. If it is still
   available, Fort reconnects it automatically. A single new untrusted machine
   still requires the existing fingerprint confirmation before first use.
4. Successful connection dismisses setup. Authenticated startup selects the
   first available open Channel and renders its transcript; the Channel list
   remains the fallback when no Channel exists.
5. The Fort orbital mark always has restrained ambient energy. Working state
   increases its energy and retains explicit textual status, so animation is
   brand presence rather than the sole status signal. Reduce Motion suppresses
   spatial drift, rotation, and scaling while preserving a slow glow pulse.

This spec supersedes the Working-only motion restriction in specs 040 and 044
for the Fort orbital mark. It does not weaken durable status or identity rules.

## Affected files

- `gateway/web/lib/native-token.ts`, `gateway/web/lib/session.ts`, and
  `gateway/web/app/api/native/session/route.ts`
- `ui/apple/iOS/FortApp.swift`, `GatewayCoordinator.swift`, and a device-only
  Keychain session store
- `ui/apple/FortKit/Sources/FortKit/GatewayRelay.swift`
- `ui/apple/FortKit/Sources/FortKit/PrimaryChannelsStyle.swift`
- Gateway, FortKit, and Apple source-contract tests

## Test criteria

- Native-token tests prove the 30-day boundary and reject tampering/expiry.
- Native-session tests prove only a valid, still-allowlisted bearer can renew.
- FortKit checks prove renewal sends a bearer-authenticated POST and decodes
  the replacement token without exposing it in diagnostics.
- iPhone source checks prove credentials use device-only Keychain persistence,
  successful connection dismisses setup, trusted selection survives reauth,
  and startup still opens the first available Channel.
- Motion checks prove idle and Working marks animate, Working is more energetic,
  and Reduce Motion disables spatial movement in both states.

## Rollback

Remove the renewal endpoint and Keychain store, restore the 15-minute token,
restore Working-only motion, and return setup dismissal to manual control. No
server-side data migration is required.

## Release checkpoint

- TestFlight marketing version: `1.0.3`
- Prior TestFlight build: `2608151`
- Combined Agent Channels release candidate: `2608211`
- Release scope: durable native session renewal, direct chat entry after
  connection, ambient Fort mark motion, and the Spec 046 agent-first shell.
- Production gateway deployment: `dpl_EpDbB8HTdmbJovd5VbFHvaiVFExw`, promoted
  to `https://fort-gateway.vercel.app`. An unauthenticated renewal POST returns
  `401`; the root retains its sign-in redirect.
- TestFlight delivery: `7c06f888-d6c3-45ba-a259-a04c76d2528b`. Apple
  independently reports `1.0.3 (2608211)` as `VALID`, available to internal
  testers, and ready for external beta submission.
- The notarized macOS build carries the same `1.0.3 (2608211)` release identity
  and compiles `FORT_AGENT_CHANNELS=primary`; source defaults remain closed.
