import { createV2OwnerProxyHandler } from "@/lib/v2-owner-proxy";
import { resolveGatewayOwnerSession } from "@/lib/v2-owner-session";
import { createFortControlServiceClientFromEnvironment } from "@/lib/v2-service-client";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 60;

const ownerProxy = createV2OwnerProxyHandler({
  resolveOwnerSession: resolveGatewayOwnerSession,
  service: createFortControlServiceClientFromEnvironment(),
});

export const GET = ownerProxy;
export const POST = ownerProxy;
export const PATCH = ownerProxy;
