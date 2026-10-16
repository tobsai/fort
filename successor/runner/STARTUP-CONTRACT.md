# Codex 0.157.1 contract audit — blocked before another live attempt

The third Candidate B attempt stopped at `features/unified_exec`, with one consumed
start and no receipts. This audit makes no runtime/provider call and reads no
credentials or real user configuration. All three attempt directories and historical
packets remain immutable. The current manual entry still targets the consumed third
packet; its source pins are not updated by this audit.

Three startup checks were incompatible with the reviewed implementation. Their
corrections are covered by failing-then-passing public-interface regressions:

1. `unified_exec` is forced on unless a managed requirement pins it off. It selects
   an execution backend. `shell_tool:false` and no execution environment prevent
   shell-tool registration. Turning `unified_exec` off alone still permits one-shot
   execution. Fort now requires an explicit boolean row, retaining the explicit
   false `shell_tool` requirement and empty thread/turn environments.
2. MCP status lists disabled configured servers. Fort now accepts only known,
   explicitly disabled configuration entries with no tools/resources/capabilities,
   `authStatus:unsupported`, no plugin origin, and the exact inert runtime status
   (`null` globally, `disabled` in thread scope). Unknown, duplicate, incomplete or
   active rows still fail. This is not permission to discover enabled servers.
3. Model catalog `id` and invocation `model` are distinct fields. Availability must
   match the exact `model` slug. A matching display ID cannot authorize another model.

**A separate hard incompatibility remains:** Fort sends
`sandboxPolicy.access.type:restricted` in `turn/start`. The pinned Rust deserializer
explicitly rejects it. Omitting the field would discard the requested read boundary,
so this revision preserves it. The new source-grounded conversation fixture models
that refusal and produces `outcomeUnknown`, never a successful provider reply.
Earlier transport fixtures remain synthetic behavior tests, not compatibility proof.

## Immutable source

Official repository: https://github.com/openai/codex

- Tag: `rust-v0.157.1`; annotated object `ac0e23e5232692b95268583c8278c50b8c436d2b`.
- Source commit: `36650394c5b38c2990ccf2a3457165ca3e9d9726`.
- Installed executable pin, read as bytes only:
  `/opt/homebrew/Caskroom/codex/0.157.1/bin/codex`;
  SHA-256 `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`.
- Public source URLs and hashes are sealed in the linked audit evidence. No upstream
  code was compiled or executed. The tag/source inspection is not a reproducible-build
  attestation linking the installed executable to every source byte.

Key source anchors at that commit:

- [Managed feature normalization](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/core/src/config/managed_features.rs), `normalize_candidate`.
- [Tool registration](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/core/src/tools/spec_plan.rs), `add_shell_tools` and environment-dependent tools.
- [Feature registry](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/features/src/lib.rs) and `features/src/legacy.rs`.
- [Catalog pagination](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/catalog_processor.rs).
- [Config read](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/config_manager_service.rs), `read`; `request_processors/config_processor.rs` overlays only its enumerated effective features.
- [MCP response](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/mcp_processor.rs), `list_mcp_server_status_response`; `codex-mcp/src/connection_manager.rs` filters disabled servers before connection startup; `mcp/auth.rs` returns unsupported before credential discovery for a disabled server.
- [Model mapping](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/models.rs), `model_from_preset`.
- [Thread fields](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/thread.rs); `request_processors.rs` preserves an explicit empty environment list.
- [Sandbox deserialization](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/permissions.rs), `SandboxPolicyDeserialize` and `Deserialize for SandboxPolicy` (restricted reads rejected).

## Gate-by-gate findings

