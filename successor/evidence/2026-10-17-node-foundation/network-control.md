# Explicit secure listener control

October 19, 2026. [Spec 059](../../../specs/059-modular-node-v1.md), base
`fc32c964fd817eccbdf8e0537b744f8af59e178b`, under unchanged-scope
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).

The optional controller starts stopped and requires an explicit owner authorization
callback. Its JSON command carries exact owner, controller session and generation.
A start advances the generation once; an exact retry observes that attempt without
binding again. Changed retries and stale stops fail. Close is final and shared.
Opening failure and interrupted shutdown distinguish confirmed absence from an
unconfirmed result; unconfirmed cleanup blocks replacement. Status never contains
TLS material. Configuration is bounded, copied before asynchronous work and kept
only in memory. Import and construction have no listener or filesystem side effect.

This slice uses injected authorization in fixtures. It does not yet compose the
actual primary-owner check, private host channel or native UI. The controller does
not itself watch service membership changes after activation. Its owning host must
close the listener when authority ends. No public HTTP route receives control
privilege. Agent-host access and computer-tool confinement remain distinct.

## Verification

- Public controller tests first failed for the absent module. Subsequent behavioral
  reds exposed failed-open cleanup classification, cancellation while binding,
  adoption by a different owner and authority loss during opening. Each now passes.
  A temporary TypeScript parameter-property syntax was unsupported by Node's
  strip-only execution and was replaced with a separately declared readonly field.
- Controller focus: 12 passed, one opt-in budget skipped, zero failures. Cases cover
  real disposable TLS/private sockets, exact retries, malformed/foreign commands,
  concurrent requests, caller mutation, final close, interrupted closure and missing
  shutdown acknowledgment. Faults enter through Node's public server boundary.
  `/tmp/fort059-network-control-focus.log`.
- The compiled export test first failed because the controller artifact was absent.
  Export/declarations plus confirmed and unconfirmed failed-open cases then passed:
  three checks, zero failures. The permission-mode artifact import/inspect/close ran
  without network permission and with filesystem reads confined to the artifact.
  `/tmp/fort059-network-control-artifact-red.log` and
  `/tmp/fort059-network-control-compiled-focus.log`.
- Full portable suite: **1,292 passed, 14 skipped, zero failures**, 41.056 seconds.
  Skips are 13 opt-in budgets and the unavailable alternate 127.0.0.2 source address.
  No interface setting was changed. `/tmp/fort059-network-control-portable.log`.
- Strict TypeScript passed using Node 26.5.0, TypeScript 6.0.3 and @types/node 26.6.5.
  `/tmp/fort059-network-control-typecheck.log`.
- Serial benchmark, five warmups then 20 samples: inspect/start/exact retry/TLS
  request/confirmed stop p50 **3.207 ms**, p95 **3.841 ms**, maximum **5.377 ms**;
  p95 target 100 ms. Identity generation, provider, LAN and user time are excluded.
  One disposable identity/private socket, no concurrent Fort build/test/QA workload.
  MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2 arm64. Samples:
  `/tmp/fort059-network-control-samples.json`; log:
  `/tmp/fort059-network-control-budget.log`.
- Fresh artifact `/tmp/fort059-network-control-core`: **46 manifest payload files,
  262,242 payload bytes; 47 files and 269,585 bytes including manifest**. Every size
  and SHA256 matched independently; no unexpected file was present. Manifest SHA256:
  `6d4734d7917091795bb766c2e11253cc62633b5999a638428ffd5311b5ca83af`.
  Previous engineering packages remain untouched.

Use `/opt/homebrew/opt/node@26/bin/node` with
`successor/verification/test-suite.mjs --suite portable`; strict check is
`successor/node_modules/typescript/bin/tsc -p successor/tsconfig.json`.
The isolated benchmark uses `--test --test-name-pattern='twenty isolated owner
listener' successor/host/network-control.test.ts` and a fresh output path in
FORT_NETWORK_CONTROL_BENCHMARK. The builder uses
`successor/packaging/core-artifact.ts --destination <new-absolute-directory>`.

## Remaining gates

Private owning-host composition and Mac listener/invitation controls follow next.
Native, guarded Mac, signed and installed UI suites were not repeated for this
controller-only slice. No real provider, production listener/TLS identity/trust,
startup registration, physical device, archive, upload or TestFlight check ran.
No new dependency, account, credential store, entitlement or automatic restore was
added. Source/build/fixture evidence does not establish a usable installed product.

Separately authorized Apple G2 creation remains unperformed. Safari now has an
existing authenticated session for team T3JB5MYZ93 with role Admin; Developer ID
Application creation is disabled. Apple lists Tobias Gunn as Account Holder and
shows an updated agreement requiring that holder's action. The agreement was not
accepted. The installed 1Password extension is unlocked and offers two relevant
saved Apple logins; exact Account Holder selection is pending with the owner.
No password was revealed, copied or transmitted. No new key, CSR or certificate
was generated, installed, exported or revoked. Xcode's system-component Install
prompt remains untouched. G2 direct distribution does not unblock TestFlight.
