// POST /api/sse — the streaming sealed-frame proxy. The browser posts a sealed
// `req` frame whose plaintext asks for an SSE endpoint (Accept: text/event-
// stream). We forward it to the worker and stream the worker's NDJSON body
// (one opaque Frame per line) straight back to the browser, unbuffered. The
// payload stays sealed end to end.

import type { Frame } from "@fort/gateway-shared";

import { requireSession } from "@/lib/session";
import { relaySse } from "@/lib/worker";
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

  let upstream: Response;
  try {
    upstream = await relaySse(body.machine_id, body.frame, requestID);
  } catch {
    return correlatedJSON({ error: "relay stream failed", request_id: requestID }, 502, requestID);
  }
  if (!upstream.ok || !upstream.body) {
    return correlatedJSON({ error: "relay stream failed", request_id: requestID }, 502, requestID);
  }

  // Pipe the worker's NDJSON stream straight through.
  return new Response(upstream.body, {
    headers: {
      "content-type": "application/x-ndjson",
      "cache-control": "no-store",
      [FORT_REQUEST_ID_HEADER]: requestID,
    },
  });
}
