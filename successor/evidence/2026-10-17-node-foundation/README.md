# Node foundation evidence — first packaging slice

[Spec 059](../../../specs/059-modular-node-v1.md) and owner decision at
`922c6e1d5fcc74dd68110e868cd687437238fec8` govern this work.
[results.json](results.json) records exact source hashes and observed limits.

The packaging CLI traverses the installed Node dependency closure, copies it to
a new owned directory, rewrites load commands, signs each copied image with the
explicitly selected existing identity and records source/output hashes plus legal
attribution. It does not change installed Homebrew libraries or install a service.

Passed with Node 26.5.0: one real artifact test, zero failures/skips; execution of
SQLite returns `v26.5.0|ready`; every executable/library has no Homebrew load path
and passes strict signature verification; TypeScript typecheck passes. The test
uses a fresh synthetic HOME and minimal PATH, then removes its own artifact.
Invocation: existing Apple Development fingerprint supplied through
`FORT_SIGNING_IDENTITY` to `npm --prefix successor run test:packaging` (the actual
verified run invoked the equivalent Node test command directly). Without an
explicit identity this credential-dependent check reports a skip, not a pass.

Hardened runtime rejected ad-hoc signatures on the shared executable/libraries;
using the same existing development identity fixed the Team-ID requirement.
Library validation was not disabled. `--jitless` and disabled built-in type
stripping select precompiled JavaScript, avoiding TypeScript's WebAssembly loader.
An SQL quoting mistake in the test fixture was corrected with bound parameters.
No performance conclusion is drawn from this single test duration.

SQLite's installed package has no top-level license file. Its public header
contains the dedication; that exact header is preserved with package attribution.
This agrees with [SQLite's official copyright statement](https://sqlite.org/copyright.html).
Node's [jitless option](https://nodejs.org/api/cli.html#--jitless) disables runtime
executable-memory allocation and can affect performance; benchmarks remain open.

This is **not** an App Sandbox or TestFlight pass. The next proof is the signed
Swift parent plus inherited-sandbox Node child, synthetic IPC/storage and negative
controls. Official runtime authentication, persistent grants, background lifecycle,
archive/upload/Apple availability and physical acceptance remain unrun. No provider
was contacted, key exported, credential created or startup job registered.
