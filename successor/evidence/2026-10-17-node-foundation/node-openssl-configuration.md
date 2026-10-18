# Packaged Node OpenSSL configuration — October 18, 2026

Governing [Spec 059](../../../specs/059-modular-node-v1.md), existing decision 059.
Following `6c9895e`, both ordinary Node launch paths now use the fixed bundled
configuration already proven by the [sandbox probe](sandbox.md). This closes a
concrete production omission; it does not broaden TLS, network or sandbox authority.

The executor builder writes `node-openssl.cnf`; host composition copies it to its
resource root. Both manifests include the bytes. The content is exactly:

```ini
nodejs_conf = fort_node
[fort_node]
```

Native program inspection uses the existing bounded, no-follow file reader and
requires those exact bytes. The private executor checks its fixed sibling resource
with a bounded descriptor read, rejecting links, multiple hardlinks, writable or
oversized files and altered content. Both Node launch arguments include the fixed
`--openssl-config=<resource>` path and retain an empty environment. No machine
configuration, legacy algorithms, global environment or caller-selected flags were
added. The assembler also requires both exact files. The official
[Node option](https://nodejs.org/api/cli.html#--openssl-configfile) documents explicit
configuration selection; the runtime tests below demonstrate its actual use here.

Package integrity/signatures and the existing single-owner host remain the boundary.
This does not claim an atomic defense against hostile same-user modification between
inspection and execution. A late malformed-file change is tested to fail startup;
source metadata checks are not a substitute for the final signed package.

## Behavioral checks

- Worker red: `/tmp/fort059-openssl-worker-red.log` reported a ready synthetic runtime
  after its configuration was made malformed, proving the old path ignored it.
- Native red: `/tmp/fort059-openssl-native-red.log` failed eight expectations: missing,
  empty, directory, linked, writable, oversized and altered configuration were ignored.
- A preliminary edit helper failed Python syntax validation before changing any source;
  the unchanged worker red repeated. The helper was corrected before implementation.
- `/tmp/fort059-openssl-worker-final.log`: **10 passed**, including complete behavior,
  exact host identity, mutation, and missing/altered/linked configuration refusal.
  The late mutation starts no synthetic provider and verifies the guardian group absent.
- `/tmp/fort059-openssl-assembly-tests.log`: **17 passed**, including missing and altered
  configuration even when the input manifest has been updated to match the bad bytes.
- `/tmp/fort059-openssl-portable.log`: **1,104 passed**, **five optional budget skips**.
  Artifact content/manifests and provider-neutral core regression checks passed.
- The first full native run, `/tmp/fort059-openssl-native-green.log`, overlapped the
  portable workload. Two timing assertions failed (ordinary removal metadata and the
  7.007-second host journey); other behavior passed. No threshold was relaxed.
- `/tmp/fort059-openssl-native-isolated.log`: serial native run with explicit compiled
  resources, guardian and Node, **250 functions passed**, **nine optional checks skipped**
  (259 discovered). The full native journey took **1675.626 ms <=5,000 ms** and confirmed
  owned groups absent. The second argument case proves malformed configuration stops
  native startup before listener availability, provider marker or core socket creation.
- `/tmp/fort059-openssl-typecheck-final.log`: strict TypeScript passed after final tests.
- `/tmp/fort059-openssl-assembly-native.log`: one read-only package check passed. Synthetic
  signature callback reaches missing profiles; the normal public inspector refuses the
  unsigned signature. No product process, account or startup activation occurred.

## Isolated performance

MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2, Node 26.5.0, Xcode 27.1/Swift 6.4.
Twenty samples per seam, no concurrent task tests/builds. Warm tools, fresh synthetic
runtime directories; build/setup/provider latency excluded. Nearest-rank p95:

- Private compiled executor discovery/turn/cleanup: **533.575 ms <=2,000 ms**,
  max 535.361 ms; `/tmp/fort059-openssl-worker-samples.json` and `...-worker-budget.log`
  (29 passed including benchmark scenarios).
- Native package metadata: **0.916 ms <=50 ms**, max 1.779 ms;
  `/tmp/fort059-openssl-program-samples.json` and `...-program-budget.log` (one passed).

## Artifacts and limits

Fresh unsigned Release helper, Mac client and iPhone Simulator builds passed:
`/tmp/fort059-openssl-{helper,client,ios}-build.log`. Simulator launch remains unavailable
because the installed CoreSimulator is older than this Xcode; these are compile results.
Fresh host resources: `/tmp/fort059-openssl-host-resources`, with both configurations.
No upstream code, dependencies, permission declarations or provider payload changed.

The new `/tmp/fort059-mac-openssl-review-20261018/Fort.app` is **review-only**, unsigned
and unlaunched. Independent read-only inspection verified all 173 output file modes,
bytes/hashes, exact membership, both configurations and 171 unchanged input hashes.
Payload: 391,474,961 bytes. Its report SHA256 is
`e56fc79d61d34fb41874a7641ecf7c27a8be2a7b562ff8ca8215dd13a81891a7`;
sorted-key compact `files` JSON plus newline SHA256 is
`2fd90cb88e95ea443f56868de17efdb26d9b1b7a593cb9ebb49a6ba962d174e4`.
The previous 171-file review app remains untouched.

Independent source app file/mode manifests and their hashes:

- mac: `/tmp/fort059-openssl-release/Build/Products/Release/FortMac.app`, 5 files; `/tmp/fort059-openssl-mac-manifest.json`, SHA256 `48307270cfc371522bf0736dbfc028b2a009fe0a833c3f9040a24d2d0f1c76c1`.
- helper: `/tmp/fort059-openssl-release/Build/Products/Release/FortHost.app`, 5 files; `/tmp/fort059-openssl-helper-manifest.json`, SHA256 `f3850b829251b261dd64dfab6e229e2e6c6d90a4bee8c72d4d478953f8cf0876`.
- ios: `/tmp/fort059-openssl-ios/Build/Products/Release-iphonesimulator/FortiOS.app`, 5 files; `/tmp/fort059-openssl-ios-manifest.json`, SHA256 `80808d4d38518e766e71798b9b4d7e6d168848aeb3a0fcf9cafef1fa1d1da39d`.

Remaining work: full host-ledger and iPhone saved-history deletion, Codex third-party
notice closure, exact Apple account/app/group/profile lookup, final signing and
provisioning, authorized provider and installed native accessibility/screenshots,
reachable iPhone transport, TestFlight availability and physical acceptance.
No new credentials, sign-in, official Codex invocation, persistent registration,
provider cleanup, real-data deletion, deployment or remote publication occurred.
