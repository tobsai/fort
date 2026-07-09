# Fort remote gateway — self-host runbook (spec 028)

Drive your Forts from anywhere — web, iOS, Mac — through a gateway **you** host,
with Google sign-in, no Tailscale, and no inbound ports. The daemon dials *out*
to the gateway; the gateway brokers your browser/app requests back down that
connection, **end-to-end encrypted** (the gateway relays ciphertext only).

The gateway is three deployables in this repo:

| Piece            | What it is                                   | Deploy to  |
| ---------------- | -------------------------------------------- | ---------- |
| `gateway/worker` | Cloudflare Worker + Durable Objects (broker) | Cloudflare |
| `gateway/web`    | Next.js app: Google auth + machines + board  | Vercel     |
| the daemon       | `fort serve` + `fort relay join` on each box | your machine |

`gateway/shared` is the crypto/frame library shared by the worker and web app
(imported as `@fort/gateway-shared`); it is not deployed on its own.

> Everything below is **self-host, per-owner**. One gateway = one owner, gated by
> an email allowlist. There is no multi-tenant/accounts model (spec 028, D1).

---

## 0. Prerequisites

- Node 22 / npm 10 (for both `gateway/worker` and `gateway/web`).
- A Cloudflare account with Workers + Durable Objects (`npm i -g wrangler`, then
  `wrangler login`).
- A Vercel account (`npm i -g vercel`, then `vercel login`) — or deploy the web
  app to any Node host that supports Next.js App Router.
- A Google Cloud project for the OAuth client.
- `fort` installed on each machine you want to reach.

---

## 1. Create the Google OAuth client

1. Google Cloud Console → **APIs & Services → OAuth consent screen**. Pick
   *External*, add your app name and your email, and add yourself as a **test
   user** (or publish the app). No special scopes are required beyond the default
   `openid email profile`.
2. **APIs & Services → Credentials → Create credentials → OAuth client ID →
   Web application**.
3. Add **Authorized redirect URIs** (exactly these paths):
   - Production: `https://<your-web-app-domain>/api/auth/callback/google`
   - Local dev: `http://localhost:3000/api/auth/callback/google`
4. Copy the **Client ID** and **Client secret** — these become `AUTH_GOOGLE_ID`
   and `AUTH_GOOGLE_SECRET`.

> Tip: this repo ships a `google-cloud-oauth` skill that can create the project +
> client for you.

---

## 2. Deploy the worker (Cloudflare)

```sh
cd gateway/worker
npm install

# Pick a strong shared secret (used by the web app to authenticate to the broker):
GATEWAY_SECRET=$(openssl rand -base64 32)
echo "GATEWAY_SECRET=$GATEWAY_SECRET"   # copy this — you'll paste it into Vercel too

# Store it as a Worker secret (NOT the [vars] placeholder in wrangler.toml):
echo -n "$GATEWAY_SECRET" | npx wrangler secret put GATEWAY_SECRET

# Deploy:
npm run deploy    # == wrangler deploy
```

`wrangler deploy` prints the worker URL, e.g.
`https://fort-gateway.<your-subdomain>.workers.dev`. That is your **`WORKER_URL`**
and, unless you put a custom domain in front of it, also your
**`NEXT_PUBLIC_GATEWAY_URL`** (the host in the `fort relay join` line).

Optional: `CODE_TTL_SECONDS` (join-code lifetime, default 900) can be set in
`wrangler.toml [vars]`.

---

## 3. Deploy the web app (Vercel)

```sh
cd gateway/web
npm install
npm run build      # sanity-check the build locally first
```

Set these environment variables in the Vercel project (Project → Settings →
Environment Variables), then deploy with `vercel deploy --prod` (or push to a
connected Git repo). All are **server-side only** except
`NEXT_PUBLIC_GATEWAY_URL`.

See `gateway/web/.env.example` for the annotated template. Generate `AUTH_SECRET`
with `openssl rand -base64 32` (or `npx auth secret`).

> **The `GATEWAY_SECRET` must never be exposed to the browser.** It lives only in
> the web app's server routes/components and is injected as the
> `X-Gateway-Secret` header on internal calls to the worker. Do **not** give it a
> `NEXT_PUBLIC_` prefix. (Verified: it does not appear in any client bundle.)

