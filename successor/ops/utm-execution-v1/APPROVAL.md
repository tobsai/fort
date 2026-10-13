# Proposed eight-input acquisition approval

**Prepared for review; approval has not been given and the operation has not run.**
The accepted disabled base is `56916fa8f994b5f8b59ca79e2f0bc5b105d6cf5e`, seal
`a77c433f047412ded2245daf648a518f044f05b8e97604618d04b18aa51c89bb`.
This R6 candidate changes only the Python and Node input-inspection gates, related
validation expectations and review/evidence documents. Guest execution remains
false. Independent review must name the new commit and seal before the owner
decides. The new identity is reported alongside this artifact after sealing;
embedding its own seal hash here would be circular.

The proposed decision is: **authorize one fresh, consumed acquisition,
authentication and inventory attempt using that exact accepted activation revision
and seal**, including the authenticated OpenPGP verifier described below. This is
separate from any later UTM/VM or provider decision. Preparing the source does not
supply that authority.

## Eight exact inputs

| Local name | Initial HTTPS source | Maximum retained bytes |
| --- | --- | ---: |
| `noble-server-cloudimg-arm64.tar.gz` | https://cloud-images.ubuntu.com/noble/20260926/noble-server-cloudimg-arm64.tar.gz | 805,306,368 |
| `ubuntu.sums` | https://cloud-images.ubuntu.com/noble/20260926/SHA256SUMS | 65,536 |
| `ubuntu.sig` | https://cloud-images.ubuntu.com/noble/20260926/SHA256SUMS.gpg | 65,536 |
| `ubuntu.pub` | https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xD2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81 | 65,536 |
| `ubuntu.manifest` | https://cloud-images.ubuntu.com/noble/20260926/noble-server-cloudimg-arm64.manifest | 65,536 |
| `openpgp-6.3.2-metadata.json` | https://registry.npmjs.org/openpgp/6.3.2 | 65,536 |
| `npm-public-signing-keys.json` | https://registry.npmjs.org/-/npm/v1/keys | 65,536 |
| `openpgp-6.3.2.tgz` | https://registry.npmjs.org/openpgp/-/openpgp-6.3.2.tgz | 8,388,608 |
| **Total** | | **814,088,192** |

Exact unchanged SHA-256/SHA-512 pins, tool pins and policy are in
[activation-operation.json](activation-operation.json), copied from the accepted
review and the original input inventory. No UTM.dmg is included. Initial requests
use Ubuntu's image/key services and npm's registry. Redirects retain the unchanged
five-host allowlist: `cloud-images.ubuntu.com`, `keyserver.ubuntu.com`,
`registry.npmjs.org`, `github.com`, `release-assets.githubusercontent.com`.
At most four redirects per input, at most 40 HTTPS requests overall, no retries.
Requests use TLS verification and fixed headers, with no credentials, proxy,
cookies, netrc or provider sign-in. No listener is created.

814,088,192 bytes (776.375 MiB) is the retained response-body budget, not an
aggregate on-wire quota. A one-byte oversize probe may be read per attempted input
but is never retained; the first failure ends the attempt. HTTP headers, redirect
responses and TLS/network overhead are additional and are not metered against an
aggregate wire-byte cap. This distinction is part of the proposed scope.

## Code that would execute

Existing `/usr/bin/python3` (selected Xcode Python 3.9.6) runs this repository's
inspection controller, HTTPS transport, bounded workers and tar parsers. Existing
`/bin/ps` observes only owned child RSS. Existing Homebrew Node 26.5.0 runs this
repository's registry/publisher verifier and built-in cryptography. Entrypoint
hashes and resolved paths match the recorded pins. Supplemental Xcode-runtime and
Node linked-library observations are retained; they do not attest a full OS or
shared-library closure. These existing runtimes remain trusted host dependencies.

The only downloaded code entry to import is
`package/dist/node/openpgp.mjs` from npm's pinned `openpgp@6.3.2` archive. Before
that import, built-in registry signature verification, the archive digest,
physical tar/path/type/size checks, package identity and complete member manifest
must pass. The publisher worker independently repeats input/registry checks,
binds the exact selection-manifest digest and rehashes the extracted closure.
The checked package LICENSE and regular-file closure are retained. No package
manager, lifecycle hook, external dependency installation or executable addon runs.

