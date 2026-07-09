// DELETE /api/machines/:id — revoke (remove) a machine. Authenticated; the
// worker call injects X-Gateway-Secret so the browser never holds it.

import { requireSession } from "@/lib/session";
import { revoke } from "@/lib/worker";

export async function DELETE(
  _request: Request,
  { params }: { params: { id: string } },
): Promise<Response> {
  const unauth = await requireSession();
  if (unauth) return unauth;

  try {
    await revoke(params.id);
    return Response.json({ revoked: true });
  } catch (e) {
    return Response.json({ error: e instanceof Error ? e.message : "worker error" }, { status: 502 });
  }
}
