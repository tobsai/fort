# R7 revised approval draft — disabled source

**Source-only preparation is authorized. Corrected live workers, validation and acquisition are not authorized or enabled. Independent review is pending.** The fixed preparation commit and seal are delivered separately to avoid a self-referential seal. An execution decision must name the later independently accepted activation identity and fixed smoke command, not treat this disabled packet as executable authority.

## Exact changed control

On macOS only, omit the child `RLIMIT_RSS`/`RLIMIT_AS` setter. Preserve the current core, CPU, file-size and descriptor setters, all numeric profiles, and the existing parent RSS monitor. Linux and other non-Darwin setter behavior is unchanged. Do not catch and ignore other setup failures, raise any limit, set inherited limits to infinity, or alter the parent or host settings.

The monitor requests 100 ms between RSS samples, allows 250 ms per owned-child `ps` query, and has additional loop, scheduling and termination delays. It continues through pipe EOF, rejects excessive or unobservable RSS while the child remains live, and cancels the owned group before reaping. These are stop thresholds, **not a hard process-memory ceiling**. Startup and reaction gaps permit overshoot with no proven byte bound. A starved, stopped or failed supervisor loses this active control; a fast allocation can pressure the host first. No memory reservation or arbitrary-child confinement is claimed.

Node's unchanged `--max-old-space-size=256` limits V8 old space only. Young-generation, native and Buffer/ArrayBuffer memory are outside that single limit. It is not a total JavaScript-heap or process-memory cap.

## Proposed owner request (after the identity gates below)

“May I accept the revised macOS policy, with the existing sampled RSS stop thresholds and possible unbounded overshoot instead of the incompatible child RSS/AS setter, and authorize the reviewed network-free child validation followed, only if it passes, by **one new bounded eight-input acquisition/authentication/inventory attempt**? The decision must name the independently accepted activation commit/seal and smoke command.

The new attempt would use `/Users/tobiasgunn/Library/Application Support/FortUTMInputInspectionR7`, retain at most 839,843,840 logical bytes plus filesystem metadata, and execute only the authenticated OpenPGP 6.3.2 entry inside the restricted offline Node worker described below. Stop at the first validation or operation failure, retain evidence and uncertain files, and cancel/reap only positively owned workers. No automatic retry is allowed.

The consumed R6 root `/Users/tobiasgunn/Library/Application Support/FortUTMIsolation` stays untouched: no read, adoption, reset, removal or reuse. This includes no UTM/app installation, VM boot, guest extraction, mount/conversion, provider/account/credential action, purchase, deployment, startup service or host-wide security change.”

This is a draft, not a request already sent or a granted decision. Do not send it as an executable approval request before the exact activation and smoke identities exist and pass independent review.

## Identity and validation gates

1. Independently accept this disabled R7 commit/seal and the explicit control change. The R6 activation at `7ef4215c7b850fc27918b71cc9b562301d305dd1`, seal `fd4c3110c319a1097d7bc419a7bd692be3d8f50b64d946a696076421b3f3b88b`, is historical and consumed; its authority does not authorize this operation.
2. Prepare a fixed network-free local child smoke command and an activation revision with inspection gates enabled but guest execution still false. Review their exact source identities and argv. This R7 slice does not prepare or run that new live smoke command; its precise required outcomes are specified here. There is no runtime environment override for the disabled gates.
3. Bind an explicit owner decision to those independently accepted identities, this memory policy, the distinct root and the one-attempt scope. A changed execution seal is verified separately from the human decision; it never silently inherits R6 authority.
4. Under that later authority, first prove the unchanged core/CPU/file/descriptor limits by actual child set-and-readback, actual RSS observation, owned cancellation before reap, observed cleanup and unchanged parent limits. Use no network, downloaded code or consumed state. The prior RSS diagnostic did not reach file/descriptor setup. Any smoke failure stops before acquisition; no retry or weakened limit follows.
5. Recheck the exact source, entrypoint pins and resolved runtimes, fresh root absence and no-follow ancestor identities, current disk/memory capacity and required Node permission support. Old capacity observations are historical snapshots, not reservations.
6. Run the ordered stages once. Require exact pinned downloads and durable completion, two registry signatures, checked verifier selection, authenticated Ubuntu publisher identity, four rejected negative controls, and bounded archive inventory. Require positively owned worker cleanup before accepting each stage. End with `image_bound: false` and `runtime_authorized: false`.