For local dev, copy `.env.example` to `.env.local`, fill it in, and
`npm run dev` (http://localhost:3000).

---

## 4. Join a machine and verify the fingerprint

On the machine running `fort serve`:

1. In the web app, open **Add machine** → **Generate join code**. It shows a
   paste-ready line:
   ```
   fort relay join https://fort-gateway.<you>.workers.dev --code XXXX-XXXX
   ```
2. Run it on the machine. `fort relay join` exchanges the single-use code for a
   device token, generates the daemon's X25519 static identity (persisted 0600
   in `relay.yaml`), and **prints the key fingerprint**.
3. Start (or restart) `fort serve`. It dials the gateway outbound and comes
   **online** on the **Machines** page.
4. **Verify the fingerprint.** The fingerprint printed by `fort relay join` must
   match the one shown for that machine on the Machines page and the Board page.
   The browser **pins** this key on first connect (TOFU); if it ever changes, the
   Board page shows a loud warning and refuses to connect — so compare it once,
   here, at enrollment.

Revoke a machine any time with the **Revoke** button (or `fort relay remove` on
the machine): the device token is invalidated and the tunnel drops.

---

## 5. Environment variables — complete list

### `gateway/worker` (Cloudflare)

| Variable           | Where it comes from                                   | Secret? |
| ------------------ | ----------------------------------------------------- | ------- |
| `GATEWAY_SECRET`   | You generate it (`openssl rand -base64 32`); set via `wrangler secret put`. Must match the web app's `GATEWAY_SECRET`. | **yes** |
| `CODE_TTL_SECONDS` | Optional, `wrangler.toml [vars]`; join-code lifetime in seconds (default `900`). | no |

### `gateway/web` (Vercel)

| Variable                  | Where it comes from                                                                 | Reaches browser? |
| ------------------------- | ----------------------------------------------------------------------------------- | ---------------- |
| `AUTH_SECRET`             | You generate it (`openssl rand -base64 32` / `npx auth secret`). Signs the session JWT. | no |
| `AUTH_GOOGLE_ID`          | Google OAuth **Client ID** (step 1).                                                | no |
| `AUTH_GOOGLE_SECRET`      | Google OAuth **Client secret** (step 1).                                            | no |
| `FORT_ALLOWLIST`          | You choose it: comma-separated Google emails allowed to sign in. Everyone else is denied. | no |
| `GATEWAY_SECRET`          | The **same** value you set on the worker (step 2). Injected as `X-Gateway-Secret`.  | **NO — never** |
| `WORKER_URL`              | The worker's URL from `wrangler deploy` (step 2), no trailing slash.                 | no |
| `NEXT_PUBLIC_GATEWAY_URL` | The public gateway URL shown in the `fort relay join` line (usually = `WORKER_URL`, or a custom domain in front of it). | yes (public) |
| `AUTH_URL` (optional)     | Only if auto-detection needs help behind a proxy: `https://<your-web-app-domain>`.  | no |

### the daemon (`relay.yaml`, written by `fort relay join`)

| Field           | Where it comes from                                              |
| --------------- | --------------------------------------------------------------- |
| gateway URL     | The `<gateway-url>` you passed to `fort relay join`.             |
| device token    | Issued by the worker when the join code is redeemed.            |
| X25519 keypair  | Generated locally on first join (0600); its public key is pinned by clients. |

---

## What the web board does (v1) vs. what's deferred

- **Auth**: Google OIDC via Auth.js v5, JWT sessions. The `signIn` callback
  rejects any email not on `FORT_ALLOWLIST`. All pages and internal `/api/*`
  routes require a session.
- **Machines / Add / Revoke**: fully working through the worker's internal API.
- **Board (`/m/[id]`)**: the browser runs the **Noise IK initiator handshake**
  over the tunnel and pins the daemon's static key (TOFU). It then does **sealed
  requests end-to-end**:
  - a sealed `GET /api/summary` renders a native status view,
  - **Open board** does a sealed `GET /` and renders the returned board HTML in a
    sandboxed `srcdoc` iframe — a **static snapshot** (scripts disabled),
  - **Tail events** opens a sealed `GET /api/events` SSE stream over the tunnel.
- **Deferred (v1)**: a *fully interactive* proxied board (where the iframe's own
  JS drives `/api/*` and live SSE back through the tunnel) is not built. For a
  fully interactive experience, use a native client (iOS/Mac), which ships its
  own crypto. The sealed handshake, sealed GET, and sealed SSE are all real in
  the web client today — only the in-iframe interactivity is deferred.
