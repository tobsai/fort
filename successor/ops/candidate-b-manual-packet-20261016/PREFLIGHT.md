# Candidate B — exact no-turn preflight

This is a specification for the minimal disabled execution wiring still needed,
not an executable instruction to start the provider. A scope decision alone is
insufficient until that wiring has a reviewed pin. Current `CodexChat.execute()`
does not expose this boundary; using it as a preflight would dispatch a turn.

## Before the session clock

Verify the 759 reviewed source files, five app files, runtime hash and exact
launch-plan arguments against the existing evidence. Verify the proposed
empty workspace, temporary directory and fresh service-data path identities;
refuse symlinks, reused state or unrelated pending work. Do not erase existing
state to make the check pass. All session paths are specified in `packet.json`.
The app's normal data directory is `~/Library/Application Support/FortCandidateB`;
inspect only candidate-owned state after authorization. It must contain no old
connection or pending work before the first enrollment. An occupied directory is
a stop condition, not permission to delete or silently switch a client path.

Retained HOME requires a narrow, redacted **configuration metadata** check before
launch: establish the applicable user/system/project layers and the complete MCP
server names/enablement plus startup hooks/plugins for this cwd. Do not read auth
files or Keychain, execute config commands, resolve secret references or output
raw TOML, command arguments, URLs, environment values or integration credentials.
Any locally embedded secret must never enter tool output/evidence. This metadata
inspection has not been performed; its narrow access is part of the proposed
later preflight authority, not permission granted by this preparation.

The current three explicit MCP overrides are acceptable only if the full merged
set proves no other server enabled. Unknown layers/defaults, an unreviewed enabled
server or startup integration stop the test before launching. Do not edit global
config or add overrides reactively. If this metadata cannot be established without
additional sensitive access, report that specific obstacle. No wholesale config
copy, fresh login or alternate HOME is allowed as a workaround. The prelaunch
check limits avoidable discovery; it does not establish host confinement.

Prepare all non-sensitive UI instructions before starting. Obtain the user's
readiness, but do not prestart the runtime or app. The full session clock starts
with the first candidate service/app/runtime start and includes enrollment,
configuration checks, samples and cleanup. Any child timer must use the earlier
outer deadline; starting the provider later does not reset the five-minute clock.

## Same-process startup checks, before accepting any sample

Launch exactly one owned official process per session using the pinned plan.
No shell, inherited environment spread, alternate binary or preliminary
`codex --version` process is permitted. The environment contains only HOME, PATH,
LANG, TZ and TMPDIR as recorded. Startup may itself read/write provider-owned
state and contact its provider; “no turn” does not mean “no side effects.”

Use one connection and initialize it once. The minimal wiring must preserve that
initialized connection for all turns; it must not send a second initialize when
entering `CodexChat`. All requests below are non-inference protocol requests.
Reject unsolicited approval/tool requests and stop; never accept their effects.

| Order | Exact request / observation | Required evidence |
| --- | --- | --- |
| 1 | `initialize` with current Fort client identity, `experimentalApi:true`, `requestAttestation:false`; then `initialized` | Successful structured response, no rejected CLI flag, startup integration or unexpected child. Static binary hash establishes identity; a version echo is supplementary. |
| 2 | `config/read` with `{includeLayers:false,cwd:<exact empty workspace>}` | Complete effective configuration sufficient to verify the expected provider, disabled discovery, updater, analytics, notify, skills, tools and full `mcp_servers` map. Project only allowlisted booleans, counts and identifiers locally; discard raw response without recording it. Missing/ambiguous evidence is a stop, not a default. |
| 3 | `experimentalFeature/list`, paginated to completion | Every feature from the pinned disable list is false and `skip_host_skill_discovery` true. Stop on mismatch or unknown required feature. Do not invoke enablement/set. |
| 4 | After step 2 proves **zero enabled MCP servers**, `mcpServerStatus/list` with `cursor:null,limit:100,detail:"toolsAndAuthOnly"`, following all cursors | Zero connected/connecting servers, tools, resources and templates, with no discovery error or incomplete inventory. No calls to servers, resource reads, reloads or OAuth. This API may perform discovery; never use it to investigate a known enabled server. Stop first. |
| 5 | `account/read` with `{refreshToken:false}` | Only `account.type == "chatgpt"` and `requiresOpenaiAuth == true` recorded. Never retain account email, ID or token. Missing/expired/wrong auth stops; no login, account switch, explicit refresh or API billing. |
| 6 | `model/list` with `cursor:null,limit:100`, all pages within existing eight-page bound | Exact `gpt-6-astra` present, without alias/fallback. Record only requested-model availability; do not dump unrelated inventory. |

Check every `-c` value and `--disable`/`--enable` entry from the pinned launch
plan, not just its argument presence. In particular, reject an ambient custom
provider endpoint, gateway, executable credential helper or API-key credential
source for the selected OpenAI path. Record only whether the normal ChatGPT
provider route is confirmed. The earlier schema inspection did not prove that
`config/read` exposes all required values; if it does not, stop and identify the
missing evidence rather than infer safe defaults.

