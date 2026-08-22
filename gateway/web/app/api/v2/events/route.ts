import { createSignedFortControlCursorClientFromEnvironment } from "@/lib/v2-control-client";
import { createV2EventsHandler } from "@/lib/v2-events";
import { resolveGatewayOwnerSession } from "@/lib/v2-owner-session";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 300;

const controlCursorPages = createSignedFortControlCursorClientFromEnvironment();

export const GET = createV2EventsHandler({
  resolveOwnerSession: resolveGatewayOwnerSession,
  cursorPages: controlCursorPages,
  cutoffMs: configuredCutoff(process.env.FORT_V2_SSE_CUTOFF_MS),
});

function configuredCutoff(raw: string | undefined): number {
  const milliseconds = Number(raw);
  if (!Number.isSafeInteger(milliseconds) || milliseconds < 1_000 || milliseconds >= maxDuration * 1_000) {
    return 285_000;
  }
  return milliseconds;
}
