// POST /api/req — the buffered sealed-frame proxy. The browser posts one
// opaque Noise/AEAD frame (hs1 or req); we forward it to the worker with the
// shared secret and return the daemon's reply frames verbatim. This route
// never touches the sealed `b64` payload — it is a dumb, authenticated relay.

import type { Frame } from "@fort/gateway-shared";

import { requireSession } from "@/lib/session";
import { relayReq } from "@/lib/worker";
import { correlatedJSON, FORT_REQUEST_ID_HEADER, requestIDFrom } from "@/lib/request-id";

export async function POST(request: Request): Promise<Response> {
  const requestID = requestIDFrom(request);
  const unauth = await requireSession(request);
  if (unauth) {
    unauth.headers.set(FORT_REQUEST_ID_HEADER, requestID);
    return unauth;
  }

  let body: { machine_id?: string; frame?: Frame };
  try {
    body = (await request.json()) as typeof body;
  } catch {
    return correlatedJSON({ error: "invalid json", request_id: requestID }, 400, requestID);
  }
  if (!body.machine_id || !body.frame) {
    return correlatedJSON({ error: "machine_id and frame required", request_id: requestID }, 400, requestID);
  }

  try {
    const frames = await relayReq(body.machine_id, body.frame, requestID);
    return correlatedJSON({ frames }, 200, requestID);
  } catch (e) {
    const upstreamStatus =
      typeof e === "object" && e !== null && "status" in e && typeof e.status === "number"
        ? e.status
        : 502;
    const status = upstreamStatus === 503 || upstreamStatus === 504 ? upstreamStatus : 502;
    console.error("relay proxy failed", { request_id: requestID, status });
    return correlatedJSON({ error: "relay request failed", request_id: requestID }, status, requestID);
  }
}
