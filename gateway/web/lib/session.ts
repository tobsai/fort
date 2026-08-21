// session: a tiny guard for the internal /api/* routes. Middleware already
// redirects unauthenticated PAGE requests, but the API routes are called with
// fetch() and must fail closed with a 401 (not a redirect) when there is no
// session. Every internal route calls requireSession() first.

import { auth } from "@/auth";
import { isAllowed } from "@/lib/allowlist";
import { verifyNativeToken } from "@/lib/native-token";

export async function nativeSessionIdentity(request: Request): Promise<{ email: string } | null> {
  const authorization = request.headers.get("authorization") ?? "";
  if (!authorization.startsWith("Bearer ")) return null;
  try {
    const identity = await verifyNativeToken(
      authorization.slice("Bearer ".length),
      process.env.AUTH_SECRET ?? "",
    );
    return isAllowed(identity.email, process.env.FORT_ALLOWLIST) ? identity : null;
  } catch {
    return null;
  }
}

/**
 * requireSession returns null when the caller has a valid session, or a 401
 * Response to return immediately when they do not.
 */
export async function requireSession(request?: Request): Promise<Response | null> {
  const authorization = request?.headers.get("authorization") ?? "";
  if (authorization.startsWith("Bearer ")) {
    return (await nativeSessionIdentity(request!)) ? null : unauthorized();
  }
  const session = await auth();
  if (!session?.user) {
    return unauthorized();
  }
  return null;
}

function unauthorized(): Response {
  return new Response(JSON.stringify({ error: "unauthorized" }), {
    status: 401,
    headers: { "content-type": "application/json" },
  });
}
