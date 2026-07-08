# 028 — Remote gateway (Vercel + Cloudflare tunnel plane)

**Status:** design approved in brainstorm (Toby, 2026-07-08) — pending written-spec review.
**New capability — approved before implementation** (adds a self-hostable gateway + an outbound relay transport in the daemon).
**Governed by:** [021-fort-native](021-fort-native.md) · builds on the mesh ([024-mesh-enrollment](024-mesh-enrollment.md)).

## Goal
Drive your Forts from anywhere — web, iOS, Mac — through a gateway **you** host,
with Google sign-in, without Tailscale and without opening any inbound ports.
The daemon dials **out** to the gateway; the gateway brokers browser/app requests
back down that connection. Because Fort is open source, the gateway ships in-repo
so anyone can stand up their **own** instance.

## Non-goals (v1 — YAGNI)
- **No multi-tenant SaaS.** One gateway = one owner (an email allowlist). No
  org/team model, no billing, no per-user isolation beyond the allowlist.
- **No end-to-end encryption** through the relay in v1. TLS terminates at each
  hop; the tunnel broker sees plaintext frames. Documented in the threat model;
  E2E is a noted hardening follow-on.
- **No inbound-port / reverse-proxy mode here** — that path (LAN/mesh with a
  shared token) already exists (024). 028 is specifically the WAN/NAT-traversal,
  account-authenticated path.
- **No replacement of the mesh.** `fort mesh` (LAN, deterministic placement)
  stays; the gateway is an additional reach layer, not a rework of placement.

## Approach

### Three deployables
1. **`gateway/web`** — a Next.js app (deploy target: Vercel) with **Auth.js**
   Google OIDC. Restricted to an **email allowlist** (`FORT_ALLOWLIST`, comma-
   separated) — it is a personal gateway, so an authenticated non-allowlisted
   Google account is rejected. Pages: **Machines** (list with online/offline +
   remove/revoke), **Add machine** (mints a join code, shows the paste-ready
   `fort relay join …` line), and **Board** (the served Fort board, proxied
   through the tunnel for the selected machine).
