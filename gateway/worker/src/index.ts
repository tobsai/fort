// index: the router Worker — the tunnel broker's front door and composition
// root. It authenticates each request (Google-session-backed GATEWAY_SECRET for
// the web app's internal calls; device tokens for the daemon socket and revoke)
// and dispatches to the two Durable Objects. It moves only frame envelopes; the
// sealed `b64` payload is opaque to it end to end (spec 028, D5).
//
// Routes
//   POST   /api/relay/invite        internal  -> mint single-use join code
//   POST   /api/relay/join          public    -> redeem code -> device token
//   GET    /api/relay/machines      internal  -> list machines (+ fingerprint)
//   DELETE /api/relay/machines/:id  token|sec -> revoke machine, drop socket
//   GET    /tunnel                  token(ws) -> daemon attaches its socket
//   POST   /api/relay/req           internal  -> buffered sealed-frame relay
//   POST   /api/relay/sse           internal  -> streaming sealed-frame relay

import { decodeBase64, fingerprint } from "@fort/gateway-shared";
import type { Frame } from "@fort/gateway-shared";

import { RegistryDO } from "./registry";
import { TunnelDO } from "./tunnel";
import type { Env, MachineSummary } from "./types";
import { error, json, timingSafeEqual } from "./types";

export { RegistryDO, TunnelDO };

const DEFAULT_CODE_TTL = 900;

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    const path = url.pathname;
    const method = request.method;

    try {
      if (method === "GET" && path === "/") return new Response("fort-gateway\n");

      if (path === "/tunnel") return handleTunnel(request, env);

      if (method === "POST" && path === "/api/relay/invite") return handleInvite(request, env);
      if (method === "POST" && path === "/api/relay/join") return handleJoin(request, env);
      if (method === "GET" && path === "/api/relay/machines") return handleMachines(request, env);
      if (method === "DELETE" && path.startsWith("/api/relay/machines/")) {
        return handleRevoke(request, env, path.slice("/api/relay/machines/".length));
      }
      if (method === "POST" && path === "/api/relay/req") return handleReq(request, env);
      if (method === "POST" && path === "/api/relay/sse") return handleSse(request, env);

      return error(404, "not found");
    } catch (e) {
      return error(500, e instanceof Error ? e.message : "internal error");
    }
  },
};

// --- helpers ----------------------------------------------------------------

function registry(env: Env): DurableObjectStub<RegistryDO> {
  return env.REGISTRY.get(env.REGISTRY.idFromName("registry"));
}

function tunnel(env: Env, machineId: string): DurableObjectStub<TunnelDO> {
  return env.TUNNEL.get(env.TUNNEL.idFromName(machineId));
}

/** hasSecret checks the internal shared secret (web app -> broker). */
function hasSecret(request: Request, env: Env): boolean {
  const got = request.headers.get("x-gateway-secret");
  return got !== null && timingSafeEqual(got, env.GATEWAY_SECRET);
}

/** bearer extracts a device token from the Authorization header. */
function bearer(request: Request): string | null {
  const h = request.headers.get("authorization");
  if (!h) return null;
  const m = /^Bearer\s+(.+)$/i.exec(h.trim());
  return m ? m[1]! : null;
}

function ttlSeconds(env: Env): number {
  const n = env.CODE_TTL_SECONDS ? parseInt(env.CODE_TTL_SECONDS, 10) : NaN;
  return Number.isFinite(n) && n > 0 ? n : DEFAULT_CODE_TTL;
}

async function internalTunnelCall(
  env: Env,
  machineId: string,
  path: string,
  frame: Frame,
): Promise<Response> {
  return tunnel(env, machineId).fetch(
    new Request("https://tunnel" + path, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ frame }),
    }),
  );
}

// --- route handlers ---------------------------------------------------------

async function handleInvite(request: Request, env: Env): Promise<Response> {
  if (!hasSecret(request, env)) return error(401, "unauthorized");
  const code = await registry(env).mintCode(ttlSeconds(env));
  return json({ code });
}

async function handleJoin(request: Request, env: Env): Promise<Response> {
  let body: { code?: string; name?: string; public_key?: string };
  try {
    body = (await request.json()) as typeof body;
  } catch {
    return error(400, "invalid json");
  }
  if (!body.code || !body.public_key) return error(400, "code and public_key required");
  // The public key is stored and later fingerprinted for the machine list; make
  // sure it is a real 32-byte X25519 key so a bad join fails fast, not at list.
  try {
    if (decodeBase64(body.public_key).length !== 32) return error(400, "public_key must be 32 bytes");
  } catch {
    return error(400, "public_key must be base64");
  }
  const res = await registry(env).join(body.code, body.name ?? "", body.public_key);
  if (!res.ok) return error(409, `join code ${res.reason}`);
  return json({ device_token: res.device_token, machine_id: res.machine_id });
}

async function handleMachines(request: Request, env: Env): Promise<Response> {
  if (!hasSecret(request, env)) return error(401, "unauthorized");
  const machines = await registry(env).listMachines();
  const summaries: MachineSummary[] = await Promise.all(
    machines.map(async (m): Promise<MachineSummary> => {
      let online = false;
      try {
        const st = await tunnel(env, m.machine_id).fetch("https://tunnel/status");
        online = ((await st.json()) as { online: boolean }).online;
      } catch {
        online = false;
      }
      return {
        machine_id: m.machine_id,
        name: m.name,
        fingerprint: fingerprint(decodeBase64(m.public_key)),
        online,
      };
    }),
  );
  return json({ machines: summaries });
}

async function handleRevoke(request: Request, env: Env, id: string): Promise<Response> {
  if (!id) return error(404, "not found");
  // Authorize: the shared secret (web "Revoke") OR the machine's own token.
  let authorized = hasSecret(request, env);
  if (!authorized) {
    const token = bearer(request);
    if (token) authorized = (await registry(env).machineIdByToken(token)) === id;
  }
  if (!authorized) return error(403, "forbidden");

  const removed = await registry(env).removeMachine(id);
  if (!removed) return error(404, "unknown machine");
  await tunnel(env, id).fetch(new Request("https://tunnel/close", { method: "POST" }));
  return json({ revoked: true });
}

async function handleTunnel(request: Request, env: Env): Promise<Response> {
  if (request.headers.get("Upgrade") !== "websocket") return error(426, "expected websocket");
  const token = bearer(request);
  if (!token) return error(401, "unauthorized");
  const machineId = await registry(env).machineIdByToken(token);
  if (!machineId) return error(401, "unauthorized");
  // Forward the upgrade to the machine's DO, which attaches the daemon socket.
  return tunnel(env, machineId).fetch(request);
}

async function handleReq(request: Request, env: Env): Promise<Response> {
  if (!hasSecret(request, env)) return error(401, "unauthorized");
  const { machine_id, frame } = (await request.json()) as { machine_id?: string; frame?: Frame };
  if (!machine_id || !frame) return error(400, "machine_id and frame required");
  return internalTunnelCall(env, machine_id, "/relay", frame);
}

async function handleSse(request: Request, env: Env): Promise<Response> {
  if (!hasSecret(request, env)) return error(401, "unauthorized");
  const { machine_id, frame } = (await request.json()) as { machine_id?: string; frame?: Frame };
  if (!machine_id || !frame) return error(400, "machine_id and frame required");
  return internalTunnelCall(env, machine_id, "/sse", frame);
}
