# 047 — Vercel + Supabase cloud control plane

**Status:** approved for implementation with Spec 048 by Toby on 2026-08-21;
paid provisioning, production migration, cutover, and retirement retain the
explicit gates below
**Decision owner:** Toby
**Depends on:** Specs 021, 028, 041, 042, 045, and 046; Spec 048 supplies the
stable-Agent, group-chat, Handoff, and Routine domain migrated by this spec.

## Audit finding

The 2026-08-21 release did not fail to compile and the Vercel deployment did
not fail.

- GitHub main contains release record
  **712ae4694bbf9eb4a9cac8aeccfb1902e4994d30**.
- Railway built **./cmd/fort** successfully, then started the binary without
  the required **serve** argument. The process printed usage and exited.
  Railway retried **/health** eleven times over five minutes and marked
  deployments **6024331747** and **6024656397** failed.
- The Railway project has one service, **fort-portal**, and no database
  service. A missing start command, not a code build failure, caused the
  visible GitHub deployment failure.
- Vercel project **fort-gateway**
  (**prj_KWhip1la6Ckyxj5N2TuUgNkPIFQC**) is **READY** at deployment
  **dpl_EpDbB8HTdmbJovd5VbFHvaiVFExw**, with no recent runtime errors.
  It deploys only **gateway/web**; it does not deploy the Go control plane,
  scheduler, SQLite store, native runtimes, or Cloudflare relay.
- The current Vercel deployment is a manual prebuilt CLI upload. It is not
  linked to GitHub for automatic main and pull-request deployments.
- No Supabase project exists for Fort. The repository contains no Supabase
  configuration or Postgres adapter.
- The live SQLite database is healthy. At audit time it was 1,183,744 bytes,
  passed **PRAGMA integrity_check**, and contained 23 application tables,
  42 indexes, 26 triggers, and 1,197 append-only events.

A minimal Railway repair would set the start command to **./out serve**, add a
persistent volume, and configure **FORT_DB**. That is not the selected direction
in this proposal because it would retain the local-SQLite deployment model.

## Goal

Make Vercel and Supabase the authoritative public and durable control plane for
Fort:

- GitHub remains the source repository.
- Vercel builds and serves the authenticated web app and stateless Go control
  interface from exact Git commits.
- Supabase Postgres owns durable Fort Agents and binding revisions, canonical,
  secondary, and Group Conversations, Routines, Handoffs, machine enrollment,
  queued commands, leases, messages, targets, runs, and events.
- Enrolled computers become thin outbound execution workers. They retain only
  machine identity, provider credentials, workspaces, capability evidence, and
  the native processes that cannot run truthfully in a cloud function.
- Railway and the Cloudflare Durable Object relay are removed only after the
  replacement passes data, reconnect, execution, and rollback acceptance.

This is the practical meaning of “move everything to Vercel and Supabase.”
Vercel is not a Git host, and current Fort agents are machine-local CLIs.
Moving Git history away from GitHub or silently replacing enrolled native agents
with remote APIs is not part of this spec.

## Stable-Agent product prerequisite

The reference architecture decision in Spec 048 is part of this migration, not
a later UI rename. A Fort Agent is the durable account-scoped relationship. Its
canonical Conversation, secondary Conversations, Group memberships, Routines,
Handoffs, and profile history outlive explicitly approved changes in framework,
profile, model, machine, adapter, authority, or policy.

Fort-owned role, standing instructions, skills/tools, and other prompt material
form an immutable Agent Behavior Revision. Execution configuration is an
immutable Agent Binding Revision that references one exact Behavior Revision.
Every target, attempt, lease, retry, Routine run, and Handoff pins both exact
revisions. The current lifetime-bound `agent_channel` rows migrate into stable
Agent identities with initial Behavior and Binding Revisions before Postgres
becomes authoritative.

Every Agent has one canonical home Conversation. Multi-Agent Group
Conversations and durable bounded Handoffs are required in the first cloud
release. The exact behavior, limits, migration, API, and acceptance contract is
defined in Spec 048. Spec 047 must not port the current lifetime-binding schema
to production first and defer this correction.

## Contracts retained

