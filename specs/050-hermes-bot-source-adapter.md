# 050 — Hermes Bot source adapter

**Status:** Toby accepted the amended Slice 1 and directed implementation of
Spec 050 on 2026-08-22; the inventory contract/parser and Slice 2 immutable
repository/disabled option projection are implemented. Production Hermes
source registration remains uncomposed because the real local transport is
explicitly deferred by the verified process/build-binding blocker; no Hermes
execution, cloud eligibility, live mobile acceptance, or release is authorized.
**Decision owner:** Toby
**Depends on:** Spec 039 for executable identity and fail-closed readiness;
Spec 047 for the cloud control plane and enrolled workers; Spec 048 for stable
Agents, Execution Sources, Source Agents, immutable Binding Revisions, and the
provider-neutral `AgentSourceInventory` seam
**Upstream contract:** Hermes Agent `0.20.5`, tag `v2026.8.19`, commit
`fcbd1076a93841fa88855acce810e342a5b78101`
**Authoritative references:**
[release](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.8.19),
[Bot Mode](https://hermes-agent.nousresearch.com/docs/user-guide/bot-mode),
[profiles](https://hermes-agent.nousresearch.com/docs/user-guide/profiles),
[multi-connection Desktop](https://hermes-agent.nousresearch.com/docs/user-guide/multi-connection-desktop),
[pinned profile RPC](https://github.com/NousResearch/hermes-agent/blob/fcbd1076a93841fa88855acce810e342a5b78101/tui_gateway/methods_profiles.py),
[profile commands](https://hermes-agent.nousresearch.com/docs/reference/profile-commands),
[API server](https://hermes-agent.nousresearch.com/docs/user-guide/features/api-server),
and [security](https://hermes-agent.nousresearch.com/docs/user-guide/security/)

## Goal

Make every named Hermes Bot discoverable as a separate, source-qualified
Source Agent that a person can explicitly bind to a stable Fort Agent.

Hermes Bot Mode defines a Bot as a Hermes profile. Fort therefore maps:

```text
Hermes connection or gateway  -> Fort Execution Source
Hermes canonical profile name -> Source Agent opaque source ID
Hermes display name/avatar     -> presentation metadata only
Fort Agent                     -> durable Fort identity selected by a person
```

The same profile name on two computers produces two different Source Agents.
Discovery never creates a Fort Agent, changes Hermes' active profile, starts a
turn, or makes a source eligible by itself.

The first development slice implements a non-production, fail-closed Hermes
`AgentSourceInventory` adapter and derived pinned-contract fixtures. Later slices may
make those rows visible in the current Agent picker, authorize exact-profile
execution, and publish eligible cloud options only after their separate gates
in this spec pass.

## Why a separate adapter spec is required

Spec 048 defines the provider-neutral product model but explicitly authorizes
no concrete framework adapter. The shipped Hermes integration is not a Bot
adapter:

- the catalog contains only `hermes:configured-default` and one fixed
  provider/model profile;
- readiness probes the sticky configured profile with `hermes status --deep`;
- native execution invokes `hermes --oneshot` without a named `--profile`;
- the v2 production owner API injects `NoEligibleAgentOptions`; and
- the current iPhone/macOS Agent picker projects the older Primary capability
  inventory, so it cannot enumerate Hermes profiles.

The cloud Binding already stores `OpaqueSourceAgentID`, adapter revision,
source-configuration digest, authority, policy, and resolved model. The worker
currently validates those fields and then drops the source-agent identity when
it creates `runtime.RunSpec`. Exact Bot execution must close that lossy handoff
before any discovered profile becomes runnable.

## Canonical identity

A Hermes Bot identity is exactly:

```text
(execution_source_id, canonical_profile_id)
```

`canonical_profile_id` is the Hermes profile `name`, including the special
`default` ID. It must match:

```text
^[a-z0-9][a-z0-9_-]{0,63}$
```

Hermes `0.20.5` also reserves `hermes`, `test`, `tmp`, `root`, and `sudo`;
those IDs are rejected. `default` is the one special built-in identity.

It is case-sensitive after Hermes' own canonicalization. Alias commands,
filesystem paths, descriptions, display names, avatars, models, provider
names, and the currently active profile are never identity.

Fort derives the durable projection ID as:

```text
source-agent:hermes:v1:<lower-hex-sha256(
  "hermes-source-agent:v1\n" +
  decimal_utf8_byte_length(execution_source_id) + ":" + execution_source_id +
  decimal_utf8_byte_length(canonical_profile_id) + ":" + canonical_profile_id
)>
```

The canonical profile ID is framework-native, not secret. Every API and
persistence access remains account- and source-scoped. The hash is not a
substitute for the source-qualified tuple and cannot be used to merge records.

Changing `display_name` does not rebind an Agent. Renaming a canonical Hermes
profile removes the old Source Agent from the current inventory and discovers
a new one; Fort does not guess that the two are equivalent. Existing Binding
Revisions retain the old identity and become unavailable until a person
accepts an explicit Rebind.

## Accepted upstream discovery contract

Hermes `0.20.5` has no documented machine-readable public CLI command for
enumerating profiles. `hermes profile list` and `hermes profile show` are human
output without `--json`; Fort does not parse them in the first adapter. Fort
also does not scan `~/.hermes`, import Hermes' Python modules, or depend on its
SQLite schema.

The accepted release also has a structured dashboard `GET /api/profiles`
route, but neither it nor the Desktop/plugin RPC is a stable public Hermes API.
Fort deliberately selects the WebSocket JSON-RPC method `profiles.list` as one
private, exact-revision dependency:

```json
{"jsonrpc":"2.0","id":"<request-id>","method":"profiles.list","params":{"include_sessions":false}}
```

The accepted result shape is:

```json
{
  "jsonrpc": "2.0",
  "id": "<same-request-id>",
  "result": {
    "profiles": [
      {
        "name": "researcher",
        "path": "<discarded>",
        "is_default": false,
        "model": "<untrusted inventory hint>",
        "provider": "<untrusted inventory hint>",
        "description": "<untrusted presentation text>",
        "display_name": "Researcher",
        "skill_count": 12,
        "ui_meta_revisions": {},
        "has_avatar": false
      }
    ],
    "bot_mode_protocol": true
  }
}
```

`bot_mode_protocol` must be present and exactly `true`; it feature-detects the
accepted Bot Mode backend contract rather than merely finding a method with the
same name.

`model` and `provider` may be strings or JSON `null`. Release-valid rows may
include a raw `ui_meta` object, and `ui_meta_revisions` may be absent when
profile metadata cannot be read. The frozen decoder recognizes these variants
and discards their values; it does not reject valid `0.20.5` rows merely
because an optional field is present or absent.

The request always sends `include_sessions:false` and never sends preferred
session IDs. Session summaries, source filesystem paths, environment-presence
flags, credential state, aliases, distribution sources, raw UI metadata,
SOUL content, skill names, tool names, MCP configuration, and Hermes chat IDs
are not copied into Fort inventory, logs, errors, or telemetry.

Slice 1 is limited to an explicitly configured, authenticated **local** Hermes
backend belonging to the enrolled worker and Execution Source. The transport
must attest the pinned local connection/process identity and accepted Hermes
code revision; a successful response from some other reachable backend cannot
be relabeled as the configured source. The adapter does not start `hermes
serve`, install a gateway, change a profile, or fall back to another transport.
A source exposing only the documented public API-server surface cannot
enumerate profiles and remains unavailable unless its permitted profile IDs
are later provisioned through a separately approved Fort bridge.

For the local Slice 1 source, the exact supported Hermes executable, source
revision, and accepted RPC shape are all required. The version string alone is
insufficient. Fort records the launcher plus resolved Python code-root identity
in adapter catalog data after independent validation; hashing only the small
`hermes` wrapper is insufficient. An unrecorded build stays unavailable even
if it reports `0.20.5`. Current Hermes `main`, later releases, and locally
modified checkouts are unsupported until their contract fixtures and code
identities are reviewed and the adapter revision advances. A future remote or
Hermes Cloud source requires its own signed build/protocol attestation seam; a
local executable digest is never projected onto a remote gateway.

The accepted local WebSocket auth is also exact: loopback `/api/ws` with its
process-lifetime dashboard session token in the upstream-required `token`
query parameter. The enrolled source supplies only a Fort secret-store
reference to a deliberately configured `HERMES_DASHBOARD_SESSION_TOKEN`; the
value never enters inventory, URLs in logs, errors, events, or receipts.
`X-Hermes-Session-Token` is a dashboard REST credential, and
profile-specific `API_SERVER_KEY` authenticates API-server paths; neither is
silently substituted for this WebSocket contract. OAuth-gated WebSocket
tickets and cookies require a later separate transport revision.

### Local attestation implementation finding

Slice 1 implementation confirmed that the current repository cannot truthfully
produce `VerifiedRPCExchange` from a loopback TCP endpoint and dashboard token
alone. Those inputs prove token possession, but Darwin loopback TCP does not
expose the accepting server PID through peer credentials. A client-side socket,
PID file, port-table scan, or independently hashed Python tree also does not
prove that the process on that exact connection loaded the measured code.

The real transport therefore remains absent and uncomposed. The attestation
requirement is not weakened and the inventory adapter can be exercised only
through its explicit external seam. A successor transport decision requires
separate approval of one verifiable ownership mechanism, such as Fort spawning
and supervising the exact held backend, an upstream signed process/build
attestation, or an authenticated Unix-socket bridge with peer credentials. No
endpoint-plus-token fallback is permitted.

Toby accepted this deferral on 2026-08-22 as the amended Slice 1 boundary.

## Inventory adapter contract

The concrete implementation lives in `exec/hermesbot` and implements the
existing `core/runtime.AgentSourceInventory` interface. `core` imports no
Hermes package.

Its single true external-dependency seam is a bounded raw RPC round trip:

```go
type LocalProfileRosterTransport interface {
    RoundTrip(context.Context, []byte) (VerifiedRPCExchange, error)
}

type VerifiedRPCExchange struct {
    ConnectionIdentity string
    LauncherDigest     string
    CodeRootDigest     string
    HermesVersion      string
    HermesRevision     string
    Body               io.ReadCloser
}
```

This interface is transport and out-of-band local source verification only. It
cannot create, update, delete, select, or execute a profile. Its production
implementation measures the resolved launcher and Python code root separately
from the RPC response, binds that measurement to the local process accepting
the connection, and returns both in one exchange. Neither `profiles.list` nor
Hermes' `gateway.ready` frame is accepted as build attestation. The Hermes
module owns request construction, bounded response reading, strict envelope
decoding, and sanitization. Production construction is withheld until the
local attestation finding above is resolved. Tests supply sanitized, derived
JSON fixtures transcribed from the pinned source contract plus explicit
out-of-band measurements at this same boundary; they can also inspect the exact
outgoing request. These are not claimed as live-captured server bytes.

If an exchange returns a non-nil `Body`, its `Close` must be safe to call while
`Read` is active, must promptly interrupt that read, and the read must exit.
The adapter closes it at most once and joins an interrupted read before
returning. A future real transport should normally finish the bounded network
read inside its context-governed round trip and return an in-memory body; a
stream that cannot satisfy this close-to-interrupt rule is invalid.

`Inventory`:

1. rejects a request whose `ExecutionSourceID` does not exactly equal its
   configured source;
2. requires the response's connection identity to match the endpoint/process
   identity pinned by the enrolled Execution Source;
3. verifies the adapter revision, Hermes executable/source revision,
   authentication, RPC envelope, matching request ID, and result shape;
4. accepts at most 256 profiles and 1 MiB of bounded response data within a
   five-second discovery deadline;
5. validates every canonical profile ID and rejects duplicate IDs;
6. maps every row to the attested configured Execution Source without trusting
   upstream paths or source labels;
7. chooses a normalized `display_name` of at most 64 Unicode scalar values,
   falling back to the canonical profile ID when it is empty or invalid;
8. assigns one injected UTC observation instant to `ObservedAt`, the Execution
   Source's `LastSeenAt`, and every returned Source Agent's `LastSeenAt`;
9. emits an allocated list sorted by canonical profile ID; and
10. validates the complete `AgentSourceInventorySnapshot` before returning it.

Malformed envelopes, missing or unknown fields, truncated or oversized data,
duplicate/invalid profile IDs, authentication errors, timeouts, revision
drift, and unavailable connections fail the whole observation. Fort never
publishes a partial roster as current truth. A successful empty roster is
`[]`, never `null`.

Discovery evidence uses closed, non-secret codes and exact revisions. It does
not echo upstream error bodies. Initial discovered rows carry:

```text
contract_id       = source-agent.inventory.hermes-bot.v1
contract_revision = ed131bdba193ddebe4f4445296dcba9282f1d6672d2f8396fa94dd8c38959d3b
capabilities      = []
ready             = false
evidence          = [profile_discovered, execution_adapter_not_approved]
```

The revision is the SHA-256 of `InventoryContractManifest`, which binds the
pinned upstream revision, request/result schema, profile and identity rules,
limits, ordering, readiness evidence, resource disclosure, attestation fields,
privacy behavior, and cancellation/body contract.

The existing provider-neutral snapshot validator is corrected in Slice 1 to
require an allocated capability list for every Source Agent and at least one
capability only when `ready=true`. An inventory adapter's ability to enumerate
profiles is not falsely advertised as a capability of each discovered Bot.

`ready=false` is intentional. Profile existence, configured model/provider,
gateway PID, `.env` presence, and a successful roster response do not prove
that an inference turn can run. The first slice cannot be resolved into an
`EligibleAgentOption` and cannot be dispatched.

## Resource-sharing disclosure

Hermes profiles isolate Hermes configuration, memory, sessions, skills, cron
jobs, and state. They are not host sandboxes. On a host install, tools normally
retain the real OS-user `HOME`, so Git, SSH, browser, cloud CLI, and other
credentials may be shared even when Hermes profile state is separate.

The first adapter reports conservative source-level declarations:

| Resource | Initial scope | Reason |
| --- | --- | --- |
| provider credentials | `unknown` | Hermes may use profile files, a shared OAuth pool, environment, or gateway credentials |
| filesystem | `machine_shared` | profiles do not constrain host filesystem access |
| browser sessions | `machine_shared` | a profile is not a browser or OS-user boundary |
| framework sessions | `profile_scoped` | Hermes profile session/state stores are separate |
| source-managed memory | `unknown` | profile-local memory is normal, but an external memory provider may be shared |
| tool/MCP configuration | `profile_scoped` | configuration is profile-owned, while invoked tools may reach shared host state |

If Fort cannot verify a declaration for the enrolled transport and revision,
it reports `unknown`; it never upgrades isolation based on a profile name,
avatar, SOUL prompt, or `terminal.cwd`.

## Exact-profile execution gate

This spec does not yet authorize a production Hermes runtime. Before one Bot
can become `ready`, a successor implementation section and Toby's explicit
approval must freeze all of the following:

- the exact opaque Source Agent selector preserved byte-for-byte from the
  pinned Binding through adapter preparation, runtime-specific dispatch, and
  the execution receipt; the later design decides whether this deepens
  `runtime.RunSpec`, uses a binding-aware worker adapter, or enters a typed
  remote-runtime request;
- one exact transport and command/protocol revision for local and/or remote
  execution;
- exact profile selection on every operation with `--profile <canonical-id>`
  or `/p/<percent-encoded-canonical-id>/`; no sticky `profile use` and no
  default-profile fallback;
- prompt input, output framing, cancellation, timeout, maximum size, terminal
  status, provider/model attribution, and source-session behavior;
- exact provider/model and source-configuration digest revalidation before
  process or network start, with no model override or fallback chain;
- profile-specific authentication for named remote paths;
- a declared tool, filesystem, network, browser, MCP, memory, approval, and
  working-directory authority policy; and
- a real bounded smoke turn whose result and exact adapter receipt persist in
  Fort.

The existing command is explicitly rejected as that contract:

```text
hermes --oneshot <prompt> --accept-hooks --yolo
```

Hermes top-level `-z`/`--oneshot` loads the selected profile's tools, skills,
memory, rules, and working-directory context and unconditionally enables both
YOLO mode and hook acceptance, even when the two explicit flags are omitted.
It auto-resolves clarifications and bypasses dangerous-command approval. The
entire top-level one-shot surface is therefore rejected for this adapter. A
Hermes Bot is not a text-only Agent, and this authority cannot cross into the
approved Codex subscription lane merely to make a discovered Bot selectable.

Candidate upstream execution surfaces to evaluate at the later gate include:

- local stdin input through
  `hermes --profile <id> chat -Q --source tool --query-file -`;
- registered-peer `hermes peer dm <peer>/<profile> --json`; and
- authenticated standalone `/v1/runs`, or multiplexed and allowlisted
  `/p/<profile>/v1/runs`, plus detailed health/capability probes.

None is selected here. In particular, `peer dm` targets Hermes' canonical Bot
Chat, whose source-native transcript is noncanonical in Fort and must not be
silently shared across Fort Conversations. That Bot Chat can also expose
Hermes' `message_agent` capability, so `peer dm` remains rejected unless a
future contract disables and verifies that native cross-agent path or
explicitly authorizes it within Fort's Handoff limits.

## Product and API integration

Inventory, visibility, enrollment, and execution are separate claims.

### Slice 1 — non-production inventory proof

Add the new `exec/hermesbot` bounded raw RPC contract, derived `0.20.5`
contract fixtures, `AgentSourceInventory` implementation, and the minimal
provider-neutral validation correction for empty capabilities on unready
Source Agents. The real read-only local transport remains blocked and is not
included or composed. Do not change the static Spec 039 catalog, current Agent
picker, production composition, cloud option resolver, native provider, or
TestFlight application.

### Slice 2 — discovered-but-unavailable local rows

After Slice 1 is accepted, add a provider-neutral latest-inventory repository
that records successful immutable snapshots and preserves the last known
roster when a later observation fails. Current/stale state, observation time,
source last-seen time, and the closed latest failure reason remain distinct.
Then add a source-inventory-backed implementation of the existing
`control.AgentOptionSource` and compose it additively with the current Primary
option source. The existing `/api/agent-options` and FortKit wire model can
display one source-qualified, disabled Hermes row per profile with the closed
reason `execution_adapter_not_approved` or an explicit stale/offline reason.

This slice must not create an Agent Channel from an unavailable row. It is a
temporary v1 projection only; stable v2 identity remains authoritative.

#### Frozen Slice 2 contract

The provider-neutral repository is an account- and Execution-Source-scoped,
append-only observation log. A successful observation stores the validated
snapshot as Fort's RFC 8785 canonical JSON subset together with its SHA-256
digest. An allocated empty successful roster is valid and becomes the current
roster. A failed observation
stores no snapshot or upstream error text; it stores only the closed reason
`source_inventory_unavailable`. Database insertion sequence, not a caller's
timestamp, determines the latest attempt.

The latest projection keeps these facts separate:

- the last successful immutable snapshot and its original `ObservedAt`;
- `current` when the latest attempt succeeded or `stale` when it failed;
- the latest attempt time;
- the Execution Source's last successful `LastSeenAt`; and
- the closed latest failure reason and failure observation time.

A later failure never overwrites the last successful roster. A later successful
empty roster supersedes it. Snapshot digest or validation failure on read is a
repository error, not a partially trusted roster. Observation rows cannot be
updated or deleted.

The temporary v1 option projection uses the immutable Source Agent ID as its
opaque option ID and emits state `unavailable`. A current discovered row uses
reason `execution_adapter_not_approved`; a stale retained row uses
`source_inventory_unavailable`. It never promotes a Source Agent whose inventory
claims `ready=true`; that contradiction fails closed. It does not trust or
publish upstream provider/model hints.

Because the v1 `AgentOption` wire type still requires an execution-shaped
`Binding`, the disabled row carries only honest source/profile presentation
identity plus explicit `unknown`/`unapproved` placeholders. Its execution-policy
map is deliberately allocated but empty, so `AgentBinding.Validate` rejects it
even if a later defect changed the option state. The row is presentation
evidence, not an immutable executable Binding Revision.

Primary and source-inventory options compose additively. Ready Primary rows sort
first; all remaining rows sort deterministically by option ID. Duplicate IDs
fail closed. `RecheckAgentOptions` invokes only an explicitly registered
inventory adapter; a discovery failure is appended as the closed failure above
and returns any retained stale roster, while repository corruption or write
failure remains a top-level error. Caller cancellation propagates without being
misreported as source failure. With no approved real Hermes transport, no
fixture, human CLI output, or endpoint-plus-token response may seed production,
and the production command does not register a Hermes source inventory.

### Slice 3 — exact execution and eligible options

After the execution gate is approved, close the Binding-to-`RunSpec` identity
loss, add a dedicated Hermes runtime contract, and prove one exact named Bot
turn. Only then may the corresponding row become ready.

For v2, add a bounded list endpoint under `/api/v2` that returns opaque option
IDs backed by current inventory and immutable server-held evidence. Clients
never assemble source, profile, model, adapter, or authority fields. Production
`AgentOptionResolver` remains `NoEligibleAgentOptions` until the adapter,
inventory persistence, expiration rules, and exact runtime are all composed.

### Slice 4 — native client acceptance

Make named Hermes options visible in Web, macOS, and iPhone, create two stable
Fort Agents from two different Hermes profiles, and prove each opens its own
canonical Fort Conversation. Source-managed Hermes sessions remain disclosed
and noncanonical.

## Test seams proposed for approval

Toby confirmed these public seams on 2026-08-22. Slice 1 follows the red-green
sequence below.

### Slice 1 public seam

Tests call only these existing provider-neutral public seams:

```text
core/runtime.AgentSourceInventorySnapshot.Validate
core/runtime.AgentSourceInventory.Inventory
```

through a constructed `exec/hermesbot` adapter. The sole fake is the true
external `LocalProfileRosterTransport`; it receives raw request bytes and
returns raw response bytes plus out-of-band local source measurements. Clock
and request IDs are injected deterministic values. Tests do not target private
parsing helpers.

The first red suite will prove:

1. two Execution Sources exposing `researcher` produce different Source Agent
   identities and are never merged by display name;
2. reversed upstream order produces stable sorted Fort output, and an empty
   successful roster is an allocated empty list;
3. wrong-source requests, wrong attested connection identity, invalid or
   duplicate canonical IDs, envelope/request ID mismatch, malformed/oversized
   responses, unsupported revision, authentication failure, timeout, and
   unavailable transport fail closed;
4. no filesystem path, environment, credential, session, description, raw UI
   metadata, or upstream error body reaches the returned snapshot or public
   error;
5. discovery always returns `ready=false` with the exact inventory contract
   and closed evidence until an execution adapter is approved; and
6. the snapshot, including an allocated empty capability list for every
   unready Bot and one consistent observation timestamp, validates through the
   corrected provider-neutral core contract without a `core` dependency on
   Hermes.

### Slice 2 public seams

Slice 2 tests use these provider-neutral public seams:

```text
core/runtime.AgentSourceInventoryRepository
control.AgentOptionSource.AgentOptions
control.AgentOptionSource.RecheckAgentOptions
control.AgentChannelService.CreateAgentChannel
```

Repository tests exercise the real SQLite `store.Store`, including close and
reopen; they do not mock the database. The cross-slice integration test uses the
real Hermes inventory adapter and real repository, faking only the approved
external `LocalProfileRosterTransport`. Projection-focused tests may use a
small repository implementation to isolate option ordering and closed failure
behavior, but acceptance requires the real-adapter/real-store test as well.

The Slice 2 red-green sequence proves successful snapshot durability, append
sequence ordering, stale preservation after failure, empty-roster recovery,
account/source scope, immutable rows, digest verification, current/stale option
reasons, ready-claim rejection, additive Primary ordering, duplicate rejection,
and non-enrollment of every inventory-only row.

### Later execution seam

The next separately confirmed red suite will exercise:

```text
cloudworker.AdapterRegistry.Prepare -> runtime.RunSpec
runtime.Runtime.Dispatch
```

It will prove the exact opaque profile selector, source/config/adapter/policy
revisions, provider/model, and authority survive without fallback, and that
any mismatch blocks before Hermes starts.

## Verification sequence

For Slice 1, after the approved red test is observed:

```text
go test ./exec/hermesbot -count=1
go test ./core/runtime ./exec/hermesbot -count=1
go test -race ./core/runtime ./exec/hermesbot
go test ./... -count=1
```

For Slice 2:

```text
go test ./core/runtime ./core/store ./control ./exec/hermesbot ./cloud/migration -count=1
go test -race ./core/runtime ./core/store ./control ./exec/hermesbot -count=1
go test ./... -count=1
```

Later UI or release slices also require live inspection of the exact installed
application through accessibility plus a screenshot. A disabled row after the
execution contract is valid, a raw error code, a duplicate Fort bundle, a
non-ready selected Bot, or a failed real smoke turn blocks signoff.

## Acceptance criteria

- Every accepted Hermes profile appears exactly once per Execution Source.
- Same-named profiles on different sources remain distinct.
- Display-name, active-profile, model, and gateway-state changes never alter
  Source Agent identity.
- Discovery cannot mutate Hermes state or execute a model/tool turn.
- Private upstream fields and errors never leak into Fort's public surface.
- Unsupported or drifted Hermes builds fail closed with no partial current
  roster.
- Inventory-only rows cannot enroll or dispatch an Agent.
- Once a later execution adapter is approved, dispatch selects the exact
  immutable canonical profile and never another profile, source, machine,
  provider, or model.
- The final mobile flow can create at least two stable Fort Agents backed by
  two distinct Hermes profiles, and both survive relaunch with their exact
  Fort canonical Conversations and immutable receipts.

## Non-goals

- creating, cloning, editing, renaming, or deleting Hermes profiles;
- importing Hermes Bot Chat history as authoritative Fort history;
- treating Hermes avatars, SOUL, skills, memory, or cron rows as Fort-owned;
- parsing profile directories, `.env`, `auth.json`, SQLite, or human CLI tables;
- silently starting or configuring a Hermes server/gateway;
- using Hermes provider/model fallback chains;
- advertising Hermes as text-only or sandboxed;
- enabling Agent-to-Agent messages, group rooms, routines, or Handoffs through
  Hermes-native behavior; or
- changing the current Codex subscription adapter or its authority.

## Rollback

Before execution eligibility, rollback removes the new adapter composition and
leaves only immutable inventory observations; no Hermes or Fort Agent state is
changed. After a future runtime is enabled, disabling its adapter makes bound
Agents unavailable without rebinding, deleting history, or falling back to a
different source.

## Approval gates

Approval of this spec confirms only:

1. a Hermes Bot maps to one source-qualified Hermes profile;
2. the exact revision-gated `profiles.list` surface is accepted for the first
   bounded inventory adapter despite being a private upstream API;
3. Slice 1 may begin at the public `AgentSourceInventory.Inventory` seam using
   the six red-test behaviors above; and
4. discovered profiles remain `ready=false` and non-enrollable.

Approval does **not** authorize Hermes execution, tools, source-session reuse,
profile mutation, production/cloud eligibility, mobile release, or any
external side effect. Each requires its later gate above.

Toby's subsequent instruction to implement Spec 050 on 2026-08-22 authorizes
the frozen, side-effect-free Slice 2 repository and disabled option projection
above. It does not select a real roster transport, authorize Slice 3, or weaken
any execution, eligibility, smoke-turn, or release gate.
