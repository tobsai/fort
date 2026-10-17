# Interactive first-wave QA and review corrections

**Engineering QA only.** The owner requires final delivery through TestFlight. This
local artifact is for operator inspection, not standalone user distribution. Read
[the delivery plan](../../TESTFLIGHT-DELIVERY-20261017.md).

The read-only preview did not expose the requested first-wave controls. A dedicated
Debug-only **FortQA** app now opens an interactive fixture by default. It exercises
the existing client, settings, composer and local-image handlers through a simulated
in-process transport. It has no provider process, network transport, enrollment,
connection record, Keychain access or startup service. Its normal setup entry point
is excluded at compile time.

## Exact artifact and operator route

Run [open-interactive-qa.command](open-interactive-qa.command). It verifies the
recorded app hashes and signature, then opens this exact artifact without creating
another instance of an already running QA app:

`/tmp/fort058-interactive-qa-20261017/Build/Products/Debug/FortQA.app`

- Bundle ID: **`io.fort.qa.firstwave.20261017`**
- Executable/app name: **FortQA**; display name: **Fort QA First Wave**
- Main window title starts **Fort QA**; the conversation title includes the selected
  QA agent. The header labels simulated models/replies and no network.
- The launch script's fixed UUID selects `FortInteractiveQA-<UUID>` under this app's
  temporary directory. Quit/relaunch through the same script retains that QA state.
  A launch without a session UUID creates a separate new temporary session. macOS
  may eventually purge temporary files. Existing Fort storage is never selected.
- The QA signature has app sandbox and Debug task access only. It has no outbound
  network, broad filesystem or user-selected-file entitlement. No OS permissions
  were changed. The ordinary Mac app's entitlement scope is unchanged.

Use the exact bundle ID/path when selecting the app for inspection. Do not use the
older shared `io.fort.candidateb.mac` identity. The old read-only first-wave preview
PID 29922 was rechecked for its exact executable and `--offline-preview` argument,
then stopped with TERM; absence is recorded in `old-preview-cleanup.json`. No other
app or user data was removed. If exact CUA selection still fails, preserve the error
and report the inspection as blocked; do not change OS permissions or claim signoff.

## Inspect the actual controls

1. Open the QA app. Two agents, a welcome message and a bundled geometric PNG are
   visible. A prominent **Settings** button sits above the transcript, alongside
   **Add sample image** and **Reload saved fixture**. No Connection button appears.
2. Open Settings → QA Writer. Edit its name and identity, choose **QA Beta**, select
   another avatar and Save. Send text. The deterministic reply shows the saved
   identity, exact simulated model and entered text, labeled as simulated.
3. Reopen the editor. Verify saved identity/model/avatar. Use **Reload saved fixture**
   and then quit/relaunch with the same script to inspect persistence.
4. The **Catalog scenario** picker is available both in Settings and the agent
   editor. **Saved model missing** removes currently saved model selections from the
   catalog; **Discovery failure** returns an error. The saved selection remains visible
   as unavailable. **Available + unavailable models** restores the simulated catalog.
   QA Reviewer starts on the explicitly unavailable QA Retired model. No real model
   availability or dispatch is established by these controls.
5. Remove the image preview; add it again with **Add sample image**. This invokes the
   same image import path as a chosen local file. Repeat until the 20-image limit if
   desired. The bundled 240×144 PNG is generated locally, contains no user content,
   and is never included in message/provider payloads. JPEG/orientation coverage is
   in native fixtures. External file-picker access is still a separate acceptance
   gate and has no new entitlement here.
6. Inspect labels, keyboard operation, avatar animation/Reduce Motion and screenshots
   of these exact windows. Source and unit tests do not replace that UI inspection.

The fixture persists only QA agent configuration, snapshots, pending requests,
catalog scenario and image previews. Responses and model names are deterministic;
identifiers distinguish separate temporary sessions. Unimplemented routes fail
locally and never fall back to network. Pairing/uninstall are not activated by QA.

## Review corrections

- **Shutdown retry:** stopping dispatch/schedules is separate from closing management.
  Failed runtime disposal leaves authenticated management reachable. Another explicit
  HTTP disconnect can succeed; new work/enrollment/restart stays blocked. Concurrent
  stop requests are rejected while confirmation is pending. Model discovery does not
  contact a stopped runtime.
- **Nonregular image input:** descriptors open with `O_NONBLOCK` before `fstat` rejects
  FIFOs and other nonregular files. The FIFO regression releases an old blocking
  implementation after 500 ms and asserts rejection before 400 ms, so it cannot hang
  the suite. Cache-folder reuse was also fixed: adding a second image or importing
  after removal previously failed on the existing directory.
- **Image coverage:** JPEG orientation normalization, animated PNG rejection,
  per-side/pixel limits, twenty-image capacity and reuse after removal now pass.
  Normal image content has an accessibility label, not a decorative-only image.
- **Removal recovery:** disconnect intent is saved before the service request. A lost
  reply or expired credential leaves durable, explicitly unconfirmed removal and
  blocks automatic reconnect/replay. Explicit retry can use retained service access
  or an owned-host recovery adapter. Recovery proof must match the exact connection
  and confirm host stop/revocation. An explicit **Forget local credential only** action
  removes local credentials while preserving the unconfirmed service status and
  history. It never claims revocation or host shutdown.

The real installed `OwnedMacInstallation` adapter is still absent. Recovery and
uninstall ordering are tested with an injected fixture adapter; they are not a working
installed-host recovery/removal system. Actual persistent access, host installation,
revocation recovery and app removal remain gated. No persistent credentials or actual
app removal were performed. No-expiry by default and keeping history are already
owner decisions; this work does not request those decisions again.

## Revisions and validation

All older reviewed commits are preserved. The source sequence is:

- `e30f116`: shutdown retry and nonblocking image/cache fixes.
- `eeb88f4`: interactive QA target/transport plus durable removal intent/recovery.
- `14342d1`: compile-time separation of QA from normal setup.
- `09788c1`: saved-model scenario correction and image accessibility label.

Final source: `09788c132647458cbdc683506c8bd5622f0be1af`.

120 affected service/runtime tests and 56 native tests pass. Strict TypeScript and
Git whitespace checks pass. The dedicated QA app, ordinary Mac app and iPhone
Simulator app build successfully; FortKit also compiles in Release with QA excluded.
Strict signature verification and hashes are recorded in `checks.json`.

Expected red tests reproduced the shutdown retry rejection, FIFO wait, existing image
cache-folder failure and missing-model scenario tied to the wrong model. New removal
intent tests first failed to compile before their seam existed. Their final passes
cover lost responses, expired credentials, persistence without credential reads,
explicit local forget, rejected shutdown/scope proofs and successful fixture recovery.
The initial process-inventory command was blocked by the command sandbox; supported
escalation succeeded. There was no automatic approval rejection or OS permission
change. Existing CoreSimulator version warnings remain; no Simulator UI was launched.

Installed accessibility/screenshots, actual picker access, real model catalog/dispatch,
physical iPhone and real owned-host lifecycle remain unrun. The new QA app was built,
not launched by this implementation task. The operator should now inspect it using
this unique identity. No public publication, deployment or provider cleanup occurred.
