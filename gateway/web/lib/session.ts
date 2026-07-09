// session: a tiny guard for the internal /api/* routes. Middleware already
// redirects unauthenticated PAGE requests, but the API routes are called with
// fetch() and must fail closed with a 401 (not a redirect) when there is no
// session. Every internal route calls requireSession() first.

import { auth } from "@/auth";

/**
 * requireSession returns null when the caller has a valid session, or a 401
 * Response to return immediately when they do not.
 */
export async function requireSession(): Promise<Response | null> {
  const session = await auth();
  if (!session?.user) {
    return new Response(JSON.stringify({ error: "unauthorized" }), {
      status: 401,
      headers: { "content-type": "application/json" },
    });
  }
  return null;
}
