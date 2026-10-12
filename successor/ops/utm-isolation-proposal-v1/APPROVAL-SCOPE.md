# Draft owner scope — UTM route change

**Not sent; not approved; not a runnable packet.** The current authorization is
for read-only research and inert preparation. The earlier Lima approval does not
cover UTM. Before presenting an execution request, complete and independently
review the source adaptations and exact execution manifest identified below.
If only artifact assessment is ready, request that narrower stage explicitly;
do not present this document as blanket authorization for later unreviewed code.

Proposed owner request:

> Approve one temporary, synthetic-only Linux isolation experiment using the
> official free **UTM 4.7.5 build 118** and its Apple Virtualization backend, after
> the exact execution packet is independently accepted. Its DMG must match SHA256
> `a8435c93cfb5f8bbfeea4b134cfad1ac66b67632b75e438c63b1a8ae043bef0e`, pass Developer ID,
> notarization and Gatekeeper checks, and retain its complete signed bundle and
> legal notices. No security exception, re-signing or quarantine removal.
>
> Permit only the nine pinned public inputs in `scope.json`, including the Ubuntu
> 20260926 arm64 filesystem/kernel tar, and temporary OpenPGP.js 6.3.2 using existing
> Node 26.5.0. Verify registry and Ubuntu publisher signatures and tampered controls
> before the respective execution steps. Inspect the tar and stop if its raw disk,
> ARM64 kernel/initrd or root label does not support the stated direct Linux boot.
> No QEMU backend/converter, package installation, build hook or fallback download.
>
> Use one VM, 2 vCPUs, 4 GiB guest RAM, one raw disk no larger than 20 GiB and at
> most 4 GiB additional storage, including the full app and 128 MiB total evidence.
> Keep downloads, the private app, VM and seed under
> `/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`. Also permit the
> exact newly created UTM sandbox/group-container/preferences/cache/temp paths
> resolved from the accepted signed app and recorded before launch. Stop if any
> affected UTM location already belongs to another installation. Do not change
> existing parent permissions, `/Applications`, global PATH or startup settings.
>
> Approve an attended GUI session at an agreed time: I will launch UTM, register
> the exact VM as a shortcut without a disk copy, review/start it once, supply its
> displayed PTY identities, and stop/remove it and quit UTM afterwards. No
> AppleScript/utmctl/Shortcuts, Automation/Accessibility, Full Disk Access or host
> administrator grant. Any unexpected permission prompt stops for review.
>
> The guest has no NIC, shares, physical USB, audio, clipboard, Rosetta or owner
> data. Two serial PTYs carry bounded inspection and a fixed control protocol.
> Trust the signed-in Mac owner, UTM/VZ and guest root; local PTY ownership is not
> cryptographic authentication or protection against same-user host compromise.
> A guest-only root management service starts after provisioning. Runtime UID/GID
> 2001 has no login/sudo, and the fixed tool has a separate dynamic UID. No SSH
> keys, guest passwords, provider credentials, real prompts or agent sign-in.
>
> Permit the disclosed first-launch GitHub release-note metadata request and
> existing Apple OS trust/notarization traffic. Disable optional update checks;
> do not fetch templates, guest tools or updates, and keep UTM Server off. This
> does not impose a host egress sandbox or promise an offline host.
>
> After all actual guest/PTY/unit/device/readiness gates pass, allow one nominal
> synthetic prepare/grant/19-heartbeat/stop session. No replacement, ambiguous
> retry, snapshot/resume, extra fault session or provider activation. Failure stops.
> Verify guest cgroups and exact VM/process/PTY termination; preserve redacted
> evidence, then delete only owned recorded artifacts. If protected UTM state
> cannot be removed normally, report it and request the specific owner action;
> never broaden access. Shared macOS registration/audit records may remain.
>
> There is no purchase or service charge; costs are local resources, bandwidth,
> power and my attendance. This is not production isolation, T07 acceptance,
> deployment, release or authorization for provider cleanup.

The actual request must name the accepted execution commit/seal and resolved
host-state manifest, not just this proposal digest. Current unknowns may require
an **artifact-inspection-only approval** first: bounded download/mount/signature/
archive inspection and cleanup, with no application launch, provisioning or keys.
No such approval is inferred from this task.
