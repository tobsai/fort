# Review and proposed execution sequence

**Revision 3 is preparation only and must not retry the download stage yet.** The
owner approved revision 2; that run stopped at a public-key checksum mismatch and
was cleaned up. The independent review found the original export pin still matches.
The cause of the historical mismatch is unknown. Preserve its closed evidence and
all original pins. After independent review of this diagnostics correction, record
recovery under the existing authority as specified in [RECOVERY.md](RECOVERY.md).
No synthetic session has started. Commands below remain gated operations.

## 0. Static checks available now

From `/tmp/fort-core-openmaus-20261011/successor/ops/isolation-probe-v1`:

```sh
python3 -B test_validate.py
python3 -B test_corrections.py
python3 -B test_fetch_public.py
/opt/homebrew/opt/node@26/bin/node --test test-verifier.mjs
python3 -B validate.py
/bin/sh -n guest/install.sh
/usr/bin/ruby -rjson -rpsych -e 'abort "YAML mismatch" unless Psych.safe_load(File.read("lima.yaml")) == JSON.parse(File.read("lima.yaml")); puts "YAML and JSON agree"'
```

The validator checks source/config identity, fixed scope, unit policy, Python/JSON
syntax and the complete sealed file set. It does not claim Lima/systemd/nftables
semantic validation. `python3 -B validate.py --require-executable` deliberately
returns 2 even when static checks pass: this revision grants no execution authority.
Changing a blocker to “resolved” requires a new reviewed seal, not editing a flag
to make an execution guard pass. No source generator evaluates guest scripts.
Correction tests use temporary synthetic files, mocked systemd/socket calls and
`ssh -G` without connecting. Verifier tests execute only Fort wrapper logic and
Node built-in crypto; they do not import OpenPGP.js. Actual library/guest checks
belong inside the proposed staged approval.
The downloader tests run its entrypoint over an in-memory urllib HTTP transport,
including the real redirect/error handlers, with synthetic temporary files. They
make no socket connection and do not fetch or execute a public response.

## 1. Ownership, dependencies and storage preflight

Inspect, without altering, the following exact proposed roots. They must be absent,
or match a prior recorded partial run of this exact packet. An unrelated or
unattributed existing root is a blocker, not something to delete or chmod:

| Root / child | Proposed use and future permission |
| --- | --- |
| `/Users/tobiasgunn/Library/Application Support/FortIsolation` | Owner-only task root, 0700 |
| `tools/lima/2.2.1` | Verified archive extraction; preserve legal files and archive executable modes |
| `tools/openpgp/6.3.2` | Exact verifier extraction, including LGPL legal files; no hooks/build/install scripts |
| `downloads` | Only the twelve entries in `scope.json`; no package-manager cache |
| `home`, `home/.config`, `home/Library/Caches`, `cache`, `tmp` | Empty private manager home/config/cache/temp, 0700 |
| `trust` | Pinned guest public `known_hosts`; private directory 0700, files 0600; no GPG keyring needed |
| `evidence` | Bounded raw/redacted receipts, no private keys or provider data |
| `evidence/download-diagnostics` | Fresh exclusive 0700 directory; per-input receipts and optional failed public-key prefix, 0600; at most 256 KiB within the existing 128 MiB evidence allowance |
| `/Users/tobiasgunn/Library/FortIsolation` | Shorter owner-only state parent, 0700 |
| `state` | Fresh `LIMA_HOME`, 0700; only `_config` and the one selected instance plus documented Lima internals |

These permission changes are future **new task-owned files/directories only**.
Do not change any existing parent, home, repository, cache, SSH, sudoers or host
security setting. Reject symlinked owned paths or ancestors that redirect the
scope. Snapshot the exact created-path list and ownership for later deletion.
Require the loopback port 22058 to be unused; do not kill its existing owner.

Host runtime dependencies are the pinned archive and existing macOS VZ/frameworks,
OpenSSH, standard shell/file utilities, Python 3, existing Node 26.5.0 and
codesign/spctl, plus the explicitly pinned private OpenPGP.js bundle. `manager-env.py`
uses an explicit environment and PATH containing the private manager plus system
directories. No QEMU is installed or discoverable on that path. The pinned proxy
disk utility tries QEMU, then its native converter only when QEMU is absent; inspect
actual child processes and stop if any other helper is required. No Docker, Rosetta,
socket_vmnet, npm/pip/brew, additional guest-agent archive, container or startup
service is part of the dependency set.

