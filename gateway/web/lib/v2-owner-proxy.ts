import "server-only";

import type { OwnerSession } from "@/lib/v2-events";
import {
  FortControlResponseError,
  type FortControlServiceClient,
} from "@/lib/v2-service-client";

const MAX_FUNCTION_BODY_BYTES = 4 * 1024 * 1024;

export type ResolveOwnerSession = (request: Request) => Promise<OwnerSession | null>;

interface OwnerProxyOptions {
  resolveOwnerSession: ResolveOwnerSession;
  service: FortControlServiceClient;
}

interface ControlRoute {
  path: string;
  routeClass: string;
  method: "GET" | "POST" | "PATCH";
}

export function createV2OwnerProxyHandler(options: OwnerProxyOptions) {
  return async function ownerProxy(request: Request): Promise<Response> {
    let owner: OwnerSession | null;
    try {
      owner = await options.resolveOwnerSession(request);
    } catch {
      owner = null;
    }
    if (!owner) return jsonError(401, "unauthorized");

    const route = controlRoute(request);
    if (!route) return jsonError(404, "not_found");

    let body: string | undefined;
    if (route.method !== "GET") {
      const contentType = request.headers.get("content-type")?.toLowerCase() ?? "";
      if (!/^application\/json(?:\s*;|$)/.test(contentType)) return jsonError(415, "unsupported_media_type");
      try {
        body = await readBoundedRequestBody(request);
      } catch {
        return jsonError(413, "payload_too_large");
      }
    }

    try {
      const payload = await options.service.request({
        owner,
        path: route.path,
        routeClass: route.routeClass,
        method: route.method,
        ...(body === undefined ? {} : { body }),
      });
      const encoded = JSON.stringify(payload);
      if (new TextEncoder().encode(encoded).byteLength > MAX_FUNCTION_BODY_BYTES) {
        return jsonError(502, "control_unavailable");
      }
      return new Response(encoded, { status: 200, headers: responseHeaders() });
    } catch (error) {
      if (error instanceof FortControlResponseError) return jsonError(error.status, error.code);
      return jsonError(502, "control_unavailable");
    }
  };
}

