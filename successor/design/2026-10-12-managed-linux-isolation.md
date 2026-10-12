# Decision proposal: independently managed Linux execution

**Select a dedicated Linux VM using Lima/VZ for the next synthetic isolation
prototype. This is a proposed design, not provisioning authority or T07 acceptance.**
Keep the TypeScript service/ledger and native clients on the Mac. Guest PID 1 owns
execution lifetimes; credential-bearing runtime hosts and untrusted tools have
separate scopes. Provider activation remains disabled. A VM alone does not protect
runtime credentials from tools running beside them.

## Existing tools and alternatives

Read-only inspection on 2026-10-12 found macOS 27.2 (26B5101f), arm64,
`kern.hv_support=1`, 10 CPUs, 64 GiB RAM, and the existing Xcode Swift toolchain.
Docker Desktop **4.87.0 (236836)** and its CLI are installed. Its engine, VM,
contexts, credentials and settings were not queried or started. Lima, system QEMU,
Tart, Colima, Podman, Multipass and the checked standalone VM applications were not
found in the inspected PATH/package/application locations. Android SDK `qemu-img`
is a disk utility, not an inspected independent VM manager; do not repurpose it.

Reusing Docker could avoid installing a package, but would add execution to an
uninspected shared VM and does not establish the required per-attempt lifetime and
credential boundaries. Select a separate Lima instance instead. VZ uses Apple's
existing Virtualization framework; no custom Mac helper is proposed. A new signed
Swift/VZ helper could avoid a third-party manager, but would require a new
virtualization entitlement, lifecycle implementation and its own approval/review.
Do not extend deprecated sandbox-exec. [Lima VZ](https://lima-vm.io/docs/config/vmtype/vz/).

## Actors, authority and lifetime

| Actor | Authority and data |
| --- | --- |
| Mac service | Sole ledger and owner/device authority; neither is mounted into the guest |
| Trusted Mac runner | Keeps runner session/capability, validates ordered output and bridges evidence; bounded host-initiated control channel only |
| Guest controller/PID 1 | Creates exact attempt units, owns their cgroups and restricted launch policy; no owner token, ledger or provider secret |
| Runtime host unit | One exact provider's configuration/store after later sign-in authority; trusted with that provider credential only |
| Tool unit | Fresh UID, root/workspace and namespaces; no provider store, controller socket, management key, sibling root, host mount or bridge capability |

Create the unit and record `{guestBootId, installation, epoch, attempt, generation,
unitName, invocationId}` before releasing executable work. Its trusted main process
waits at a start barrier for the exact grant. Lost creation acknowledgement means
lookup/reconcile that identity, never create another attempt. Guest PID 1 remains
responsible if the Mac runner or guest supervisor dies, including before ownership
acknowledgement. A heartbeat is sent only while the authenticated control channel
and attempt grant remain valid; a timer that ignores channel loss is insufficient.

Proposed unit contract: `Type=notify`, `ExitType=main`, `NotifyAccess=main`,
`WatchdogSec=2s`, `RuntimeMaxSec=40s`, `TimeoutStartSec=5s`, `TimeoutStopSec=2s`,
`KillMode=control-group`, `SendSIGKILL=yes`, `Restart=no`, `LimitCORE=0`.
Tool units bind their lifetime to the exact runtime unit and share an attempt slice;
they receive no systemd/cgroup management authority. No automatic run restart.
Systemd documents main-process lifetime, watchdogs and whole-group stop behavior;
Linux cgroup v2 provides tree-wide `cgroup.kill`. These are candidate mechanisms,
not a proof that this composition works. [Service lifecycle](https://raw.githubusercontent.com/systemd/systemd/main/man/systemd.service.xml),
[kill policy](https://raw.githubusercontent.com/systemd/systemd/main/man/systemd.kill.xml),
[cgroup v2](https://docs.kernel.org/admin-guide/cgroup-v2.html).

Use a fresh root/workspace and UID for tools, explicit environments and only intended
stdio descriptors. Proposed guest restrictions include `NoNewPrivileges=yes`, an
empty capability bounding set, `ProtectSystem=strict`, `ProtectHome=yes`,
`ProtectControlGroups=yes`, `PrivateDevices=yes`, `PrivateTmp=yes`,
`PrivateNetwork=yes`, a private root and restricted namespace/syscall creation.
Guest systemd 255 must validate every selected setting; newer options such as
`PrivatePIDs` must not be assumed available. [Execution restrictions](https://raw.githubusercontent.com/systemd/systemd/main/man/systemd.exec.xml).

After stop, require the exact unit inactive and the attempt cgroup tree empty,
including children that call setsid. Unverified cleanup fences the guest/attempt
and preserves ledger uncertainty. Controller/VM loss never releases the Conversation
barrier. Guest boot identity changes force reconciliation; no in-flight unit starts
at boot. If PID 1 or the guest cannot be trusted/reached, stop the owned VM and
verify its termination before considering new work. A timeout alone proves neither
cleanup nor absence of external effects.

The eventual official runtime must support entering the separate tool scope without
passing its provider credentials, environment, inherited descriptors or management
sockets. If that cannot be demonstrated through a supported integration, mark the
provider/topology unsupported. Do not copy subscription tokens or grant tools a
broader scope to make the prototype run.

## Concrete future approval packet — nothing below executed

| Item | Proposed exact scope |
| --- | --- |
| Host | This Mac running `/Users/tobiasgunn/dev/fort`; single-owner, local-only prototype |
| Manager | Official `lima-2.2.1-Darwin-arm64.tar.gz`, SHA-256 `9e9eacce88f37e185c346bad73aa6136f738d8cdf8c3bb23cd42b071824bc66e`, from the [v2.2.1 release](https://github.com/lima-vm/lima/releases/tag/v2.2.1). Extract into `/Users/tobiasgunn/Library/Application Support/FortIsolation/tools/lima/2.2.1`; no global package install or PATH edit |
| Guest | Ubuntu 24.04 arm64, dated **20260926**, [image](https://cloud-images.ubuntu.com/noble/20260926/noble-server-cloudimg-arm64.img), SHA-256 `1d6bffe64b848468ac97f821d369a4846d983de1800ccf6b5ec8853e85cefc55`; [published checksums](https://cloud-images.ubuntu.com/noble/20260926/SHA256SUMS). Manifest reports kernel `6.8.0-142.142`, systemd `255.4-1ubuntu8.17`; booted versions remain unverified |
| Instance/storage | `fort-isolation-probe`; fresh `LIMA_HOME=/Users/tobiasgunn/Library/Application Support/FortIsolation/state`; 2 vCPUs, 4 GiB RAM, 20 GiB guest disk, plus up to 4 GiB pinned download/cache budget; no swap/host-memory settings changed |
| Mounts/devices | **No host mounts**, no home/repository/ledger/Keychain/Docker socket, no Rosetta/GPU/USB/audio sharing. Copy only reviewed synthetic fixture bytes into the guest |
| Control/network | `plain: true`, VZ, no dynamic/static workload forwards or LAN ingress; explicit management SSH **127.0.0.1:22058** only. Recheck availability before creation. Disable agent/X11 forwarding and ambient public-key loading. Systemd 255 means no assumption of Lima SSH-over-vsock support. Lima's underlying management network still exists; plain mode is not an air gap. Runtime/tool test units use `PrivateNetwork=yes`, with no guest egress granted to them |
| Credentials | Generate a **new VM-management SSH key pair only** inside the fresh Lima state, authorized solely in this guest. No reading/copying existing SSH/provider credentials; no provider sign-in. Review generated sudo/admin policy before accepting the guest |
| Guest privileges | Create a guest-only management account and `fort-runtime` service account, plus dynamic tool UIDs. Install the reviewed controller and unit definitions inside this guest using its administrative provisioning context; system PID 1 manages cgroups. No host accounts, host sudoers, launchd/startup registration, or general cgroup delegation to workers |
| Signing/security | Verify the downloaded manager's integrity/signature and existing VZ entitlement before execution. No Fort entitlement, Keychain/TCC/Full Disk Access, firewall or persistent host permission grant is proposed. Any extra required helper/grant changes this packet and requires a new concrete decision |
| Rollback | Stop and verify only `fort-isolation-probe`; remove that fresh state, synthetic guest disk, management keys and dedicated `tools/lima/2.2.1` tree after confirming ownership. Preserve evidence; do not stop Docker or remove any existing application |

[Lima plain mode](https://lima-vm.io/docs/config/plain/) disables mounts, guest agent,
containerd and dynamic forwarding, but still performs base-user/key setup and
provisioning. The pinned [configuration schema/template](https://raw.githubusercontent.com/lima-vm/lima/v2.2.1/templates/default.yaml)
is the basis for the explicit settings; no default template or ambient Lima override
is acceptable. [SSH transport requirements](https://lima-vm.io/docs/config/port/)
explain the systemd version caveat. No YAML was executed or validated by a Lima
binary because none is installed.

Only public checksum/package metadata was fetched. No manager binary or guest image
was downloaded. The Lima checksum file's hash matches the release page's published
hash. Retained [metadata](../evidence/process-backpressure-2026-10-12/isolation-source-metadata.json)
is provenance, not downloaded-binary, signature or boot validation. Before asking to
execute this packet, prepare and review the complete Lima/cloud-init/unit artifacts,
verify signed image checksums and manager signature requirements, and seal their
hashes. Any missing dependency/package or changed identity must be listed explicitly;
there is no blanket install permission in this proposal.

## Acceptance work still required

Test exact launch descriptors (low/high fd and pipe canaries), environment/PATH,
owner/ledger/provider/sibling/home paths, symlink escapes, systemd/management APIs,
process signaling/ptrace, IPC and denied network with positive controls. Kill the
supervisor before/after ownership acknowledgement and during input/upload/tool/stop;
exercise double-fork/setsid, stopped processes, PID reuse, lost control channel,
manager loss and VM reboot. Prove cleanup or durable quarantine without replacement
execution. Inspect actual mounts/namespaces/cgroup membership and old-unit fencing.
No real provider test may substitute for these independent synthetic checks.

**T07, OS confinement and provider activation remain unaccepted.** The proposal adds
no installation, account, service, credential or permission today.
