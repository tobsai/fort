# Candidate B named permissions — macOS implementation and offline evidence

## Current offline candidate: Codex 0.163.0

October 20 preparation pins source `83086188ae0614bd882818ff1aa23f3194962012`
and the exact reviewed arm64 binary. The installed runtime remains 0.157.1 and
its host remains disconnected. See the [offline update evidence](../evidence/2026-10-17-node-foundation/codex-0163-offline-update.md).
The candidate uses the existing managed ChatGPT device-code integration, with no
new authentication route or extracted token. No login or inference was performed.

The offline app-server reports 175 features, a bundled `gpt-6.1-sol` catalog entry,
and the selected no-files/no-network profile in an empty ephemeral thread. That
catalog is not account-entitlement or effective-inference proof. The current
adapter checks all four new opt-outs: cli_daybreak, api_key_model_discovery,
api_key_cyber_access_programs and ultrafast_mode. The upstream unified_exec selector
remains true despite its opt-out; shell_tool=false and empty environments remain
the shell authority controls. In-app Voice/browser permissions do not add Fort
modules or enable a callable operation.

A temporary diagnostic sandbox permits macOS preferences IPC after a presence-only
check confirms no forced Codex preference keys exist. It denies network and ordinary
user-file access. This is not complete host isolation and does not change Fort's
production permissions. Required administrator configuration is never bypassed.

The service refuses changing an existing ledger's runtime identity. An explicit
history-preserving binding migration is required before installing this candidate
over retained 0.157.1 data. Historical packets, pins, captured fixtures and legal
payloads below remain evidence and do not authorize execution.


The subsequent authorized [offline canary](../evidence/2026-10-16-codex-offline-canary/README.md)
ran the actual pinned CLI parser and a separate source-derived deny-rule experiment.
The exact profile parsed through to Codex's sandbox launcher; two negative controls
were rejected. macOS refused nested sandbox application, so the complete generated
policy remains untested. The isolated deny rule blocked the tested pathname and Unix
socket accesses, but pre-opened file/pipe/socket descriptors remained usable.
This was not a fourth live attempt. The original preparation scope below remains the
authority for the product source; provider activation is still blocked.

The owner authorized code and fixture validation after the complete startup audit.
This is not authority for another attempt, runtime/provider start, persistent settings,
OS/network/security changes or credential access. All three consumed packets remain
blocked; `manual.ts` still points to the third packet's historical pins.

## Chosen boundary

The profile `fort_candidate_b_no_files` narrows the old intended boundary. The old
request allowed reads beneath the empty workspace, no writes, no platform defaults
and no tool network. The new profile denies computer-tool reads and writes everywhere,
including that workspace, while retaining disabled tool networking. Text generation
needs no computer filesystem. The host's own access is a separate boundary.

The source launch plan includes these process-scoped TOML assignments:

```toml
default_permissions="fort_candidate_b_no_files"
permissions.fort_candidate_b_no_files={filesystem={"/**"="deny"},network={enabled=false}}
```

No settings file is written or loaded by this preparation. The launch-plan function
returns arguments only; its existing activation guard still prevents ordinary use.
It now also refuses non-macOS hosts because this audit relies on pinned macOS deny-glob
lowering. The model, executable hash, environment allowlist, timing, samples and
consumed-attempt fencing remain unchanged.

Why use a deny glob instead of just an empty filesystem map or `:root=deny`? Codex
adds readable paths for its runtime helpers after compiling the declared profile.
Its ordinary path-rule resolution prefers more specific grants; a root denial cannot
reliably counter those additions. The pinned macOS implementation retains unreadable
globs and appends explicit read/write deny rules after its ordinary grants. `/**`
lowers to the anchored pattern `^/.*$`. The complete policy still has a source-level
enforcement argument. The later isolated rule experiment tested ordinary,
Unicode/newline, symlink and hardlink paths on macOS; it did not apply Codex's complete
generated policy. No claim of bypass-proof OS confinement is made, and descriptor
revocation is explicitly not provided by the rule.

## Source trace at 36650394c5b38c2990ccf2a3457165ca3e9d9726

All references use official `openai/codex`, tag `rust-v0.157.1`.