The guest uses only image-bundled packages enumerated in `scope.json`. Read the
authenticated image's package inventory after boot and compare every selected
version. There is no `apt install`, update, snap refresh or internet fallback.

Storage accounting, before any allocation and between phases:

| Component | Conservative allowance |
| --- | ---: |
| Final raw guest disk **or** its native conversion temp file, renamed into place | 20 GiB |
| Downloaded image, possible manager cache copy, instance conversion input; each at most 768 MiB | 2304 MiB |
| Compressed manager archive | 64 MiB |
| Extracted manager including legal/support files | 512 MiB |
| OpenPGP.js verifier archive / complete extraction with legal files | 8 MiB / 24 MiB |
| Generated ISO and expanded cidata combined | 256 MiB |
| Raw logs plus copied/redacted evidence | 128 MiB |
| Other temp, cache, EFI, keys and metadata | 512 MiB |
| Auxiliary subtotal / remaining margin | 3808 MiB / 288 MiB |

Do not count sparse-file logical space as free budget. Inventory both logical and
allocated sizes, hard links and all temporary copies. The pinned native converter
creates a destination-side temporary raw file then renames it; there is no existing
raw disk to duplicate because the instance is fresh. Reject external QCOW backing
files, snapshots/external data files, virtual image size over 20 GiB or unexpected
conversion/copy behavior. If another 20 GiB copy is needed, stop. No snapshot/backup
of the disk is authorized. Require 24 GiB free at preflight plus the host's existing
operational reserve; do not consume that reserve. Recheck available and aggregate
space during download, expansion, conversion and boot; stop before a phase whose
conservative peak exceeds the envelope.

This is a conservative **planning and observed-use bound, not an OS disk quota**.
A dedicated quota/volume would require a different approved design. Guest RAM is
4 GiB; VZ/manager host overhead and CPU/power use are additional and must be observed.
No unsupported promise about total host memory or utility cost is made.

## 2. Public downloads, authenticity and package inspection

Only the twelve exact URLs/digests/max sizes in `scope.json` are allowed. Public
HTTPS uses the Mac's existing DNS and trust store. Permit redirects only to the
listed HTTPS domains; no credentials, cookies, auth header, netrc, proxy or signed-in
CLI is used. Verify downloaded content and actual final URL. No downloaded code
executes before its staged integrity/inspection gates. No curl-to-shell, build
hook, npm/brew/pip installation or package-manager dependency resolution is allowed.

The authored downloader uses that fixed allowlist, an empty proxy handler, an
explicit existing public `/etc/ssl/cert.pem` CA bundle, exclusive new output files,
at most four allowlisted HTTPS redirects, streaming byte caps and exact SHA256/SHA512
checks. If that CA bundle is absent/untrusted, stop; do not substitute a fetched
bundle or provider identity. Its only future invocation is:

```sh
/usr/bin/python3 -I fetch-public.py
```

Do not replace it with curl-to-shell or a package manager. Its output explicitly
records that checksum matching has not verified a publisher signature. The
selected verifier is still uninstalled. It may execute only after its own registry
signature/archive/extraction gates pass; Lima follows publisher verification.
Revision 2 downloaded seven matching inputs then stopped on the eighth input's
pin. Revision 3 has only run against synthetic in-memory responses. Do not seed
its inputs from the separate integrity review's diagnostic copies.

Before each next request, the downloader fsyncs the current input's exclusive JSON
receipt under `evidence/download-diagnostics`. It records the exact input name and
URL, expected/observed digest, observed byte count, completion state, HTTP status,
bounded Content-Type/Content-Length and final response URL. Redirect query strings
are omitted unless the URL is exactly the approved original public URL; cookies,
authentication headers and arbitrary headers are never recorded. A receipt reports
only download integrity, never publisher verification.

On failure of `lima.pub` or `ubuntu.pub`, at most the smaller of 64 KiB and that
input's existing byte cap is retained as a 0600 `*.untrusted.bin` prefix. It is inert
evidence: never execute, import, render as HTML, normalize, accept as a key, or move
it into `downloads`. Receipts distinguish full returned bytes from incomplete or
prefix-only evidence. For a declared oversize response, no body is read. A streamed
overflow reads at most one extra byte to prove the bound was exceeded, without
writing it to the download or retained prefix. Non-key payloads get metadata only.
Any failure still stops before the next URL. A receipt-write failure stops and
attempts removal of only the just-promoted owned input; bounded metadata also goes
to stderr if the receipt cannot be written. A cleanup error is retained separately
in the failure receipt, preserving the observed digest and body. Any remaining
partial input or incomplete diagnostics stay quarantined for review and scoped
cleanup; they are not accepted input or permission to continue.