| Gate | Pinned contract and current treatment | Nonlive coverage / unresolved evidence |
| --- | --- | --- |
| Entry, version, launch | Exact executable/version/hash, empty workspace, allowlisted environment, owned-process deadline; consumed attempt and source pins fence reuse. No launcher flags changed. | Existing manual, owned-process, session and launch-plan tests. No actual executable invocation. |
| Initialize | `experimentalApi:true`, `requestAttestation:false` supported. No login call. | Exact emitted capabilities checked. Initialization errors remain redacted. |
| Config/read | Typed merged config plus flattened additional keys; not an authoritative inventory of every resolved feature. Required options supplied by the launcher must be present with exact values. | Missing and changed values tested for all 13 required paths below. Ambient routing/instructions/telemetry/MCP/computer cases refused. |
| Feature pages | All 150 canonical registry entries, including removed/stable flags; no stage filtering. `limit:100` needs two pages. Explicit terminal `nextCursor:null`. | Both pages and global/thread scopes exercised. Missing flags are never false; duplicate/nonboolean rows fail. |
| Unified exec | Canonical stable flag, normally forced true by normalization; managed false remains possible. | Explicit true/false accepted only with all other gates, including shell false; absence/nonboolean and shell enablement refused in both scopes. |
| Other feature flags | Canonical keys, accepted input aliases and removed-but-listed keys classified below. | Every required canonical flag tested missing, malformed and enabled, globally and after thread start. Alias negatives retained from the previous fix. |
| MCP pages | Configured disabled entries are emitted; global runtime status null, thread disabled. Disabled entries never enter the connection loop; unsupported auth returned without per-server credential discovery. | Both scopes, missing fields, active/inconsistent rows, duplicates, tools/resources/plugins and unknown names refused. Configuration is checked first. |
| Pagination bounds | Feature, MCP and model APIs use offset cursor strings and explicit terminal null. Fort permits at most eight pages of at most 100 records. | Missing/empty/numeric/repeated cursors, oversized/nonarray data and a ninth page refused for all three APIs. Late feature/model data checked. |
| Account/read | Existing `chatgpt` account and `requiresOpenaiAuth:true`; `refreshToken:false`. | Null, other auth class, missing account and false/missing requirement refused. Actual account not read. This call can perform workspace-routing discovery; it is not a network-free probe. |
| Model/list | Server filters hidden entries unless requested, pages results, may fetch an uncached catalog. `model` is invocation identity, `id` catalog identity. | Different IDs with exact slug pass; same ID with wrong/missing slug fails; later pages checked. Actual model access remains unverified. |
| Thread/start request | Exact model/provider/cwd, ephemeral, untrusted approval, read-only mode; empty environments, dynamic tools and selected roots are supported experimental fields. Omitted provider fallback defaults false. | Exact request fields checked. Stable generated schemas omit experimental fields; Rust definitions establish support. No real thread created. |
| Thread/start response | Model/provider/cwd, ephemeral, approval, explicit readOnly/network false, empty instructionSources remain required. | Missing/changed values of every checked field refused. Empty instructionSources remains required, not guessed from project_doc_max_bytes. |
| Thread recheck | Feature/MCP APIs accept threadId and reload session layers; config/read is repeated for the same workspace. | Every required feature and MCP invariant exercised again before turn dispatch. This is not a guarantee against concurrent host-config mutation. |
| Turn/start | Empty environments and exact model/approval supported. Legacy `readOnly.access:restricted` **rejected**, not omitted, defaulted or accepted. | Source-defined rejection modeled; no completion claimed. Restriction is preserved pending a named-profile design. |
| Tool refusal, cancel and lifetime | Existing refusal of server tool/authority requests, framing/correlation, time limits and owned cleanup remain. | Existing transport/process/session suite passes. No host-confinement, escaped-descendant, remote cancellation or provider-outcome proof. |

Required config paths: `analytics.enabled=false`, `check_for_update_on_startup=false`,
`model_provider=openai`, `notify=[]`, `web_search=disabled`, `project_doc_max_bytes=0`,
`skills.include_instructions=false`, `skills.bundled.enabled=false`,
`cloud.skills.enabled=false`, `orchestrator.mcp.enabled=false`, and explicit false
for `mcp_servers.spritepipe.enabled`, `mcp_servers.node_repl.enabled` and
`mcp_servers.computer-use.enabled`. Every additional MCP entry must also be false.
Nullable optional config fields retain their documented semantics; no required
control is satisfied by an absent value. Config echoing `features.unified_exec=false`
does not overrule the effective feature inventory.

| Requested feature class | Canonical observations required |
| --- | --- |
| Ordinary disabled controls | Explicit false: apps, plugins, hooks, memories, multi_agent, multi_agent_v2, shell_tool, skill_search, skill_mcp_dependency_install, image_generation, browser_use, browser_use_external, browser_use_full_cdp_access, computer_use. |
| Accepted input aliases | memory_tool → memories; imagegenext → image_generation. Aliases need not be listed, but canonical rows must be false; if aliases are emitted, enabled/malformed values still fail. |
| Removed, still listed by this version | Explicit false: plugin_hooks, remote_control, remote_models, js_repl, skill_env_var_dependency_prompt. Removed means the input opt-out is inert; absence from the inventory still fails. |
| Backend selector | unified_exec explicit boolean; true is forced except for managed requirements. It cannot substitute for shell_tool:false. |
| Required enabled control | skip_host_skill_discovery explicit true. Missing/false is refused. |

The fixture includes remaining registry entries to represent full pagination; those
entries are not a newly approved capability allowlist or a proof of all app-server
startup effects. Synthetic effective values assume no overriding managed requirement.
An actual contradictory requirement still fails the corresponding gate.

## Fixture scope and smallest safe next change

`fixtures/codex-0.157.1-startup.json` is newly authored synthetic protocol data using
the public registry names and schema fields. It contains no real account, payload,
credential, workspace configuration or captured inventory. Default-enabled values
are for macOS; beta display copy is intentionally absent. No upstream implementation
was copied. Public source remains attributed to OpenAI's Apache-2.0 repository.
Generated stable JSON/TypeScript schemas were inspected, but no independent JSON
Schema validator or compiled Rust deserializer was run. The custom Rust deserializer
reveals a restriction that the generated SandboxPolicy JSON schema alone omits.

The smallest safe next implementation is a Fort-owned, narrowly scoped named
permissions profile that preserves the empty-workspace read boundary, no writes and
no tool network access, selected with `permissions` on both thread and turn. This
parameter cannot coexist with the legacy `sandbox` / `sandboxPolicy` selectors.
Validate config parsing, managed-requirement behavior, profile identity/provenance
(`activePermissionProfile`), resolved permissions and the absence of broader roots
through nonlive tests first. Do not merely replace `access:restricted` with omission
or `fullAccess`, or infer policy from the legacy readOnly response. No profile or
real settings were created in this audit. Exact profile syntax and enforcement still
need implementation proof; this is a bounded next design task, not a ready command.

Even an accepted computer-tool profile does not confine the same-user agent host,
which starts with HOME/access before preflight. Filesystem, Keychain, IPC and network
isolation remain separate open gates. No sign-in, provider cleanup, settings change,
startup service, deployment, native launch or new attempt is authorized here.
