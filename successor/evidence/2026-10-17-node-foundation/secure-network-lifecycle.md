# Observable secure transport shutdown

October 19, 2026. [Spec 059](../../../specs/059-modular-node-v1.md), base
`ddccfeb58dfed0c13b2859c3cd5f3813b770bb70`, under unchanged-scope
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).

The optional `openSecureNetwork` result now exposes a readonly `closed` promise.
It remains pending while serving. Explicit close, caller cancellation and server
errors use the same shutdown result. Confirmed shutdown fulfills it; an error or
missing shutdown acknowledgment rejects it. Repeated close calls retain that result.
A timeout cannot later become success. Observing the promise does not start or stop
anything. The existing five-second deadline, authentication, subnet, TLS identity,
request limits and private owner boundaries remain unchanged.

This resolves a composition gap: previously an abort or server error stopped the
transport without notifying a caller that had not itself called close. Future Mac
controls can observe the actual lifetime. No controller, ordinary host composition,
production listener, credential store or automatic restart is added here. Listener
shutdown is not whole-host removal and does not revoke a device grant.

## Verification

- The first public lifetime test failed because `closed` was absent. It then passed
  with real disposable loopback TLS, an authenticated-header request, cancellation,
  failed subsequent connection and successful exact-port reuse.
- Four focused lifecycle cases passed. These cover explicit concurrent close,
  cancellation, an injected server error and a missing server acknowledgment.
  The last case rejected every observer after approximately five seconds and kept
  rejecting on retry. Faults are injected at Node's server boundary; requests and
  listeners use actual local sockets. `/tmp/fort059-network-lifetime-focus.log`.
- Full portable suite: **1,278 passed, 13 skipped, zero failed**, 35.791 seconds.
  `/tmp/fort059-network-lifetime-portable.log`. The skips are 12 opt-in budgets and
  the unavailable 127.0.0.2 source-address check. No interface setting was changed.
- Strict TypeScript compilation passed with Node 26.5.0 and TypeScript 6.0.3:
  `node successor/node_modules/typescript/bin/tsc -p successor/tsconfig.json`.
- Both isolated budgets passed serially after the suite, with five warmups and
  20 samples each. A 1 KiB TLS response had p50 **2.346 ms**, p95 **2.774 ms**,
  maximum 2.928 ms and shutdown 0.337 ms. The open/request/abort/closed/port-reuse/
  replacement-close journey had p50 **3.374 ms**, p95 **3.718 ms**, maximum 4.005 ms.
  Both p95 budgets are 100 ms. One disposable identity and synthetic private socket
  were used; key generation, provider, LAN and user time are excluded.
  Samples are `/tmp/fort059-network-lifetime-request-samples.json` and
  `/tmp/fort059-network-lifetime-samples.json`; log is
  `/tmp/fort059-network-lifetime-budget.log`.
- Existing hardware inventory: MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2, arm64.
  Benchmarks had no concurrent Fort test/build or QA app.
- Fresh compiled core at `/tmp/fort059-network-lifetime-core` has **42 files,
  251,959 bytes**, each independently matched to the builder's manifest. Its emitted
  secure-network declaration contains readonly `closed: Promise<void>`.
  Manifest SHA256: `53461716799eec0c3018d070b8441b4ac9c767cdf6f06353290dc2567173f0ec`.
  The portable suite also passed compiled export/declaration and network-disabled
  import checks. The retained engineering app packages were not replaced.

Reproduce with the pinned Node binary `/opt/homebrew/opt/node@26/bin/node`:
`successor/verification/test-suite.mjs --suite portable`; then `--test
--test-name-pattern='twenty isolated' successor/host/secure-network.test.ts` with
fresh output paths in FORT_SECURE_NETWORK_BENCHMARK and
FORT_SECURE_NETWORK_LIFETIME_BENCHMARK. Benchmark output refuses overwrite.

## Limits and remaining work

Native Swift tests/builds, guarded Mac/signed suites and UI screenshots were not
repeated for this transport-only change. Latest native evidence remains ddccfeb's
317 passed and 15 skips; prior QA screenshots remain tied to 75d072a. No actual
provider, LAN, production TLS/trust, startup service, phone, archive or TestFlight
acceptance ran. Mac listener and companion controls still require composition and
exact installed-product verification. The agent-host and computer-tool sandbox
boundaries remain separate.

Separately approved Apple G2 Developer ID Application creation is waiting for the
owner's direct Apple sign-in. Xcode requires system-wide components before Settings;
that Install action remains untouched. Safari shows Apple's sign-in page. Both old
team T3JB5MYZ93 identities remain present; no new key/CSR/certificate was created or
installed and no certificate was revoked. This direct-distribution certificate does
not establish TestFlight readiness. This status updates the current handoff, not the
historical approval state recorded by earlier commits.
