# 028 — Remote gateway (Vercel + Cloudflare tunnel plane)

**Status:** approved; web/worker/daemon delivered, native iOS client completed 2026-07-22.
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
- **E2E is in scope (see below), with one honest limit:** the browser web client
  runs crypto in JS **served by the gateway origin**, so a malicious/compromised
  gateway *web deploy* could serve backdoored client code. E2E fully protects
  native clients (iOS/Mac ship their own crypto) and protects **all** clients
  (web included) against the **tunnel broker** (Worker/DO) reading or altering
  traffic. Hardening the web-origin trust (SRI/pinned bundle, or "native-only for
  max assurance") is documented, not a v1 build item.
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

### End-to-end encryption (the client ↔ daemon payload is opaque to the gateway)
The gateway (Vercel web + Cloudflare Worker/DO) is a **ciphertext-only relay**:
it authenticates *who* may reach *which* machine (Google session + device token)
and moves frames, but it cannot read or forge the proxied HTTP/SSE payload.
- **Daemon static identity.** `fort serve` holds a long-term **X25519** keypair
  in `relay.yaml` (0600), generated on first `fort relay join`. Its public-key
  **fingerprint** is printed by `fort relay join` and shown on the web/app
  machine list so the owner can verify it.
- **Session handshake.** Each client session runs a **Noise (IK) handshake**
  ([noiseprotocol.org]) over the tunnel — client ephemeral key ↔ daemon static
  key — deriving a symmetric session key. The client **pins** the daemon static
  key on first enroll (TOFU) and shows its fingerprint to compare; a changed key
  warns loudly (defeats a gateway that swaps in its own key to MITM).
- **Payload encryption.** Every proxied request/response and every SSE frame is
  sealed with an **AEAD (ChaCha20-Poly1305)** under the session key before it
  enters the tunnel. The Worker/DO sees only Noise handshake messages + opaque
  ciphertext with routing headers (target machine id, stream id) — never the
  board data, task bodies, logs, or tokens.
- **Client auth to the daemon.** The daemon accepts a session only when the
  gateway presents a valid **device-scoped relay token** *and* the Noise
  handshake completes; the owner authorized the device at join time. (A
  compromised gateway still cannot read traffic — that is the point of pinning
  the daemon key.)
- **Crypto lives in a Fort-owned unit** (`exec/relay/secure`), reused by the Go
  daemon and mirrored in FortKit (Swift, via CryptoKit/libsodium) and the web
  client (WebCrypto/`libsodium.js`) so all three speak the same handshake + AEAD.

### Request path
`browser/app → gateway/web (Auth.js session) → Worker (validates session, selects
machine DO) → machine DO ↔ daemon WebSocket → in-process ui/node mux → response
back up the same path`. The **application payload is E2E-encrypted** end to end
(client ↔ daemon); the Worker/DO relays only Noise/AEAD frames. SSE
(`/api/events`, the live board/drawer feed) is carried as a long-lived framed —
and encrypted — stream over the one socket, no per-event polling.

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
- **D5 — end-to-end encrypted payload; the gateway is a ciphertext relay.**
  Noise IK (X25519) session handshake + ChaCha20-Poly1305 AEAD between client
  and daemon; the daemon's static key is pinned by clients (TOFU + a visible
  fingerprint) so a compromised broker can neither read nor MITM traffic.
  Revocable device tokens still gate *who* may connect. Residual trust: the
  browser web client's crypto is gateway-served JS (see Non-goals); native
  clients are fully E2E. TLS per hop remains as defense in depth.
- **D6 — TypeScript gateway is a separate artifact.** It is not in the Go module;
  `go build ./...` and the seams tests are unaffected.

## Affected files
- `gateway/web/**` (new) — Next.js app.
- `gateway/worker/**` (new) — Worker + Durable Object.
- `gateway/SETUP.md` (new) — self-host guide.
- `exec/relay/*.go` (new) — outbound relay transport.
- `exec/relay/secure/*.go` (new) — the Fort-owned E2E unit: X25519 static key,
  Noise IK handshake, ChaCha20-Poly1305 AEAD framing (Go).
- `cmd/fort/relay.go` (new) — `fort relay` CLI (join/remove/status, prints the
  daemon key fingerprint); `cmd/fort/main.go` usage + `fort serve` wiring (dial
  the relay when `relay.yaml` present).
- `core/config/*.go` — `relay.yaml` load/precedence (gateway URL, device token,
  **X25519 static keypair**, 0600).
- `ui/apple/FortKit/Sources/**` — `GatewayAccount` + the mirrored handshake/AEAD
  (CryptoKit/libsodium) and daemon-key pinning UI (fingerprint compare).
- `gateway/web` + `gateway/worker` — carry Noise/AEAD frames opaquely; the web
  client bundles the WebCrypto/`libsodium.js` mirror of the handshake.
- `docs/notes/threat-model.md` — E2E model, the ciphertext-relay boundary,
  daemon-key pinning/TOFU, the web-origin residual trust, and token revocation.
- `README.md` — remote-access section.

## Test criteria
- `exec/relay`: with a fake Worker (in-process WebSocket test server), the
  transport dials, authenticates with a device token, completes the Noise
  handshake, serves an **AEAD-encrypted** proxied `GET /api/summary` round-trip,
  and streams an encrypted SSE event end-to-end; reconnects after a dropped
  socket (backoff).
- `exec/relay/secure` (the E2E unit): a full handshake between two parties
  derives matching session keys; an AEAD round-trip decrypts to the original
  plaintext; a **tampered ciphertext frame is rejected** (auth-tag failure); a
  handshake against a **wrong/rotated daemon static key is rejected** (pinning
  works); a passive observer holding only the relayed frames **cannot recover
  plaintext** (asserted by decrypting with the broker's view → failure).
- `fort relay join`: exchanges a code for a token, generates + writes the X25519
  keypair + token to `relay.yaml` 0600, and prints a stable key fingerprint; a
  reused code is rejected (single-use, atomic).
- Determinism guard: `go list -deps ./exec/relay` imports no
  `core/router|core/engine|core/graph`; the relay path makes zero model calls.
- `gateway/worker`: unit tests (miniflare) — allowlisted session routes to the
  right DO; non-allowlisted session → 403; join-code mint→consume is single-use.
- `gateway/web`: an allowlisted vs non-allowlisted sign-in test (Auth.js
  callback returns/denies a session).
- `go test ./...` + `-race` on `exec/relay` green; Go seams intact (the Go build
  is unaffected by `gateway/`).

## Rollback
Additive. Delete `relay.yaml` (daemon stops dialing and discards its static key;
local + mesh unaffected). Revert `exec/relay` (+ `exec/relay/secure`), `fort
relay`, and the FortKit account/crypto. The `gateway/` tree is a separate
deploy — tearing down the Vercel/Cloudflare projects fully removes the remote
path with no effect on the daemon. No persisted app data is encrypted at rest by
this spec (E2E is transport-only), so there is nothing to migrate on rollback.
