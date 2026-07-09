// POST /api/invite — mint a single-use join code and return the paste-ready
// `fort relay join` line. Authenticated; the worker call injects the secret.

import { requireSession } from "@/lib/session";
import { invite } from "@/lib/worker";

export async function POST(): Promise<Response> {
  const unauth = await requireSession();
  if (unauth) return unauth;

  try {
    const code = await invite();
    const gatewayUrl = process.env.NEXT_PUBLIC_GATEWAY_URL ?? "";
    const command = `fort relay join ${gatewayUrl} --code ${code}`.trim();
    return Response.json({ code, command });
  } catch (e) {
    return Response.json({ error: e instanceof Error ? e.message : "worker error" }, { status: 502 });
  }
}
