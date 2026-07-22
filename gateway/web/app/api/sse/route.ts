// POST /api/sse — the streaming sealed-frame proxy. The browser posts a sealed
// `req` frame whose plaintext asks for an SSE endpoint (Accept: text/event-
// stream). We forward it to the worker and stream the worker's NDJSON body
// (one opaque Frame per line) straight back to the browser, unbuffered. The
// payload stays sealed end to end.

import type { Frame } from "@fort/gateway-shared";

import { requireSession } from "@/lib/session";
import { relaySse } from "@/lib/worker";

export async function POST(request: Request): Promise<Response> {
  const unauth = await requireSession(request);
  if (unauth) return unauth;

  let body: { machine_id?: string; frame?: Frame };
  try {
    body = (await request.json()) as typeof body;
  } catch {
    return Response.json({ error: "invalid json" }, { status: 400 });
  }
  if (!body.machine_id || !body.frame) {
    return Response.json({ error: "machine_id and frame required" }, { status: 400 });
  }

  let upstream: Response;
  try {
    upstream = await relaySse(body.machine_id, body.frame);
  } catch (e) {
    return Response.json({ error: e instanceof Error ? e.message : "worker error" }, { status: 502 });
  }
  if (!upstream.ok || !upstream.body) {
    const text = await upstream.text().catch(() => "");
    return Response.json({ error: text || `worker sse: ${upstream.status}` }, { status: 502 });
  }

  // Pipe the worker's NDJSON stream straight through.
  return new Response(upstream.body, {
    headers: {
      "content-type": "application/x-ndjson",
      "cache-control": "no-store",
    },
  });
}
