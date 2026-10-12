# Proposed UTM synthetic isolation experiment

**Proposal only. No installation or execution is authorized.** Recommend the
official free UTM 4.7.5 build 118, Apple Virtualization backend, one Linux guest
with no network adapter, and two owner-scoped serial PTYs. The owner operates the
UTM GUI; Fort's eventual fixed controller uses the serial channel. This avoids
adding Apple Events authority, network bridging, SSH credentials or a converter.

The [accepted review](../../../reviews/gatekeeper-8c9129b/REVIEW.md) closes the Lima
recovery evidence while holding execution. Lima remains blocked; its approval
does not cover this different manager, image format, transport or host footprint.
This proposal changes no historical evidence, implementation, spec or approval.

**Managed execution remains a plausible bounded experiment, not a verified
capability.** PID 1 can still own the runtime/tool lifetime and consumed-attempt
fence. The PTY adapter and guest management service must first be implemented,
tested and independently reviewed. Owner-operated start/stop is a deliberate
limitation: this does not establish unattended VM lifecycle management or T07.

## Candidate and authenticity gates

| Input | Exact candidate |
| --- | --- |
| UTM | [Official v4.7.5 DMG](https://github.com/utmapp/UTM/releases/download/v4.7.5/UTM.dmg), build 118; release ID 273984640, asset ID 335896119; observed 250,021,057 bytes; cap 256 MiB |
| UTM SHA256 | `a8435c93cfb5f8bbfeea4b134cfad1ac66b67632b75e438c63b1a8ae043bef0e` |
| UTM source | `048ca7498ea3a374439149d51739d94c5300bcda`; this identifies inspected source, not a reproducible-build attestation |
| Ubuntu | [Noble 20260926 arm64 filesystem/kernel archive](https://cloud-images.ubuntu.com/noble/20260926/noble-server-cloudimg-arm64.tar.gz), cap 768 MiB |
| Ubuntu archive SHA256 | `1800eb56b6b839a08c020551e16fbd8c95b2772452ecce3737a9ad25fe594126` |
| Ubuntu publisher | Exact primary fingerprint `D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`; exact prior checksum/signature/public-key/manifest pins retained |
| Private verifier | OpenPGP.js 6.3.2 and existing Node 26.5.0; same registry/integrity/default-policy verification, no package manager, hooks or dependencies |

[scope.json](scope.json) lists all nine proposed URLs, digests and byte caps.
Neither new payload was downloaded. The Ubuntu checksum text was read and matches
the previously verified metadata digest; that is not verification of the new
archive. The [official image index](https://cloud-images.ubuntu.com/noble/20260926/)
describes this tar as filesystem and kernel data. Its actual member names, raw
disk, root label, kernel and initrd remain uninspected.

After new approval, use a bounded downloader adapted from the accepted diagnostic
implementation: exclusive files, approved HTTPS redirects only, no credentials or
proxy, exact pins/caps, immutable terminal receipts, failure observations that veto
acceptance, and no automatic retry. Retain those behavior regressions. Do not run
the old downloader with renamed files or alter its sealed scope.

For UTM, require the exact hash, then the following **inspection commands only**
before any application launch. Here `DMG`, `APP` and `MOUNT` denote the concrete
owned paths in scope; these are instructions for a future reviewed runbook, not
commands executed by this preparation:

1. `hdiutil verify "$DMG"`; `xcrun stapler validate "$DMG"`;
   `spctl --assess --type open --context context:primary-signature --verbose=4 "$DMG"`.
2. Mount read-only with `hdiutil attach -readonly -nobrowse -noautoopen -mountpoint "$MOUNT" "$DMG"`.
   Inventory before copying; no installer or bundled script runs. Copy only the
   unchanged app bundle into the private task root, preserving signed contents,
   modes, symlinks, legal files and extended attributes. No `/Applications` write.
3. Inspect Info.plist for version/build/identifier; verify nested code and the
   complete app with `codesign --verify --deep --strict --verbose=2 "$APP"`.
   Record `codesign -d --verbose=4 --entitlements :- "$APP"`, public certificate
   chain, TeamIdentifier and relevant nested helper signatures/entitlements.
4. Require `spctl --assess --type execute --verbose=4 "$APP"` to accept the actual
   copied app as notarized Developer ID software. Keep all OS checks in force;
   errors/rejection stop, with no launch experiment, re-signing or quarantine edit.

The [versioned release workflow](https://github.com/utmapp/UTM/blob/048ca7498ea3a374439149d51739d94c5300bcda/.github/workflows/build.yml#L299-L374)
packages via Developer ID, notarizes and staples the DMG. **Actual certificate
subject/Team ID and ticket are unknown.** Intended provenance is the official UTM
distribution pinned above, not any arbitrary Developer ID signer. Turing Software,
LLC is the [App Store seller](https://apps.apple.com/us/app/utm-virtual-machines/id1538878817);
that is not a certificate pin for the free DMG. Resolve and record the publisher
identity from the accepted artifact before a first-launch decision; unexplained
identity, missing ticket, altered code or additional authority requires review.

Source entitlements include app sandbox, application group, user-selected file
read/write, network client/server, virtualization, USB, audio input and VM device/
network access. These are broader than this guest's configuration. Inspect the
actual bundle and nested helpers rather than promising a VZ-only binary. The full
bundle, including QEMU components, is trusted software; QEMU VM execution and image
conversion are excluded. No building UTM, Homebrew QEMU or helper download.

## Minimal VM and provisioning design

[vm-intent.json](vm-intent.json) is design data, **not importable UTM configuration**.
The [Apple drive documentation](https://docs.getutm.app/settings-apple/drive/)
requires raw disks. The inspected import path copies Apple drive bytes without
converting QCOW2. Do not reuse the prior `.img` as though it were raw.

Use the proposed signed Ubuntu tar only if inspection confirms one usable raw
root disk, matching ARM64 kernel and initrd. Direct Linux boot is documented by
[UTM](https://docs.getutm.app/settings-apple/boot/) and implemented in the pinned
boot configuration. Require ARM64 Image format (bounded standard-library gzip
decompression if applicable) and the verified filesystem label before selecting
`console=hvc0 root=LABEL=cloudimg-rootfs rw`. Unexpected layout, compression or
missing components stops; no alternate image, QEMU conversion or kernel download
is an implicit fallback. This boot combination is not yet tested.

Inspect every archive member before extraction: reject absolute/traversing or
duplicate paths, links and special entries; enforce the one-disk and auxiliary
expanded-byte limits. Extract only reviewed members as inert files, and do not
mount the guest filesystem on the host. Required legal/manifest material remains
preserved within the bounded staging record.

Keep the raw disk at its original size, at most 20 GiB; no resize, snapshot or
second full disk. Extract directly into the recorded package data location and
move/rename within the same owned tree. Register that package as a shortcut,
without UTM copying it into default storage. The pinned import code supports that
path; the actual owner GUI flow must be confirmed before allocating/importing.

Attach one small **read-only internal virtio block** ISO labeled `CIDATA`, created
using existing macOS `hdiutil makehybrid` from reviewed synthetic files. Include
only NoCloud user-data, meta-data and network-config. NoCloud's
[local seed mechanism](https://docs.cloud-init.io/en/latest/reference/datasources/nocloud.html)
supports provisioning without guest network. Exact seed rendering, module order,
ISO recognition and installed guest content must be validated; no installation
claim follows from this documentation.

| Area | Proposed setting |
| --- | --- |
| CPU/RAM | 2 vCPUs, 4 GiB guest RAM; app/VZ overhead additional and observed |
| Network | Zero NICs from first boot; no NAT, bridge, DHCP, forwarding, host listener or network management |
| Devices | No display, keyboard, pointer, audio, clipboard, Rosetta or host directory shares; entropy enabled, balloon disabled |
| USB | No physical attachments/autoconnect; UTM adds a virtual xHCI controller on macOS 15+, which must be disclosed and inspected |
| Serial 0 | Bounded boot and fixed pre/postflight evidence, expected guest `/dev/hvc0`; no login shell |
| Serial 1 | Exclusive fixed controller protocol, expected `/dev/hvc1`; no arbitrary commands |
| Guest administration | Trusted root provisioning/management service; UID 1000 `fort-admin` locked, nologin, no sudo; no interactive admin channel |
| Workloads | Preserve runtime UID/GID 2001, no login/sudo; separate systemd dynamic tool UID and existing canary restrictions |
| Keys/accounts | No management SSH keys, guest SSH keys, authorized keys, passwords or provider credentials; disable SSH/key generation |
| Packages | Image-bundled packages only; exact prior required versions rechecked against the actual filesystem; no updates or installation |

The guest root bridge is a **new, guest-only boot service**, replacing SSH/sudo
transport. It may emit bounded readiness/inspection facts; it may not start the
runtime/tool until the single exact grant. Workload units retain no automatic
start/restart, persistent consumed-attempt fencing, original invocation checks,
4-second preparation lease, 2-second watchdog, 40/38-second runtime/tool limits,
bounded stop and empty-cgroup requirement.

The fixed tool still shares the guest's read-only OS root. This does not implement
a private root filesystem per tool or prove a real provider-credential boundary.
The Mac agent/manager is trusted host software; the proposed computer sandbox is
the guest plus its measured UID, namespace and cgroup restrictions.

## Serial authority, proof and limits

UTM documents [PTY serial support](https://docs.getutm.app/settings-apple/devices/serial/).
Its pinned implementation uses `openpty`, raw terminal mode and VZ file-handle
serial attachments. This is a supported transport building block, not proof of
the proposed controller composition.

Before opening any device, correlate the exact package UUID, UTM executable/PID
and VZ process identity, displayed PTY path, device type/owner/mode and open handles.
Do not guess `/dev/ttysNNN`, follow a substituted path, chmod a device, or attach to
an unrelated terminal. Require exclusive access and a startup hello bound to this
VM, guest boot ID and reviewed fixture identity; then bind the controller's unit
invocation/cgroup identity. A nonce provides correlation, **not cryptographic
authentication**. The signed-in Mac owner and same-user processes remain trusted.
No protection against a compromised host manager, guest root or same-UID host
process is claimed. This is not shared or multi-user hosting.

Author tests first for framing/echo/partial I/O, wrong or stale PTY/device identity,
wrong boot/invocation, bounded output, timeout/hangup, no ambiguous resend and
cleanup uncertainty. Separate the fixed inspection channel from execution control;
inspectors accept no shell text. The existing SSH host-session is not silently
reused. Confirm guest tools cannot open either management device or root paths.

Before the one nominal `prepare → grant → 19 heartbeats → stop`, require actual
package versions, root-owned unit/config hashes, no NIC/shares/credentials,
systemd semantic validation, correct UIDs/namespaces/capabilities and inactive
workload units. Afterwards independently inspect unit state, journals and the
entire owned cgroup, including the setsid descendant. Receipts alone are not
independent proof. Full crash/fault coverage, real runtimes and T07 remain open.

## Owner steps, host footprint and network

The owner must schedule a brief attended desktop session, launch the accepted app,
decline optional updates and USB attachment, open the exact package as a shortcut,
review configuration, start it once, identify the two displayed PTYs, and later
stop/remove the exact VM and quit UTM. No surprise foreground action during other
work. Owner interaction avoids requesting Automation/Accessibility grants.
[utmctl uses AppleScript](https://docs.getutm.app/scripting/scripting/); it, Shortcuts,
AppleScript, global symlinks and PATH edits are excluded. A newly encountered TCC,
admin, microphone, USB, network-access or other permission prompt stops for review.

New tooling/data go under
`/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`.
UTM also writes its normal sandbox container, preferences, VM registry/bookmarks,
cache and possibly a group container. Changing `HOME` does not isolate these.
The expected app container is `~/Library/Containers/com.utmapp.UTM`; derive exact
group IDs from the accepted signed entitlements before launch. Require every
affected UTM-specific location absent, or stop to protect existing state. Current
metadata-only checks found the app at neither standard Applications path, and
the proposed root and expected container absent. These are not a whole-host audit.

UTM's [privacy policy](https://mac.getutm.app/privacy/) and pinned release helper
disclose a first-launch release-note request to `api.github.com`. Optional update
checks must be disabled; do not claim that this prevents the release-note request.
The proposal allows that public metadata request and existing Apple OS trust/
notarization traffic, separately from the nine public downloads. No account sign-in.
The app retains outbound network capability; this plan adds no host firewall and
does not prove the absence of other host traffic. Guest NIC absence is a different
boundary. Unexpected app downloads, server listeners or traffic stop continuation.

## Resource, cost and cleanup bounds

Preserve the original ceiling: **20 GiB for one raw disk plus 4 GiB auxiliary,
24 GiB total**, 2 vCPUs and 4 GiB guest RAM. This is a planning/observed-use bound,
not an OS quota. The 4 GiB includes 2 GiB app, 256 MiB DMG, 768 MiB Ubuntu tar,
32 MiB verifier, 192 MiB kernel/initrd/scratch, 16 MiB seed, 128 MiB all raw/copied
evidence and 512 MiB app state/cache/temp, leaving 144 MiB. Measure the mounted app
before copying; over-cap data or a second raw disk stops. Preserve at least the
previous 32 GiB host disk reserve and 8 GiB RAM reserve beyond planned guest use;
recheck observed host overhead between phases. Proposed boot/readiness deadline
120 seconds; one bounded session; stop the VM within a 15-minute attended window.

Free distribution: no purchase or paid service. Costs are local bandwidth, disk,
memory, CPU, power and owner attention. No startup service, kernel extension,
host account/sudoers change or installed package manager.

Before deletion: observe exact guest units/cgroups stopped, owner stop the exact
VM (force-stop only that verified instance if necessary), quit the fresh UTM app,
prove owned UTM/VZ/helper processes and PTYs closed, and detach the recorded
read-only DMG. Never kill by a stale PID or broad name match. Unknown termination
means quarantine and report, not deletion or retry.

Retain bounded redacted evidence, then remove only manifest-owned files, keys
(none expected), app, VM/seed/disks, downloads, new container/preferences/cache
and temporary paths, after identity checks. No broad container/group deletion,
UTM factory reset, TCC reset, host cache purge or unrelated provider cleanup.
Sandbox-container deletion may require an OS-supported owner action; if blocked,
leave it reported rather than seek Full Disk Access or bypass protection. macOS
LaunchServices/Gatekeeper/log databases may retain shared registration/audit
metadata; do not edit shared databases or promise zero OS residue/secure erasure.

## Decision boundary

[APPROVAL-SCOPE.md](APPROVAL-SCOPE.md) is a draft for the parent to present after
review. [UNVERIFIED.md](UNVERIFIED.md) lists the actual unresolved gates. A proposal
seal identifies these documents only; it is not an execution seal. No approval
question was sent, no downloaded executable/image was fetched or run, and no
VM, key, host directory outside scratch/worktree, permission or service was created.

The smallest concrete next owner decision is the separate
[artifact-assessment scope](ASSESSMENT-APPROVAL.md): only the pinned DMG, existing
Apple assessment tools, bounded staging and cleanup, with no app launch, image
download or VM. That resolves actual signer/entitlement/footprint facts before
investing in the source adaptations or presenting a synthetic execution request.
