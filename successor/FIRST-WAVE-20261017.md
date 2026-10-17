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
with optional expiry settings and straightforward removal/uninstall. Prepare and
fixture-test that lifecycle separately. Actual owner setup must confirm its exact
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

Validation to date: 114 affected service/runtime tests and 45 native tests pass,
including restart, prior-revision preservation, unavailable-model handling, local
image scope/relaunch and rejection of URLs, symlinks, disguised and oversized files.
Installed screenshots, accessibility, file-picker entitlement behavior, actual
model discovery and persistent lifecycle signoff remain unverified. Existing demo
source/app pins and ledgers are historical evidence and are not resealed here.
