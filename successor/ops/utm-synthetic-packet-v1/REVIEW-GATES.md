# Review and activation gates

This draft deliberately has no executable closure. “Ready for design review” must
not be relabeled “ready to execute.” The independent reviewer can assess scope,
identity, limits, interactions and stop rules now. The following source work must
be separately authored, tested and included in an accepted execution seal before
the owner receives the execution request.

| ID | Before an execution request | Future observed receipt before progressing |
| --- | --- | --- |
| R1 | Implement bounded nine-input downloader/Ubuntu-only verifier adapter, archive/config renderer, owner/PTY recorder, controller and fixed root inspector. Seal every source/dependency. Test behavior through public seams, one red→green slice at a time. | Fresh exact download/trust results and all failure receipts accounted for; accepted source bytes still match. |
| R2 | Test an inert archive examiner with malicious/traversing/duplicate/link/special/sparse/oversized/missing/multiple raw/kernel/initrd members. Define deterministic accepted format and binding rules, not guessed member names. | Authenticate the pinned archive; record exact member paths/hashes, raw format/layout/label and ARM64 kernel/initrd facts; stop on unsupported layout before app launch/VM creation. |
| R3 | Author exact NoCloud files, guest units, root broker and no-NIC firewall. Tests must exclude inherited Lima/SSH/network assumptions and prove no automatic runtime/tool grant. | Seed bytes and rendered unit hashes match; guest packages, boot/module order and no-key/no-login/no-getty/no-update state actually verified. |
| R4 | Test PTY framing, identity swaps, exclusive ownership, deadlines, missing/duplicate receipts, no resend, consumed-attempt persistence, stale grants, slow inspection and cleanup uncertainty. Test guest protocol using labeled doubles, without claiming Linux semantics. | Exact owner/process/package/PTYS and guest boot/unit/cgroup identity correlate. Root readiness and all positive controls pass before prepare/grant. |
| R5 | Implement owned-state manifest, storage accounting, non-destructive before/after state attribution and bounded cleanup. Exercise unknown-object/race/symlink/mount/protected-path cases on synthetic repository/temp fixtures only. | Exact live processes, open handles, PTYs, image/device and newly created paths recorded; normal detach and owned deletion independently rescanned. |
| R6 | Independent reviewer names accepted commit/seal, checks source and test evidence, rejects mutable or unsealed launch inputs. No self-issued review acceptance. | Launch preflight checks review revision and all immutable inputs; derived observations match the reviewed parser/config contract. |
| R7 | Present the consolidated request once, bound to the accepted execution commit/seal and complete host-effect scope; record owner decision and attended window. | Owner available, Mac identity/resources fresh, affected paths absent, no changed targets/costs/permissions. |

R2's actual archive/member receipts necessarily follow future authorized download.
The parser, selection rules, expanded limits and refusal behavior must be reviewed
before that approval. A mechanical binding within those exact reviewed rules is
not a new human choice. Any unsupported layout, new dependency, changed pin or
manual fallback is a scope change and stops; it cannot be improvised under the
same approval. A plan hash never blesses subsequently written implementation.

## Minimum source acceptance cases

- Downloader retains bounded diagnostics on hash/status/size/write failure,
  refuses destination/redirect changes and existing targets, and performs no
  automatic retry. The old `fetch-public.py` is a reference; its old multi-input
  main and scope must not execute.
- Ubuntu verification authenticates the exact tar filename, full fingerprint,
  current key/signature validity and every signature result with unchanged
  OpenPGP defaults. Registry verification precedes importing the library. The old
  two-publisher entrypoint is inapplicable. Negative cases use only the nine
  inputs: mutate checksum/signature/key bytes and expected fingerprint. The last
  case proves identity binding, not unrelated-key cryptographic verification.
- NoCloud's explicit reviewed module list/configuration prevents SSH key
  generation, default interactive accounts, passwords, gettys on both serial
  ports, package install/update, automatic growth/resize and automatic workload
  starts. Keep networking disabled from the configuration, not merely after a
  guest firewall happens to start. Cloud-init defaults are not evidence.
- The root broker has fixed typed pre/postflight operations, bounded output,
  read-only inspection, no arbitrary paths/commands and no user-controlled shell.
  It may administer only the fixed guest fixture units and state. Runtime UID
  2001 has no login/sudo; tool uses a different dynamic UID and receives no
  management descriptors. Clear inherited environment except explicit constants.
- Runtime/tool units preserve the consumed-attempt and original-invocation fence.
  A prepare fsyncs both file and parent directory before the runtime starts.
  A stale dependency activation cannot start a replacement. Controller death,
  serial loss, timeout, reboot or uncertain write cannot renew the heartbeat or
  reset the consumed record. Source tests do not replace actual systemd checks.
- One fixed setsid/double-fork descendant is attributed by PID/start time and
  owned cgroup, then removed by normal stop. The independent root inspector must
  examine the entire owned cgroup tree. A unit receipt or GUI “stopped” label
  cannot stand in for observed process absence.
- A failed denial test cannot pass merely because its target never existed.
  `canaries.json` replaces the historical absent `.ssh` and Lima-CIDATA paths
  with root-proven synthetic objects. Actual host Keychain/credentials are never
  read; topology/synthetic canaries provide only their explicitly bounded claims.

## Review outcome required

Record one of: **design accepted for source implementation**, **changes required**,
or, only for a later complete implementation, **execution revision accepted for
owner decision**. Keep review artifacts under reviewer ownership. This preparation
does not ask another agent to review itself or modify any existing review file.

Current status: R1–R7 are open as execution gates. Static UTM artifact acceptance
is established; none of the missing guest/PTY/lifecycle evidence is fabricated.
