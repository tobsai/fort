# Future runbook — blocked pending reviewed implementation and owner authority

These are ordered review requirements, not commands to execute from this draft.
Every phase needs an immutable receipt with UTC/monotonic times, input hashes,
outcome and applicable owner/process/path identity. Do not log environment,
tokens, account data, unrelated paths or message bodies. An unavailable check is
unproven. A failed phase prohibits subsequent launch/grant phases; only scoped
inspection, stop and cleanup may continue.

## 0. Accept the exact executable revision and prepare attendance

Require closed pre-approval gates in `REVIEW-GATES.md`, independent acceptance of
the complete source/dependency seal, and the single recorded owner decision for
that exact scope. This draft's false execution flags cannot be toggled into
authority. No launch input may be supplied by an unreviewed script or rewritten
after acceptance. Derived path/boot/device records must come from the sealed
parsers and be separately bound to the session.

At the scheduled window, confirm the same arm64 Mac, supported OS/tool identities,
owner presence, exact UTM candidate paths absent and real non-symlink task-root
ancestors. Existing paths or processes stop the setup without opening their data.
Before allocation require at least 24 GiB budget plus 32 GiB reserve free; check
reclaimable memory against 4 GiB guest plus 8 GiB reserve and observe manager
overhead. Establish exact Darwin temp/cache parent identities and bounded state
attribution before launch. No ambient provider/SSH/keychain discovery is allowed.

Create only fresh owner-private task directories from `scope.json`, journal their
device/inode/owner/mode immediately and fsync the journal. Create the one public
host-canary file and record its digest. No management keys or host credentials.
Use a minimal explicit controller/verifier environment, closing unrelated fds.
Staging has a 30-minute overall deadline, all downloads together 15 minutes,
archive inspection five minutes and verifier execution 90 seconds. Bound each
host command's retained stdout/stderr to 2 MiB. Staging precedes the separate
15-minute attended app window; expiration stops, never starts a partial trial.

## 1. Stage and assess the exact app

Download only `UTM.dmg` from `inputs.json`, enforcing exact size/SHA256, HTTPS
allowlist, redirect/time/byte limits and exclusive destination. Failure diagnostics
are durable and veto acceptance; no automatic retry or overwrite. Verify disk-image
integrity and stapled ticket with existing Apple tools. Preserve the known
outer-container primary-signature result and explicitly bound owner exception;
do not change macOS policy to turn it into acceptance.

Mount read-only, non-browsing and no-autoopen at the exact private mountpoint.
Before copying, match the complete relative app manifest and measured cap. Copy
the unchanged bundle once to the private app path with contents, modes, symlinks,
extended attributes and all legal files preserved. Compare portable manifest
fields (not original disk-image inode/device/packaging owner) and xattr values by
hash, then repeat full/deep/all-architecture signature checks, actual signer/Team
ID/version/build and app Gatekeeper **Notarized Developer ID** acceptance. The
main binary and both architecture CDHashes are pinned in `inputs.json`.

Any rejection, signer/content drift, copy/permission problem or over-cap footprint
stops. No alternate distribution, self-build, re-signing, quarantine edit or
security exception. Record no open mount users; detach only its verified device,
confirm absence and delete only that owned DMG. Retain the private verified app
and evidence. No application launch yet.

## 2. Authenticate and inspect Ubuntu and the temporary verifier

Acquire the other eight exact inputs under their caps, preserving receipts and
rejecting any changed/expired/unresolvable pin. Authenticate OpenPGP.js registry
metadata/integrity with existing Node built-ins before extracting/importing it.
Inspect the tar/import closure and legal material; no package manager, install
hook, dependency fetch or native addon. The private extraction is at most 24 MiB
inside its 32 MiB allowance. Run only the exact reviewed Ubuntu-only verification
entrypoint with no network, writes, child processes, workers or addons. No old
Lima verifier main or unlisted key download is allowed.

Verify Ubuntu's exact primary fingerprint, current key and every detached-signature
result with unchanged library policy. Require one signed checksum entry for
`noble-server-cloudimg-arm64.tar.gz`, matching the packet and computed archive hash.
All four pinned negative-control cases must reject. Store their actual outcomes;
do not infer live crypto success from contract doubles or prior `.img` checks.

