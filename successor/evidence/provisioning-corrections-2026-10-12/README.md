# Provisioning packet corrections — 2026-10-12

Corrects the two P2 findings in `reviews/provisioning-fef99a5/REVIEW.md` against
base `fef99a5c3c08c63343244c928d886842f61d3f2a`. The old seal `76cff597…8ea2`
remains preserved in Git and the prior evidence; it must not execute unchanged.
The revised [packet](../../ops/isolation-probe-v1/README.md) is still blocked for
provisioning pending independent review and owner approval.

## Changes and failing-then-passing regressions

- **Readiness/lifetime fence:** Before sending any readiness notification, the
  runtime rejects an existing identity/grant for another invocation or a grant
  without a matching identity. The controller rejects a grant if the exact
  original runtime is no longer active. BindsTo/After stop propagation remains.
  Safe mocks reproduced four failures: three replacement/stale-grant cases emitted
  readiness, and an inactive original released the tool. All now pass, with
  positive controls for initial/original readiness and an active original grant.
  See `readiness-red.log`, `readiness-green.log` and `corrections-green.log`.
- **SSH path:** `UserKnownHostsFile` now quotes the Application Support path inside
  the SSH option. The actual command's option is captured without spawning a
  connection; installed `ssh -G -F /dev/null` parses it with 31 dummy filenames.
  The old option exceeds OpenSSH's 32-file limit; the corrected one passes. A
  deliberate 33-file control still fails. See `ssh-red.log` and the combined green
  log. No SSH connection or credential read occurred.
- **Concrete verifier:** Proposes private OpenPGP.js 6.3.2 on existing Node 26.5.0,
  adds exact registry metadata/key/tarball pins and SHA512 download support, and
  supplies a reviewed-source wrapper plus an offline permission-limited launcher.
  No npm install, hook, runtime dependency, owner keyring or library import occurred.
  The 8 MiB archive plus 24 MiB extraction stay inside the previous 4 GiB auxiliary
  budget; remaining margin is 288 MiB. Package LGPL legal files must be retained;
  this temporary private tool is excluded from Fort distribution.

## Executed checks

**36 tests passed:** 14 packet-policy tests; 9 readiness/controller/SSH tests;
13 verifier tests. Initial missing-module/scope failures and subsequent passes are
retained. Verifier tests validate the two actual public npm metadata ECDSA signatures
with installed Node built-in crypto, including modified registry signature/integrity
and missing-key failures. Detached-OpenPGP tests use an explicitly labeled contract
double to exercise awaited rejection, unrelated/expired/revoked keys, missing or
additional rejected signatures, exact filename and payload digest checks. They
prove wrapper control flow, not OpenPGP cryptography or publisher compatibility.

Python parsing, both Node source syntax checks, three inline shell syntax checks,
independent Ruby/Psych YAML/JSON equality and unit-policy/source-config/seal checks
pass. The execution guard intentionally returns 2. No full product Node/Swift suite
was rerun: accepted product, native, donor and spec files were not changed.

Public npm metadata/key URLs were re-fetched read-only; original bytes match the
reviewed digests. Existing Node version/help was inspected to confirm the selected
runtime and permission flags. No package/VM payload, executable library, image or
private key was downloaded/generated. No install, chmod, host setting change,
provider action, VM boot, or publisher-signature run occurred. Parent-owned
`reviews/` and prior evidence remain untouched.

## Seal and remaining gate

The revised seal binds **28 files** (plus `seal.json`). Its SHA256 is
`bc6aa4b100bb6792030d359fd71290f6a824c5a48f9da4ddcff3e3d4be4904c2`.
`validation.json` records the retained evidence hashes and explicit unrun checks.

Actual tarball/image integrity, extraction inspection, OpenPGP.js import and
Lima/Ubuntu publisher/tampered-input verification are unrun. Manager signature/
entitlements, Lima schema/effective media, systemd dependency race, guest boot,
network/identity/resource checks, the nominal session and independent cleanup
remain staged checks after approval. They need not already pass to ask for the
experiment; every required pre-grant gate must pass before the synthetic tool runs.

The full [human-readable approval scope](../../ops/isolation-probe-v1/APPROVAL-SCOPE.md)
includes the selected verifier, shorter state path, guest administrator, disclosed
bootstrap trust, precise resources/network/keys/permissions, costs and cleanup.
The packet supports one single-use nominal session only, with no replacement
attempt or wider fault matrix. It does not accept T07 or activate a provider.
No live approval question has been sent, and no automatic-approval rejection occurred.
