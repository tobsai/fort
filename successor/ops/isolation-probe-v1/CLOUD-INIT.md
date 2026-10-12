# Generated configuration contract

`lima.yaml` is JSON, a YAML subset, generated from the local reviewed files by
`python3 -B render_config.py`. The static validator requires byte-equivalent
embedded script/file content after parsing. The renderer does not execute any
provisioning source. Lima's actual schema validation remains unrun.

The sole image is an already verified local file, not a remote template. Fresh
`LIMA_HOME` must have no `_config/default.yaml`, `override.yaml`, `base.yaml`,
network definition or existing instance. No `base`, template URL, CLI `--set`,
environment proxy, ambient SSH public key, custom CA or user provision is allowed.
Plain mode skips Lima's nonessential Linux boot scripts, including its package
installer and guest-agent setup. `skipDefaultDependencyResolution: true` is also
explicit; its dependency script is not itself expected to run in plain mode.
The three pinned essential Linux scripts are Alpine-only and exit on Ubuntu.
Their hashes are in `provenance.json`. Unexpected helpers/dependencies stop boot.

## Expected cloud-init effects to inspect

This is an expected generated-data contract, **not a captured cloud-init receipt**.
The generated keys, instance IDs, network MAC and ISO cannot have current hashes
because creation has not been authorized. Do not mark these as verified in advance.

- Guest identity: `fort-admin`, UID 1000, `/home/fort-admin`, `/bin/bash`, locked
  password, and guest sudo `ALL=(ALL) NOPASSWD:ALL`. Lima cannot select its primary
  group ID through this config; record actual GID and reject unexpected supplemental
  privileges. The runtime's UID/GID is separately fixed at 2001. Existing collisions
  stop provisioning. No host user/group/account is created or changed.
- One fresh public management key in guest `authorized_keys`; no existing
  `~/.ssh` discovery or SSH agent forwarding. `passwordlessSudo: false` is not used:
  this Lima version would generate another guest password file.
- Package update/upgrade/reboot flags absent/false. No package list to install,
  no package repository addition, no snap/container pull. UTC timezone; filesystem
  growth to the selected disk; cloud-init's normal SSH host-key generation.
- The early `bootcmd` contains the exact inline nftables transaction, before
  final provisioning. It permits DHCP, established responses, guest loopback and
  incoming management SSH from the Lima gateway only. DNS is explicitly localhost,
  host resolver integration is off, and no local DNS server is installed.
- Read-only root-owned `cidata` ISO at `/mnt/lima-cidata`, mounted using mode/dmode
  0700. It contains upstream metadata/network/user-data, Lima environment and boot
  scripts, the selected guest utility payload if Lima includes it, and our inline
  data/system provisions. The host private key must never occur in that media.
- Lima's generated `/var/lib/cloud/scripts/per-boot/00-lima.boot.sh` mounts and
  executes the ISO boot dispatcher. Its authored files become root-owned 0644.
  No Fort service has `[Install]`, is enabled, or starts as part of provisioning.
  First installation masks the listed update/time/snap services inside this guest.
  Reboot/reprovision fails the installation marker guard; it never resets an attempt.

Lima may generate `cidata/`, `cidata.iso`, `cloud-config.yaml`, instance `lima.yaml`,
version/boot metadata, EFI variables, disk/temp files, serial and host-agent logs,
SSH configuration/control sockets, host-agent socket/PID and key files. Inventory
the actual set under the two owned roots. Record relative paths, sizes, file modes,
owners and hashes, redacting key contents. Unknown content, external symlinks,
environment variables, downloads or paths stop continuation before the grant.

## Bootstrap trust and management keys

Lima's built-in `CommonOpts` sets `StrictHostKeyChecking=no`,
`UserKnownHostsFile=/dev/null` and localhost authentication bypass. That behavior
is not configurable through the inspected `ssh` YAML fields. The proposed first
boot therefore relies on the trusted signed manager, its owned VZ process and
fresh loopback mapping. It is not cryptographically pinned SSH bootstrap.

After provisioning, the guest prints its **public** Ed25519 host key with the
`FORT_PROBE_HOST_ED25519` marker to its local serial console. Match that output to
the owned VZ process/instance and record its SHA256 fingerprint. Write only that
public key as `[127.0.0.1]:22058 ssh-ed25519 …` to the private task `known_hosts`.
Do not use `ssh-keyscan` as an identity proof. Multiple/conflicting markers, absent
serial evidence, changed host key or changed VM identity stop control; never delete
the known-host entry to bypass the mismatch.

Fresh management key paths are `LIMA_HOME/_config/user` and `user.pub`; private
mode 0600, owned by the signed-in Mac user, within owner-only state directories.
New guest private keys are `/etc/ssh/ssh_host_*_key`, guest-root-only; only public
keys/fingerprints leave the guest. Raw generated private key files are excluded
from all evidence and backups. Guest tools cannot access host keys, ISO, home,
cloud-init state, runtime canary, controller paths or systemd management sockets.
Teardown removes owned management keys and guest disks; no existing key is touched.

## Explicit network limits

The pinned VZ driver creates an in-process user network even with `networks: []`.
Its constants select `eth0`, `192.168.5.0/24`, gateway `192.168.5.2` and guest
`192.168.5.15`. SSH maps host `127.0.0.1:22058` to guest 22. No VMNet/socket_vmnet,
extra NAT network, forwarded workload port, host share, reverse socket or host
firewall change is included. Verify the actual bind address and packet source.

VZ still presents its built-in virtual entropy, balloon, socket, keyboard and
pointing devices. Disabling SSH-over-vsock does not remove the virtual socket
device. No physical USB/GPU/audio passthrough is proposed. Inspect actual devices
and host-side vsock listeners; AF_UNIX-only workload restrictions are intended to
deny workload AF_VSOCK use and must be verified in Linux.

Before the early cloud-init filter runs, this network can reach host, LAN and
Internet. The actual first-boot traffic is unobserved; no zero-egress claim is
made. After the filter, new non-DHCP guest egress, including DNS/NTP/HTTP(S), is
dropped. Root could change this guest policy. Runtime/tool units additionally
have private network namespaces and AF_UNIX-only socket creation. Verify both
positive controls and denials after boot; policy source alone proves neither.

Sources: [pinned Lima template](https://raw.githubusercontent.com/lima-vm/lima/v2.2.1/templates/default.yaml),
[cloud-init template](https://raw.githubusercontent.com/lima-vm/lima/v2.2.1/pkg/cidata/cidata.TEMPLATE.d/user-data),
[boot dispatcher](https://raw.githubusercontent.com/lima-vm/lima/v2.2.1/pkg/cidata/cidata.TEMPLATE.d/boot.sh),
[SSH options](https://raw.githubusercontent.com/lima-vm/lima/v2.2.1/pkg/sshutil/sshutil.go),
[VZ network implementation](https://raw.githubusercontent.com/lima-vm/lima/v2.2.1/pkg/driver/vz/vm_darwin.go).
