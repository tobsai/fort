# Phone access over Tailscale — October 23, 2026

Decision: Toby chose Tailscale in cmsg_01WzifFCZgpREpCjjmswqPPvJBX7hvzCLbWERj7QWLEujF
(2026-10-23 00:01:45 UTC) and asked for builds through his existing TestFlight app
on 2026-10-22. Scope is in spec 059, "Phone access over Tailscale and TestFlight
iPhone identity".

## Source checks (this branch)

- Red first: the new Tailscale-range listener test failed (0 of 2 binds reached the
  listener), then passed after the private-range change. `host/secure-network.test.ts`:
  18 pass, 3 optional budget skips.
- Red first: `CompanionInvitationLinkTests` and the paste-invitation model test
  failed to compile before the type and model method existed; both pass now.
- Strict typecheck passes. Portable suite: 1,426 pass, 16 skipped, 0 fail.
- macOS suite with the synthetic guardian and fresh host resources: 82 pass, 0 fail.
  Its compile lists (`host/guardian.test.ts`, `packaging/sandbox-feasibility.ts`)
  now include `MacNetworkControl.swift`.
- Native FortKit suite (`--no-parallel`, CI fixture variables): 375 pass, 16 optional skips.
- Not run: the signed packaging suite (needs `FORT_SIGNING_IDENTITY`), native HTTP
  journey checks and UI tests.

## Builds

- iPhone: Release archives of `FortiOS` as `io.mtree.fort`, team T3JB5MYZ93, uploaded
  to App Store Connect: 0.1.0 (202610220730, before pairing) and 0.1.0
  (202610221922, with Paste invitation). Upload receipts only; TestFlight install
  and phone behaviour are not yet observed.
- Mac: the installed 310b06a9 app was backed up outside `~/Applications`, then
  patched in a copy: new `FortMac` and `FortHost` executables and FortKit bundles
  from unsigned Release builds, fresh host resources (only `secure-network.js`,
  `main.mjs` and the two manifests differ), helper entitlement
  `com.apple.security.network.server`, app entitlement
  `com.apple.security.files.user-selected.read-only`. Signed with the existing
  Apple Development identity, hardened runtime and embedded profiles;
  `codesign --verify --deep --strict` passes. Installed and launched; it opened.
- A Tailscale certificate for the Mac's tailnet name was created with
  `tailscale cert` in an owner-only folder outside the repository (expires
  2027-01-20). No key material is recorded here.

## Not yet observed

Host activation, owner sign-in, listener start on the Tailscale address, a real
invitation, phone join over Tailscale, and revocation. These need the owner's own
actions on the Mac and iPhone and are reported separately when they happen.
