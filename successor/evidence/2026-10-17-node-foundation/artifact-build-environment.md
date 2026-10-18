# Artifact build-environment repair — October 18, 2026

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), recoverable local packaging
work under [decision 059](../../../docs/architecture/decisions/059-node-foundation.md).

Inspection found that the core builder still inherited its parent environment when
launching TypeScript. A failing test set an authored synthetic `NODE_OPTIONS` preload:
the marker was observed, compiler exit failed and the new failed artifact was removed.
No real credential or unrelated program was involved in that reproduction.

The compiler now receives an empty environment. Both core and executor Git license
fallbacks also receive empty environments and five-second timeouts. The core builder
requires pinned Node 26.5.0 and an exact new normalized/nonroot destination, records
its created device/inode, rejects nonregular or multiple-link output files and removes
a failed destination only when its current directory identity still matches. This
does not provide an atomic cleanup transaction against same-user replacement races.
No runtime contract, dependency, installation hook or provider authority changed.

## Verification

- Red: `syntheticPreloadRan:true`, `failedArtifactRemoved:true` in
  `/tmp/fort059-core-build-env-red.log`.
- Green: the same build succeeds with no preload marker. Six targeted checks passed,
  including original contents surviving unsafe/existing destination refusal, a replaced
  destination surviving compiler failure, and symlink/hardlink outputs being refused
  without changing their external synthetic targets. The replacement and emitted-link
  faults were injected through the compiler-process seam; they are not OS race proofs.
- **1,085 portable tests passed, 0 failed, 4 opt-in budgets skipped**. This includes
  complete manifest file/size/SHA256 checks, independent provider core contracts,
  compiled owned-core setup/send/replay/restart/disposal and executor artifact checks.
- Strict TypeScript and structural/accepted-spec checks passed.
- Five serial fresh core builds: p50 **1340.732 ms**, p95/max **1424.836 ms**, target
  <=10 seconds. Warm toolchain/filesystem caches, distinct empty destinations, no
  incremental output reuse or provider/network latency. Raw samples:
  `/tmp/fort059-core-build-env-budget.json`; benchmark passed separately.
- Previously recorded hardware: M1 Max, 10 physical cores, 64 GiB; current arm64
  macOS 27.2, Node 26.5.0 and TypeScript 6.0.3. The sandboxed Git fallback emitted
  Apple temporary/cache-directory warnings; builds completed successfully without
  changing permissions or installing tools. The first run also emitted Apple
  developer-tool cache/FSEvents warnings while resolving the platform Git command.

Logs: `/tmp/fort059-build-env-{targeted,portable,typecheck}.log` and
`/tmp/fort059-core-build-env-budget.log`.

## Artifact and limits

Retained core: `successor/.build/owned-core-isolated-059-20261018`, 30 files and
190826 bytes excluding manifest. Manifest SHA256:
`ca21c510d8f0333cc02560eb2aba03251d1bda6a68f953b2933a8c5b17f79413`.
An independent complete file-set/length/hash check passed. Every payload file matches
`owned-core-059-20261018`; only manifest ordering changed. Earlier artifacts remain.

No Swift/project/runtime behavior changed, so native builds and the Mac guardian
suites were not repeated for this packaging-only repair. No actual Codex/account,
new signing, profile, App Group, startup registration, installed UI, provider,
TestFlight or provider-cleanup action occurred. Helper bootstrap/resource assembly,
private directory/host identity preparation and external Apple/account/legal/runtime
acceptance remain open. This repair does not establish a working installed application.