2. **`gateway/worker`** — a Cloudflare Worker + **Durable Object** that holds the
   long-lived WebSocket tunnels (Vercel serverless can't). One DO instance per
   registered machine: the daemon's outbound socket on one side, browser/app
   HTTP+SSE requests on the other, multiplexed as framed messages. The Worker
   validates the gateway-issued session (from `gateway/web`) before routing a
   request into a machine's DO.
3. **Daemon transport (`exec/relay` + `fort relay` CLI)** — `fort relay join
   <gateway-url> --code XXXX-XXXX` registers this machine (deliberately mirroring
   `fort mesh join`): it exchanges the single-use code for a **device token**,
   persisted (0600) alongside the node identity. `fort serve` then maintains an
   **outbound reconnecting WebSocket** to the Worker, authenticating with the
   device token, and serves proxied HTTP requests + SSE streams over it by
   handing them to the same in-process `ui`/`node` mux the local listener uses.

### Registration (join code, like the mesh)
- On the web app, **Add machine** calls the Worker to mint a **single-use join
  code** (SHA-256 stored, short TTL — reuse the invite semantics from 024's
  `store.invite` design as the model). It renders `fort relay join <gateway-url>
  --code XXXX-XXXX`.
- On the machine, `fort relay join` posts the code + a chosen machine name to the
  Worker, receives a durable **device token** + the machine id, and writes them
  to `relay.yaml` (0600). Codes are consumed atomically (single-use; no
  double-registration — the same TOCTOU care taken in 024).
- `fort relay remove` (or the web **Revoke**) invalidates the device token; the
  daemon's socket is dropped and the machine goes offline.

### Request path
`browser/app → gateway/web (Auth.js session) → Worker (validates session, selects
machine DO) → machine DO ↔ daemon WebSocket → in-process ui/node mux → response
back up the same path`. SSE (`/api/events`, the live board/drawer feed) is carried
as a long-lived framed stream over the one socket — no per-event polling.

### Clients
- **Web** = `gateway/web` itself.
- **iOS / Mac (FortKit)** gain a **gateway account**: Google sign-in → gateway
  session token → API/SSE requests routed at `<gateway-url>` instead of a
  tailnet host. A machine picker selects which registered machine to drive. (The
  Mac app in 032 consumes this.)

### Determinism / seams
The gateway is **transport only** — it moves already-formed HTTP/SSE bytes. Zero
routing/placement/inference happens in the gateway or the relay transport; the
daemon still routes deterministically and infers only at task nodes. `exec/relay`
implements an outbound transport; it imports no `core` deterministic packages
beyond the `ui`/`node` mux it already exposes locally. The `gateway/` tree is a
separate deploy artifact (TypeScript), not part of the Go module.

### Failure handling
- Gateway unreachable → daemon retries with backoff; local listener + mesh keep
  working (the relay is additive, never required).
- Device token revoked → socket closes; `fort serve` logs it and stops
  reconnecting until re-joined.
- Web session expired/not allowlisted → 401/403 at the Worker before any machine
  is touched.

## Architecture (respects the seams)
- **`gateway/web/`** — Next.js + Auth.js (Google), allowlist, machines UI,
  board proxy. Deploy: Vercel.
- **`gateway/worker/`** — Cloudflare Worker + Durable Object (tunnel broker,
  join-code mint/consume, session validation). Deploy: wrangler.
- **`exec/relay/`** (Go, new) — outbound reconnecting WebSocket transport;
  serves proxied requests/SSE via the existing in-process mux.
- **`cmd/fort/relay.go`** (Go, new) — `fort relay join|remove|status`.
- **`core/config`** — `relay.yaml` (gateway URL + device token, 0600), env
  precedence consistent with `node.yaml` (024).
- **`ui/apple/FortKit`** — a `GatewayAccount` (Google session, base URL,
  machine selection) added behind the existing HTTP/SSE client.
- **`gateway/SETUP.md`** — self-host runbook: create a Google OAuth client,
  `vercel deploy`, `wrangler deploy`, set env (`FORT_ALLOWLIST`, Google client
  id/secret, Worker URL, a shared gateway secret).

## Decisions
- **D1 — self-hostable, per-owner.** The gateway ships in-repo; each user runs
  their own. An email allowlist, not accounts-for-everyone.
- **D2 — Vercel + Cloudflare (option B).** Chosen by Toby. Vercel hosts the
  authenticated web/UI; a Cloudflare Worker+DO holds the long-lived tunnels that
  Vercel serverless cannot.
- **D3 — outbound-only daemon socket.** No inbound ports; NAT-agnostic. Same
  join-code UX as `fort mesh join` so it feels like one product.
- **D4 — transport only, determinism preserved.** The gateway never routes or
  infers; it proxies bytes. Asserted by keeping `exec/relay` free of
  router/engine imports.
- **D5 — no E2E in v1.** TLS per hop; broker sees plaintext. Documented;
  revocable device tokens limit blast radius. E2E deferred.
- **D6 — TypeScript gateway is a separate artifact.** It is not in the Go module;
  `go build ./...` and the seams tests are unaffected.

## Affected files
- `gateway/web/**` (new) — Next.js app.
- `gateway/worker/**` (new) — Worker + Durable Object.
- `gateway/SETUP.md` (new) — self-host guide.
- `exec/relay/*.go` (new) — outbound relay transport.
- `cmd/fort/relay.go` (new) — `fort relay` CLI; `cmd/fort/main.go` usage +
  `fort serve` wiring (dial the relay when `relay.yaml` present).
- `core/config/*.go` — `relay.yaml` load/precedence.
- `ui/apple/FortKit/Sources/**` — `GatewayAccount`.
- `docs/notes/threat-model.md` — relay trust boundary + token revocation.
- `README.md` — remote-access section.

## Test criteria
- `exec/relay`: with a fake Worker (in-process WebSocket test server), the
  transport dials, authenticates with a device token, serves a proxied
  `GET /api/summary` round-trip, and streams an SSE event end-to-end; reconnects
  after a dropped socket (backoff).
- `fort relay join`: exchanges a code for a token, writes `relay.yaml` 0600;
  a reused code is rejected (single-use, atomic).
- Determinism guard: `go list -deps ./exec/relay` imports no
  `core/router|core/engine|core/graph`; the relay path makes zero model calls.
- `gateway/worker`: unit tests (miniflare) — allowlisted session routes to the
  right DO; non-allowlisted session → 403; join-code mint→consume is single-use.
- `gateway/web`: an allowlisted vs non-allowlisted sign-in test (Auth.js
  callback returns/denies a session).
- `go test ./...` + `-race` on `exec/relay` green; Go seams intact (the Go build
  is unaffected by `gateway/`).

## Rollback
Additive. Delete `relay.yaml` (daemon stops dialing; local + mesh unaffected).
Revert `exec/relay` + `fort relay` + the FortKit account. The `gateway/` tree is
a separate deploy — tearing down the Vercel/Cloudflare projects fully removes the
remote path with no effect on the daemon.
