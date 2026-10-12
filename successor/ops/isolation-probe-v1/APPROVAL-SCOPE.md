# Proposed owner approval scope — held for correction review

Approve the following one-time local experiment only after the corrected packet's
seal is independently accepted. This is the proposed scope, not recorded approval.

Privately install **Lima 2.2.1** and **OpenPGP.js 6.3.2**, using the existing
**Node 26.5.0**, then create one temporary **Ubuntu 24.04 arm64** VM named
`fort-isolation-probe` from the pinned **20260926** image for **one fixed synthetic
isolation session**. No real prompts, providers or subscription stores are involved.

- **Resources and paths:** 2 virtual CPUs, 4 GiB guest RAM, one 20 GiB raw guest disk
  and at most 4 GiB planned auxiliary storage, including verifier/archive/cache/
  conversion inputs and at most 128 MiB total raw/copied evidence. The 24 GiB
  envelope is a planning/observed-use bound, not an OS quota; host overhead is
  additional and monitored. Tools/downloads stay under
  `/Users/tobiasgunn/Library/Application Support/FortIsolation`; VM state stays at
  `/Users/tobiasgunn/Library/FortIsolation/state`. The source is the isolated
  `/tmp/fort-core-openmaus-20261011` worktree. There are no shared host directories,
  repository/ledger/Keychain/Docker-socket mounts or physical USB/GPU/audio sharing.
  The generated read-only provisioning ISO is an explicit guest input.
- **Downloads and verification:** Only the twelve exact URLs, hashes and byte caps
  in the sealed scope: the pinned Lima archive, Ubuntu image, their public checksum/
  signature/key/manifest metadata, and the exact OpenPGP.js archive plus npm version/
  public-key metadata. Host HTTPS is limited to `github.com`,
  `release-assets.githubusercontent.com`, `cloud-images.ubuntu.com`,
  `keyserver.ubuntu.com` and `registry.npmjs.org`. Inspect archives and verify registry
  signatures/integrity before importing the private verifier; verify both publisher
  signatures and tampered-input controls before running Lima. Preserve all legal
  files; the LGPL verifier is temporary private tooling, excluded from Fort
  distribution. No npm/brew/pip installation, hooks, builds, transitive dependency
  installation, guest package installation or automatic updates are included.
- **Keys and permissions:** Generate only new VM-management and guest SSH host
  keys. Create owner-only task directories and private files inside the listed new
  roots; do not alter existing parent/home permissions. Guest `fort-admin` UID 1000
  has passwordless sudo for this guest. `fort-runtime` is UID/GID 2001 with no login
  or sudo, and the tool uses a separate dynamic UID. Existing credentials and owner
  keyrings remain untouched. No host account/sudoers/firewall/TCC/Full Disk Access
  change, launch-at-login service, global PATH change or Docker change is included.
- **Network and trust:** Check that `127.0.0.1:22058` is unused, then use it only for
  guest management. Accept Lima's initial SSH probes without guest host-key
  verification and the guest's possible host/LAN/Internet reachability before its
  early firewall starts. Only synthetic data is present. After filtering, permit
  the sealed DHCP/control/established-response policy; no workload forwards or
  public ingress. Later Fort control pins the guest's public key from the owned
  serial console and uses strict SSH. Runtime/tool networking is isolated. The
  verifier runs offline with no Node network/write/child/worker/addon grant. This
  trusts the signed-in Mac owner, downloaded manager/verifier and guest root; it
  does not protect against their compromise or claim an air gap.
- **Staged execution and failures:** Inspect manager code signature/entitlements,
  effective configuration, generated media, guest identities, actual package/unit/
  device/network/resource behavior and unit ownership before the single synthetic
  grant. The corrected runtime refuses stale invocation readiness; the controller
  rejects inactive originals. No ambiguous retry, replacement attempt, record reset
  or provider activation is included. Any failed gate or additional package,
  privilege, download, path, entitlement or permission stops for review. Live checks
  are performed after approval; approving the experiment does not certify their
  outcome or accept T07.
- **Cleanup and cost:** Stop and independently verify the exact VM, processes,
  cgroups and listener. If orderly stop fails, force-stop only that verified owned
  instance; unverified cleanup leaves it quarantined. Preserve bounded redacted
  evidence in the isolated worktree, then remove only the recorded owned manager,
  verifier, downloads/cache/temp, disks/media and generated keys/state. No unrelated
  deletion or secure-erasure promise is included. No paid service or purchase is
  involved; local disk, CPU, memory, power and download bandwidth are the costs.

Approval is needed because the next step installs downloaded executable code,
creates persistent local paths and fresh keys, starts a VM and provisions guest
accounts/services. Current authority covers source preparation and safe regressions.
This is not an automatic-approval rejection, and no live question has been sent.
