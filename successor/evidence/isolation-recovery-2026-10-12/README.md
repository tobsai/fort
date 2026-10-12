# Controlled recovery stopped at macOS Gatekeeper

The accepted diagnostic correction was executed against the original download and
verification scope. All twelve original download pins, both npm signatures, both
publisher signatures and six tampered-input controls passed. **Lima did not run:**
macOS Gatekeeper rejected the extracted manager. The operation stopped at that
gate, and every recorded temporary artifact was removed and independently proven
absent. No VM, fresh SSH key or synthetic session was created. T07 remains open.

## Authority and exact revision

- Branch: `codex/fort-core-openmaus-20261011`.
- Worktree: `/tmp/fort-core-openmaus-20261011` (canonical `/private/tmp/...`).
- Accepted source: `e770db9d924e1281b95695b49e809c818f87a4af`.
- Accepted 31-file seal SHA256:
  `31e22d8e99e3141e9083f7a2246c15469d32362279cf537c56aa35911936e406`.
- The [authority record](authority.json) binds the parent's explicit controlled
  recovery instruction to original owner approval
  `Sentinel_7f07a5939a408191a8e7036c9ec04dbb`, 2026-10-12 21:20 UTC, and the
  accepted independent review. Historical message bodies were not replayed.
- Prior failed attempt `28f76cf49bdf686177204c9599a178caa55d188a` stays closed.
  Its discarded Ubuntu key response remains unexplained. This successful download
  does not establish the cause of that historical mismatch.

The packet's preparation flags and historical wording remain byte-identical.
Execution authority and actual outcomes live in this separate record. No pin,
signature policy, dependency grant, resource limit, permission, session count or
cleanup gate was changed. The main Fort checkout and parent-owned `reviews/`
were not edited. No remote publication, provider cleanup or external agent contact
occurred.

## Checks actually performed

| Check | Result and evidence |
| --- | --- |
| Fresh source/ownership/resources/port preflight | Passed; [preflight.json](preflight.json). Both task roots absent before creation; port unused; 738,958,528,512 bytes free disk and approximately 31.45 GB free/inactive/speculative RAM. |
| One unchanged downloader invocation | Passed, exit 0; [download-result.json](download-result.json), [download inventory](download-inventory.json). Twelve accepted schema-2 receipts, no failures or pending publications. |
| npm signatures and verifier archive | Passed, two signatures; [registry output](registry-stdout.txt). This first stage imported no downloaded library. The outer result separately verifies archive integrity; the nested `tarballVerified:false` is the metadata helper's narrower result. |
| Verifier archive and dependency inspection | Passed, 43 regular files, 17,421,480 expanded bytes; [inspection](openpgp-archive-inspection.json), [dependency review](verifier-dependency-review.json), [extraction](verifier-extraction.json). No hooks, builds or additional package installation. |
| Actual publisher signatures | Passed with default current-time policy: one detached signature each for Lima and Ubuntu, exact primary fingerprints and signed payload hashes; [publisher output](publisher-stdout.txt). |
| Tampered-input controls | All six rejected: changed checksum bytes, changed signature bytes and unrelated publisher key for each publisher. |
| Lima archive inspection/extraction | Passed, 215 members and 81,620,424 expanded bytes; [inspection](lima-archive-inspection.json), [extraction](manager-extraction.json). One explicitly inspected internal documentation link; no escaping links or special files. Legal files retained throughout installation. No manager/helper executed. |
| QCOW2 header | Passed, virtual size 3,758,096,384 bytes, no backing file, snapshots, encryption or incompatible/external-data features; [header](guest-image-header.json). This does not inspect a booted filesystem. |
| Strict code-signature verification | Passed; ad-hoc signature, no TeamIdentifier. Entitlements observed: virtualization, network client and network server; [assessment](manager-signing-assessment.json). No Developer ID/notarization claim. |
| Gatekeeper assessment inside command sandbox | Exit 1, internal Code Signing subsystem error. No manager launch. |
| Read-only Gatekeeper confirmation outside command sandbox | **Failed, exit 3: `rejected`**; [confirmation](gatekeeper-confirmation.json). This is the execution blocker. No re-signing, quarantine removal, security-policy change or attempted launch followed. |
| Precleanup inspection | Passed: all 298 manifest entries matched their owner/device/inode/mode, regular-file contents and internal-link target; copied public receipts matched. No task-root file users, matching task executables or listener. [Inspection](cleanup-inspect-preflight.json). |
| Exact owned cleanup | Passed: 267 regular files, 30 directories and one internal link; [journal](cleanup-journal.jsonl). No wildcard/recursive parent deletion or manager cleanup command. |
| Independent absence check | Passed: 298 removed paths plus two never-created VM state paths, no matching task process, port 22058 unused; [absence report](absence-verification.json). |
| Unchanged packet static validation | Passed: all 31 sealed source files match; execution flag remains false. [Final validation](validation.json). The independently accepted 67 regression tests and three review reproductions were not rerun; this recovery changed no sealed implementation. |

The temporary OpenPGP library did execute in the fixed offline Node permission
wrapper after its authenticity and inspection gates. The wrapper granted no
network, writes, child processes, workers or native addons. This is a trusted
verifier execution, not an OS-sandbox claim. Its optional stream peer was not
installed; dormant external crypto helper calls were not needed by these inputs.
Lima and its bundled helpers were never invoked. Existing credentials, Keychain,
provider stores and sign-in flows were not accessed.

Both verifier stages emitted the same benign Python temporary-directory warning
inside the command sandbox and exited zero. A later read-only process check was
denied by that sandbox; the task-scoped process and cleanup checks were then
performed outside it under the existing cleanup authority. No automatic approval
review rejected an action. The blocking rejection came from macOS Gatekeeper.

## Resource and cleanup evidence

Before cleanup, the complete owned tree used 762,149,233 logical bytes and
763,789,312 allocated bytes, below the approved 4 GiB auxiliary allowance. No raw
guest disk was allocated. Raw plus copied evidence was 311,642 bytes at the
deletion preflight; final retained evidence remains below the 128 MiB cap.
The exact manifest is [created-paths.json](created-paths.json). It records public
tooling and download identities, not private key material. The private tooling
retained its legal files until approved removal. No tool is distributed with Fort.

Both new host roots are absent. No host service, guest process, startup entry,
VM state or fresh key was created. Removal makes no secure-erasure promise about
APFS snapshots or storage media.

## Unrun gates and next decision

Manager schema validation/create/start, generated-media inspection, guest package
and device verification, strict SSH pinning, systemd/network/cgroup checks, the
single synthetic session and installed-product/provider acceptance remain unrun.
No successful application, supported isolation boundary or T07 acceptance is
claimed.

The accepted packet explicitly prohibits bypassing an OS refusal. Review the
pinned manager's distribution/signing compatibility and propose a concrete
reviewable resolution before further execution. Do not treat a valid ad-hoc
signature or publisher checksum as Gatekeeper acceptance, weaken this gate, alter
macOS policy, change the binary, fetch another dependency or retry provisioning
under this record. The original infrastructure/session approval is preserved;
any material change needs its corresponding scope review. No additional approval
question was sent by this worker.
