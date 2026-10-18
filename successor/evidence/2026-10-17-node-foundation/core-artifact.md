# Compiled core without provider implementations

Spec 059 slice. `npm --prefix successor run build:core -- --destination
/absolute/new-directory` compiles only the application dependency graph into ESM
JavaScript and declarations. It refuses an existing destination, includes Fort's
existing MIT license and emits file hashes. No production npm dependency is needed.
The Node runtime is separate; this is not the complete signed Apple app package.

The service-owned attempt type now has a provider-neutral file. All callers moved
to it, so the core's declarations do not pull in the Codex evidence protocol.
Codex source, prototype launchers, donor source and node_modules are absent from
the artifact. The donor's Apache license/provenance remains in foundation and is
not relicensed or copied into this independent source artifact.

`npm --prefix successor run test:core-artifact` passed one artifact test, zero
skips, which runs the same public HTTP journey against two independently shaped
fixtures. One handles each batch from supplied history. The other creates session
handles, retains a transcript and streams output pieces, then reconstructs a new
session from durable history after restart. The same checks cover exact Unicode,
selected binding/model, history, duplicate receipts, restart, unsupported discovery
and cancellation. They do not establish actual provider compatibility.

The proof process uses only the generated artifact and a copied inert fixture in
its permitted temporary read tree. Writes are limited to its data directory;
child-process permission is absent. JIT and TypeScript stripping are disabled.
These Node permissions constrain this trusted fixture; they are not proof of OS
confinement for an agent host. Only local HTTP is exercised by the fixture code.

Two intermediate failures were resolved without widening those permissions:
macOS /tmp symlink traversal required canonical paths; Node built-in fetch failed
with WebAssembly unavailable under --jitless, so the synthetic HTTP client uses
node:http. The server itself had started. Production adapter/network transport
compatibility under this constraint still needs validation.

Strict TypeScript passed after moving all attempt imports. Logs:
`/tmp/fort059-core-artifact-red.log`, `/tmp/fort059-core-artifact-final.log`,
`/tmp/fort059-core-artifact-typecheck-final.log`. A reviewable local artifact is at
`successor/.build/core-059-20261018`, including manifest.json. Its manifest names
exact produced bytes; it is a source slice artifact, not a released installation.
Optional memory/voice/connector/VM removal and real runtimes remain separate work.