This spec changes deployment and persistence topology, not Fort's product truth.
It retains:

- deterministic routing with zero model calls in the routing path;
- exact immutable agent seats, models, machines, adapters, and authority
  snapshots;
- atomic, idempotent turns and durable target lifecycle;
- no silent provider, model, computer, or runtime substitution;
- append-only evidence and exact error attribution;
- actionable retry/cancel/recheck behavior;
- stable Agents, canonical and secondary Conversations, pins, and the
  living-mark contract;
- human gates, schedules, and cross-machine execution semantics.

Upon approval and completed cutover, this spec supersedes Spec 021 only where it
requires SQLite to be the authoritative store or requires one permanent process
to own scheduling. It supersedes Spec 028 only for the Vercel + Cloudflare
transport topology and machine registry. Spec 028's enrollment, revocation,
identity-pinning, fail-closed behavior, and disclosure of the cloud trust model
remain governing until explicitly replaced here.

## Hard execution boundary

**fort serve** cannot be copied unchanged into Vercel:

- it runs permanent scheduler, probe, relay, HTTP, and inbox loops;
- it creates local work directories;
- it spawns and controls installed Codex, Claude, OpenClaw, and Hermes
  processes;
- it depends on machine-local OAuth state, executables, files, and process
  groups;
- Vercel functions and Services have bounded lifetimes and may be recycled or
  scaled to different instances.

The target therefore separates the planes:

~~~text
Web / iPhone / Mac
        |
        v
Vercel web + Auth.js
        |
        v
Vercel stateless Go control interface
        |
        v
Supabase Postgres  <----> durable event cursor / schedule tick
        ^
        |
outbound HTTPS claim + lease + heartbeat + append
        |
Fort execution workers on enrolled computers
        |
        v
exact local provider CLI + exact local workspace
~~~

A future Vercel Sandbox or remote-provider seat is a new agent/runtime with its
own exact identity. It is never a transparent fallback for a Mac seat.

## Vercel deployment

### Source and environments

- Keep **tobsai/fort** on GitHub as the source of truth.
- Connect production deployments to main.
- Create pull-request previews from exact branch commits.
- Do not publish a dirty prebuilt worktree.
- Every deployment records the Git commit and migration version it serves.
- Remove the Railway GitHub integration only after Vercel production acceptance
  so a failed migration does not eliminate the existing fallback prematurely.

### Deployables

Use two Vercel projects initially:

1. Reuse the existing **fort-gateway** project for the Next.js/Auth.js web
   tier. Git-link it to **tobsai/fort** with Root Directory
   **gateway**—not **gateway/web**—while preserving the
   **fort-gateway.vercel.app** origin, Google/Auth.js callbacks, and production
   environment. Build the **web** workspace from that root because Vercel does
   not allow a project Root Directory to import its sibling/parent files and
   **gateway/web** depends on **gateway/shared**. A clean Git-triggered preview
   build must prove the workspace boundary; the existing manual prebuilt
   deployment is not evidence for it.
2. **fort-control** uses Vercel's Beta Go Functions runtime only for bounded,
   non-streaming handlers under **/api**. It never starts a native runtime,
   scheduler loop, relay loop, filesystem watcher, or permanent listener.

This proposal explicitly accepts the documented Beta status of the Go Functions
runtime. Two projects remain a deliberate authentication, compatibility, and
promotion boundary; the choice is not based on an obsolete Vercel Services beta
limitation.
**fort-gateway** proxies authenticated commands to **fort-control** through a
server-only origin and scoped service assertion. The assertion binds a
server-derived account UUID, route class, audience, request digest, issued and
expiry times, nonce, and key ID. **fort-control** never trusts account identity
from a client header or body. Missing, forged, stale, replayed, wrong-audience,
or wrong-route assertions fail closed. Machine and cron credentials are
separate and cannot invoke owner routes; owner and machine credentials cannot
invoke cron routes. Key rotation overlaps only authenticated keys and never
opens an unsigned window.

Native clients continue to use the stable gateway origin. A later reviewed
release may consolidate both projects with Vercel Services after preview
evidence proves its routing and lifecycle.

