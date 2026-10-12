# Fort synthetic Linux isolation packet, revision 1

**Prepared and content-sealed for review; blocked for provisioning. No execution
authority is granted by these files or by a passing static check.** This packet
answers the missing-artifact review at `reviews/process-fix-6423c02/REVIEW.md`.
It changes no accepted runner, service, native or provider code. The parent must
resolve the decisions below before presenting the consolidated approval draft.

The candidate is one temporary Lima 2.2.1 / Apple VZ Ubuntu 24.04 arm64 VM,
`fort-isolation-probe`: 2 vCPUs, 4 GiB guest RAM, one 20 GiB raw disk and a separate
4 GiB allowance for downloads, conversion inputs, tools, media and evidence.
There are no shared host directories or provider credentials. The generated
read-only provisioning ISO is explicitly part of the guest's inputs.

## Decisions that remain before approval

1. **Signature verifier.** No GPG/GPGV/SQ verifier was found in the inspected
   installed locations. Both publishers' signing fingerprints and exact public
   metadata digests are recorded in `scope.json`; the signatures have not been
   verified. Choose an already trusted available verifier or authorize a separately
   pinned verifier/dependency packet. This packet installs none. Neither a checksum
   nor an ad-hoc Apple code signature replaces publisher verification.
2. **Correct the proposed state path.** The earlier application-support state path
   makes Lima's longest SSH socket path 112 bytes, beyond macOS's 103-byte usable
   limit. The revised proposal is `/Users/tobiasgunn/Library/FortIsolation/state`
   (92 bytes including that socket suffix). It replaces, rather than adds to, the
   earlier state location. The manager/download directory remains in Application
   Support. Both exact roots need inclusion in the later approval. Neither exists
   as a result of this task.
3. **Accept or reject the bounded bootstrap limits.** Lima 2.2.1's internal SSH
   probes explicitly disable host-key checks. Only fresh management keys and
   synthetic inputs are present. Subsequent Fort control uses its own strict SSH
   host-key check, pinned from the owned VM's local serial output. This does not
   retroactively authenticate Lima's probes. The guest also has a network interface
   before cloud-init's early firewall runs. Its possible initial DHCP/DNS/NTP/
   image-service traffic has not been observed. This is not an air gap. If these
   limits are unacceptable, stop and revise the bootstrap/manager design; do not
   silently add a host firewall, helper, image editor or SSH bypass.
4. **Retain execution gates.** The archive/image, effective Lima config, generated
   media, image-local package versions, SSH identity, systemd 255 syntax and actual
   resource usage remain unvalidated. The proposed approval would allow staged
   inspection and a synthetic grant only after those checks succeed. Any unexpected
   download, process, privilege, path, entitlement or permission stops that scope.

## Complete authored inputs

| File | Purpose |
| --- | --- |
| `scope.json`, `provenance.json` | Exact payloads, fingerprints, limits, identity and inspected public sources |
| `lima.yaml`, `render_config.py` | Standalone config with sealed inline boot/data/system provisions; no base template |
| `guest/controller.py` | Root-owned fixed prepare/grant/heartbeat/stop/inspect protocol; one persistent consumed attempt |
| `guest/runtime.py`, `guest/tool.py` | Trusted barrier/watchdog main process and fixed untrusted canaries, including setsid descendant |
| `guest/fort-*.service`, `guest/fort-attempt.slice` | PID 1 lifetime, separate UIDs and constrained namespaces/resources |
| `guest/egress.nft`, `guest/sshd.conf`, `guest/install.sh` | Guest-only early network policy, SSH policy, dependencies/users/unit installation |
| `host-session.py`, `manager-env.py`, `fetch-public.py` | Explicit future host control and bounded public download commands; no ambient credentials |
| `CLOUD-INIT.md`, `RUNBOOK.md` | Generated-input contract, exact review gates, lifecycle and cleanup procedure |
| `validate.py`, `test_validate.py`, `seal.json` | Static policy/consistency checks, negative cases and exact file hashes |

All Python/shell sources are new Fort synthetic prototypes. No upstream
implementation is copied, and no upstream license or attribution is removed.
The source references in `provenance.json` establish inspection provenance, not
cryptographic release validation. The future manager archive retains its LICENSE,
NOTICE and bundled third-party licensing files in the private installation.

## Boundary and lifetime

The Mac owner, manager and guest root/controller are trusted. The management SSH
private key stays on the Mac; its guest administrator can use passwordless sudo
for any guest command. This bootstrap privilege is deliberately disclosed. It is
not a restricted service account, and no claim is made that a compromised manager
or guest root is confined by the guest firewall. There is no multi-user hosting.

`fort-runtime` is UID/GID 2001, no login, no sudo and no writable management path.
Its only credential-like input is a literal synthetic canary. A separate dynamic
tool UID receives a private `/tmp`, read-only OS filesystem, network namespace,
restricted syscalls/capabilities, hidden management paths and no delegated cgroup.
This prototype shares the guest's read-only OS root; it does **not** implement the
proposal's eventual private per-tool root filesystem. It must not receive actual
provider stores, owner data, real prompts or untrusted general commands.

Before launch, the controller exclusively creates a persistent consumed-attempt
record. The runtime enters a start barrier; its observed boot ID, unit invocation
ID and cgroup join the fixed installation/epoch/attempt/generation identity. Only
an exact returned identity and consecutive control sequence can release the fixed
tool. Missing acknowledgement does not permit prepare again. `inspect` reconciles
the record without starting work. There is no restart/retry/record-reset command.

Only validated control messages renew the runtime heartbeat. Prepare has a bounded
four-second pre-grant lease. Runtime expiration/main-process exit, watchdog or its
40-second hard deadline lets PID 1 stop its cgroup; tool lifetime binds to runtime.
Explicit stop waits for inactive units and an empty attempt cgroup tree. A missing
receipt, reboot or changed unit identity remains unknown/quarantined; stopping the
VM may prove process absence, never external-effect absence. The actual composition
has not run on Linux. No T07 or Stage 2 acceptance is claimed.

## Consolidated future request — draft, not a live question

After the preceding decisions are resolved and a reviewer accepts this exact seal:

> Approve installing the pinned Lima manager in Fort's private application-support
> directory and creating one temporary VM with state at
> `/Users/tobiasgunn/Library/FortIsolation/state`, using 2 vCPUs, 4 GiB RAM, a 20 GiB
> disk and at most 4 GiB additional planned storage. Approve only the listed public
> downloads and newly generated VM-management/guest-host SSH keys. Permit the
> disclosed guest-only administrator and Lima bootstrap network/SSH behavior.
> Inspect the actual package signatures, configuration and media; run only the
> sealed synthetic session if every gate passes; stop and verify the VM, preserve
> redacted evidence and remove only its owned files. Any additional dependency or
> permission stops the operation for a revised decision.

This does not include installing a verifier, provider sign-in, copying existing
credentials, host sudo/account/security changes, Docker changes, launch-at-login,
global PATH/package changes, shared hosting, spending, deployment or a release.
No utility/download cost estimate is invented. These are design/authority gaps,
not an automatic-approval rejection.