function controlRoute(request: Request): ControlRoute | null {
  if (request.method !== "GET" && request.method !== "POST" && request.method !== "PATCH") return null;
  let url: URL;
  try { url = new URL(request.url); } catch { return null; }
  const raw = url.pathname.split("/");
  if (raw[0] !== "" || raw[1] !== "api" || raw[2] !== "v2" || raw.length < 4) return null;
  const parts: string[] = [];
  for (const component of raw.slice(3)) {
    const decoded = decodeIdentity(component);
    if (decoded === null) return null;
    parts.push(decoded);
  }

  const query = listQuery(url);
  if (request.method === "GET") {
    if (parts.length === 1 && parts[0] === "agents" && query !== null) {
      return { path: `/api/v2/agents${query}`, routeClass: "owner.agents.list", method: "GET" };
    }
    if (parts.length === 2 && parts[0] === "agents" && noQuery(url)) {
      return readRoute(parts, "owner.agents.read");
    }
    if (parts.length === 3 && parts[0] === "agents" && parts[2] === "conversations" && noQuery(url)) {
      return readRoute(parts, "owner.agent_conversations.list");
    }
    if (parts.length === 3 && parts[0] === "agents" && parts[2] === "routines" && noQuery(url)) {
      return readRoute(parts, "owner.routines.list");
    }
    if (parts.length === 5 && parts[0] === "agents" && parts[2] === "routines" &&
        parts[4] === "runs" && noQuery(url)) {
      return readRoute(parts, "owner.routines.runs");
    }
    if (parts.length === 4 && parts[0] === "agents" && parts[2] === "conversations" && noQuery(url)) {
      const routeClass = parts[3] === "canonical"
        ? "owner.agent_conversations.canonical"
        : "owner.agent_conversations.read";
      return readRoute(parts, routeClass);
    }
    if (parts.length === 1 && parts[0] === "groups" && query !== null) {
      return { path: `/api/v2/groups${query}`, routeClass: "owner.groups.list", method: "GET" };
    }
    if (parts.length === 2 && parts[0] === "groups" && noQuery(url)) {
      return readRoute(parts, "owner.groups.read");
    }
    if (parts.length === 1 && parts[0] === "handoffs" && noQuery(url)) {
      return readRoute(parts, "owner.handoffs.list");
    }
    if (parts.length === 2 && parts[0] === "handoffs" && noQuery(url)) {
      return readRoute(parts, "owner.handoffs.read");
    }
    return null;
  }

  if (!noQuery(url)) return null;
  if (request.method === "PATCH") {
    if (parts.length === 2 && parts[0] === "agents") {
      return mutationRoute(parts, "owner.agents.mutate");
    }
    if (parts.length === 4 && parts[0] === "agents" && parts[2] === "conversations" && parts[3] !== "canonical") {
      return mutationRoute(parts, "owner.agent_conversations.mutate");
    }
    if (parts.length === 4 && parts[0] === "agents" && parts[2] === "routines") {
      return mutationRoute(parts, "owner.routines.mutate");
    }
    if (parts.length === 2 && parts[0] === "groups") {
      return mutationRoute(parts, "owner.groups.mutate");
    }
    return null;
  }
  if (parts.length === 1 && parts[0] === "agents") {
    return commandRoute(parts, "owner.agents.create");
  }
  if (parts.length === 3 && parts[0] === "agents" && parts[2] === "rebind") {
    return commandRoute(parts, "owner.agents.rebind");
  }
  if (parts.length === 3 && parts[0] === "agents" && parts[2] === "conversations") {
    return commandRoute(parts, "owner.agent_conversations.create");
  }
  if (parts.length === 3 && parts[0] === "agents" && parts[2] === "routines") {
    return commandRoute(parts, "owner.routines.create");
  }
  if (parts.length === 5 && parts[0] === "agents" && parts[2] === "routines" && parts[4] === "test") {
    return commandRoute(parts, "owner.routines.test");
  }
  if (parts.length === 1 && parts[0] === "groups") {
    return commandRoute(parts, "owner.groups.create");
  }
  if (parts.length === 3 && parts[0] === "groups" && parts[2] === "members") {
    return commandRoute(parts, "owner.group_members.replace");
  }
  if (parts.length === 3 && parts[0] === "groups" && parts[2] === "turns") {
    return commandRoute(parts, "owner.group_turns.send");
  }
  if (parts.length === 1 && parts[0] === "handoffs") {
    return commandRoute(parts, "owner.handoffs.create");
  }
  if (parts.length === 3 && parts[0] === "handoffs" && parts[2] === "cancel") {
    return commandRoute(parts, "owner.handoffs.cancel");
  }
  if (parts.length === 5 && parts[0] === "agents" && parts[2] === "conversations" && parts[4] === "turns") {
    return commandRoute(parts, "owner.agent_turns.send");
  }
  if (parts.length === 7 && parts[0] === "agents" && parts[2] === "conversations" && parts[4] === "targets") {
    if (parts[6] === "retry") return commandRoute(parts, "owner.agent_targets.retry");
    if (parts[6] === "cancel") return commandRoute(parts, "owner.agent_targets.cancel");
  }
  return null;
}

function readRoute(parts: string[], routeClass: string): ControlRoute {
  return { path: canonicalPath(parts), routeClass, method: "GET" };
}

function commandRoute(parts: string[], routeClass: string): ControlRoute {
  return { path: canonicalPath(parts), routeClass, method: "POST" };
}

function mutationRoute(parts: string[], routeClass: string): ControlRoute {
  return { path: canonicalPath(parts), routeClass, method: "PATCH" };
}

function canonicalPath(parts: string[]): string {
  return `/api/v2/${parts.map(encodeURIComponent).join("/")}`;
}

function decodeIdentity(raw: string): string | null {
  if (!raw || raw.length > 1_536) return null;
  let value: string;
  try { value = decodeURIComponent(raw); } catch { return null; }
  if (!value || value === "." || value === ".." || new TextEncoder().encode(value).byteLength > 512 || /[\/\\\r\n\0]/.test(value)) return null;
  return value;
}

function listQuery(url: URL): string | null {
  if (url.search === "") return "";
  return url.search === "?state=open" ? "?state=open" : null;
}

function noQuery(url: URL): boolean { return url.search === ""; }

async function readBoundedRequestBody(request: Request): Promise<string> {
  const declared = request.headers.get("content-length");
  if (declared !== null && (!/^\d+$/.test(declared) || Number(declared) > MAX_FUNCTION_BODY_BYTES)) {
    throw new Error("payload limit");
  }
  if (!request.body) throw new Error("body required");
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_FUNCTION_BODY_BYTES) {
      await reader.cancel();
      throw new Error("payload limit");
    }
    chunks.push(value);
  }
  const combined = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    combined.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(combined);
}

function jsonError(status: number, code: string): Response {
  return new Response(JSON.stringify({ code }), { status, headers: responseHeaders() });
}

function responseHeaders(): HeadersInit {
  return { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" };
}