The session supervisor must keep HTTP dispatch closed or explicitly gated during
preflight (`autoRun:false` alone is not a complete sample gate), bind only
`127.0.0.1`, admit only the exact authorized sample for the current ordinal, and
retain a metadata-only pass receipt. Do not log raw runtime stdout/stderr or
configuration. Do not make the local device credential until this passes and the
candidate-specific decision permits it. Enrollment and workspace setup then
occur within the same session budget. If insufficient time remains, stop.

## Before each of the five turns

Prepare the existing ephemeral `thread/start` for the exact accepted recipient,
Agent instructions, selected instruction-skill revision, model and workspace.
Pause after its response and before `turn/start`; this is the reviewed
cancellation fence plus the new preflight gate, not a separate diagnostic turn.
Require the current response checks: exact model/provider/cwd, ephemeral thread,
`untrusted` approval, readOnly sandbox, `networkAccess:false`, and no instruction
sources. Verify thread-specific MCP status with the returned `threadId`, after
the global no-MCP check, and require the same zero-capability result.

The empty environments/tools/capability roots and `sandboxPolicy.access` requests
are not all ordinary v0.157.1 schema fields. A successful reply alone cannot
prove they apply. Use the effective configuration and reported thread policy as
the evidence available; retain unknown host/tool limitations in the result.
This packet does not require new OS confinement or claim these extras enforce it.
It does require the stated disabled feature/server settings to be observable.

Only then may the exact authorized `turn/start` be written, with text_elements
present and cancellation rechecked synchronously. No tool invocation, diagnostic
prompt, warmup turn, extra sample, automatic uncertain retry or model fallback.
Current per-turn interruption occurs at 60 seconds with a two-second confirmation
allowance; the earlier outer-session cutoff always wins. Record selected model
and unreported effective model separately.

## Shutdown and conditional second start

Stop accepting samples before the work cutoff. Close the actual app; await
service close and runtime disposal, observed child exit and owned group absence,
closed loopback listener and actual app-process exit. Timer expiry or SIGKILL is
not proof of provider-side cancellation or absence of escaped descendants. Any
unconfirmed cleanup or unknown run ends the packet without restart.

Before the second start, require four retained accepted receipt IDs and four
distinct durable runs in `completed`, matched to samples 1–4 and their exact
conversation/recipient/revision. The actual Mac queue must be empty after app
exit and again before relaunch. Its file is
`FortCandidateB/<sha256(installationId UTF-8)>/pending.json` in Application Support.
Check decoded array count only; do not print pending text or tokens. A missing
file after four sends, malformed file, nonempty queue or identity mismatch fails.
Checking the visible conversation or synthetic harness queue is insufficient.

Session two may reopen only that same ledger/installation and unchanged native
state. Revalidate all four completed runs before permitting sample 5. Repeat the
same runtime preflight in its one permitted process. Native restore automatically
replays eligible pending requests; therefore the queue check precedes app launch.
Do not clear, edit or flush it. Repeated reads are allowed; duplicate submissions
from the conformance harness are not part of this live packet.

## Minimal wiring and evidence still to prepare

Implement only the above same-connection preflight pause, exact-sample admission,
outer deadline/start counter and cleanup/queue receipts. Keep the activation guard
closed until an exact decision is recorded. Cover failure-before-turn and the
restart gates nonlive, then commit and record the changed source pin without
claiming the old 759-file list matches it. Rebuild/repin the app only if Apple
source or signing actually changes. This is bounded execution plumbing, not a
new runtime architecture or a request for unrelated hardening.

Capture only timestamps, pins, start/sample ordinals, request/run/conversation/
Agent/skill revision IDs, selected-model identity, config pass/fail booleans,
queue count, completion state, listener/child/group/app-exit results and the user's
observations. Sample text may be referenced by packet ordinal/hash. Never record
auth/config contents, enrollment code, device token or raw provider output. Local
Fort history naturally holds the five sample exchanges; do not export unrelated
history. Keep any failure evidence and stop rather than consuming extra starts.

## Versioned protocol references

These public schemas were inspected while preparing this document; no installed
runtime was executed. They describe the proposed requests, not observed behavior:

- [ClientRequest methods](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/ClientRequest.ts)
- [ConfigReadParams](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ConfigReadParams.ts), [ConfigReadResponse](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ConfigReadResponse.ts), [Config](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/Config.ts)
- [ExperimentalFeature](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ExperimentalFeature.ts)
- [ListMcpServerStatusParams](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ListMcpServerStatusParams.ts), [McpServerStatusDetail](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/McpServerStatusDetail.ts), [McpServerStatus](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/McpServerStatus.ts)
- [ThreadStartResponse](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ThreadStartResponse.ts)