## Eight unchanged inputs

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

All exact SHA-256/SHA-512 pins and existing tool pins are in [activation-operation.json](activation-operation.json), unchanged from the original input inventory. Initial requests use Ubuntu's image/key services and npm. The unchanged redirect allowlist is `cloud-images.ubuntu.com`, `keyserver.ubuntu.com`, `registry.npmjs.org`, `github.com`, and `release-assets.githubusercontent.com`. At most four redirects per input, 40 requests total, no retries. TLS verification, fixed headers, no credentials, proxy, cookies or netrc; no listener.

The body budget is not an aggregate wire-byte cap. One extra byte per attempted input may detect oversize and is never retained; headers, redirect responses and TLS/network overhead are additional. Unchanged checked verifier expansion is at most 25,165,824 bytes; receipt/marker allowance is 524,288 and selection manifest allowance 65,536, for 839,843,840 logical bytes total. Filesystem rounding and metadata are additional. The 20 GiB raw image member is streamed, not extracted to disk.

## Code origins and trust boundary

Existing pinned `/usr/bin/python3` (selected Xcode Python 3.9.6) runs repository controller/transport/parser code. Existing `/bin/ps` reads only owned-child RSS. Existing pinned Node 26.5.0 runs repository verification code and built-in cryptography. These host runtimes and their libraries remain trusted dependencies; the entrypoint pins do not attest a full OS/shared-library closure.

The sole downloaded entry is `package/dist/node/openpgp.mjs` from pinned `openpgp@6.3.2`. Built-in registry authentication, archive digest, physical tar/path/type/size checks, exact package identity and complete member manifest must pass first. The publisher worker repeats the checks and binds the selection digest. Retain the package LICENSE. No package manager, install/lifecycle hook, dependency install or native addon runs.

Node gets read permissions only for the fixed repository verifier files, input inventory, acquired inputs, selected verifier closure and manifest. It has no network, filesystem-write, child-process, worker, addon, FFI or WASI grants. This runs on the Mac host; Node permissions are not a VM/computer-isolation boundary.

Ubuntu publisher fingerprint remains `D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`. Four controls must reject changed checksum, signature, public-key bytes and expected fingerprint. Archive inventory is limited to 64 physical entries, a 20 GiB raw member, and 192 MiB combined auxiliary data, without guest-filesystem extraction or guessed member selection.

## Unchanged worker bounds

| Stage | Wall limit | CPU soft/hard | Sampled RSS stop | Per-file write limit |
| --- | ---: | ---: | ---: | ---: |
| Acquisition | 900 s total; 600 s/input | 120/121 s | 256 MiB | 768 MiB |
| Registry | 90 s | 80/81 s | 1 GiB | 0 |
| Verifier archive | 30 s | 25/26 s | 256 MiB | 24 MiB |
| Publisher | 90 s | 80/81 s | 1 GiB | 0 |
| Ubuntu inventory | 300 s | 280/281 s | 3 GiB | 0 |

Each worker retains 64 descriptors, zero core allowance and 64 KiB combined output. Controller staging deadline is 1800 s. Socket operations use the remaining per-input deadline capped at 30 s. Limits affect owned children only; macOS memory supervision depends on the parent as disclosed above.

## Owned state and failure

The new root must be absent; existence is a veto, never a reason to adopt it or select another name. Fresh private directories and exclusive partial files retain no-follow ancestry/identity checks. Failure consumes the attempt and stops subsequent work, preserving partial/uncertain state and failure markers. Keep successes for an explicit ownership/evidence handoff. No broad process kill, recursive deletion, automatic fallback or provider cleanup is included.

UTM.dmg, VM seed/ISO, real image/member binding, fresh UTM registry/bookmark capture, app/helper/PTY attribution, guest trial, T07 and installed-product/provider acceptance remain separate later gates.
