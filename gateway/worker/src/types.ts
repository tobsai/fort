// types: the Worker environment bindings + small HTTP helpers shared by the
// router and the Durable Objects.

import type { RegistryDO } from "./registry";
import type { TunnelDO } from "./tunnel";

export interface Env {
  /** Singleton registry: join codes + machine registry (see registry.ts). */
  REGISTRY: DurableObjectNamespace<RegistryDO>;
  /** One instance per machine_id: the daemon socket + session mux (tunnel.ts). */
  TUNNEL: DurableObjectNamespace<TunnelDO>;
  /** Shared secret the web app presents on internal endpoints. */
  GATEWAY_SECRET: string;
  /** Join-code lifetime in seconds (string; wrangler vars are strings). */
  CODE_TTL_SECONDS?: string;
}

/** A registered machine as the web UI sees it. `fingerprint` is derived. */
export interface MachineSummary {
  machine_id: string;
  name: string;
  fingerprint: string;
  online: boolean;
}

/** The stored registry record for a machine (public key kept, token hidden). */
export interface MachineRecord {
  machine_id: string;
  name: string;
  /** Daemon static X25519 public key, standard base64 (as posted by join). */
  public_key: string;
  device_token: string;
  created_at: number;
}

export function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

export function error(status: number, message: string): Response {
  return json({ error: message }, status);
}

/** randomToken returns `n` random bytes as url-safe base64 (no padding). */
export function randomToken(n = 32): string {
  const b = new Uint8Array(n);
  crypto.getRandomValues(b);
  let bin = "";
  for (const byte of b) bin += String.fromCharCode(byte);
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/** timingSafeEqual compares two short ASCII secrets without early-out leaks. */
export function timingSafeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}
