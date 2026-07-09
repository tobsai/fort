// GET /api/machines — the authenticated machines list (incl. public_key for
// browser pinning). Thin proxy: the worker call injects X-Gateway-Secret.

import { requireSession } from "@/lib/session";
import { listMachines } from "@/lib/worker";

export async function GET(): Promise<Response> {
  const unauth = await requireSession();
  if (unauth) return unauth;

  try {
    const machines = await listMachines();
    return Response.json({ machines });
  } catch (e) {
    return Response.json({ error: e instanceof Error ? e.message : "worker error" }, { status: 502 });
  }
}