The two projects cannot be promoted atomically. Database changes use
expand/contract migrations, and every deployment publishes explicit compatible
API and schema version ranges. Deployment Checks block an incompatible alias.
Promotion order is schema expansion, **fort-control**, then **fort-gateway**;
contract cleanup occurs only after the compatibility window. Tests prove that
a failed second deployment leaves the previously aliased compatible pair live.

Preview deployment is control-first, not two uncoordinated automatic aliases.
An authenticated orchestration job creates or selects the exact preview data
environment, generates preview-only assertion and AEAD keys, deploys
**fort-control**, waits for its immutable URL to report the expected commit and
schema range, then deploys **fort-gateway** with that immutable control URL and
a server-only Deployment Protection bypass credential. The gateway rejects a
control response whose commit does not match its own. Preview credentials,
database roles, keys, and bypass tokens cannot authenticate to production, and
are revoked when the preview closes.

### Bounded background work

- Production scheduling requires a Vercel plan with per-minute Cron precision;
  Hobby's once-daily/hourly behavior is not acceptable. A fresh quote and plan
  confirmation are required before production.
- A Vercel Cron request bearing the exact **CRON_SECRET** invokes one bounded,
  authenticated schedule tick. Missing or incorrect secrets fail before any
  database access.
- The tick uses a durable last-success watermark, a bounded catch-up/look-ahead
  window, database locking, and occurrence uniqueness to materialize exact
  six-field schedule timestamps idempotently, then returns. Duplicate,
  overlapping, missed, and late invocations cannot duplicate an occurrence.
- Workers never begin before the persisted exact due time. Initial production
  accepts at most 90 seconds of lateness; an occurrence beyond that bound enters
  an explicit **missed_needs_attention** state instead of running silently.
- It does not run providers.
- Execution workers claim queued work independently.
- Go Function streaming is not an accepted Fort dependency because Vercel's
  supported streaming contract is documented for Node.js and Python, not Go. A
  **fort-gateway** Node.js route authenticates the owner, repeatedly calls a
  bounded cursor/long-poll JSON handler on **fort-control**, emits ordered SSE
  events, and closes with a reconnect cursor before its configured
  **maxDuration**.
- Preview acceptance proves time to first byte, progressive chunks, forced
  cutoff, reconnect, no gaps or duplicates, and the exact deployed runtime and
  duration.
- Supabase Realtime may later be used only as a wake-up signal; persisted rows
  remain truth.
- Rollback disables the cloud Cron before re-enabling any local scheduler, so
  two schedule owners are never active together.

### Native and web protocol transition

Do not reinterpret the existing **/api/req** and **/api/sse** payloads as cloud
control traffic. They are legacy v1 Noise frames encrypted to the selected
Mac's pinned public key; Vercel cannot decrypt them, and replacing the responder
would violate the stored pin.

- Keep the v1 routes, Cloudflare relay, and Mac Noise responder active during a
  bounded compatibility window.
