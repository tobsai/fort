# Provisioning packet preparation — 2026-10-12

Prepared [revision 1](../../ops/isolation-probe-v1/README.md) from accepted source
base `6423c02cf07d08a3f51207e9476b53181e4f3ace` on
`codex/fort-core-openmaus-20261011`, in `/tmp/fort-core-openmaus-20261011`.
This is inert source/design preparation, not a VM installation, isolation proof,
working provider runtime or T07/Stage 2 acceptance.

The seal covers **22 files**, plus `seal.json` itself. SHA256 of that seal:
`76cff597d9784b85db86abbedead8a3e9cf7104c1a3a7922c4f8764428cc8ea2`.
It includes fixed VM configuration, inline cloud-init provisions, guest
controller/runtime/tool sources, units, network and SSH policy, fixture hashes,
download allowlist, trust roots, host paths/resources, control commands and teardown.

## Executed safe checks

- Test-first validator: initial missing-module failure in `validator-red.log`;
  all **12 static policy/negative tests pass** in `validator-green.log`.
- Packet seal, embedded fixture consistency, JSON and nine Python source parses,
  and three unit-policy parses pass in `static-validation.json`.
- Independent Ruby/Psych parsing confirms the JSON-formatted Lima YAML; shell
  syntax checks pass for all three inline scripts. `--require-executable` returns
  the intended blocked status 2. See `syntax-and-guard.json`.
- Both proposed host roots remain absent. File modes and task-action observations
  are recorded in `observations.json`; no chmod, installation, credential creation,
  boot, provider invocation or host setting change occurred.
- Accepted service/runner/native code, specs, donor and main checkout are unchanged.
  The prior 145-test process/bridge suite was not rerun for this independent static
  packet. Parent-owned `reviews/` stays untracked and untouched.

Only public source text, checksums, detached signatures, public signing keys and
package metadata were fetched. No archive/image/executable was downloaded. The
first direct metadata fetch was blocked by sandbox DNS; the same bounded public
read succeeded with reviewed network escalation. No automatic-approval rejection
occurred. Public key material here means publisher verification keys, not owner
credentials; only digests/identifiers are retained in the packet.

## Remaining blockers and unrun checks

No inspected OpenPGP verifier is available. Signature validation is mandatory and
cannot be replaced with SHA256 checks. The exact verifier/toolchain still needs a
separate concrete selection before a live consolidated request is possible.

The original state path exceeds macOS's socket-path limit (112 bytes). The packet
proposes the shorter `/Users/tobiasgunn/Library/FortIsolation/state` (92 bytes);
this proposed change needs to be included in future approval. No directory was
created. The manager/download root remains in Application Support.

Lima's internal loopback SSH probes do not authenticate the guest host key, and
the VM network exists before the cloud-init firewall. These bootstrap limits and
full guest-only administrator sudo are disclosed for review. Subsequent Fort
control pins the guest's public key from owned serial output. No air-gap or
restricted-administrator claim is made. If pre-boot rendered-media inspection or
authenticated bootstrap is mandatory, the current manager approach needs revision.

Unrun: publisher signatures; archive paths/hooks and code signatures/entitlements;
Lima schema and effective/generated configuration/media; actual image package and
storage checks; systemd 255 semantic unit validation; nftables validation; guest
boot/key generation/control session; independent process/cgroup absence; all fault
and descriptor/IPC/network matrices; actual runtime/provider tool-boundary support.
The authored packet supports one nominal single-use synthetic session only. It
does not authorize deleting its consumed record or running replacement attempts.

The [draft request](../../ops/isolation-probe-v1/README.md#consolidated-future-request--draft-not-a-live-question)
is explicitly held. No installation or permission question has been sent.
