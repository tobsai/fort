export const MAX_CURSOR_PAGE_BYTES = 1024 * 1024;

export interface OwnerSession {
  normalizedEmail: string;
}

export interface DurableEvent {
  cursor: string;
  kind: string;
  data: unknown;
}

export interface CursorPage {
  events: DurableEvent[];
  nextCursor: string;
}

export interface CursorPageClient {
  readPage(input: {
    owner: OwnerSession;
    afterCursor: string;
    signal: AbortSignal;
  }): Promise<CursorPage>;
}

interface EventsHandlerDependencies {
  resolveOwnerSession(request: Request): Promise<OwnerSession | null>;
  cursorPages: CursorPageClient;
  cutoffMs: number;
  now?: () => number;
}

type ReconnectReason = "cutoff" | "control_unavailable" | "idle" | "invalid_page" | "page_too_large";

const encoder = new TextEncoder();

export function createV2EventsHandler(dependencies: EventsHandlerDependencies) {
  if (!Number.isFinite(dependencies.cutoffMs) || dependencies.cutoffMs <= 0) {
    throw new Error("cutoffMs must be positive");
  }
  const now = dependencies.now ?? Date.now;

  return async function GET(request: Request): Promise<Response> {
    const owner = await dependencies.resolveOwnerSession(request);
    if (!owner) {
      return Response.json(
        { code: "unauthorized" },
        { status: 401, headers: { "cache-control": "no-store" } },
      );
    }

    const initialCursor = reconnectCursor(request);
    if (!validCursor(initialCursor)) {
      return Response.json(
        { code: "invalid_cursor" },
        { status: 400, headers: { "cache-control": "no-store" } },
      );
    }

    const deadline = now() + dependencies.cutoffMs;
    let cancelled = false;
    let cutoffReached = false;
    const pollAbort = new AbortController();
    const cutoff = setTimeout(() => {
      cutoffReached = true;
      pollAbort.abort();
    }, dependencies.cutoffMs);
    const abortForDisconnect = () => {
      cancelled = true;
      pollAbort.abort();
    };
    request.signal.addEventListener("abort", abortForDisconnect, { once: true });

    const stream = new ReadableStream<Uint8Array>({
      async start(controller) {
        let cursor = initialCursor;
        let reconnectReason: ReconnectReason = "cutoff";
        const seenEventCursors = new Set([initialCursor]);

        controller.enqueue(encoder.encode(": fort-v2-events\n\n"));
        try {
          while (!cancelled && now() < deadline) {
            let page: CursorPage;
            try {
              page = await readPageBeforeAbort(dependencies.cursorPages, {
                owner,
                afterCursor: cursor,
                signal: pollAbort.signal,
              });
            } catch {
              if (cancelled) return;
              reconnectReason = cutoffReached || now() >= deadline ? "cutoff" : "control_unavailable";
              break;
            }

            const pageJSON = stringify(page);
            if (pageJSON === null || !validPage(page)) {
              reconnectReason = "invalid_page";
              break;
            }
            if (encoder.encode(pageJSON).byteLength > MAX_CURSOR_PAGE_BYTES) {
              reconnectReason = "page_too_large";
              break;
            }
            if (!validProgression(page, seenEventCursors)) {
              reconnectReason = "invalid_page";
              break;
            }
            if (page.events.length === 0 && page.nextCursor === cursor) {
              reconnectReason = "idle";
              break;
            }

            for (const event of page.events) {
              seenEventCursors.add(event.cursor);
              controller.enqueue(encoder.encode(eventFrame(event)));
            }
            cursor = page.nextCursor;
          }

          if (!cancelled) {
            controller.enqueue(encoder.encode(reconnectFrame(cursor, reconnectReason)));
          }
        } finally {
          clearTimeout(cutoff);
          request.signal.removeEventListener("abort", abortForDisconnect);
          if (!cancelled) controller.close();
        }
      },
      cancel() {
        cancelled = true;
        pollAbort.abort();
        clearTimeout(cutoff);
        request.signal.removeEventListener("abort", abortForDisconnect);
      },
    });

    return new Response(stream, {
      headers: {
        "cache-control": "no-cache, no-store",
        connection: "keep-alive",
        "content-type": "text/event-stream; charset=utf-8",
        "x-accel-buffering": "no",
      },
    });
  };
}

function reconnectCursor(request: Request): string {
  const headerCursor = request.headers.get("last-event-id");
  if (headerCursor !== null && headerCursor !== "") return headerCursor;
  return new URL(request.url).searchParams.get("cursor") ?? "cursor-0";
}

function validPage(page: CursorPage): boolean {
  if (!page || !Array.isArray(page.events) || !validCursor(page.nextCursor)) return false;
  return page.events.every(
    (event) =>
      event !== null &&
      typeof event === "object" &&
      validCursor(event.cursor) &&
      typeof event.kind === "string" &&
      event.kind.length > 0,
  );
}

function validCursor(cursor: string): boolean {
  return cursor.length > 0 && cursor.length <= 1024 && !/[\r\n\0]/.test(cursor);
}

function validProgression(page: CursorPage, seenEventCursors: ReadonlySet<string>): boolean {
  if (page.events.length === 0) return true;
  if (page.events[page.events.length - 1]?.cursor !== page.nextCursor) return false;

  const pageCursors = new Set<string>();
  for (const event of page.events) {
    if (seenEventCursors.has(event.cursor) || pageCursors.has(event.cursor)) return false;
    pageCursors.add(event.cursor);
  }
  return true;
}

function stringify(value: unknown): string | null {
  try {
    return JSON.stringify(value) ?? null;
  } catch {
    return null;
  }
}

function eventFrame(event: DurableEvent): string {
  return `id: ${event.cursor}\nevent: fort.event\ndata: ${JSON.stringify({ kind: event.kind, data: event.data })}\n\n`;
}

function reconnectFrame(cursor: string, reason: ReconnectReason): string {
  return `id: ${cursor}\nevent: fort.reconnect\ndata: ${JSON.stringify({ cursor, reason })}\n\n`;
}

function readPageBeforeAbort(
  client: CursorPageClient,
  input: Parameters<CursorPageClient["readPage"]>[0],
): Promise<CursorPage> {
  return new Promise((resolve, reject) => {
    const aborted = () => {
      input.signal.removeEventListener("abort", aborted);
      reject(new DOMException("cursor read aborted", "AbortError"));
    };
    if (input.signal.aborted) {
      aborted();
      return;
    }
    input.signal.addEventListener("abort", aborted, { once: true });

    void Promise.resolve()
      .then(() => client.readPage(input))
      .then(
        (page) => {
          input.signal.removeEventListener("abort", aborted);
          resolve(page);
        },
        (error: unknown) => {
          input.signal.removeEventListener("abort", aborted);
          reject(error);
        },
      );
  });
}