Existing downloads, partials or diagnostic directories are never overwritten or
resumed, and symlinked directories/files are not followed. Publication uses an
exclusive hard link to the completed private partial, then removes that partial;
there is no extra payload copy. Final input presence plus its receipt and all
subsequent original gates are required. Inventory the diagnostic files and include
raw and copied evidence in the unchanged 128 MiB budget and owned cleanup manifest.
At most twelve 16 KiB receipts and one failed 64 KiB key prefix occupy 256 KiB.

Required trust chain:

1. Follow [VERIFIER.md](VERIFIER.md): verify pinned registry metadata/public-key
   hashes and both ECDSA signatures using existing Node crypto; match the exact
   OpenPGP.js 6.3.2 tarball SHA512 integrity. Inspect all archive paths, complete
   contents and 24 MiB extraction bound, preserve legal files, and extract privately.
   Inspect its declared bundled Node ESM entrypoint; run no package hooks. Only
   then may `python3 -B verify-offline.py` import the verifier. Public keys stay in
   memory; no owner GPG home, private key or key generation is involved.
2. Verify Lima `lima.sig` over exact `lima.sums`; require valid signature chained to
   `C020EA876CE4E06C7AB95AEF49524C6F9F638F1A`. Verify Ubuntu `ubuntu.sig` over exact
   `ubuntu.sums`; require `D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`. Reject missing,
   bad, expired/revoked, unrelated or unresolved signatures. The sealed wrapper
   uses the library's default policy, awaits every detached verification, and
   validates the primary/signing keys at current time. Run the changed-checksum,
   changed-signature and unrelated-public-key negative controls before acceptance.
   Any incompatible key/signature policy stops the operation; do not weaken it.
3. Match both signed checksum entries and this packet's pinned SHA256 values for
   the exact archive/image. Also match all public metadata digests. A changed
   public-key export is a re-review, not an automatic key update.
4. Inspect the archive table before extraction: relative paths only, no traversal,
   escaping links, devices, setuid/setgid files or unexpected installer hooks.
   Bound total extracted bytes to 512 MiB. Preserve upstream LICENSE/NOTICE and
   third-party files. Extract only within the fresh private manager directory.
5. Before running the manager, `/usr/bin/codesign --verify --strict --verbose=2
   <manager>` and inspect `/usr/bin/codesign -d --entitlements :- <manager>`.
   Require the selected official VZ entitlement and no unexplained extra authority.
   Record signing identity and gatekeeper/quarantine assessment via existing
   `/usr/sbin/spctl --assess --type execute --verbose=4 <manager>`. Do not remove
   quarantine, re-sign, grant TCC/Full Disk Access or bypass an OS refusal. Ad-hoc
   signing proves neither publisher identity nor notarization.

