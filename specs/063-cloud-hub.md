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
   Limit: the core pins one runtime identity per ledger, so one hub serves one
   provider on one Mac. Serving several Macs or providers from one hub changes how
   bindings pin provider and node, so it gets its own spec revision before it is built.
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
   Still to build: the helper launching runner mode, storing the runner token (a new
   credential on the Mac, so it needs approval first) and the Mac app's status view.

Rejected alternatives:
- A VPN mesh (Tailscale everywhere): it puts a client on every device.
- Replicated ledgers: they break the single-writer rule.
- Railway Postgres: it ties storage to the host and adds a dependency.
- A Railway-specific SDK: it creates lock-in.

Open: the domain name (a Railway subdomain or Toby's own), and whether the iPhone
keeps the Tailscale path as a fallback.
