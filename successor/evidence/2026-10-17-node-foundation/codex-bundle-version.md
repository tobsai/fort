# Pinned Codex executable in the signed sandbox

October 18, 2026, following `5119736`. Scope: [Spec 059](../../../specs/059-modular-node-v1.md)
under the existing [Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
The exact reviewed Codex 0.157.1 binary answered **`codex-cli 0.157.1`** when copied
into the existing engineering sandbox bundle and invoked only with `--version`.
This is executable feasibility, not app-server, account, provider or release acceptance.

The scenario requires an explicit `--codex-runtime` path and checks its bytes before
creating any artifact. Missing, wrong or implicitly selected payloads are refused.
The installed source is unchanged:

- `/opt/homebrew/Caskroom/codex/0.157.1/bin/codex`, 238,223,808 bytes, arm64.
- SHA-256 `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`.
- Reviewed official source commit `36650394c5b38c2990ccf2a3457165ca3e9d9726`.

The local copy is at the normal `Contents/MacOS/codex` code location. Re-signing
changes its signature bytes; it is explicitly a Fort engineering copy, not the
original vendor signature. No installed runtime, settings or credentials are modified.
Its only entitlements are App Sandbox and sandbox inheritance. The parent retains
App Sandbox alone, and Node retains the same two child entitlements. No network,
JIT, file-access, Keychain or App Group entitlement was added.

The fixture supplies a fresh empty HOME, explicit existing private CODEX_HOME,
separate temporary directory and fixed PATH, with no inherited environment or stdin.
The command has a five-second limit inside the existing parent/process-group bound.
Only bounded version stdout/stderr are captured. No app-server, sign-in, model call,
account query, startup registration, updater or provider session is invoked.
The Node fixture's existing outside-file read/write and loopback-listener denial
controls still pass. They are distinct from actual Codex computer-tool enforcement,
which is untested here; agent-host and tool-sandbox boundaries remain separate.

## Startup and package audit

The [source audit manifest](codex-package-audit.json) pins the public source URLs,
byte sizes and hashes. Read-only source copies are in `/tmp/fort059-codex-package-audit`.
No source was compiled or executed. This is not a reproducible-build attestation
connecting every upstream source byte to the installed payload.

[CLI main](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/cli/src/main.rs)
initializes build information and passes through the arg0 dispatcher before its CLI
parser. The [dispatcher](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/arg0/src/lib.rs)
loads a Codex-home dotenv file and prepares PATH aliases before that parser. Therefore
an ambient `--version` invocation would not be a metadata-only read. This probe instead
uses fresh synthetic storage inside the existing sandbox. The
[home resolver](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/utils/home-dir/src/lib.rs)
requires an explicitly selected home to exist and canonicalizes it. No owner config
or authentication was copied into this fixture.

[Package detection](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/install-context/src/lib.rs)
recognizes particular layouts: a package `bin` directory, resources layout, and an
app-like `CodexCLI.app` wrapper whose metadata/helpers remain in the outer package.
A generic `FortSandboxProbe.app/Contents/MacOS` copy is not that wrapper layout.
[Build-info resolution](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/build-info/src/lib.rs)
uses package metadata when present and a source-build representation otherwise.
The observed CLI version response does not prove the eventual app-server's metadata
or helper-discovery behavior. Do not silently relax its identity/version checks.
The production payload layout remains an integration gate.

The installed package contains separate code-mode, rg, shell and voice payloads.
None was copied or enabled. The copied executable's 15 dynamic dependencies resolve
only to `/System/Library` or `/usr/lib`; this verifies that limited dynamic closure,
not optional resources, runtime-loaded helpers or a complete redistribution audit.

## Legal provenance

The exact upstream Apache-2.0 `LICENSE` and `NOTICE` are retained under
`packaging/vendor/codex-0.157.1/` and copied into the bundle's `Resources/CodexLicenses`.
The notice retains OpenAI and Ratatui attribution. Hashes are checked before packaging
and again against the retained artifact:

| File | SHA-256 |
| --- | --- |
| LICENSE | `d17f227e4df5da1600391338865ce0f3055211760a36688f816941d58232d8dc` |
| NOTICE | `9d71575ecfd9a843fc1677b0efb08053c6ba9fd686a0de1a6f5382fd3c220915` |

`codex-provenance.json` binds source commit, original hash and re-signed copy hash.
This engineering copy is not a completed third-party redistribution closure or
TestFlight packaging decision. The quarantined donor and its attribution are untouched.

## Tests and retained failures

- `/tmp/fort059-codex-bundle-red.log`: the public packaging test initially failed
  because `--codex-runtime` was unsupported. No runtime was launched.
- `...-typecheck.log`: TypeScript rejected an unknown JSON property captured in a
  callback. Narrowing an immutable local PID fixed it; `...-typecheck-pass.log` passes.
- `...-first-run.log`: the first signed fixture passed a URL to Node's string-only
  spawn API and stopped before Codex launch. The failed artifact remains at
  `/tmp/fort059-codex-version-test-Y8r6JF`. Its parent/Node PIDs 72952/72960, group
  72952 and synthetic container were confirmed absent; no Codex PID was created.
  The installed source hash remained unchanged. Converting to a filesystem path
  corrected the fixture without changing runtime or sandbox policy.
- `...-version.log`: explicit-scope refusal and actual version-only tests both passed.
- `...-signed.log`: **all six affected packaging checks passed with zero skips**,
  including the existing XPC identity/rejection, compiled core, abrupt recovery and
  timeout/cleanup scenarios. They ran serially with the exact selected signing identity
  and `FORT_TEST_CODEX=/opt/homebrew/Caskroom/codex/0.157.1/bin/codex`.

No application/core/native implementation changed in this slice. Portable service and
native suites/client builds were not repeated. Their prior evidence remains separate:
the preceding storage slice passed 173 executed native functions and fresh unsigned
Mac/iPhone Simulator builds. CoreSimulator UI, physical devices and provider use remain
unrun. The new probe is optional and does not enter the vendor-free core artifact.

## Retained artifact and independent verification

Artifact:
`successor/.build/codex-version-059-20261018/FortSandboxProbe.app`.
Identity: `io.fort.foundation.probe.43249c4a-5fd2-4810-946d-6e0060d70e81`.
Bundled Codex SHA-256:
`4c7c2360a0868be40c72d6c1242a5e28dc228589ef4a76749da82279c5f85084`.

The independent read-only verifier `/tmp/fort059-verify-codex-bundle.py` passed:
deep/strict signature verification; exact parent/Node/Codex entitlement sets;
team `T3JB5MYZ93` and hardened-runtime flags; original/bundled/legal/provenance hashes;
15 system-only Codex dynamic dependencies; PIDs **75160, 75177, 75179** and process
group **75160** absent; all three exact executable paths absent from the process list;
and the exact synthetic container absent. There was no forced cleanup.
Result: `/tmp/fort059-codex-bundle-verify.json`.

Raw retained run: `/tmp/fort059-codex-bundle-retained.json` and the artifact's
`../result.json`; diagnostics file `...-retained.log` is empty. Codex exited zero with
empty stderr in **29.821459 ms**, within five seconds. The Node fixture's execution
window was **168.762958 ms**; parent startup was **5,524.659208 ms** and the full
observed run **5,697.023083 ms**. The startup delay was not attributed to a particular
OS operation. This is one fresh-identity engineering sample with warm installed
runtime bytes, not a percentile or production startup-budget pass. Build/signing,
provider, account and UI costs are separate. No concurrent workload ran during this
retained invocation. Hardware/toolchain match the preceding storage evidence.

The retained artifact is not an installed product, persistent host or TestFlight build.
Production app-like wrapper/configuration, exact signed App Group/profile, official
app-server metadata/auth integration, guarded executor adapter and reachable iPhone
transport remain open. No activation is claimed.