- Add owner-facing cloud routes under a versioned namespace such as
  **/api/v2/** at the same **fort-gateway.vercel.app** origin.
- Web and Apple clients use v2 Auth.js or renewable native-bearer sessions and
  durable cloud cursors. Machine enrollment and worker authentication remain a
  separate pinned machine-identity contract.
- Ship the v2-capable native release before authoritative cloud cutover.
- Prove old-client/new-client coexistence, token renewal, minimum-version
  behavior, and rollback. Never silently fall back from v2 to a different
  machine or protocol.
- Retire the v1 relay only through a separate evidence-backed retirement gate.

There is always one write authority, identified by a signed, monotonically
increasing authority epoch:

- Before cutover, **legacy_v1_write** is authoritative. A v2-capable client may
  be distributed, but it uses v1 only after explicitly reading that signed
  mode; cloud preview data is not production authority.
- At the frozen cutover, the epoch changes to **cloud_v2_write**. Old v1 clients
  may read the labelled frozen SQLite snapshot, but every v1 mutation receives
  an encrypted, actionable **upgrade_required** response. There is no dual
  write and no Mac v1-to-cloud translation proxy.
- Rollback first freezes cloud writes, imports and verifies the cloud ledger in
  SQLite, and only then advances the epoch to a new **legacy_v1_write** value.
  v2-capable clients support this explicit signed rollback mode through their
  bundled v1 transport. No client infers authority from an unreachable route.

### Function payload boundary

Vercel Function request and response bodies are limited to 4.5 MB. Fort uses
smaller normative limits after JSON/base64 encoding:

- any complete Function request or response body: at most **4 MiB**;
- plaintext content per encrypted chunk: at most **2 MiB**;
- one cursor page: at most **1 MiB** encoded, with an explicit next cursor;
- one logical context/output artifact: at most **128 MiB** unless a later spec
  deliberately raises the bound.

Larger context and terminal output use idempotent, resumable encrypted chunks.
The durable manifest records attempt/artifact identity, ordered chunk indexes,
encoded and plaintext lengths, key IDs, per-chunk authenticated digests, final
logical digest, and completion state. Initial production stores both manifests
and immutable encrypted chunk rows in **fort_private** Postgres tables so the
same verified database backup contains all authoritative content. Neither
clients nor workers receive a general Supabase credential. Missing, repeated,
reordered, or conflicting chunks cannot finalize an artifact. Cursor/event
pages carry references rather than inlining a large body. Crossing an aggregate
limit produces a typed **payload_limit** failure; Fort never silently truncates
an Answered result.

Moving chunks to Supabase Storage or another object service is a later storage
spec. It must first define independent encrypted object backup/replication,
immutable object identity and retention, bucket-policy acceptance, and a restore
drill that verifies every referenced object's length, digest, and key ID before
traffic reopens. Postgres metadata alone is never accepted as proof that an
external object still exists.

## Supabase deployment

### Project

Production recommendation:

- dedicated Supabase organization: **Fort**;
- Pro plan;
- project: **Fort Production**;
- region: **us-east-1**, paired with Vercel **iad1**.

The zero-cost pilot alternative is **Fort Preview** in the existing
**MapleTreeEnterprises** Free organization. The current quote is $0/month for
that standalone project, but Free does not provide scheduled automatic backups
and does not support Supabase preview branches. On Pro, a preview branch is
currently quoted at $0.01344/hour before disk, egress, and other usage, receives
no compute credit, and is not protected by Spend Cap. Costs must be re-read and
confirmed immediately before every project or branch creation.

If Pro branches are enabled, they contain synthetic data only, are deleted
automatically when the pull request closes, and have a maximum 24-hour orphan
TTL. A Free pilot uses one dedicated preview database and no branch automation.

The environment matrix is explicit:

| Environment | Data and use | Pairing/concurrency |
| --- | --- | --- |
| **Fort Production** | Empty until final import; authoritative data only after cutover. | Production aliases only. |
| **Fort Migration Staging** (recommended on Pro) | Restricted, encrypted real-data rehearsal after separate project cost confirmation. | One controlled migration rehearsal; never a public PR preview. |
| Pro PR branch | Synthetic seed data only. | One branch per exact Vercel PR pair; automatic teardown. |
| Free **Fort Preview** pilot | One restricted imported rehearsal snapshot or synthetic development data, never both concurrently. | One serialized preview deployment; no concurrent schema-changing PR previews. |

Using the pre-live production project instead of **Fort Migration Staging**
requires explicit approval, a documented full reset, a second empty-state
verification, and the final frozen import. It is not the recommended path.

Do not reuse Nimbus Lighting, clockwork, or Clean Casts.

### Database access and security

- Put operational tables in an unexposed **fort_private** schema.
- Disable the project Data API because Fort uses only direct server-side
  Postgres. Also keep **fort_private** out of Exposed schemas and revoke schema,
  object, default-object, sequence, and function privileges from **PUBLIC**,
  **anon**, **authenticated**, and **service_role**. Lock down default grants in
  **public** so later migrations do not accidentally expose new objects.
- Enable and force RLS as defense in depth.
- Add an indexed **account_id** to account-owned roots and enforce both
  **USING** and **WITH CHECK** ownership policies.
- Use a least-privilege **fort_gateway** login with **NOBYPASSRLS**.
- The migration owner remains separate from the runtime role.
- Every account-scoped operation opens an explicit transaction, rejects an
  absent or malformed account UUID, sets identity with transaction-local
  **set_config(..., true)**, and runs every statement in that same transaction.
  Concurrent A-to-B-to-anonymous tests over Supavisor transaction pooling must
  prove that identity cannot leak through a reused connection.
- Every view uses **security_invoker=true** on Postgres 15+ or is inaccessible
  to runtime/API roles. Any unavoidable **SECURITY DEFINER** function lives
  outside exposed schemas, uses an empty **search_path** and fully qualified
  objects, validates identity, and has **EXECUTE** revoked from PUBLIC and API
  roles. Every UPDATE policy is tested with its corresponding SELECT policy.
- Store only the pooled runtime URL in server-only Vercel environment
  variables. Never expose a database password, Supabase secret/service key, or
  runtime role to JavaScript, Swift, a worker node, or source control.
- Vercel uses the Supavisor transaction pooler on port 6543 with SSL and
  prepared statements disabled. Schema migrations, dumps, and restores run
  from an approved IPv6-capable runner over the direct connection or from
  IPv4-only CI over Supavisor session mode on port 5432. They never use
  transaction mode. Price and approve Supabase's IPv4 add-on before relying on
  a direct connection from Vercel or GitHub Actions.
- Auth.js remains the owner authentication system for this migration. A
  normalized authenticated email maps to one durable Fort account UUID.
- Machine enrollment tokens are random, scoped, revocable, stored hashed, and
  never grant arbitrary database access.

### Cloud trust change

Today Spec 028 keeps application payloads opaque to the Vercel/Cloudflare relay
because authoritative state remains on a computer. A cloud database changes
that trust boundary.

The recommended first production contract is:

- Vercel is a trusted application tier.
- Sensitive message, context, output, receipt, and error bodies use a versioned
  application-level AEAD envelope before storage.
- The active data-encryption key is server-only, versioned by **key_id**, and
  never stored in Supabase.
- Supabase can read routing metadata but not encrypted content.
- Vercel can decrypt content to serve an authenticated owner and prepare exact
  worker commands.
- Provider credentials, CLI OAuth state, Keychain material, and workspace files
  never enter Vercel or Supabase.

The production key ring has separate production and preview keys, explicit
rotation and retirement dates, and retains old decryption keys while their
ciphertext exists. The runtime copy is injected through encrypted Vercel
environment configuration. An independently recoverable encrypted escrow is
held outside both Vercel and Supabase in Toby's approved password-manager vault.
Access is least-privilege and audited. A clean restore must prove that old
ciphertext can still be decrypted before any key is retired.

A zero-knowledge design in which Vercel also cannot decrypt history requires
client/worker key distribution and offline-read decisions. That is a separate
security design and is not silently implied by “move to Supabase.”

## Postgres persistence module

The current concrete **Store** and SQLite SQL are not portable. This is not an
environment-variable swap.

Create one durable ledger seam consumed by core/control code, with two real
adapters:

- existing SQLite adapter for rollback and local tests;
- Postgres adapter for preview and production.

The ledger interface exposes domain commands and projections, not raw SQL,
generic CRUD, transactions, dialect flags, or query strings. Conversation,
execution, enrollment, and scheduling operations keep their atomic use-case
interfaces. SQL dialect and error translation remain inside each adapter.

Postgres-native schema conversion includes:

- RFC3339 text timestamps to **timestamptz**;
- integer booleans to **boolean**;
- JSON text to **jsonb**;
- event/message sequence IDs to generated **bigint** identities;
- case-insensitive uniqueness to indexes over normalized values;
- SQLite GLOB checks to Postgres regular expressions;
- JSON extraction to native JSON operators;
- SQLite abort triggers to named constraints or PL/pgSQL triggers with stable
  Fort error-code translation;
- immediate-write serialization to row locks, partial unique indexes, scoped
  advisory transaction locks, or serializable transactions.

All existing immutability, single-flight, exact-authority, idempotency, receipt,
and lifecycle invariants remain database-enforced.

## Worker protocol

An enrolled computer runs **fort worker**, not the cloud coordinator.

1. It authenticates with its scoped machine token.
2. It posts capability/readiness evidence and a bounded heartbeat.
3. It claims one compatible queued target through an atomic cloud command.
4. Supabase assigns a lease with attempt ID, expiry, stable Agent ID, immutable
   Agent Behavior and Binding Revisions, seat/effective-authority snapshot, and
   encrypted input.
5. The worker verifies exact local readiness again before provider start.
6. It renews the lease while Working and appends bounded events.
7. It commits one terminal receipt and output using the attempt ID.
8. A lost lease prevents later writes from the stale attempt.
9. Cancellation is durable and the worker terminates the exact process group.
10. Expired leases return to an explicit recoverable state; they are not
    silently rerouted.

Worker endpoints take machine, target, attempt, and idempotency identities
explicitly. The Vercel control module owns claim/lease logic; workers never
write Supabase directly.

## Data migration

The current data set is small enough for a bounded write freeze. Do not add a
temporary dual-write system.

Add deterministic commands:

- **fort db export-sqlite**;
- **fort db import-postgres --dry-run**;
- **fort db verify-migration**;
- **fort db export-postgres** for rollback.

The archive contains schema version, source database hash, every row in
dependency order, exact IDs/timestamps, per-table counts, and stable record
digests. Because those rows include plaintext conversations, context, outputs,
and errors, the archive and manifest are encrypted before leaving the source
Mac, transferred only through an approved restricted channel, and accessible
only to the migration operator. They never enter logs, Git, pull-request seeds,
or Supabase branches. Branches use synthetic seed data only.

Digests cover canonical logical records and are either HMAC-protected or kept
inside the encrypted manifest; raw hashes of low-entropy content are not
published, and nondeterministic AEAD ciphertext is not used as the logical
digest. Intermediate archives are destroyed within 30 days after production
acceptance. The designated rollback backup follows the separately documented
retention policy.

Cutover:

1. Prove zero queued or Working targets.
2. Disable new writes and scheduler ticks.
3. Take a checksummed SQLite backup.
4. Export from the frozen backup.
5. Import in one controlled Postgres migration transaction.
6. Verify row counts, identity maxima, timestamp bounds, foreign keys,
   invariants, and record digests for every table.
7. Run Supabase security and performance advisors.
8. Deploy the exact Vercel rehearsal pair against the restricted imported
   **Fort Migration Staging** project (or the explicitly approved serialized
   Free pilot/pre-live production alternative).
9. Run authenticated web/native, reconnect, schedule, cancel, retry, and worker
   claim acceptance.
10. Import the final frozen source into production.
11. Point clients and workers to production.
12. Keep SQLite and the existing relay read-only until the acceptance window
    closes.

## Realtime and reconnect

- The append-only event table remains authoritative.
- Every list/detail response includes a durable revision or event cursor.
- The v2 protocol keeps authenticated reconnecting SSE delivery semantics but
  does not reuse the opaque v1 Noise frames. Streams end and reconnect before
  Vercel's Node function deadline.
- Reconnect sends the last durable cursor and receives every later event in
  order.
- Supabase Broadcast may notify clients that a cursor advanced, but clients
  always refetch durable state.
- Do not use Postgres Changes as the durable event bus.
- Direct Supabase Realtime clients remain out of scope until Fort deliberately
  chooses Supabase Auth or a compatible custom-token contract.
- Fort migrations do not create or alter objects in Supabase's locked
  **realtime** schema. Any future Broadcast authorization is expressed through
  policies on **realtime.messages** using the supported contract.

## Delivery sequence

1. **Prove the Vercel and client boundary:** deploy a non-production spike for
   bounded Go JSON handlers, Node SSE cutoff/reconnect, v1/v2 coexistence, and
   exact runtime/duration evidence. Failure stops the migration.
2. **Provision preview:** approved organization/project/region, committed
   migrations, private schema, runtime/migration roles, and advisor baseline.
3. **Extract the ledger seam:** move callers from concrete Store dependencies
   while adding Spec 048's stable Agent, Behavior/Binding Revision,
   canonical/secondary Conversation, Group, Routine, and Handoff contracts to
   the shared behavior suite without changing current SQLite dispatch behavior.
4. **Postgres adapter:** run the same ledger behavior suite against both
   adapters, including real concurrent Postgres tests.
5. **Cloud control interface:** stateless Go composition, Auth.js-to-account
   propagation, schedule tick, cursor reads, and worker commands.
6. **Worker mode:** claim/lease/heartbeat/cancel/result protocol around the
   existing native runtime, always pinned to exact Agent Behavior and Binding
   Revisions.
7. **Client transition:** release versioned cloud routes and v2-capable web and
   Apple clients while v1 remains operational.
8. **Vercel Git deployments:** exact main production and pull-request preview
   linkage; no dirty manual production artifacts.
9. **Migration rehearsal:** restricted staging import, exact paired Vercel
   deployment, parity verification, advisors, failure injection, and rollback
   rehearsal. PR branches remain synthetic-only.
10. **Production cutover:** frozen final import, clients/workers switched,
   evidence captured.
11. **Retirement:** remove Railway integration and Cloudflare relay only after
   the acceptance window.

Each phase is a separate reviewable commit and retains a runnable rollback
path.

## Test and acceptance criteria

- The same durable ledger contract suite passes against SQLite and Postgres.
- Concurrent duplicate Send requests create one human message, turn, target,
  and immutable context snapshot.
- Concurrent claims assign one active attempt to one exact machine.
- Active-target single-flight, pins, schedule occurrences, immutable seats,
  authority snapshots, receipts, retries, cancellation, and archive-state
  guards remain enforced in Postgres.
- Spec 048 acceptance passes for canonical home chats, source-qualified Agents,
  one-wave two-framework Groups, bounded Handoffs, `fort_cloud` Agent-owned
  Routines, and both human-directed and structured Agent-initiated Handoffs.
- Production cutover uses two independently approved real `runtime.Runtime`
  adapter families and one separately approved structured `HandoffEmitter`;
  fakes never satisfy this gate and this spec authorizes no provider.
- Cross-account reads and writes fail at both the control interface and RLS
  layers.
- Transaction-pooled A-to-B-to-anonymous operations prove no account identity
  leaks between reused database connections.
- Data API, schema, object, default-object, function, and sequence privilege
  checks prove no Fort row is reachable through Supabase API roles.
- Runtime and machine credentials never appear in client bundles, logs,
  migration archives, or Supabase tables.
- A recycled Vercel instance loses no accepted command, event, or schedule.
- Cron tests cover missing/invalid secrets, duplicate and overlapping ticks,
  missed-tick catch-up, exact occurrence uniqueness, the 90-second lateness
  boundary, and disabling the cloud owner during rollback.
- SSE disconnect/reconnect resumes from the exact durable cursor without gaps
  or duplicates.
- The deployed Node SSE route proves progressive streaming and bounded cutoff;
  no Go Function owns a streaming response.
- Legacy v1 Noise and v2 cloud transports remain available during the published
  transition while the signed authority epoch permits mutations through only
  one of them; unsupported versions fail with an actionable upgrade response.
- Authority-epoch tests prove one write owner before cutover, encrypted
  **upgrade_required** for frozen v1 mutations, no dual write, and explicit
  v2-to-v1 rollback only after cloud export/import verification.
- Cross-project tests reject missing, invalid, stale, replayed, forged-account,
  wrong-audience, and wrong-route assertions; credential rotation creates no
  unauthenticated interval.
- Paired-preview tests prove control-first orchestration, immutable same-commit
  URLs, Deployment Protection access, isolated database/keys, and that no
  preview credential or route can reach production.
- Expand/contract and version-range tests prove control-first/web-second
  promotion, compatibility during partial rollout, and survival of a failed
  second deployment.
- Payload tests cover every byte below, at, and above the 4 MiB body, 2 MiB
  chunk, 1 MiB cursor-page, and 128 MiB aggregate boundaries; interrupted,
  duplicated, reordered, and digest-conflicting uploads resume or fail without
  silent truncation or partial Answered state.
- A worker disconnect during execution yields an expired lease and an
  actionable durable recovery state.
- Every SQLite table count and stable digest matches Postgres after import.
- Identity sequences advance beyond imported maxima.
- Supabase security advisor has no unresolved error-level findings; performance
  findings are dispositioned.
- The initial production recovery objective is RPO at most 24 hours and RTO at
  most 4 hours. A tighter RPO requires a separately quoted and approved PITR
  configuration.
- A restore drill reconstructs the database and passes the same verification,
  including Pro daily-backup availability and retention, an off-site encrypted
  logical dump (mandatory on Free), external AEAD key recovery, custom runtime
  and migration-role password reset, Vercel secret refresh, and post-restore
  RLS/advisor checks. Custom-role passwords are never assumed to be restored by
  a Supabase backup.
- GitHub identifies the exact Vercel production commit; Railway no longer posts
  deployment failures after retirement.
- Existing local Mac and native clients can be returned to the SQLite backup
  through the documented rollback.
- Full Go, race, vet, gateway, FortKit, and Vercel preview verification passes.

## Rollback

Before production writes, rollback is a configuration switch to the preserved
local service and SQLite backup.

After production accepts cloud writes:

1. stop new cloud writes and schedule ticks;
2. wait for or cancel active worker leases;
3. export the full Postgres ledger;
4. import it into a new SQLite database;
5. verify every count, digest, identity, and invariant;
6. restore the local service and clients;
7. retain Supabase read-only for investigation.

Never point an older binary at a partially migrated database. Never delete the
SQLite backup, Railway service, Cloudflare worker, or cloud database as the
first rollback action.

## Explicit non-goals

- Moving Git history from GitHub to Vercel.
- Running permanent native CLI processes inside Vercel functions.
- Replacing exact local agents with provider APIs or Vercel Sandbox.
- Enabling or upgrading Codex, OpenClaw, Claude, Hermes, or another provider.
- Multi-tenant SaaS, billing, team administration, or public sign-up.
- Direct client access to the Fort database.
- Destructive deletion of local evidence during cutover.

## Approval gates

Implementation and cloud provisioning may begin only after Toby confirms:

1. **Agent relationship contract:** approve Spec 048's stable-Agent identity,
   canonical home chat, first-release multi-Agent Groups, bounded Handoffs, and
   Agent-owned Routine decisions together with this infrastructure spec.
2. **Supabase home:** dedicated **Fort** organization on Pro in **us-east-1**
   (recommended), or a zero-cost **Fort Preview** pilot in
   **MapleTreeEnterprises**. The recommended Pro path also uses a separately
   cost-confirmed **Fort Migration Staging** project for the real-data
   rehearsal; PR branches remain synthetic-only.
3. **Execution boundary:** enrolled computers remain the execution workers
   (recommended). A zero-local-node product requires a separate runtime spec.
4. **Trust boundary:** Vercel is trusted to decrypt authenticated content while
   Supabase stores application-encrypted bodies (recommended), or a separate
   zero-knowledge design is required.
5. **Vercel runtime:** accept Beta Go Functions for bounded non-streaming control
   handlers and use the supported Node.js runtime for SSE (recommended), or
   choose a different hosting boundary.
6. **Key custody and recovery:** use encrypted Vercel environment configuration
   for the runtime key ring and a separate approved password-manager escrow,
   with the initial RPO/RTO of 24 hours/4 hours (recommended), or specify a
   different key custodian and recovery objective.
7. **Scheduling contract:** use a freshly quoted Vercel production plan with
   per-minute Cron, preserve exact intended six-field timestamps, and accept a
   90-second maximum lateness before **missed_needs_attention** (recommended),
   or choose a different durable scheduler.

Provisioning is itself gated. Create or select the organization separately,
then re-list organizations, select the exact organization ID, fetch a fresh
cost quote, repeat its amount and recurrence to Toby, obtain explicit cost
confirmation, create the project, and wait for **ACTIVE_HEALTHY**. Repeat that
cost gate for every branch. The Supabase MCP cannot create an organization.

Pin the Supabase CLI version. Author committed migrations with
**supabase migration new**, reconcile local and linked history with
**supabase migration list** and **supabase db pull**, and run security and
performance advisors after every DDL batch. Do not use one-off remote migration
calls as the iterative authoring workflow.

Approval of this spec does not itself authorize provider changes, destructive
retirement, production data deletion, or an unpriced Supabase plan change.
