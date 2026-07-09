// Shared helpers for the miniflare-backed tests: thin wrappers over SELF.fetch
// against the router Worker, plus a valid X25519 public key for joins.

import { SELF } from "cloudflare:test";

import { encodeBase64, generateKeypair, type KeyPair } from "@fort/gateway-shared";

export const BASE = "https://gateway.test";
export const SECRET = "test-secret"; // matches vitest.config.ts miniflare binding

export function newDaemonKey(): KeyPair {
  return generateKeypair();
}

export function pubB64(kp: KeyPair): string {
  return encodeBase64(kp.publicKey);
}

export function invite(secret: string | null = SECRET): Promise<Response> {
  const headers: Record<string, string> = {};
  if (secret !== null) headers["x-gateway-secret"] = secret;
  return SELF.fetch(BASE + "/api/relay/invite", { method: "POST", headers });
}

export function join(code: string, name: string, publicKey: string): Promise<Response> {
  return SELF.fetch(BASE + "/api/relay/join", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ code, name, public_key: publicKey }),
  });
}

export function listMachines(secret: string | null = SECRET): Promise<Response> {
  const headers: Record<string, string> = {};
  if (secret !== null) headers["x-gateway-secret"] = secret;
  return SELF.fetch(BASE + "/api/relay/machines", { headers });
}

/** mintAndJoin runs the full happy path and returns the new machine's creds. */
export async function mintAndJoin(
  name: string,
  kp: KeyPair,
): Promise<{ device_token: string; machine_id: string }> {
  const inv = await invite();
  const { code } = (await inv.json()) as { code: string };
  const res = await join(code, name, pubB64(kp));
  return (await res.json()) as { device_token: string; machine_id: string };
}