Read the authenticated archive as inert data before extraction. The reviewed
examiner must reject unsafe names/links/special/sparse-extension entries, duplicate
or ambiguous selections and excessive expansion. Bind exact raw-disk, kernel and
initrd member names, sizes, digests, raw format/layout/filesystem label and ARM64
Image header. The outer archive pin is exact now; these derived member facts are
intentionally unknown in this draft. The accepted parser must make the selection
deterministic and bounded. Unsupported layout stops before boot—no guessing,
converter, extra kernel, mount of a guest filesystem or alternative image.

Extract the one raw disk once into the package's final `Data/root.raw` path, at
its original logical size <=20 GiB. Extract kernel/initrd within the combined
192 MiB budget; bounded standard-library gzip decoding only if the accepted
kernel format requires it. Account for compressed and decompressed overlap.
No resize, grow, second full copy, QCOW passthrough or host filesystem mount.
Preserve required licensing/manifest material within the evidence allowance.

## 3. Render the reviewed seed and package

Render only sealed synthetic NoCloud user-data, meta-data and network-config.
No secrets or real prompts. The exact module allowlist must suppress key creation,
default interactive users, gettys, package/update/snap activity and filesystem
growth. A locked/no-login UID 1000 and runtime UID/GID 2001 are guest-local workload
identities, not login credentials. SSH/sudo are neither a management channel nor
a readiness dependency. Inspect/mask any image services that would generate keys
or start unauthorized work; inability to establish first-boot ordering stops.

The guest-only root broker/inspector starts only after verified provisioning.
It cannot release runtime/tool work without the single exact control grant.
Install no packages; actual image package versions must match `inputs.json`.
Use only the new no-NIC firewall (loopback allowed, all other input/output/forward
dropped), never the inherited Lima DHCP/SSH rules. Keep runtime/tool/slice disabled
for autostart/restart and verify exact unit semantics under guest systemd before
any attempt. Cloud-init completion alone does not prove these conditions.

Create a CIDATA ISO with existing `hdiutil makehybrid` from only those three
synthetic files. Source plus ISO <=16 MiB; ISO <=8 MiB. It is one internal read-only
virtio block device. The raw root is the only writable disk. Render the package
from the reviewed schema using `vm-design.json`'s exact device policy and recorded
derived member identities. No importable package may be emitted with unresolved
boot/member fields. Validate byte-level config and retain a digest before GUI use.

## 4. Owner launch, in-place registration and one boot

The owner follows `HOST-EFFECTS.md` at the attended time. Launch the exact accepted
private app, decline optional updates, keep Server/template/guest-tools/USB
autoconnect off and accept no new privilege. Ordinary first-open confirmation is
within the eventual explicit scope; a Gatekeeper rejection or unexpected TCC/admin
prompt is not. Never use `utmctl`, AppleScript, Shortcuts or UI automation.

Register only the exact package as a shortcut. Verify its UUID/name and that no
disk was duplicated, then reread rendered configuration and inspect the GUI:
Apple backend; 2 CPUs; 4 GiB RAM; one raw root plus read-only seed; two Ptty ports;
**Network=[]**; no shares/display/audio/clipboard/Rosetta/physical USB/keyboard/
pointer; no saved state or autostart. UTM's default shared NIC makes explicit
configuration and readback mandatory. Stop before Start on any discrepancy.

Start once. Record UTM and VZ process executable identity, PID/start time and
attribution to the exact package. Do not assume a PID or broad process name proves
ownership. Owner supplies both displayed PTYs; the sealed recorder must correlate
their character-device owner/mode/rdev/inode and open handles with these processes.
No guessed terminal, device chmod or competing consumer is allowed. Check no
unexpected host listener/download/permission prompt. First-launch host egress is
not technically confined by this design.

Require readiness within **120 seconds**. Guest-root inspection on serial 0 must
correlate boot ID and fixture/seed hashes, report only loopback NIC, zero shares,
expected mounts/devices/package versions, disabled SSH/keygen/getty/update/grow
services, locked users, correct ownership and unit hashes, supported cgroup v2,
empty attempt cgroup and no consumed record. Verify root positive controls for
every protected synthetic object. Serial ordering and no-login behavior must be
observed, not inferred from `/dev/hvc0` and `/dev/hvc1` names.

