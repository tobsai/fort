# Approved isolation experiment — stopped at input integrity

The approved experiment stopped before package extraction or execution because
the Ubuntu public signing-key response did not match its sealed SHA-256. No retry,
pin update, alternate key, verifier or policy exception was attempted. Exact owned
cleanup completed and a separate post-cleanup path/listener scan passed.

## Authority and exact source

- Worktree: `/tmp/fort-core-openmaus-20261011` (canonical `/private/tmp/...`).
- Branch: `codex/fort-core-openmaus-20261011`.
- Approved commit: `95ab4a250dc4b8dce0452d8054e2f4a7dc09a509`.
- Approved packet seal: `bc6aa4b100bb6792030d359fd71290f6a824c5a48f9da4ddcff3e3d4be4904c2`.
- Owner approval: `Sentinel_7f07a5939a408191a8e7036c9ec04dbb`, 2026-10-12 21:20 UTC,
  as recorded in the parent delegation. [authority.json](authority.json) preserves
  identifiers and scope, without replaying historical message bodies.
- Scope: [the unchanged sealed approval scope](../../ops/isolation-probe-v1/APPROVAL-SCOPE.md).
  The packet's historical preparation status was retained; authority was recorded
  separately. All 28 sealed files matched their hashes and approved Git bytes.

Only this new evidence directory was authored. The sealed packet, accepted
service/runner/native code, donor quarantine, historical specs and parent-owned
untracked `reviews/` were not edited. No push, deployment, provider cleanup or UI
interaction occurred.

## Actual outcomes

| Gate | Result |
| --- | --- |
| Approved revision and seal | Passed, 28 files; exact commit contents |
| Fresh roots and ancestors | Passed; absent task roots and no symlink redirection |
| Host resources | Passed: 64 GiB RAM, 10 CPUs, virtualization available; 739,207,135,232 bytes free disk; about 28 GiB free/inactive/speculative memory estimate |
| Existing runtime and CA | Passed: Node v26.5.0; existing root-owned, non-writable-by-group/world system CA |
| Port preflight | Passed: no TCP listener on port 22058 |
| Sealed downloader | **Failed**, exit 1, `Pinned checksum mismatch`; one invocation, no retry |
| Seven completed inputs | All seven passed independent SHA-256 and byte-cap checks |
| Ubuntu public-key export | **Failed exact-byte pin**; actual failing digest/size unavailable because the sealed downloader deleted its partial output without recording them |
| npm metadata signatures / OpenPGP.js archive | Unrun; these downloads were never reached |
| Publisher signatures / negative controls | Unrun; no actual publisher-verification result is claimed |
| Extraction, code signing / Gatekeeper, manager validation | Unrun |
| VM, keys, guest configuration and synthetic grant | Unrun; no VM or new keys created |
| Owned cleanup | Passed: seven files unlinked and two empty directories removed after owner/device/inode/mode/hash checks |
| Independent absence scan | Passed: 15 exact paths absent and port 22058 unused |

The initial sandbox rejected a read-only `sysctl` resource query. The scoped
escalated query then passed; this was not an approval-review rejection. The
operation stopped for a real input-integrity failure, not a missing approval.

The single download invocation ran from 21:25:26 to 21:28:21 UTC. The completed
files were the Lima archive, Ubuntu image, `lima.sums`, `lima.sig`, `lima.pub`,
`ubuntu.sums` and `ubuntu.sig`. They occupied 658,566,061 logical bytes and
660,103,168 allocated bytes at inventory, within the approved storage envelope.
The image was 620,224,512 bytes and matched its pinned digest; it was never booted.

The downloader processes the scope sequentially and renames an output only after
its digest passes. Its explicit checksum exception plus the first seven completed
entries identifies the failed eighth input as `ubuntu.pub`. Its URL is
`https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xD2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`.
Expected SHA-256:
`927138de292fac4d861105a052cf56e9c5921987da385c844d88f34d19b39ab0`.
No failed response was retained or fetched again. The difference does not by
itself establish that the primary key or publisher signature is invalid, and its
cause is unknown. See [blocker.json](blocker.json) and the
[unchanged downloader's error](download-stderr.txt).

## Cleanup and evidence

[created-paths.json](created-paths.json) binds the two created directories and seven
completed files to UID, device, inode and modes. The downloader's transient
`.partial` names were renamed on success or removed on failure; the failed
partial's inode and content were not retained. Before deletion, the cleanup
checked the full child set, every recorded identity, each successful digest,
absence of the untouched state root and unused port. It used directory file
descriptors, `O_NOFOLLOW`, exact `unlink` calls and empty-directory `rmdir` calls.
There was no recursive parent deletion or unrelated cleanup.

- [preflight.json](preflight.json): actual resource/path/seal checks.
- [download-result.json](download-result.json) and [download-inventory.json](download-inventory.json): one failed invocation and the surviving verified inputs.
- [cleanup-before.json](cleanup-before.json): identity/hash verification and retained evidence hashes before removal.
- [cleanup-report.json](cleanup-report.json): each exact removal and completion.
- [absence.json](absence.json): separate post-cleanup rescan.
- [validation.json](validation.json): final evidence hashes and consistency checks.

No Lima/OpenPGP.js installation, VM disk conversion, manager/VZ process, guest
account, guest service, SSH key, provider invocation or secret access occurred.
No process-termination proof is substituted for that history: no VM process was
started, so instance/cgroup shutdown checks were inapplicable. The state root
remained absent. Deletion does not claim secure erasure from APFS snapshots.

## Remaining gate

Review the changed public-key export against the full pinned Ubuntu publisher
identity and unchanged cryptographic policy before preparing any revised seal or
new execution attempt. Do not silently refresh its pin or treat approval of the
original packet as acceptance of changed verification material. This record makes
no source correction or request to weaken verification.

The 36 prior packet tests were not rerun because no packet source changed. The
actual publisher checks, VM/guest controls, single synthetic session, broader fault
matrix and T07 acceptance remain unrun. Official providers stay disabled. This
attempt supplies failure and cleanup evidence, not a tested isolation boundary.