Public trust references: [Lima governance](https://lima-vm.io/docs/community/governance/),
[immutable release assets](https://github.com/lima-vm/lima/releases/expanded_assets/v2.2.1),
[Ubuntu image verification](https://ubuntu.com/docs/public-images/public-images-how-to/verify-image-checksum/).

## 3. Create, review and boot without a synthetic grant

Only after steps 1–2 pass:

```sh
python3 -B manager-env.py validate
python3 -B manager-env.py create
python3 -B manager-env.py inspect
```

Review the actual instance YAML against `lima.yaml`, with defaults resolved. Check
all paths, users, mounts, devices, cloud-init inputs and templates/overrides. Record
newly generated files and new management-key permissions without printing the key.
`create` may prepare state or disks; it is a mutating command and is not run now.
No blanket “yes” input is supplied. Unexpected interactive choices stop the run.

```sh
python3 -B manager-env.py start
python3 -B manager-env.py inspect
```

Boot timeout is 120 seconds. No Fort units start automatically. Inspect the owned
VZ process, listener, generated media/files and cloud-init results against
`CLOUD-INIT.md` before releasing work. The public config describes the expected
media, not an already rendered ISO. Lima's inspected API does not provide this
packet a proven “render media but do not boot” command; if pre-boot media inspection
is mandatory, this remains a design blocker. Do not invent a dry-run flag.

Require the provisioning install marker, exact bundled package versions, cgroup v2,
successful `systemd-analyze verify`, effective root-owned units, nft rules, no
unapproved process/download/package, no host mounts, and fresh guest identities.
Inspect `systemctl cat/show` and `systemd-analyze security` output; a score is not a
confinement proof. Record whether SSH packets actually originate at the expected
gateway. A mismatch is a stop, not a broader firewall rule.

Pin the public guest host key using the local serial output procedure, then verify
strict SSH with the options in `host-session.py`. Inspect actual effective sshd
configuration; password/root login and forwarding must be disabled. Do not use
`limactl shell` as the subsequent Fort control transport. A bootstrap failure leaves
the VM quarantined for inspection/stop; it does not authorize extra packages.

## 4. One bounded synthetic session

The copy manifest is `fixture-manifest.json`; it binds each authored fixture to
its exact guest destination and content. After the preceding gates pass:

```sh
python3 -B host-session.py
```

This fixed script prepares and grants one attempt, sends 19 validated heartbeats
over the same strict SSH session, then requests stop. It never substitutes another
attempt, retries an ambiguous grant, accepts an arbitrary executable, or reads a
provider configuration. The fresh guest cannot run this session twice; a consumed
attempt is a deliberate fence. Do not delete its records to rerun it.

Record preparation/grant/cleanup identities, runtime canary access, tool canary
denials, positive `/tmp` control, dynamic UID, namespace/cgroup/capabilities and
descendant PID. Require the exact runtime and tool inactive and the whole recorded
attempt cgroup empty, including the setsid descendant. Capture bounded journal
entries and the cgroup inspection independently. The stdout cleanup receipt alone
is not independent evidence.

This packet covers one nominal bounded prototype session. The full review matrix
(controller/runner deaths before and after acknowledgement; streaming/stop crashes;
high/low inherited FDs; sibling filesystem/IPC/signaling; management spoofing;
channel loss; boot identity changes) is **unrun and not automatically authorized
as additional replacement attempts**. Seal those fault cases before a broader
test request. This session alone cannot accept T07 or activate providers.

## 5. Stop, reconcile and preserve bounded evidence

```sh
python3 -B manager-env.py stop
python3 -B manager-env.py inspect
```

If orderly stop fails, the same exact owned instance may use the reviewed
`python3 -B manager-env.py force-stop`. Reconcile its current PID/boot/instance
identity first. Never kill a stale PID or an unrelated process. Require Lima state
stopped, the recorded host-agent/VZ process absent, all exact instance sockets
closed and loopback port 22058 released. Process/listener checks must use the
captured identity, not names alone. Failure to prove termination blocks deletion
and any new work; keep the state quarantined and report it.

Retain configuration/content hashes, package/signature results, redacted generated
media inventory, resource accounting, unit properties, synthetic receipts and
independent absence evidence. No private keys, guest private host keys, real message
text, owner ledger or credentials belong in evidence. Cap aggregate raw plus copied
evidence at 128 MiB. Copy only reviewed redacted receipts back to the isolated
worktree; never include a disk, home directory or raw SSH key in a source commit.

## 6. Exact owned-path teardown

After termination and evidence verification, review the recorded created-path
manifest and its original owner/inode identity. Remove only:

- The one instance directory under the shorter `LIMA_HOME`, its disk/conversion
  remnants/EFI/media/logs/control sockets and newly generated `_config/user*` keys.
- Other Lima state/cache files only if that manifest attributes them to this run;
  then remove the empty owned `state` and state parent.
- The private `tools/lima/2.2.1`, `tools/openpgp/6.3.2`, twelve downloads and fresh `trust/known_hosts`,
  private home/cache/tmp and raw evidence after the redacted copy is verified;
  then remove empty owned task parents.

No wildcard deletion, `limactl factory-reset`, package-manager cleanup, host cache
prune or recursive removal of an unverified parent is supplied. A pre-existing or
changed owner/symlink/unattributed file requires review. This is why the teardown
is manifest-driven instead of a blind `rm -rf` command. Independently rescan every
created path, process and listener; retain the absence report outside deletion
scope. Deletion does not promise secure erasure from APFS snapshots or storage
media. No disk backup, provider cleanup or unrelated Fort teardown is authorized.
