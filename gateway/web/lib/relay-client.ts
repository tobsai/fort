// relay-client: a mini fetch-over-Noise-over-worker client that runs in the
// BROWSER. It is the initiator side of the spec-028 E2E tunnel:
//
//   1. connect()  — run the Noise IK handshake against the daemon's pinned
//                    static key by POSTing an hs1 frame to /api/req and reading
//                    the daemon's hs2 reply, yielding a transport Session.
//   2. fetch()    — seal a `req` (a normal HTTP request) with the session, POST
//                    it to /api/req, and open the daemon's sealed `res`.
//   3. stream()   — seal a `req` for an SSE endpoint, POST it to /api/sse, and
//                    open the daemon's `res` header + each `chunk` line.
//
// The gateway (this app's /api routes + the worker) only ever sees Noise/AEAD
// frames; the request/response plaintext is opaque to it (spec 028, D5).
//
// One RelayClient == one stream id == one daemon-side session. Because the AEAD
// nonce advances per direction, callers MUST NOT interleave concurrent requests
// on the same client: fetch() is internally serialized, and a stream() should
// own its own client. The BoardClient uses a fresh client per operation.

import {
  generateKeypair,
  hs1Frame,
  newInitiator,
  openChunk,
  openRes,
  readHS2,
  sealReq,
} from "@fort/gateway-shared";
import type { ChunkPayload, Frame, ResPayload, Session } from "@fort/gateway-shared";

/** randHex returns 16 random bytes as hex — a stream or request id. */
function randHex(): string {
  const b = new Uint8Array(16);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

export interface FetchOptions {
  method?: string;
  headers?: Record<string, string>;
  body?: Uint8Array;
}

export class RelayClient {
  private session: Session | null = null;
  private readonly streamId = randHex();
  private queue: Promise<unknown> = Promise.resolve();

  constructor(
    private readonly machineId: string,
    private readonly daemonStatic: Uint8Array,
  ) {}

  /** postReq forwards one frame through the buffered proxy, returning replies. */
  private async postReq(frame: Frame): Promise<Frame[]> {
    const res = await fetch("/api/req", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ machine_id: this.machineId, frame }),
    });
    if (!res.ok) {
      const t = await res.text().catch(() => "");
      throw new Error(`relay req failed (${res.status})${t ? ": " + t : ""}`);
    }
    const body = (await res.json()) as { frames: Frame[] };
    return body.frames;
  }

  /** connect runs the Noise IK handshake and stores the transport session. */
  async connect(): Promise<void> {
    const kp = generateKeypair(); // fresh per-session client static key
    const hs = newInitiator(kp, this.daemonStatic);
    const msg1 = hs.writeMessage();
    const frames = await this.postReq(hs1Frame(this.streamId, msg1));
    const hs2 = frames[0];
    if (!hs2) throw new Error("handshake: daemon sent no hs2 (machine offline?)");
    this.session = readHS2(hs, hs2);
  }

  /**
   * fetch seals a request, sends it, and opens the daemon's response. Calls are
   * serialized on this client so the per-session AEAD nonce order is preserved.
   */
  fetch(path: string, opts: FetchOptions = {}): Promise<ResPayload> {
    const run = async (): Promise<ResPayload> => {
      if (!this.session) throw new Error("relay: not connected");
      const rp = {
        id: randHex(),
        method: opts.method ?? "GET",
        path,
        ...(opts.headers ? { headers: opts.headers } : {}),
        ...(opts.body ? { body: opts.body } : {}),
      };
      const frame = sealReq(this.session, this.streamId, rp);
      const frames = await this.postReq(frame);
      const res = frames[0];
      if (!res) throw new Error("relay: no response frame (machine offline?)");
      return openRes(this.session, res);
    };
    const next = this.queue.then(run, run);
    this.queue = next.then(
      () => undefined,
      () => undefined,
    );
    return next;
  }

  /**
   * stream seals a request for an SSE endpoint and invokes onChunk for each
   * decoded chunk until the daemon ends the stream or `signal` aborts. This
   * client must be dedicated to the stream (do not also call fetch() on it).
   */
  async stream(
    path: string,
    onChunk: (chunk: ChunkPayload) => void,
    signal?: AbortSignal,
  ): Promise<void> {
    if (!this.session) throw new Error("relay: not connected");
    const rp = {
      id: randHex(),
      method: "GET",
      path,
      headers: { Accept: "text/event-stream" },
    };
    const frame = sealReq(this.session, this.streamId, rp);
    const res = await fetch("/api/sse", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ machine_id: this.machineId, frame }),
      ...(signal ? { signal } : {}),
    });
    if (!res.ok || !res.body) throw new Error(`relay sse failed (${res.status})`);

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      let nl: number;
      while ((nl = buf.indexOf("\n")) >= 0) {
        const line = buf.slice(0, nl).trim();
        buf = buf.slice(nl + 1);
        if (!line) continue;
        const f = JSON.parse(line) as Frame;
        // The first frame is the sealed `res` header (stream:true); opening it
        // advances the receive nonce in lock-step with the daemon. Chunks follow.
        if (f.kind === "res") {
          openRes(this.session, f);
          continue;
        }
        if (f.kind === "chunk") {
          const chunk = openChunk(this.session, f);
          onChunk(chunk);
          if (chunk.end) return;
        }
      }
    }
  }
}
