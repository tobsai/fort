# Optional secure local-network transport

October 18, 2026, continued from `09a7d6c` under [Spec 059](../../../specs/059-modular-node-v1.md).
This is source, compiled-artifact and synthetic loopback evidence. No LAN listener,
startup registration, real credential, installed trust or provider was enabled.

`openSecureNetwork` is an optional HTTPS-to-private-socket boundary. Import and
ordinary host startup remain inert. The caller must supply the exact private IPv4
address/subnet, port, matching valid TLS identity, server name and private socket.
TLS 1.3 and core device authentication are independent requirements. The module
does not expose private owner-setup/recovery callbacks or treat loopback as owner
authority. It forwards only bounded ordinary Fort HTTP requests. Agent-host access
is still separate from the executor's computer-tool sandbox; this proxy is not a
host sandbox or a shared-hosting design.

The selected subnet must fit wholly inside RFC 1918 private space, or be exactly
127.0.0.1/32 for local use. Limits are eight connections/active requests, 16 KiB
headers, 1 MiB request, 8 MiB response and 30 seconds after request headers arrive.
TLS handshake/header timeouts are separately configured. Stop aborts pending work,
destroys owned connections and has a five-second deadline. There is no retry,
ambient identity discovery, key generation, tunnel or paid dependency in production.
`network-test-identity.ts` creates and removes disposable test identities; it is
absent from the compiled artifact and installs no trust.

## Observed regressions and fixes

- The first authenticated enrollment/read/revocation journey failed against the
  unimplemented public entry, then passed over real loopback TLS/private HTTP.
- A client trickling a body exceeded the total 30-second deadline. The red test
  ran 33.045 seconds. Aborting now destroys incomplete input as well as the upstream
  request; the final full-suite test completed at 30.012 seconds.
- Abort during bind left the opening promise unresolved; its test timed out at
  five seconds. A lifecycle signal now cancels bind and settles the opening promise.
  The same exact port can be reopened, and a later abort closes its listener.
- A private address with an overly broad subnet reached the listener boundary.
  The red pre-bind test observed one attempted bind. Both invalid ranges now fail
  before bind; a valid loopback configuration reaches the intercepted boundary as
  the test's positive control. This test never binds a LAN interface.
- Compiled artifacts initially omitted the optional entry. The packaging regression
  failed, then passed after the explicit export/build entry was added. Import succeeds
  under Node permission mode with no network permission. Two independent synthetic
  runtime contracts still pass with provider implementations absent.

The first broader TLS run also found fixture issues: macOS does not expose source
address 127.0.0.2, and changing the negative Host header initially changed the TLS
test's server name. The latter is corrected with an explicit matching test SNI.
The former is an explicit skipped peer-rejection case, not a passing subnet result.
Initial typechecks reported an Error.code narrowing issue and an inferred optional
header union in the tests. Both were corrected; strict TypeScript now passes.

## Final checks

Commands use Node `/opt/homebrew/opt/node@26/bin/node` 26.5.0 and installed pinned
TypeScript 6.0.3. Socket/process tests ran with the platform-approved permission
needed for temporary loopback/private sockets. No source changed during each run.

| Check | Result |
| --- | --- |
| `node verification/test-suite.mjs --suite portable` | 1,178 discovered; 1,170 passed, eight explicit skips, zero failed/canceled; 35.607 s |
| `node node_modules/typescript/bin/tsc --noEmit` | Passed |
| TLS lifecycle controls | Four passed, including bounded response, active stop, eight connections and bind cancellation |
| Pre-bind private-subnet regression | Passed after observed red |
| Optional compiled entry and provider-independent journey | Passed after observed missing-entry red |
| Final 36-file compiled core | Every manifest length/hash independently verified; 221,556 payload bytes |
| Isolated TLS benchmark | Passed; twenty samples after five warmups, 1 KiB synthetic JSON per new TLS connection |

Eight full-suite skips: six pre-existing opt-in budgets, this TLS budget (run
separately below), and the unavailable outside-subnet source address. No physical
LAN peer, iOS network execution, production certificate, trust installation or
real provider was exercised. The full native suite was not rerun for this transport;
the one shared presentation change has exact running-app inspection and fresh
Mac/iPhone compile evidence in [apple-access-ui.md](apple-access-ui.md).

Benchmark environment: MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2, arm64,
Node 26.5.0. One test process hosts the TLS boundary, private synthetic upstream
and client; no provider/build/test process ran concurrently. Round-trip samples in
milliseconds: 2.761875, 2.603708, 2.531083, 2.602208, 2.591167, 2.474666,
2.363041, 2.349917, 2.295042, 2.305167, 2.738666, 2.393209, 2.309667,
2.243375, 2.252834, 2.177458, 2.209583, 2.174667, 2.169459, 2.202708.
Nearest-rank p50 2.310 ms, p95 2.739 ms, maximum 2.762 ms; idle close 0.388 ms.
The separate active-stop regression completed below one second. These establish
local fixture overhead only, not Wi-Fi, physical-device or provider performance.

Retained core: `/tmp/fort059-secure-network-core`; manifest SHA256
`30049725d749542d5dcbff9d076b27db5fe0dd59af23a783cbf77c5275454371`.
The builder emitted a harmless Git fallback warning about the sandbox's unavailable
DARWIN_USER_TEMP_DIR while reading the preserved license; compilation and exact
payload verification passed.

## Remaining integration and external prerequisites

The transport is deliberately uncomposed in ordinary startup. Native HTTPS retains
platform certificate validation. Production TLS provisioning/trust, explicit network
activation, packaged lifecycle and physical iPhone acceptance remain open. No
accept-all certificate callback, trust setting or persistent grant is introduced.
The current owner pairing supports one device; joining a phone without replacing
the Mac's control identity is a separate required implementation slice. It must
preserve exact revocation/removal and recovery semantics, not overwrite owner control.

Primary guidance reviewed: [Node 26.5.0 HTTPS](https://nodejs.org/download/release/v26.5.0/docs/api/https.html),
[Apple local-network TLS identities](https://developer.apple.com/documentation/network/creating-an-identity-for-local-network-tls),
and [Apple secure connection validation](https://developer.apple.com/documentation/security/preventing-insecure-network-connections).

Local evidence remains under `/tmp/fort059-*`; these files contain only synthetic
results. SHA256s bind the principal final records:

| Record | SHA256 |
| --- | --- |
| `secure-network-portable-final.log` | `a9686ad34552f6a5158f8a3cd70b25fe3ded4a889581b3c121219de43c011067` |
| `secure-network-typecheck-final2.log` (empty success output) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `secure-network-benchmark-final.json` | `06001bc5bb50ad22006b0af742c9397e78aedd6518fb8277104687c2b6c06673` |
| `secure-network-deadline-red.log` | `c1a8e1ee4e9236aa32e53d4aee25ff6a34a152cb19e42edbdcfd7d5d366d6385` |
| `secure-network-lifecycle.log` (bind cancellation red) | `03569416206ffcd6b3d0f835438210e5c6b31d544ebf69b7431a0c3371e9c179` |
| `secure-network-subnet-red.log` | `f34321ec040c52956d76b3beb5cc42ed9192b1a499416bb74343ec2239e26c21` |
| `secure-network-artifact-red.log` | `4851275d9333b2bebaeeee6ace929a2503110f61565007966365cafd23ed8d5d` |
