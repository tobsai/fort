# Grok Bot and Hermes Bot Mode — reference architecture for Fort

**Researched:** 2026-08-21
**Method:** first-party product documentation, repositories, and release records
only.
**Feeds:** [Spec 047](../../specs/047-vercel-supabase-cloud-control-plane.md)
and [Spec 048](../../specs/048-stable-agents-group-chats-and-handoffs.md).

## Conclusion

Grok Bot and Hermes Bot Mode independently validate the central Fort product
idea: the primary thing in the sidebar is a durable, named agent; opening that
agent opens a continuing relationship; conversations, activity, approvals,
handoffs, and routines live around that agent.

They validate the product model, but neither is a drop-in deployment design for
Spec 047:

- **Grok Bot** is the closer product and cloud-execution reference. It is a
  managed, closed service with account-synchronized clients and one persistent
  cloud computer per user. Its public materials do not identify its database,
  control-plane services, dispatch protocol, or streaming transport.
- **Hermes Bot Mode** is the closer open implementation and multi-machine
  identity reference. It is a UI over Hermes profiles and gateways; state and
  execution remain on the gateway that owns each profile. It does not provide a
  centralized durable cloud ledger.
- **Fort's useful synthesis** is the agent-as-contact model from both, the exact
  `(connection, profile)` identity discipline from Hermes, and the durable
  cross-client control experience from Grok. Fort should keep its own provider-
  neutral ledger, stable Agent identity, immutable execution-binding revisions,
  and worker lease protocol.

Toby clarified on 2026-08-21 that “here” did not identify a missing URL: Grok
Bot and Hermes Bot Mode themselves are the references. He also made multi-Agent
group chats and Agent-to-Agent Handoffs requirements for the first cloud
release. Spec 048 records that product contract.

## Identity resolution and ambiguity

### Grok

