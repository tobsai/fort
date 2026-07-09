// POST /api/req — the buffered sealed-frame proxy. The browser posts one
// opaque Noise/AEAD frame (hs1 or req); we forward it to the worker with the
// shared secret and return the daemon's reply frames verbatim. This route
// never touches the sealed `b64` payload — it is a dumb, authenticated relay.

import type { Frame } from "@fort/gateway-shared";

import { requireSession } from "@/lib/session";
import { relayReq } from "@/lib/worker";

export async function POST(request: Request): Promise<Response> {
  const unauth = await requireSession();
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

  try {
    const frames = await relayReq(body.machine_id, body.frame);
    return Response.json({ frames });
  } catch (e) {
    return Response.json({ error: e instanceof Error ? e.message : "worker error" }, { status: 502 });
  }
}
