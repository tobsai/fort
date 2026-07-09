// tunnel: one Durable Object per machine_id. It holds that machine's single
// outbound daemon WebSocket (the `/tunnel` socket, authenticated upstream by the
// router) and multiplexes many browser sessions onto it.
//
// The DO is a DUMB relay: it parses only {stream, kind} to route, and forwards
// the opaque sealed `b64` untouched. It never holds Noise keys and never sees
// plaintext — the browser (initiator) and the daemon (responder) own the E2E
// session; this object just moves frames between the two, keyed by stream.
//
// Wiring (internal requests from the router):
//   GET  <upgrade>  -> the daemon attaches its socket (hibernatable).
//   POST /relay     -> buffered: forward one browser frame, return the daemon's
//                      next frame on that stream (hs1->hs2, req->res); end/bye
//                      are forwarded fire-and-forget.
//   POST /sse       -> streaming: forward a browser req, stream back every
//                      daemon frame on that stream as NDJSON until it ends.
//   GET  /status    -> { online }.
//   POST /close      -> drop the daemon socket (revoke / delete).

import type { Frame } from "@fort/gateway-shared";
import { DurableObject } from "cloudflare:workers";

import { Multiplexer, OfflineError, TimeoutError, replyExpected } from "./mux";
import type { Env } from "./types";
import { error, json } from "./types";

const RELAY_TIMEOUT_MS = 10_000;

export class TunnelDO extends DurableObject<Env> {
  // The multiplexer is rebuilt lazily and bound to whatever daemon socket is
  // currently attached; a single instance is shared by fetch() (browser side)
  // and webSocketMessage() (daemon side) within one woken isolate.
  private mux: Multiplexer | null = null;

  async fetch(request: Request): Promise<Response> {
    if (request.headers.get("Upgrade") === "websocket") {
      return this.acceptDaemon();
    }
    const url = new URL(request.url);
    switch (url.pathname) {
      case "/relay":
        return this.relay(request);
      case "/sse":
        return this.sse(request);
      case "/status":
        return json({ online: this.daemonSocket() !== null });
      case "/close":
        return this.close();
      default:
        return error(404, "not found");
    }
  }

  // --- daemon socket lifecycle ---------------------------------------------

  private acceptDaemon(): Response {
    // One daemon per machine: replace any stale socket (a reconnect wins).
    for (const s of this.ctx.getWebSockets("daemon")) s.close(1000, "replaced");
    this.mux?.fail();
    this.mux = null;

    const pair = new WebSocketPair();
    const client = pair[0];
    const server = pair[1];
    this.ctx.acceptWebSocket(server, ["daemon"]);
    return new Response(null, { status: 101, webSocket: client });
  }

  webSocketMessage(_ws: WebSocket, message: string | ArrayBuffer): void {
    let frame: Frame;
    try {
      const text = typeof message === "string" ? message : new TextDecoder().decode(message);
      frame = JSON.parse(text) as Frame;
    } catch {
      return; // ignore malformed daemon frames
    }
    if (typeof frame.stream !== "string" || typeof frame.kind !== "string") return;
    this.muxFor()?.deliver(frame);
  }

  webSocketClose(): void {
    this.mux?.fail();
    this.mux = null;
  }

  webSocketError(): void {
    this.mux?.fail();
    this.mux = null;
  }

  private daemonSocket(): WebSocket | null {
    return this.ctx.getWebSockets("daemon")[0] ?? null;
  }

  /** muxFor returns the multiplexer bound to the live daemon socket, or null. */
  private muxFor(): Multiplexer | null {
    if (this.daemonSocket() === null) {
      this.mux?.fail();
      this.mux = null;
      return null;
    }
    if (this.mux === null) {
      this.mux = new Multiplexer((frame) => {
        const sock = this.daemonSocket();
        if (sock) sock.send(JSON.stringify(frame));
      });
    }
    return this.mux;
  }

  // --- browser side ---------------------------------------------------------

  private async relay(request: Request): Promise<Response> {
    const { frame } = (await request.json()) as { frame: Frame };
    const mux = this.muxFor();
    if (!mux) return error(503, "machine offline");
    try {
      const reply = await mux.relay(frame, {
        expectReply: replyExpected(frame.kind),
        timeoutMs: RELAY_TIMEOUT_MS,
      });
      return json({ frames: reply ? [reply] : [] });
    } catch (e) {
      if (e instanceof OfflineError) return error(503, "machine offline");
      if (e instanceof TimeoutError) return error(504, "daemon did not respond");
      throw e;
    }
  }

  private async sse(request: Request): Promise<Response> {
    const { frame } = (await request.json()) as { frame: Frame };
    const mux = this.muxFor();
    if (!mux) return error(503, "machine offline");

    const stream = frame.stream;
    const encoder = new TextEncoder();
    let controller: ReadableStreamDefaultController<Uint8Array> | null = null;
    const body = new ReadableStream<Uint8Array>({
      start: (c) => {
        controller = c;
      },
      cancel: () => {
        mux.unsubscribe(stream);
      },
    });

    mux.subscribe(stream, {
      push: (f) => controller?.enqueue(encoder.encode(JSON.stringify(f) + "\n")),
      end: () => {
        try {
          controller?.close();
        } catch {
          // already closed
        }
      },
    });
    // Fire the browser's request frame; replies flow back over the subscription.
    void mux.relay(frame, { expectReply: false, timeoutMs: RELAY_TIMEOUT_MS }).catch(() => {});

    return new Response(body, {
      headers: { "content-type": "application/x-ndjson", "cache-control": "no-store" },
    });
  }

  private close(): Response {
    for (const s of this.ctx.getWebSockets("daemon")) s.close(1000, "revoked");
    this.mux?.fail();
    this.mux = null;
    return json({ closed: true });
  }
}