The high-confidence match is **Grok Bot**, launched by SpaceXAI/xAI on
2026-08-11. Its official name is “Grok Bot,” not “Grok Bot mode.” The launch
describes always-on, named AI teammates that work on a cloud computer, message
one another, and continue when the client is closed.
([announcement](https://x.ai/news/introducing-grok-bot),
[official overview](https://docs.x.ai/grok-bot/overview))

The following similarly named first-party surfaces are different products:

- Grok Build's `grok agent` modes expose the coding agent over stdio,
  WebSocket server, relay, and ACP transports; they are runtime integration
  modes, not the persistent teammate roster.
  ([Grok Build agent mode](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md))
- Grok Build Mode is an in-chat app/site/game creation experience, not a
  standing team of Bots.
  ([Grok Build Mode announcement](https://x.ai/news/grok-build-mode))
- The Grok Build Agent Dashboard manages parallel coding sessions, not named
  cross-tool teammates.
  ([Agent Dashboard announcement](https://x.ai/news/agent-dashboard))
- `@grok` on X is a separate bot surface. Its published prompt does not expose
  the Grok Bot control architecture.
  ([official prompt repository](https://github.com/xai-org/grok-prompts))

No Grok Bot implementation repository is linked by the first-party product
materials reviewed. The public
[`xai-org/grok-build`](https://github.com/xai-org/grok-build) repository is the
Grok Build coding-agent runtime and must not be treated as Grok Bot source.

### Hermes

The high-confidence match is **Hermes Bot Mode**, now bundled and default-on in
Hermes Desktop. The earlier
[`NousResearch/Hermes-Bot-Mode`](https://github.com/NousResearch/Hermes-Bot-Mode)
plugin is archived; current development is in
[`NousResearch/hermes-agent/apps/desktop/src/plugins/hermes-bots`](https://github.com/NousResearch/hermes-agent/tree/main/apps/desktop/src/plugins/hermes-bots).
The bundle and core teammate protocol merged on 2026-08-17 in
[`NousResearch/hermes-agent#87886`](https://github.com/NousResearch/hermes-agent/pull/87886).

This is distinct from Hermes' WhatsApp setting named `WHATSAPP_MODE=bot`, which
only controls whether WhatsApp uses a dedicated bot number or a self-chat.
([Hermes Bot Mode guide](https://hermes-agent.nousresearch.com/docs/user-guide/bot-mode),
[Hermes messaging guide](https://hermes-agent.nousresearch.com/docs/user-guide/messaging))

## Architecture observed from first-party sources

### Grok Bot

#### Control plane and clients

- The managed service is account-scoped. The macOS, Windows, and iPhone clients
  use the same Bots, conversations, routines, connectors, and cloud computer;
  work continues when a client or laptop is closed.
  ([iOS guide](https://docs.x.ai/grok-bot/mobile),
  [FAQ](https://docs.x.ai/grok-bot/faq))
- User authentication is Cursor-account browser sign-in. Existing organization
  SSO and team membership apply.
  ([getting started](https://docs.x.ai/grok-bot/get-started),
  [teams and enterprises](https://docs.x.ai/grok-bot/teams-and-enterprises))
- The reviewed public docs describe product behavior, not the internal control-
  plane implementation. They do **not** specify a database engine, event-log
  schema, task-claim protocol, SSE/WebSocket choice, or public Bot dispatch API.
  The separate xAI Responses and Grok Build transports are not evidence of
  Grok Bot's internal transport.

#### Bot and conversation identity

- A Bot is explicitly a single persistent, named agent or AI teammate. It has a
  name, job, profile, its own conversation, and working context that develops
  over time.
  ([overview](https://docs.x.ai/grok-bot/overview),
  [Bot management](https://docs.x.ai/grok-bot/bots))
- Pinning and hiding are presentation operations. Hiding preserves the Bot and
  does not pause its routines. Deleting removes its active profile,
  conversation, and routines, but may leave shared computer files and sign-ins.
- A duplicate carries profile, settings, enabled skills, routines, and avatar,
  but not conversation history, learned memory, or attachments. This is useful
  evidence that Bot identity and conversation identity are separate.
- Group chats seat two to six Bots. Mentions direct ownership, group handoffs
  remain visible in the transcript, and Bot-to-Bot messages are asynchronous:
  the recipient wakes, works, and can reply later.
  ([chat and collaboration](https://docs.x.ai/grok-bot/chat-and-collaboration))

#### Persistent state

- Saved Bot role/context, conversations, routines, and client-visible state are
  durable across signed-in devices.
- `/workspace` files, browser sessions, and supported sign-ins live on the
  shared computer. Recovery and computer updates preserve durable state; reset
  can lose recent unsynchronized work.
  ([computer and apps](https://docs.x.ai/grok-bot/computer-and-apps),
  [troubleshooting](https://docs.x.ai/grok-bot/troubleshooting))
- Conversation/memory isolation and computer isolation are different. Bots
  have separate roles and conversations, while shared files, browser sessions,
  and logins can move context between them.

#### Worker and execution model

The overview initially says that each Bot runs on a persistent cloud VM, but
the same overview and the enterprise guide clarify the actual deployment and
security unit: **one managed Linux VM per user/member, shared by all of that
user's Bots**. Each Bot has its own screen and several Bots can work in
parallel, but those screens are not security boundaries. The Bot runs as a
non-root user.
([overview](https://docs.x.ai/grok-bot/overview),
[teams and enterprises](https://docs.x.ai/grok-bot/teams-and-enterprises))

Bots can use the VM's browser, filesystem, terminal, plugins, and MCP servers.
Local-computer execution is a separate capability and defaults to requiring
approval. Cloud-computer access remains available when local access is denied.
Hosted-MCP tokens stay in Cursor's backend, which performs those tool calls on
the computer's behalf.
([approvals and security](https://docs.x.ai/grok-bot/approvals-security-and-privacy),
[teams and enterprises](https://docs.x.ai/grok-bot/teams-and-enterprises))

#### Authentication and secrets

- Owner access uses Cursor authentication and organization SSO.
- Passwords, passkeys, two-factor codes, and CAPTCHAs use human takeover of the
  computer instead of ordinary chat.
- A supported secure-secret request is masked, excluded from the transcript,
  and not shown to the model.
- Browser sign-ins and command-line credentials on the user-scoped computer are
  available across that user's Bot roster. Separate Bots must not be treated as
  a credential boundary.
  ([getting started](https://docs.x.ai/grok-bot/get-started),
  [approvals and security](https://docs.x.ai/grok-bot/approvals-security-and-privacy))

#### Streaming and activity

The product transcript shows tool activity, computer use, created files,
questions, and approval requests alongside messages. A user can redirect or
stop work while a turn is running, inspect the live computer, and receive
working/attention states and notifications. The first-party docs do not state
the network transport or durability semantics behind those updates.
([chat and collaboration](https://docs.x.ai/grok-bot/chat-and-collaboration),
[settings and notifications](https://docs.x.ai/grok-bot/settings-and-notifications))

#### Scheduling

A routine belongs to one Bot and runs a workflow on a schedule or, where
supported, an external event. The UI exposes active/paused state, schedule,
next run, test execution, recent success/failure history, and deletion. Work
continues with clients closed. Current documented limits are 50 routines per
Bot and 20 retained recent run records per routine.
([skills and routines](https://docs.x.ai/grok-bot/skills-routines-and-automations))

#### Deployment

Grok Bot is a managed beta, not a self-hosting blueprint. Client application
and Agent Computer update separately. Administrators can kill a member VM
while retaining durable storage; a later session provisions a fresh VM.
Managed computers have static egress IPs available for enterprise allowlisting.
These are observable operating contracts, not enough information to reproduce
the backend on Vercel and Supabase.
([launch](https://x.ai/news/introducing-grok-bot),
[teams and enterprises](https://docs.x.ai/grok-bot/teams-and-enterprises))

### Hermes Bot Mode

#### Control plane and clients

Bot Mode is deliberately a UI over existing Hermes primitives, with no new
daemon or storage layer. A Desktop connection registry can include local,
remote HTTP(S), SSH, and Hermes Cloud gateways. It eagerly enumerates agents
over REST but opens sockets lazily. An unreachable gateway is represented as an
error on its rows rather than collapsing the roster.
([Bot Mode guide](https://hermes-agent.nousresearch.com/docs/user-guide/bot-mode),
[multi-connection Desktop](https://hermes-agent.nousresearch.com/docs/user-guide/multi-connection-desktop))

The Desktop union roster is therefore federated, not a single authoritative
cloud roster. Profiles, chats, memory, files, and routines remain scoped to the
gateway that owns them.

#### Bot and conversation identity

- A Bot is a Hermes profile. Each profile has isolated Hermes configuration,
  memory, skills, credentials, chat history, and cron state under its own
  `HERMES_HOME`.
- Each Bot has a canonical, persistent **Bot Chat** created and pinned when the
  Bot is created. `/new` inside that canonical chat is redirected to compacting
  the same relationship rather than forking it.
- The exact identity is `(connection/gateway, profile)`. Duplicate profile
  names across machines are disambiguated as `@name-device`; opening an agent
  always activates its exact route.
- Group rooms have durable internal IDs independent of display names. Members
  can span gateways, and same-named agents remain source-qualified.
  ([Bot Mode guide](https://hermes-agent.nousresearch.com/docs/user-guide/bot-mode),
  [multi-connection Desktop](https://hermes-agent.nousresearch.com/docs/user-guide/multi-connection-desktop))

This is the strongest direct precedent for Fort's immutable agent seat plus
mutable presentation identity.

#### Persistent state

Each profile directory contains its own `config.yaml`, `.env`, `SOUL.md`,
memories, sessions, skills, cron jobs, and state database. Hermes session
history is stored in a profile-local SQLite database in WAL mode, with sessions,
messages, routing metadata, full-text indexes, lineage, and schema versioning.
([profiles](https://hermes-agent.nousresearch.com/docs/user-guide/profiles),
[session storage](https://hermes-agent.nousresearch.com/docs/developer-guide/session-storage))

Group-room recent history and membership are mirrored through profile metadata
across connected gateways; the full orchestration log remains in Desktop local
storage. That projection is a Bot Mode-specific replication design, not a
global event ledger.

Profile isolation is also not OS isolation. On host installs, subprocesses use
the real user's `HOME` by default, so host CLI credentials can be shared across
profiles. `HERMES_HOME` separates Hermes data; a different OS user, VM,
container, or explicit profile home mode is needed for a harder boundary.
([profiles](https://hermes-agent.nousresearch.com/docs/user-guide/profiles))

#### Worker and execution model

Every agent runs on the gateway that owns its profile. Each `(connection,
profile)` gets its own backend and WebSocket in the Desktop pool, and background
agents keep streaming when the user looks at another gateway. Remote gateways
must already be running, except that SSH connections can start the dashboard
through a tunnel on demand.
([multi-connection Desktop](https://hermes-agent.nousresearch.com/docs/user-guide/multi-connection-desktop))

Local direct handoff invokes the recipient through `hermes -p <bot> chat`.
Cross-machine `hermes peer dm` invokes one turn on a remote profile through its
API server. Messages carry sender attribution. Live interruption of a Bot that
is already mid-conversation remains future work.
([Bot Mode guide](https://hermes-agent.nousresearch.com/docs/user-guide/bot-mode))

#### Authentication

- A local Desktop connection is app-managed.
- A remote gateway uses a saved session token or OAuth.
- SSH uses an SSH key plus an adopted dashboard token.
- Hermes Cloud uses portal sign-in.
- Direct gateway peers use a strong `API_SERVER_KEY`; names/URLs and keys are
  stored separately.
- Messaging-platform users are authorized by platform allowlists or persistent
  DM pairing, defaulting to deny.
  ([multi-connection Desktop](https://hermes-agent.nousresearch.com/docs/user-guide/multi-connection-desktop),
  [gateway internals](https://hermes-agent.nousresearch.com/docs/developer-guide/gateway-internals))

These are separate trust relationships. Sharing one roster does not make the
credentials interchangeable.

#### Streaming and activity

Roster discovery and turn streaming use separate paths: REST enumeration is
eager, while each exact agent's WebSocket is activated lazily and pooled. Tests
probe both HTTP and WebSocket legs before calling a connection reachable. The
gateway normalizes incoming platform events, authorizes the user, resolves a
fully scoped session key, runs the agent, and sends the response through the
origin adapter.
([multi-connection Desktop](https://hermes-agent.nousresearch.com/docs/user-guide/multi-connection-desktop),
[gateway internals](https://hermes-agent.nousresearch.com/docs/developer-guide/gateway-internals))

#### Scheduling

Bot routines are ordinary Hermes cron jobs with namespaced names. They remain
visible in the core Cron page and their runs land in the owning Bot's chat
history. The long-running gateway performs cron ticks and delivery; delivery
can target the origin conversation, local output, or a configured messaging
platform.
([Bot Mode guide](https://hermes-agent.nousresearch.com/docs/user-guide/bot-mode),
[cron guide](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron),
[architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture))

#### Deployment

Hermes can run gateways locally, as persistent system services, in Docker, on a
remote machine/VPS, or as a managed Hermes Cloud connection. Its architecture
is a long-running gateway with local profile state and execution. It is useful
evidence for Fort's enrolled worker boundary, but it does not solve Spec 047's
serverless scheduling, Postgres authority, leases, or data migration.

## Mapping to Spec 047

| Concern | Grok Bot pattern | Hermes Bot Mode pattern | Fort/Spec 047 implication |
| --- | --- | --- | --- |
| Primary navigation | Bot is a durable teammate/contact. | Bot is a profile row. | `agent_channel` is the primary contact; provider/runtime details are inspected, not used as the display hierarchy. |
| Conversation topology | One Bot conversation plus separate groups and threads. | One canonical Bot Chat plus profile sessions and group rooms. | Give each Agent Channel a canonical home conversation while retaining separately identifiable pinned/recent/archived conversations. A group is a conversation, not another agent identity. |
| Stable identity | Managed service exposes Bot identity but not its storage key. | Exact `(connection, profile)`; `@name-device` resolves collisions. | Keep immutable `agent_id` and seat snapshot separate from mutable name/avatar. Always display machine/runtime when needed to disambiguate. |
| State authority | Managed cloud state syncs across clients; filesystem state is a separate VM layer. | Profile and session state remain on the owning gateway. | Supabase remains the authoritative Fort ledger after cutover; worker filesystem/provider state remains explicit machine-local evidence. Do not copy Hermes' worker-owned conversation store. |
| Execution | One user-scoped cloud VM shared across Bots; optional approved local execution. | Exact local/remote gateway owns execution. | Model both as explicit runtime seats. Spec 047's initial seats remain enrolled local workers; a future cloud-computer seat must have a new, disclosed identity and cannot be a fallback. |
| Auth | Cursor owner/SSO, tool auth, computer takeover, and local-exec approval are distinct. | Session/OAuth/SSH/Cloud owner connections, peer keys, and platform pairing are distinct. | Preserve separate owner, service, machine, cron, and provider credentials. A roster is not an authorization domain. |
| Activity | Transcript carries work, files, questions, and approvals; wire protocol undisclosed. | Per-agent sockets stream while roster discovery remains REST. | Keep durable ordered events as truth and SSE as a replaceable delivery projection. Presence/working indicators must derive from leases/events, not socket reachability alone. |
| Scheduling | Routine belongs to one Bot and runs while clients are offline. | Namespaced cron belongs to one profile and posts into Bot Chat. | Persist schedule ownership by exact agent/seat and post every occurrence/result into an explicit owning conversation with run evidence. |
| Handoffs | Asynchronous Bot messages and visible group ownership. | Attributed CLI/API invocation of exact recipient. | Persist source agent, target agent/seat, request, attempt, reply, and terminal state as visible events. Never reduce a handoff to untracked prompt text. |
| Offline behavior | Cloud work continues; clients resynchronize. | Unreachable gateway rows remain visible with last-known identity. | Keep Agent Channels visible while workers are offline and show exact readiness. Reconnect from durable cursors without gaps or duplicates. |
| Deployment | Proprietary managed cloud service and user VM. | Self-hosted/federated long-running gateways. | Neither selects Vercel or Supabase. Spec 047's Vercel control + Supabase ledger + outbound workers remains an independent design requiring its own tests. |

## Reusable patterns for Fort

1. **An agent is a contact, not a provider selector.** Provider, model, machine,
   tools, and authority are capabilities of a stable agent seat.
2. **Create a canonical relationship, then allow more conversations.** The
   first click should always open the agent's durable home conversation. Pinned
   conversations are additional named work contexts, not replacement agent
   identities.
3. **Separate stable IDs from presentation.** Rename, avatar, pin, hide, and
   ordering must not change routing identity or conversation ownership.
4. **Source-qualify duplicate agents.** Hermes' `@name-device` is the right
   failure-safe model. Fort should never resolve a duplicate display name by a
   hidden default machine.
5. **Keep unavailable agents in the roster.** Offline is an explicit readiness
   state, not deletion. A connectivity probe is insufficient; the exact runtime
   and credentials must be ready for a real turn.
6. **Make handoffs first-class and visible.** Cross-agent work needs durable
   attribution, exact target identity, lifecycle, and reply linkage.
7. **Make routines agent-owned and conversation-visible.** The schedule,
   occurrence, output, failure, and next action should be inspectable from the
   agent's channel.
8. **Separate conversation durability from execution durability.** A VM,
   workspace, browser session, or CLI process can fail without erasing the Bot
   relationship or ledger.
9. **Separate authentication planes.** Owner login, machine enrollment,
   service-to-service assertion, scheduler authority, provider credentials, and
   human takeover are different credentials with different scopes.
10. **Treat streaming as projection.** Working animation, presence, and live
    output are helpful, but durable events and cursors must recover the same
    truth after every disconnect.

## Framework-specific pieces not to transfer

### Do not copy from Grok Bot

- One user-wide shared VM, browser session, filesystem, and credential pool as
  Fort's default security boundary.
- The implication that a Bot name alone proves its machine or runtime.
- Product limits such as 50 Bots/groups or 50 routines unless Fort derives its
  own capacity constraints.
- Cursor-only account, provider, plugin, or billing assumptions.
- Any internal transport or database claim: none is published in the reviewed
  first-party materials.

### Do not copy from Hermes Bot Mode

- `HERMES_HOME`, profile-directory layout, `SOUL.md`, or Hermes' SQLite schema
  as Fort domain primitives.
- A direct `hermes -p <bot> chat` subprocess as the general cross-provider
  handoff contract.
- Per-profile gateways, plugin storage, mirrored room metadata, or WebSockets as
  Fort's source of truth.
- Profile isolation as if it were OS or credential isolation.
- Hermes cron timing/delivery behavior in place of Spec 047's exact occurrence,
  idempotency, lateness, and single-authority requirements.

### Product ideas that require a separate approved Fort capability

The references also contain potentially useful features not authorized merely
by citing them: agent-created agents, multi-agent group rooms, direct bot-to-bot
delegation, workflow learning from demonstrations, shared cloud computers, and
remote browser takeover. They need their own Fort contracts and approval before
implementation.

## Acceptance criteria worth adding when Spec 047 is revised

These are reference-driven refinements, not an implementation authorization:

1. Opening an Agent Channel after relaunch restores the same canonical agent
   conversation by stable ID.
2. Pinning, hiding, renaming, or reordering never changes the exact agent seat
   or conversation target.
3. Two same-named agents on different workers remain independently addressable
   and visibly source-qualified.
4. An offline worker leaves its agents and history visible with an actionable,
   exact readiness state.
5. A routine names one immutable agent/seat owner and one result conversation;
   every run records schedule, occurrence, attempt, result, and failure state.
6. Every cross-agent handoff records source, exact target, conversation,
   idempotency key, attempt, streamed activity, reply, and terminal state.
7. Desktop, web, and iPhone reconstruct identical ordered history after a
   forced stream cutoff using durable cursors.
8. Loss or rebuild of an execution machine cannot delete cloud-authoritative
   agent identity, conversations, schedules, approvals, or events.
9. A cloud-executed and a local-executed version of the same framework are
   separate seats with separate authority and never substitute silently.
10. Credentials shared by a worker or cloud computer are disclosed as such;
    UI-level Bot separation is never presented as a security boundary.

## Clarified requirements and remaining decisions

Confirmed by Toby on 2026-08-21:

- Grok Bot and Hermes Bot Mode are the references; no third link is missing.
- Multi-Agent group chats are required in the first cloud release.
- Agent-to-Agent Handoffs are required in the first cloud release.

Spec 048 proposes the remaining exact choices for approval: stable Agent
identity with binding revisions, one canonical home Conversation plus secondary
Conversations, bounded two-to-six-member Groups, three rounds/ten Agent messages
and Handoff depth three per human Group Turn, Agent-owned Routines, and deferred
learned Fort memory.

## Primary sources

### Grok Bot

- [Introducing Grok Bot](https://x.ai/news/introducing-grok-bot)
- [Grok Bot overview](https://docs.x.ai/grok-bot/overview)
- [Create and manage Bots](https://docs.x.ai/grok-bot/bots)
- [Message and collaborate](https://docs.x.ai/grok-bot/chat-and-collaboration)
- [Get started](https://docs.x.ai/grok-bot/get-started)
- [Grok Bot for iOS](https://docs.x.ai/grok-bot/mobile)
- [Use the computer and apps](https://docs.x.ai/grok-bot/computer-and-apps)
- [Skills and routines](https://docs.x.ai/grok-bot/skills-routines-and-automations)
- [Approvals, security, and privacy](https://docs.x.ai/grok-bot/approvals-security-and-privacy)
- [Grok Bot for teams and enterprises](https://docs.x.ai/grok-bot/teams-and-enterprises)
- [Settings and notifications](https://docs.x.ai/grok-bot/settings-and-notifications)

### Hermes Bot Mode

- [Bot Mode user guide](https://hermes-agent.nousresearch.com/docs/user-guide/bot-mode)
- [Bundled Bot Mode merge, PR #87886](https://github.com/NousResearch/hermes-agent/pull/87886)
- [Current in-tree Bot Mode source](https://github.com/NousResearch/hermes-agent/tree/main/apps/desktop/src/plugins/hermes-bots)
- [Archived standalone Bot Mode repository](https://github.com/NousResearch/Hermes-Bot-Mode)
- [Profiles: Running Multiple Agents](https://hermes-agent.nousresearch.com/docs/user-guide/profiles)
- [Connecting Desktop to Many Hermes Instances](https://hermes-agent.nousresearch.com/docs/user-guide/multi-connection-desktop)
- [Architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture)
- [Gateway internals](https://hermes-agent.nousresearch.com/docs/developer-guide/gateway-internals)
- [Session storage](https://hermes-agent.nousresearch.com/docs/developer-guide/session-storage)
- [Scheduled tasks](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron)
