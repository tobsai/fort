# First-wave native refinements

The owner requested identity, model choice, animated avatar, settings and image
rendering (message Sentinel_c5a65efc2f1881919230ff550ff5221a). Ordinary source,
fixture and build work is authorized. No new live session, provider access,
persistent credential, deployment, paid service or download is authorized here.

Agent identity uses the existing saved instructions field (16,384 UTF-8 bytes).
Edits create a new binding revision; accepted turns and conversation IDs retain
their prior values. Optional bundled avatar IDs are additive metadata, so older
agents decode unchanged. Settings presents these choices together. Model choices
come from the authenticated connected Codex runtime; unavailable, missing and
failed discovery never replace a saved selection. Selection is checked again at
runtime dispatch; catalog membership is not a quota or effective-model guarantee.

Three bundled native avatars use SwiftUI shapes/system glyphs and obey Reduce
Motion. The native local image module imports owner-selected still PNG/JPEG files
(maximum 5 MB, 16 megapixels and 8,192 pixels per side), normalizes previews, and
stores at most twenty per installation/conversation. Previews are displayed inline
and persist on this device. They do not enter provider payloads, fetch remote URLs
or generate images. The original selected files remain untouched.

The owner superseded the proposed thirty-day pairing default in message
Sentinel_102d9824b0e48191be308694aeb42dc3: pairing should have no default expiry,
with optional expiry settings and straightforward removal/uninstall. The explicit owner-local lifecycle is now implemented and fixture-tested. Actual owner setup must confirm its exact
access scope once; do not activate persistent credentials while implementing it.
Stop/Disconnect preserve history. Uninstall must confirm owned-host shutdown and
revoke pairing before removing the exact app; keep history by default and require
a separate explicit data-removal choice. No login startup, other-device access or
network/tool scope expansion belongs to this slice.

Architecture: TypeScript contracts/service/official runtime adapter remain the
shared backend. Swift/SwiftUI FortKit owns native settings and presentation.
`AgentSettings.swift` owns model-choice presentation; `AgentAvatar.swift` owns
bundled animation; `LocalChatImages.swift` owns device-local previews. Durable agent
configuration remains in the existing SQLite prototype ledger. A separately owned
local host launcher and actual persistent setup remain planned, not installed.

The owner-local pairing mode is opt-in on a fresh ledger and currently appears only
in fixtures. It allows one paired device and persists either no expiry or a chosen
1–365-day expiry; it cannot extend/adopt bounded-demo credentials. Authenticated
stop/disconnect await runtime disposal before successful revocation. Native removal
uses the exact Keychain account, retains history and records incomplete local cleanup
for retry. `OwnerLifecycle` orders host-exit/revocation proof, credential removal and
app removal, with a separate history-deletion choice. Its real owned-host adapter is
still absent, so uninstall remains disabled in ordinary app construction.

Validation: 119 affected service/runtime tests, 50 native tests, strict TypeScript
checking, fresh Mac and iPhone Simulator builds, and strict signature checks pass.
The builds use source `1a3e5bdc4902ba7fdf4c936730174754f6a6b2e6`; the prior
settings/image source is `60675b5fd8b4cc6c9e86cca188ee684628c84e0a`.
Read [the evidence and remaining gates](evidence/2026-10-17-first-wave/README.md).
Installed screenshots/accessibility, file-picker access, real model discovery,
owned-host exit/recovery and persistent activation remain unverified. The fresh
Mac build has no user-selected-file entitlement; no security configuration changed.
Historical demo pins/ledgers remain preserved and are not resealed here.