The Node worker receives read access only to the fixed verifier source files,
input inventory, acquired inputs, checked verifier closure and selection manifest.
It receives no network, filesystem-write, child-process, worker, addon, FFI or WASI
permission, and uses a 256 MiB JS heap cap. This verifier runs on the Mac host;
Node's permissions are not a VM/computer-isolation boundary. Actual enforcement
and downloaded-code compatibility remain unrun and must fail closed if unsupported.

OpenPGP authenticates Ubuntu checksums against fingerprint
`D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`. Four negative controls mutate checksum,
signature, public-key bytes and expected fingerprint. No new key is generated or
another key fetched. Ubuntu inventory then streams hashes and observed member
names, with no guest filesystem extraction. It allows at most 64 physical entries,
20 GiB raw member and 192 MiB combined auxiliary data within the 768 MiB compressed
archive cap. It finishes with `image_bound: false` and `runtime_authorized: false`.
Unknown formats stop; there is no alternate image, guessed selection or decoder.

## Paths, storage and resource bounds

All operation files would be under the initially absent private root:
`/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`.
It would contain `downloads/` with the eight exact names, exclusive `.partial`
files, per-input receipts, attempt/completion/failure markers; checked `verifier/`
files and `verifier-selection.json`; and root-level consumed-stage/result markers.
No other output or temporary extraction path is planned. Bytecode and core dumps
are disabled for the workers.

Retained logical file content is conservatively bounded by **839,843,840 bytes
(800.9375 MiB)**: 814,088,192 downloads + 25,165,824 verifier expansion + 524,288
receipt/marker allowance + 65,536 selection manifest. Filesystem allocation
rounding and directory metadata are additional. Partial publication uses hardlinks,
not a second payload copy. The 20 GiB raw image member is streamed, not retained as
an extracted image. This is not the disk allocation of a future VM.

The read-only snapshot records **733,184,000,000 available filesystem bytes**,
leaving **732,344,156,160 bytes** after that logical budget, before filesystem
metadata. It is writable internal APFS; all target ancestors are real directories,
and the owner has parent access in the approved inspection context. The restricted
sandbox cannot write this host path; the future run needs its specific execution
allowance. No write probe, new host directory or permission change was performed.

Physical memory is **64 GiB**. The snapshot reports 81% on the macOS memory-pressure
free metric; raw free/inactive/speculative page counts are retained separately in
[host prerequisites](evidence/r6/host-prerequisites.json). This is an observation,
not a reservation or a guarantee that all inactive pages are immediately available.
No additional numeric reserve requirement is invented. Recheck changing capacity
before an approved run.

Worker limits are unchanged: acquisition 900 seconds total/600 per input;
registry 90 seconds; verifier archive 30 seconds; publisher 90 seconds; Ubuntu
inventory 300 seconds; controller staging deadline 1800 seconds. CPU soft/hard
limits are 120/121, 80/81, 25/26, 80/81 and 280/281 seconds respectively. RSS stop
thresholds are 256 MiB, 1 GiB, 256 MiB, 1 GiB and 3 GiB. RSS remains sampled, with
possible overshoot; Darwin RLIMIT_RSS is a pressure hint, not a hard quota.
Each worker has 64 descriptors, zero core limit and 64 KiB combined output cap.
File limits are 768 MiB for acquisition, 24 MiB for verifier extraction, and zero
for the other workers. Limits apply only to those children; no global host setting
is changed. Actual enforcement, TLS and signatures are future operation evidence.

## Completion, failure and recovery

Close owned resources and cancel/reap only positively owned workers. Keep successful
inputs, verifier closure, manifest and receipts for an explicit evidence/ownership
handoff. On failure, stop subsequent stages, consume the attempt and retain partial
or uncertain state. Do not retry, adopt an existing root, broadly kill processes,
recursively delete files or clean up a provider. Any later removal needs an exact
positively owned-object decision. The operation root must still be absent at start.

## Explicit exclusions and later decisions

No UTM.dmg, app installation or registration, startup service, package hook,
filesystem mount/conversion, guest filesystem extraction, seed/ISO creation,
UTM/app/VM launch, provider connection, sensitive account, sign-in, credential/key
creation, purchase, deployment, shared hosting or global security change is included.
No cleanup of existing provider resources is authorized.

Real image/member binding, seed identity, fresh UTM registry/bookmark capture,
app/helper/PTY attribution, owned cleanup and exact attended runtime authority
remain prerequisites to a separate guest trial. Authentication here does not
establish bootability or confinement. T07 and installed-product/provider acceptance
remain open.