## 5. Exactly one bounded synthetic session

Open the two verified PTYs exclusively with no-follow/fstat identity checks and
bounded raw framing. Serial 0 supports only fixed pre/postflight facts; serial 1
supports the fixed protocol in `protocol.json`. No free-form command, shell,
credential or alternate path crosses either channel. A broker/status message
cannot reset the consumed marker or renew the runtime lease.
Both host PTYs must be owned by UID 501 with mode 0600. A different mode/owner
or unsupported exclusivity stops; it never authorizes chmod or relaxed checks.

Run only `prepare(0) → grant(1) → heartbeat(2..20) → stop(21)`.
There are 19 heartbeats, target 250 ms apart, 750 ms host acknowledgement deadlines,
500 ms broker operation budgets and a 45-second host session deadline. Prepare has
a four-second lease; runtime/tool hard limits are 40/38 seconds, watchdog two
seconds and unit stop two seconds. PID 1 owns the entire attempt cgroup. Stop
acceptance also requires an independent empty-tree observation within three
seconds. The original runtime invocation/boot/cgroup must match every operation.

Execute exactly the eleven fixed canary cases, including positive scratch/IPC and
runtime reads plus independently established denied targets. Tool reports are
untrusted observations; guest-root pre/postflight must substantiate identity,
namespace/capability/fd facts, target existence and complete cgroup cleanup.
Observe the setsid descendant's PID/start time and cgroup before normal stop.
Host-side canary bytes must be unchanged. No actual owner/provider/Keychain data
is touched. Do not claim a missing host path proves a permission boundary.

Lost acknowledgement, partial/ambiguous write, unexpected/duplicate frame,
identity swap, out-of-order operation, timer failure, EOF, reboot or failed
canary ends execution authority. Do not resend or replace. Close the control
channel and inspect through the fixed read-only route; if attribution is lost,
proceed to owner stop with outcome unknown. There are no live fault-injection,
second-boot, replacement, snapshot/resume or retry sessions in this scope.

## 6. Stop, reconcile and remove only owned artifacts

Success requires actual positive controls and all denials/settings/lifetime checks,
not just a response or process exit. Record outcome as `passed-subset`, `failed`,
or `unproven/unknown`; all preserve bounded evidence and cleanup obligations.

Ask the attended owner to stop the exact VM within 30 seconds of completion/stop
condition, remove its registry shortcut and quit UTM. The 15-minute attended
window is an operator deadline, not an automatically enforced VM power limit.
An unavailable owner or hung app is a blocker; do not claim the VM stopped.
If explicitly included in the final owner request, force-stop only that positively
identified VM through UTM's normal UI. No kill-by-name or stale-PID fallback.

Before deletion, independently establish inactive guest units/empty cgroups when
reachable, and always exact VM/VZ/UTM process absence and closed owned PTYs/open
handles. A stopped VM can prove process absence, not that earlier effects never
occurred. If termination cannot be proven, leave artifacts quarantined and report
the exact remaining process/state; do not delete its disk or start another VM.

Retain <=128 MiB combined redacted evidence/canaries/raw-plus-copied logs; hash and
verify retained copies. Detach any still-mounted owned DMG normally by verified
device identity. Before removal, match every observed owned path's parent/owner/
device/inode/type/mode and file hash or recorded changed-file identity. Refuse
unknown entries, ownership changes, symlink traversal and mountpoints. Unlink
only recorded owned files using no-follow directory descriptors, child-first;
fsync a per-object journal. Check live mutable VM files only after all writers are
proven gone. Never delete based on a pre-launch hash that is expected to change.

For new sandbox/container state, use normal owner-supported removal only if
ownership is proven. Permission denial or ambiguity leaves an explicit residual;
no broader access or cleanup. Never edit shared registration/TCC/trust/audit
databases. Rescan exact task/state paths, owned processes/PTYS/mounts and host
canary disposition separately. Report each residual and any unproven gate. A
successful run does not activate a provider or permit persistent VM hosting.
