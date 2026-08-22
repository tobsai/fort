import "server-only";

import { auth } from "@/auth";
import { isAllowed } from "@/lib/allowlist";
import { nativeSessionIdentity } from "@/lib/session";
import type { OwnerSession } from "@/lib/v2-events";

export async function resolveGatewayOwnerSession(request: Request): Promise<OwnerSession | null> {
  const authorization = request.headers.get("authorization") ?? "";
  const identity = authorization.startsWith("Bearer ")
    ? await nativeSessionIdentity(request)
    : (await auth())?.user;
  const email = identity?.email?.trim().toLowerCase();

  if (!email || !isAllowed(email, process.env.FORT_ALLOWLIST)) return null;
  return { normalizedEmail: email };
}
