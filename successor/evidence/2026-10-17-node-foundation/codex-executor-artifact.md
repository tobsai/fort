# Optional compiled Codex executor

October 18, 2026. Governed by [Spec 059](../../../specs/059-modular-node-v1.md)
and [decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
This is a local compiled library and synthetic process proof. No real Codex,
provider account, credential, signing operation, startup registration, installation
or distribution was performed.

## Artifact and independence

`buildCodexExecutor` compiles the static dependency graph of `codex-executor.ts`
to JavaScript plus declarations, with Fort's existing MIT license, private package
metadata and a per-file size/SHA256 manifest. It exports the guarded worker and
parent composition functions. The artifact contains no service/database,
optional context implementation, prototype, fixture, donor or `node_modules`
directory. No install scripts, external package dependencies or updater are added.

Node 26.5.0, Codex 0.157.1 and the native Mac guardian are explicitly separate
requirements. Their executable provenance, signing, runtime legal attribution
and account integration remain separate gates. This library contains Fort-owned
adapter source, not a redistributed Codex binary. Donor Apache/NOTICE material
and the previously retained OpenAI license/NOTICE were not changed.

The builder refuses existing/relative destinations and retains unrelated files.
It creates a fresh private directory and, on failure, removes that directory only
when its current device/inode still matches the directory created by the call.
This is a local build ownership check, not protection against arbitrary concurrent
filesystem mutation. The compiler has an explicit empty environment; ambient
`NODE_OPTIONS` preload hooks are not inherited. No dependency install or network
operation is part of the build.

Retained artifact:
`successor/.build/codex-executor-059-20261018`.
Its 40 inventoried files total **128273 bytes**, excluding `manifest.json` itself.
Manifest SHA256:
`956a6f77c8fbabd3027dc0f904a17124fd2e405c2e399609fe4bf5cb115f8fea`.
An independent read of every file verified size/hash and complete file-set equality
after the build. The directory is local engineering output, not committed binaries.

Build command:
`node successor/packaging/codex-executor-artifact.ts --destination <absolute-new-directory>`.
The `build:codex-executor` npm script exposes the same explicit destination CLI.

## Verification and failures

Logs below share `/tmp/fort059-executor-artifact-`.

- The first build test failed with the missing builder module (`red.log`), then
  built, imported and typechecked the public declarations from an external strict
  TypeScript consumer (`portable-first.log`). Artifact runtime imports resolved
  with `--no-experimental-strip-types` and an empty environment.
- The first compiled Mac test failed because its contract fixture had not yet
  been added (`mac-red.log`). With that fixture, three checks passed
  (`mac-green.log`). Both the parent and worker imported only the generated
  JavaScript from a separate temporary directory. Their separate synthetic
  runtime used copied plain-JavaScript fixture files. No source checkout import
  or TypeScript loader was needed by those processes.
- Compiled discovery preserved the exact model. A subsequent new worker returned
  exact synthetic text. Both operations independently proved worker/runtime PIDs
  and both guardian process groups absent. A foreign host request produced no
  ready models and started no runtime child. Recorded successful journey PIDs:
  5424–5431; foreign-host outer guardian/worker: 5433/5434. These are historical
  observations, not authority to signal those IDs later.
- A new ambient-preload test reproduced an inherited `NODE_OPTIONS` hook running
  in the compiler (`env-red.log`). The authored synthetic hook wrote only a
  temporary marker and exited 42. The failed artifact directory was confirmed
  removed. Passing `env:{}` fixed that behavior; the same test now proves the
  hook did not run and the artifact completed (`portable-green.log`).
- Existing directory, file and symlink destinations are refused, with their
  exact unrelated contents preserved. Complete manifest/file/hash/license,
  absence of runtime dependencies/scripts and public declarations passed.
- The first full portable run passed 1014 and failed the explicit Mac test
  inventory (`portable-final.log`). Its expected list had not included the new
  Mac artifact test. The inventory now includes it in the privileged Mac suite
  and the portable artifact test in the ordinary suite.
- Final portable run: **1015 passed, zero failed, two opt-in budgets skipped**
  (`portable-confirmed.log`). The existing core artifact test still compiled and
  exercised two independently shaped providers with Codex implementations absent.
- Final affected Mac run: **24 passed, zero failed, three opt-in budgets skipped**
  (`mac-final.log`), covering compiled-artifact journeys plus prior guarded
  execution, discovery, cancellation and process/crash cleanup regressions.
- Strict TypeScript and the structural gate passed (`types.log`). Structural,
  accepted-implementation, exact staged-path and whitespace gates passed before
  committing. No new source warnings were waived.

## Build measurement

Same recorded host/toolchain as the preceding executor evidence: M1 Max, 10 cores,
64 GiB, macOS 27.2/26B5112a, Node 26.5.0, TypeScript 6.0.3, Swift 6.4 and Xcode
27.1/27A9275. Hardware is the prior recorded inventory, not a new observation.
Five serial builds used new destination directories with installed tools and warm
filesystem caches, no incremental compiler cache, and no concurrent test/build.
The timer includes compilation, declarations, license/package writes and all file
hashes. Signing, provider/network time and runtime executable packaging are excluded.
The first sample is retained.

Samples in milliseconds: **1252.579, 1217.763, 1228.664, 1231.343, 1233.159**.
p50 **1231.343 ms**, nearest-rank p95/maximum **1252.579 ms**; target p95 <=10000 ms.
Raw samples/log: `build-budget.json` and `build-budget.log` under the log prefix.
Command sets `FORT_EXECUTOR_BUILD_BENCHMARK` and runs the test named
`five isolated fresh executor builds meet the packaging budget` alone.

## Remaining

Production host bootstrap/helper composition, actual bundled runtime provenance,
provider/account grants, native packaging, Apple profiles, installed UI and
TestFlight delivery remain. The compiler artifact can be removed independently
of the provider-neutral core. It does not make the project a working installed
application. Swift/native/signed checks were not rerun for this JavaScript
packaging-only slice. No remote or visibility change, upload, provider cleanup,
or historical approval reset occurred.
