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
// own its own client. The BoardClient uses a fresh client per operation and
// closes it with a `bye` frame in `finally`.

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

import { FORT_REQUEST_ID_HEADER, newRequestID } from "./request-id";

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

type RelayOperation = "handshake" | "request" | "stream";

export class RelayRequestError extends Error {
  constructor(
    readonly requestID: string,
    readonly status: number | null,
    operation: RelayOperation,
  ) {
    super(`relay ${operation} failed (${status === null ? "network" : status}; request ${requestID})`);
    this.name = "RelayRequestError";
  }
}

function requestHeaders(source: Record<string, string> | undefined, requestID: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [name, value] of Object.entries(source ?? {})) {
    if (name.toLowerCase() !== FORT_REQUEST_ID_HEADER.toLowerCase()) result[name] = value;
  }
  result[FORT_REQUEST_ID_HEADER] = requestID;
  return result;
}

function isTransientHandshakeFailure(cause: unknown): boolean {
  return (
    cause instanceof RelayRequestError &&
    (cause.status === null || cause.status === 502 || cause.status === 503 || cause.status === 504)
  );
}

export class RelayClient {
  private session: Session | null = null;
  private streamId = "";
  private readonly startedStreams = new Set<string>();
  private queue: Promise<unknown> = Promise.resolve();
  private closed = false;

  constructor(
    private readonly machineId: string,
    private readonly daemonStatic: Uint8Array,
  ) {}

  /** postReq forwards one frame through the buffered proxy, returning replies. */
  private async postReq(frame: Frame, requestID: string): Promise<Frame[]> {
    const operation: RelayOperation = frame.kind === "hs1" ? "handshake" : "request";
    let res: Response;
    try {
      res = await fetch("/api/req", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          [FORT_REQUEST_ID_HEADER]: requestID,
        },
        body: JSON.stringify({ machine_id: this.machineId, frame }),
      });
    } catch {
      throw new RelayRequestError(requestID, null, operation);
    }
    if (!res.ok) {
      throw new RelayRequestError(requestID, res.status, operation);
    }
    try {
      const body = (await res.json()) as { frames: Frame[] };
      if (!Array.isArray(body.frames)) throw new Error("invalid relay response");
      return body.frames;
    } catch {
      throw new RelayRequestError(requestID, 502, operation);
    }
  }

  /** connect runs the Noise IK handshake, retrying only that handshake once. */
  async connect(): Promise<void> {
    if (this.closed) throw new Error("relay: client is closed");
    const requestID = newRequestID();
    try {
      await this.connectOnce(requestID);
    } catch (cause) {
      if (!isTransientHandshakeFailure(cause)) throw cause;
      await this.connectOnce(requestID);
    }
  }

  private async connectOnce(requestID: string): Promise<void> {
    this.session = null;
    this.streamId = randHex();
    this.startedStreams.add(this.streamId);
    const kp = generateKeypair(); // fresh per-session client static key
    const hs = newInitiator(kp, this.daemonStatic);
    const msg1 = hs.writeMessage();
    const frames = await this.postReq(hs1Frame(this.streamId, msg1), requestID);
    const hs2 = frames[0];
    if (!hs2) throw new RelayRequestError(requestID, 502, "handshake");
    try {
      this.session = readHS2(hs, hs2);
    } catch {
      throw new RelayRequestError(requestID, 400, "handshake");
    }
  }

  /**
   * fetch seals a request, sends it, and opens the daemon's response. Calls are
   * serialized on this client so the per-session AEAD nonce order is preserved.
   */
  fetch(path: string, opts: FetchOptions = {}): Promise<ResPayload> {
    const requestID = newRequestID();
    const run = async (): Promise<ResPayload> => {
      if (!this.session) throw new RelayRequestError(requestID, null, "request");
      const rp = {
        id: requestID,
        method: opts.method ?? "GET",
        path,
        headers: requestHeaders(opts.headers, requestID),
        ...(opts.body ? { body: opts.body } : {}),
      };
      let frame: Frame;
      try {
        frame = sealReq(this.session, this.streamId, rp);
      } catch {
        throw new RelayRequestError(requestID, 502, "request");
      }
      const frames = await this.postReq(frame, requestID);
      const res = frames[0];
      if (!res) throw new RelayRequestError(requestID, 502, "request");
      let opened: ResPayload;
      try {
        opened = openRes(this.session, res);
      } catch {
        throw new RelayRequestError(requestID, 502, "request");
      }
      if (opened.id !== requestID) throw new RelayRequestError(requestID, 502, "request");
      return opened;
    };
    const next = this.queue.then(run, run);
    this.queue = next.then(
      () => undefined,
      () => undefined,
    );
    return next;
  }

  /** Close this stream and release its daemon-side Noise session. */
  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    await this.queue.catch(() => undefined);
    this.session = null;
    const streams = [...this.startedStreams];
    this.startedStreams.clear();
    for (const stream of streams) {
      await this.postReq({ stream, kind: "bye" }, newRequestID()).catch(() => undefined);
    }
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
    const requestID = newRequestID();
    if (!this.session) throw new RelayRequestError(requestID, null, "stream");
    const session = this.session;
    const rp = {
      id: requestID,
      method: "GET",
      path,
      headers: requestHeaders({ Accept: "text/event-stream" }, requestID),
    };
    let frame: Frame;
    try {
      frame = sealReq(session, this.streamId, rp);
    } catch {
      throw new RelayRequestError(requestID, 502, "stream");
    }
    let res: Response;
    try {
      res = await fetch("/api/sse", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          [FORT_REQUEST_ID_HEADER]: requestID,
        },
        body: JSON.stringify({ machine_id: this.machineId, frame }),
        ...(signal ? { signal } : {}),
      });
    } catch {
      throw new RelayRequestError(requestID, null, "stream");
    }
    if (!res.ok || !res.body) throw new RelayRequestError(requestID, res.status, "stream");

    try {
      await this.consumeStream(res.body, session, requestID, onChunk);
    } catch (cause) {
      if (cause instanceof RelayRequestError) throw cause;
      throw new RelayRequestError(requestID, 502, "stream");
    }
  }

  private async consumeStream(
    body: ReadableStream<Uint8Array>,
    session: Session,
    requestID: string,
    onChunk: (chunk: ChunkPayload) => void,
  ): Promise<void> {
    const reader = body.getReader();
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
          const opened = openRes(session, f);
          if (opened.id !== requestID) throw new RelayRequestError(requestID, 502, "stream");
          continue;
        }
        if (f.kind === "chunk") {
          const chunk = openChunk(session, f);
          if (chunk.id !== requestID) throw new RelayRequestError(requestID, 502, "stream");
          onChunk(chunk);
          if (chunk.end) return;
        }
      }
    }
  }
}
