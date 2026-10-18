# Private Codex launch plan — October 18, 2026

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), unchanged local
implementation authority from [decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
This is an optional adapter export and synthetic validation, not a working installed
application or provider/account acceptance. Historical demo launchers and captured
startup fixtures were preserved.

## Behavior and boundary

`privateCodexLaunchPlan` accepts explicit executable, guardian and owned-directory
paths. It derives private `home`, `state`, `workspace` and `tmp` children and refuses
unsafe/noncanonical locations, unexpected home/workspace contents and ambient
startup files. Optional auth metadata is checked without reading or parsing bytes.
It creates, repairs, copies and deletes nothing. Directory identities are checked
twice; this is not an atomic launch lease, ACL audit or protection against same-user
replacement between inspection and execution.

The returned immutable plan uses fixed stdio app-server arguments, existing no-tools
controls, explicit ChatGPT-only login and file-scoped credential stores. Telemetry
exporters and prompt logging are explicitly disabled. No model or fallback billing
path is selected. Only private HOME/CODEX_HOME/TMPDIR, system PATH, locale and UTC are
supplied; no ambient environment is copied. A macOS synthetic Node child additionally
created `__CF_USER_TEXT_ENCODING`; the test validates only that platform-added key
before comparing all other environment bytes.

This requests a private file store; it neither provisions credentials nor authorizes
moving an existing sign-in. Executable signature/hash/dependency provenance, creation
and ownership of these directories, actual runtime configuration and any account grant
remain composition/installation gates. The agent host and its computer-tool sandbox
remain distinct: a private environment does not confine the host process.

## Pinned official source

The prior [package audit](codex-package-audit.json) pins Codex source
`36650394c5b38c2990ccf2a3457165ca3e9d9726` (0.157.1). Its
[arg0 implementation](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/arg0/src/lib.rs)
reads `.env` from CODEX_HOME before ordinary CLI startup and can populate non-CODEX
environment keys. The plan refuses that entry, including a dangling symlink.

The [credential types](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/config/src/types.rs)
and [storage implementation](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/login/src/auth/storage.rs)
define file storage under CODEX_HOME and select a separate keyring backend for keyring
or auto mode. The new [source manifest](codex-auth-source-audit.json) records exact
public-file sizes/hashes. No credential content was retrieved. These source findings
do not establish behavior of a running packaged binary.

## Verification

- Initial public launch test failed because the export did not exist, then passed.
- 64 focused behavior checks passed; one opt-in benchmark skipped in the ordinary run.
  Negative controls include all three malformed supplied paths; missing, file,
  symlink and permission faults for every required directory; simulated foreign UID;
  ambient files/directories/dangling links; and synthetic auth symlink, hardlink,
  directory, empty, oversized and nonprivate metadata. Original bytes/modes/inodes
  were retained. No real foreign-owned directory was inspected.
- Full portable suite: **1,079 passed, 0 failed, 3 opt-in budgets skipped**.
- Affected Mac suites: **24 passed, 0 failed, 3 opt-in budgets skipped**. Compiled-only
  discovery/execution, foreign-host refusal, cancellation, parent death and independent
  process/group absence passed using synthetic runtimes and temporary guardians.
- Strict TypeScript and structural/accepted-spec gates passed. Compiled exports and
  external consumer declarations passed without TypeScript source loading.
- Twenty serial fresh-layout inspections: p50 **0.270 ms**, p95 **0.467 ms**, maximum
  **0.565 ms**, target p95 <=5 ms. Warm toolchain/filesystem caches, no provider or
  child startup in this measurement. Previously recorded hardware: Apple M1 Max,
  10 physical cores, 64 GiB; fresh checks: arm64 macOS 27.2, Node 26.5.0. The sandbox
  refused a fresh hardware `sysctl` query; no permission change was made. Raw samples:
  `/tmp/fort059-private-codex-plan-budget.json`; selected benchmark passed separately.

The first child-environment equality check failed on the macOS-created encoding key;
the test was narrowed to validate that exact addition, with no product environment
expansion. The first typecheck rejected the test mocking an optional `process.getuid`
method; the test now saves/restores the function directly. Final suites above passed.
An initial public-source download could not resolve DNS in the sandbox; the approved
read-only retry succeeded. No runtime/provider access was attempted.

Logs: `/tmp/fort059-private-codex-{artifact,portable,macos,typecheck}.log` and
`/tmp/fort059-private-codex-plan-budget.log`.

## Retained artifact and open gates

`successor/.build/codex-executor-private-059-20261018` contains 42 files and 131982
bytes, excluding its manifest. Manifest SHA256:
`78574fbddb709eba01e2e029d763d4fa4b8d3c0ecde0e4c0daf56dbe74bead4d`.
An independent file-set, length and SHA256 check passed. The prior 40-file artifact
is unchanged. This library contains no runtime binary, credentials, installer or
startup hooks; existing Fort licensing and donor provenance are unchanged.

Unrun: actual Codex invocation/configuration/account behavior; full signed helper
bootstrap/resource and legal closure; actual App Group or Mach listener; startup
registration; installed accessibility/screenshots; real provider; TestFlight.
Native builds were not repeated because this slice changes no Swift/project files.
Do not use this evidence to activate the incomplete helper or reuse a consumed demo.
