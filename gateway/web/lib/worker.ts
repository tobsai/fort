// worker: the SERVER-ONLY client for the Cloudflare worker broker.
//
// `import "server-only"` makes this module a hard build error if it is ever
// pulled into a client bundle — the guarantee that GATEWAY_SECRET (injected
// here as the X-Gateway-Secret header) never reaches the browser. The browser
// only ever talks to this app's own /api/* routes, which call in here.

import "server-only";

import type { Frame } from "@fort/gateway-shared";

import type { MachineSummary } from "./types";
import { FORT_REQUEST_ID_HEADER } from "./request-id";

interface WorkerConfig {
  url: string;
  secret: string;
}

export class WorkerRelayError extends Error {
  constructor(
    readonly status: number,
    readonly requestID: string,
  ) {
    super(`worker relay failed (${status}; request ${requestID})`);
    this.name = "WorkerRelayError";
  }
}

function config(): WorkerConfig {
  const url = process.env.WORKER_URL;
  const secret = process.env.GATEWAY_SECRET;
  if (!url) throw new Error("WORKER_URL is not set");
  if (!secret) throw new Error("GATEWAY_SECRET is not set");
  return { url: url.replace(/\/+$/, ""), secret };
}

/** headers builds the internal-call headers with the shared secret injected. */
function headers(cfg: WorkerConfig, extra?: Record<string, string>): HeadersInit {
  return { "x-gateway-secret": cfg.secret, ...extra };
}

/** listMachines fetches the registered machines (incl. public_key) from the worker. */
export async function listMachines(): Promise<MachineSummary[]> {
  const cfg = config();
  const res = await fetch(`${cfg.url}/api/relay/machines`, {
    headers: headers(cfg),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`worker machines: ${res.status}`);
  const body = (await res.json()) as { machines: MachineSummary[] };
  return body.machines;
}

/** getMachine returns one machine by id, or null if not registered. */
export async function getMachine(id: string): Promise<MachineSummary | null> {
  const machines = await listMachines();
  return machines.find((m) => m.machine_id === id) ?? null;
}

/** invite mints a single-use join code at the worker. */
export async function invite(): Promise<string> {
  const cfg = config();
  const res = await fetch(`${cfg.url}/api/relay/invite`, {
    method: "POST",
    headers: headers(cfg),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`worker invite: ${res.status}`);
  const body = (await res.json()) as { code: string };
  return body.code;
}

/** relayReq forwards one buffered browser frame to a machine, returning replies. */
export async function relayReq(machineId: string, frame: Frame, requestID: string): Promise<Frame[]> {
  const cfg = config();
  const res = await fetch(`${cfg.url}/api/relay/req`, {
    method: "POST",
    headers: headers(cfg, {
      "content-type": "application/json",
      [FORT_REQUEST_ID_HEADER]: requestID,
    }),
    body: JSON.stringify({ machine_id: machineId, frame }),
    cache: "no-store",
  });
  if (!res.ok) {
    throw new WorkerRelayError(res.status, requestID);
  }
  const body = (await res.json()) as { frames: Frame[] };
  return body.frames;
}

/**
 * relaySse forwards a browser request frame to a machine and returns the
 * worker's streamed (application/x-ndjson) Response so the caller can pipe the
 * body straight back to the browser without buffering.
 */
export async function relaySse(machineId: string, frame: Frame, requestID: string): Promise<Response> {
  const cfg = config();
  return fetch(`${cfg.url}/api/relay/sse`, {
    method: "POST",
    headers: headers(cfg, {
      "content-type": "application/json",
      [FORT_REQUEST_ID_HEADER]: requestID,
    }),
    body: JSON.stringify({ machine_id: machineId, frame }),
    cache: "no-store",
  });
}

/** revoke removes a machine at the worker (drops its socket, invalidates token). */
export async function revoke(machineId: string): Promise<void> {
  const cfg = config();
  const res = await fetch(`${cfg.url}/api/relay/machines/${encodeURIComponent(machineId)}`, {
    method: "DELETE",
    headers: headers(cfg),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`worker revoke: ${res.status}`);
}