| Step | Source and finding |
| --- | --- |
| Parse and inheritance | [permissions_toml.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/config/src/permissions_toml.rs): named entries are flattened; a profile has description, extends, workspace_roots, filesystem and network. Parents merge before children. Fort refuses inheritance and all extra declarations. |
| Compile | [core permissions.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/core/src/config/permissions.rs): custom profiles start with restricted empty filesystem and restricted network. Explicit false stays restricted. An absolute deny-glob entry becomes a GlobPattern. No `:minimal` entry means no profile-requested platform read defaults. |
| Managed policy | [config/mod.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/core/src/config/mod.rs): configured/managed profile ID collisions error. Allowed-profile requirements can substitute a required default; constraint fallback can clear active profile identity. Helper read paths are added later. |
| Catalog | [permission_profile_catalog.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/core/src/config/permission_profile_catalog.rs): `allowed` includes compilation, ID requirements and permission constraints. [catalog_processor.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/catalog_processor.rs) exposes paginated `permissionProfile/list` with cwd; no threadId or resolved rule list is returned. |
| Thread selection | [thread_processor.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/thread_processor.rs): `permissions` becomes the typed default_permissions override; cannot coexist with `sandbox`. Explicit empty runtime roots and environments remain empty. Response contains active profile ID/parent and legacy sandbox projection. |
| Turn selection | [turn_processor.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/turn_processor.rs): `permissions` cannot coexist with `sandboxPolicy`. It reloads the named profile through [config_manager.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/config_manager.rs), retaining the thread's session flags, and refuses a requirements-disallowed fallback before accepting the override. Explicit environment selections own their roots; top-level runtimeWorkspaceRoots is only a compatibility input when environments are not supplied. |
| Projection | [protocol permissions.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/protocol/src/permissions.rs): a restricted policy with no write entries projects to legacy readOnly; that projection does not enumerate read limits/deny globs. `with_additional_readable_roots` can add helpers but does not remove glob denials. |
| macOS enforcement lowering | [seatbelt.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/sandboxing/src/seatbelt.rs), `build_seatbelt_unreadable_glob_policy`, `seatbelt_regex_for_glob` and final policy assembly: unreadable globs become read/write denies appended after grants; protected ancestors receive unlink denials. Platform/default helper grants cannot be accepted as proof of the requested boundary without checking this lowering. |
| Reporting | [v2 permissions.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/permissions.rs) and [v2 thread.rs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/thread.rs): ActivePermissionProfile contains only id/extends. ThreadSettings reports that identity and a legacy sandbox summary. Neither is a complete effective-rule attestation; turn/start returns only the Turn. |

## Fort verification and refusal behavior

The declaration must contain exactly the deny-glob filesystem rule and explicit
`network.enabled:false`. Known optional fields may be absent or null, reflecting
Rust Option serialization; any inherited profile, extra read/write root, platform
default, glob-depth setting, proxy/domain/socket configuration or unknown field is
refused. This does not infer a required control from omission.

Preflight and the check after thread creation both verify config, all catalog pages,
explicit `allowed:true` for the unique exact profile ID and all existing feature/MCP
controls. Preflight also retains the existing authentication/model checks. Profile catalog missing,
false, malformed, duplicate, truncated or cyclic observations fail closed. The
configuration default must also name the exact profile.

Thread and turn requests use `permissions:fort_candidate_b_no_files`, empty environments
and roots, exact model/cwd, untrusted approval and explicit user review. No legacy
sandbox selector is sent. Thread creation must report the exact profile with explicit
null parent, empty runtime roots, unchanged model/provider/cwd, readOnly/network false,
user reviewer and no instruction files. A named-profile identity alone cannot pass.

Every observed `thread/settings/updated` report for the active thread is checked for
the same profile, policy, reviewer, model/provider and cwd. A contradiction closes
the transport and interrupts the turn if its ID is known; it cannot become a successful
reply. Notifications are not assumed to arrive on every turn, and this check does not
prove that a reported change had no earlier effect. Existing server permission, file,
command, tool and credential requests remain refused without a grant. User message
text cannot select a different profile. Feature gates still disable tools, and empty
environments remain the primary no-execution selection.

This combines an exact declaration, catalog availability, observed profile provenance,
legacy policy checks and pinned-source semantics. The API offers no complete resolved
filesystem snapshot at these seams. Thus this is **source-conditioned contract
verification**, not independent measurement of the runtime's final ACL or OS sandbox.
Concurrent same-user host mutation and a compromised runtime are outside this proof.

## Validation and real work remaining

The public test seams remain CodexChat preflight/execute, CodexTurn's wire protocol,
and the inert launch plan. Red-to-green tests cover the named-profile migration, TOML
launch arguments and previously ignored policy-change reports. The full fixture suite
covers allowed synthetic text; refusal of workspace/external/helper file permission
requests, writes and network escalation; altered/inherited declarations; managed
catalog denial/substitution; missing thread-start policy fields; pagination; user-text injection;
and shutdown/unknown-outcome handling. They do not run a filesystem access probe.

In the original preparation, all 15 `-c` assignments were independently parsed with Python's standard-library
TOML parser and compared with literal expected profile/default values. This validates
syntax, not Codex's Rust parser or runtime compilation. Source manifests and exact
passed-test counts are in that original evidence packet. No actual profile was applied
then. The later offline canary now separately establishes actual CLI parser acceptance
and isolated deny-rule behavior, with the limits stated above.

Before any real-provider proposal, separate authorization and evidence would be needed
for isolated non-provider runtime/OS canaries: exact binary profile parsing, actual
config/catalog/selection reports, helper additions, fallback behavior, and macOS
read/write/network denials. Include workspace and external files, symlinks/hardlinks,
rename paths, unusual/Unicode/newline filenames, pre-opened descriptors, IPC and
privileged daemon transport. Verify cancellation and cleanup against the same pins.
The offline canary covers a subset; actual app-server reports, complete generated
policy application, privileged-daemon transport and provider compatibility remain
unrun. Neither the rule experiment nor synthetic contracts replace those checks.

The agent host itself still starts with the user's HOME/access and provider-owned
authentication before Fort can inspect controls. The computer-tool profile does not
confine that host, Keychain, ambient startup effects or provider networking. Actual
provider compatibility, effective-model evidence, native pairing/UI signoff and
installation/release acceptance remain open. No further attempt is requested here.
