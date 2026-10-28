# 063: Cloud hub with Mac runners

**Decision:** Toby set the direction on 2026-10-23 ("run fort in the cloud and then
delegate down to my machines ... I don't want to have to tunnel into my machines").
On 2026-10-24 he chose Railway as the host and OK'd conversation history living
there. Fixture-only building is covered by those decisions. Creating the Railway
project, deploying, and enrolling a real Mac each need their own approval.

## What and why

Today one Mac is the whole of Fort. The core and its single SQLite ledger run inside
the Fort Host helper, and the iPhone reaches it only over Tailscale. After this
change:

- **Hub.** The same Node core and ledger run as one always-on container on Railway.
  Every client (iPhone, Mac app) talks only to the hub, over HTTPS, from anywhere.
- **Runners.** Each Mac's Fort Host helper becomes a runner. It opens one outbound
  HTTPS connection to the hub, keeps it open, and executes the turns the hub sends
  it. No Mac opens a port, and nobody tunnels in.
- **Credentials stay put.** Claude (through the `claude` binary, Spec 062) and the
  ChatGPT plan sign-in (Spec 059, October 22) stay on the Mac that signed in. The hub
  and its ledger never hold a provider credential.
- **The local path stays.** A single Mac can still run as its own hub, as today. The
  constitution requires a self-hosted path with no mandatory vendor control plane, so
  the cloud hub is a deployment choice, not a dependency.

## Risk

**Data custody.** Conversation history moves to a Railway volume. Railway terminates
TLS at its edge and operates the disks, so Railway is inside the trust boundary.
Railway staff state that customer data is encrypted at rest at the storage level, but
not per volume in software. Fort adds no encryption of its own in this spec, because
the hub needs the key to serve history, and that key would also live on Railway.
Message bodies stay out of logs. Removal means deleting the Railway service and
volume, which is a destructive action that needs its own approval.

**Internet exposure.** The hub becomes publicly reachable. The guards are:
- existing device pairing, with digest-only tokens and revocation, for clients;
- per-runner enrollment tokens, issued once and revocable, for runners;
- the existing request bounds, plus a limit on concurrent connections;
- no admin surface;
- a plain-HTTP listener for Railway's proxy that serves only the authenticated routes.
  It never trusts forwarded headers for identity.

**Persistent access.** A runner connection is a long-lived grant to execute turns on
a Mac. Enrolling a Mac is an explicit owner action, the Mac app shows each runner's
status, and both sides can revoke. A runner executes only bindings pinned to it, with
the existing dispatch digest, deadline and evidence checks.

**Billing/provider path.** Unchanged. A binding pins provider, model, account and now
node. If that node is offline, the turn fails visibly, or waits up to a window the
owner sets on the binding (the default is no wait). It never moves to another Mac,
account or billing path. Railway's own bill is new and separate.

**Outcome honesty.** A hub restart or deploy, or a runner that drops mid-turn, leaves
the attempt `outcomeUnknown`. Nothing is replayed blindly.

**Undo.** Each Mac can return to local-hub mode. The ledger can be exported from the
hub and imported locally with the existing offline migration. Removing the Railway
service needs approval.

## Plan

1. **Runner channel (fixtures).** A runner POSTs `/v1/runner/connect` with its token,
   and the hub streams newline-delimited JSON frames back: dispatch, cancel and
   ping. Results and evidence go back as separate authenticated POSTs. The frames are
   the existing executor wire (`executor-wire.ts`, `executor-account-wire.ts`) with a
   lease ID and heartbeat added. This uses plain HTTP/1.1 streaming with no new
   dependency; I rejected WebSocket because it would need a server library.
   Tests: hub and runner on loopback cover a completed turn, cancel, missed heartbeats
   (outcome unknown), reconnect without replay, a foreign lease, an oversized frame
   and a revoked token.
2. **Enrollment and routing.** Runner slots work like companion pairing: an invite
   code, a digest-only token and revocation. Bindings already carry `hostId`, which
   the executor wire checks on both ends, so `hostId` is the node; no new field is
   needed. Tests cover an offline node (visible failure, then the wait window), the
   wrong node refused, and a revoked runner's pending work becoming unknown.
   Built in `service/runner-enrollment.ts` and `routeByNode` in `host/runner-hub.ts`.
   The wait window is a router option here; the owner-facing setting on the binding
   arrives with the hub entry point in step 3.
3. **Headless hub artifact.** `host/cloud-hub.ts` is the Linux entry point: one
   process on `$PORT`, the ledger on `$FORT_DATA`, and plain HTTP behind the proxy. A
   plain Dockerfile with no Railway-specific API keeps the move to any VM a redeploy.
   Tests cover start, stop, a restart mid-turn leaving the attempt unknown, and
   refusing to start with a second writer on the same ledger.
   Built in `host/cloud-hub.ts` and `successor/Dockerfile`. The owner claims a fresh
   hub with a one-time setup code; the hub's environment holds only its SHA-256
   (`FORT_SETUP_CODE_SHA256`), so Railway never sees the code. Runner management
   (`/v1/runners`, invitation, revoke) is owner-only; `/v1/runners/join` takes the
   invitation. Configuration: `PORT`, `FORT_DATA`, `FORT_PROVIDER`,
   `FORT_RUNTIME_VERSION`, `FORT_NODE`, `FORT_OFFLINE_WAIT_MS`.
   Limit: the hub entry point configures one runtime, so it serves one provider on one
   Mac. Spec 064 (merged 2026-10-24) lets the core serve extra runtimes, each matched
   by provider, version and host id, so the next step is to give the hub one runtime per
   enrolled Mac and provider, all routed through `routeByNode`.
4. **Mac runner mode.** The helper runs the runner client instead of a local core,
   using the network entitlements added in PR #31. The Mac app shows hub and runner
   status. This step also covers ledger migration (export from the Mac, import on the
   hub, the local copy kept until the owner confirms). It is tested by fixture first.
   A real Railway deploy and a real Mac enrollment come after approval.
   Built so far: `keepConnected` in `runner/hub-runner.ts` reconnects with capped,
   jittered backoff, treats 45 seconds of silence as a dead connection, and stops for
   good when the hub refuses its token. For one Mac the ledger needs no binding
   rewrite: the Mac enrolls with its existing `hostId` as its runner ID, and the hub
   runs with `FORT_NODE` set to that value, so `migrateLedger`'s copy is enough.
   Runner token: Toby chose on 2026-10-24 to keep it like the ChatGPT sign-in, in an
   owner-only `hub-runner.json` in the helper's private state directory
   (`runner/hub-credential.ts`). It is written only after the hub confirms the
   enrollment. Removing provider credentials on the Mac deletes it too. A revoked
   runner stops connecting and keeps the file until the owner removes it.
   Still to build: the helper launching runner mode (bundle entry and packaging) and
   the Mac app's enrollment and status view. Both need a Mac to test.

Rejected alternatives:
- A VPN mesh (Tailscale everywhere): it puts a client on every device.
- Replicated ledgers: they break the single-writer rule.
- Railway Postgres: it ties storage to the host and adds a dependency.
- A Railway-specific SDK: it creates lock-in.

Open: the domain name (a Railway subdomain or Toby's own), and whether the iPhone
keeps the Tailscale path as a fallback.

## Revision, 2026-10-25: phone and web clients, one Mac with two providers

**Decision:** On 2026-10-25 Toby asked to move Fort off the Mac so he can use it from his
phone and the web ("move this experience off my local Mac only"). The coordinator relayed
that as OK to deploy the hub to Railway. Storing his provider credentials in the cloud,
and spending plan usage, still need their own approval. Neither happens in this revision.

What changes:

- **Several runtimes per hub.** `FORT_RUNTIMES` lists every runtime the Mac at
  `FORT_NODE` serves (for example ChatGPT and Claude), and each one becomes a Spec 064
  lane. All of a Mac's lanes share that Mac's runner ID as their `hostId`, so "hostId is
  the node" holds and one runner connection serves them all. The runner chooses the lane
  from the exact binding (provider, version, host) and refuses anything else.
- **Accounts and models over the channel.** Model discovery and account confirmation
  are leased to the Mac like turns, over the same executor wire. The hub pins each turn to
  the account the Mac reports, which the owner confirms once per provider. Sign-in itself
  still happens on the Mac. The hub never sees a provider credential, and with the Mac
  offline a check comes back "unconfirmed", never invented.
- **Web client.** The hub serves a small static web client at `/` with a strict content
  policy (its own scripts only, no framing). Browser calls are allowed only from
  `FORT_PUBLIC_ORIGIN`, the hub's own address. The origin is removed before the core sees
  the request, so the core's own refusal of browsers is unchanged for every other origin,
  and the runner channel never accepts a browser.
- **New credential: the web owner.** The browser that claims the hub with the setup code
  becomes the owner device. Its device token lives in that browser's local storage, and
  the hub keeps only its digest, as for every paired device. Anyone with that browser
  profile can use Fort, so this suits a personal device. Recovery is a fresh setup code.
- **iPhone.** No app change. The owner creates a phone invitation in the web client,
  which shows the existing `fort-invite:` text with the hub's https address, and the
  iPhone pairs as the companion.

**Mac runner.** Beside its local core, the helper keeps one runner connection for all its
runtimes once the owner pastes the hub's `fort-runner:` invitation (which names the
runner ID) under Devices, Cloud hub. The local core gains owner-only `/v1/hub/runner`
routes for status, enroll and remove. ChatGPT gets a second executor under the runner
ID; Claude's runtime is shared with the local core. The connection lives only as long as
the helper does, so no new login item or startup registration is added. Keeping the Mac
reachable with no Fort window open would need one, and that needs Toby's OK.

Still to do, in order: verify the runner on Toby's Mac; then ledger migration with a
hostId rewrite to the runner ID; then the Mac app as a hub client.

## Revision, 2026-10-28: Mac mini first, laptop as overflow

**Decision:** On 2026-10-28 Toby chose the Mac mini (nearly always on, battery backed) as the primary
runner and his laptop as overflow, with no cloud runner. The laptop stays signed in to the same
ChatGPT and Claude accounts as the mini, queued work raises an alert after 15 minutes, and FileVault
stays on for the mini. Enrolling either Mac and changing the hub's Railway variables still need his OK.

What changes:

- **One node, several Macs.** `FORT_RUNNER_POOL` lists the Macs that serve `FORT_NODE`, primary first
  (for example `toby-mini,toby-laptop`). Every Mac in the pool offers its lanes under the node's ID, so
  bindings, dispatch digests and the executor wire are unchanged: `hostId` is still the node. The hub
  tells each Mac its node when it joins, and the Mac keeps it beside its runner token.
- **Picking a Mac.** Each run goes to the first connected Mac in the pool: the mini, or the laptop while
  the mini is away. The core runs one turn at a time, so "overflow" means "the mini isn't connected";
  load-based overflow can come later if the core runs turns in parallel.
- **Pinning.** The first Mac a run reaches is recorded (`run_pins` in `runners.sqlite`, kept 30 days).
  A routine's later steps go to that Mac only. If it is gone, the step waits up to
  `FORT_OFFLINE_WAIT_MS` and is then refused with "Nothing was sent". A started run never moves.
- **Queueing.** With no Mac in the pool connected, the lane reports itself unavailable, so the core keeps
  accepted runs queued in its ledger instead of failing them. When a Mac connects, the hub wakes the core
  and the queue drains in order.
- **Alert.** When queued work has waited `FORT_QUEUE_ALERT_MINUTES` (default 15) with no Mac connected,
  the hub logs one line (counts and Mac IDs only, never message text), `GET /v1/runners` shows
  `pool.queue.alert`, and the runtime status says how long work has waited. A push to the phone is not
  built yet.

Risk:

- **Billing/provider path.** A run can now reach either Mac. Both runtimes check the account on every
  turn: a Mac signed in to a different ChatGPT or Claude account refuses the turn ("Nothing was sent")
  instead of running it on another account. Model and account checks go to the first connected Mac.
- **Persistent access.** Unchanged: each Mac enrolls with its own one-time invitation and token, and the
  owner can revoke either one. A Mac outside the pool can't take the pool's turns.
- **Undo.** Remove `FORT_RUNNER_POOL` and the hub routes to `FORT_NODE` alone, as before. Pins are
  advisory data in `runners.sqlite`; deleting them changes nothing else.

Tests: `host/runner-pool.test.ts` (choice, pinning, waiting, a Mac outside the pool) and
`host/cloud-hub-pool.test.ts` (join names the node, mini then laptop, queued until a Mac connects, one
alert), plus the pooled credential case in `runner/hub-credential.test.ts`.
