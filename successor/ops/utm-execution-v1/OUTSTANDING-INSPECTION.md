# Outstanding read-only input inspection

This document records a future approval step. It does not request or grant
execution now. No listed input was acquired during R2.

The smallest input acquisition can omit UTM.dmg entirely. Reuse the accepted
static app assessment as historical evidence; do not install or launch UTM.

1. Acquire only the eight non-UTM entries in the unchanged
   `../utm-synthetic-packet-v1/inputs.json`: the dated Ubuntu ARM64 tar, its sums,
   detached signature, publisher public key and package manifest, plus pinned
   OpenPGP 6.3.2 metadata/archive and npm public signing keys. Preserve each exact
   URL, digest, cap, allowed redirect host and no-credential/no-proxy policy.
   Combined maximum payload is 814088192 bytes. No retry, alternative image,
   additional key, package installation, account, credential or guest program.
2. Acquire these three public source blobs from the already pinned UTM commit
   `048ca7498ea3a374439149d51739d94c5300bcda`. These are additional source inputs
   beyond the previous nine-input proposal and need explicit inclusion in that
   future scope. Use unauthenticated GitHub blob API requests, at most 128 KiB of
   response per blob, verify decoded byte size and Git blob SHA-1, record SHA-256.
   Do not execute the source.

| File | Git blob SHA-1 | Decoded bytes |
| --- | --- | ---: |
| `Configuration/UTMConfiguration.swift` | `abfe34709f2953d6860b950ec291826d3699b2e5` | 9199 |
| `Configuration/UTMConfigurationInfo.swift` | `ba30dcc0a5c739b6ed9ee056639972425b8b9e41` | 5019 |
| `Services/UTMRegistryEntry.swift` | `982acc682b49fd9bb6431df4e1f59e50e8a8faa4` | 18537 |

Exact API URL pattern: `https://api.github.com/repos/utmapp/UTM/git/blobs/<blob SHA>`.
The blob identities and sizes come from the cached pinned Git tree, not from a
new network lookup. `api.github.com` must be included explicitly for this narrow
source retrieval. No authentication header, Git credential helper or account is
needed. Total payload cap including three API responses is 814481408 bytes;
reserve at most 1 GiB for these inputs plus bounded verification receipts.

After pin and registry checks, inspect the verifier archive without hooks,
workers or dependencies, and load only its accepted bounded Node module with the
existing pinned Node 26.5.0. Verify Ubuntu signed checksums against fingerprint
`D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`. Run the four existing negative-control
classes with in-memory mutations only. No generated key or tenth verification
input is needed. The six current JavaScript fixtures do not substitute for this.

List the authenticated Ubuntu archive without extracting a guest filesystem.
Reject unsafe or duplicate members; record exact selected paths before asking the
bounded examiner to hash/decode them. Maximum compressed input is 768 MiB,
expanded bytes 20 GiB plus 192 MiB, at most 64 entries. Keep observations and
metadata under the existing evidence cap. Use a bounded worker process for any
large decompression/hash operation and stop on resource/deadline failure. No
host filesystem mount, disk conversion, resizing, alternate kernel or VM boot.
If raw layout is partitioned or otherwise unsupported by the current examiner,
record the fact and prepare the exact decoder source as a subsequent reviewed
change. Do not infer a root offset or label from a filename.

Read the three UTM definitions to close config-version, information and registry
schema gaps. Bind the registry collector and in-memory generator to those exact
bytes, then rerun source fixtures and independent review. No app registration,
PTY access, guest seed application, host process control or protected app-state
mutation belongs in this inspection step. Delete only positively owned acquired
inputs after retaining bounded redacted receipts; preserve uncertain state.
