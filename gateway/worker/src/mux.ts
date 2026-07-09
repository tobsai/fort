// mux: the pure stream-multiplexing logic at the centre of the tunnel broker.
//
// A machine's TunnelDO holds ONE daemon WebSocket. Many browser sessions ride
// it at once, each identified by a `stream` id the browser chose. The daemon
// answers on the same stream. This class routes frames between the two sides
// WITHOUT ever looking inside a frame's sealed `b64` — it keys purely on
// (stream, kind). Extracted from the Durable Object so it can be unit-tested
// with a fake daemon sink, no workerd runtime required.
//
// Two client-facing shapes:
//   relay(frame)      — buffered: forward one outbound frame, resolve with the
//                       daemon's next frame on that stream (hs1->hs2, req->res).
//   subscribe(stream) — streaming (SSE): every daemon frame on that stream is
//                       pushed to the sink until the stream is closed.
//
// `deliver(frame)` is fed every frame that arrives from the daemon socket.
// `fail()` is called when the daemon socket drops: it rejects in-flight relays
// and ends every subscription so no browser session hangs forever.

import type { Frame } from "@fort/gateway-shared";

/** A subscriber receives every daemon frame for its stream, then one `end`. */
export interface Subscriber {
  push(frame: Frame): void;
  end(): void;
}

interface Waiter {
  resolve(frame: Frame): void;
  reject(err: Error): void;
  timer: ReturnType<typeof setTimeout> | null;
}

/** Thrown by relay() when the daemon socket is not connected. */
export class OfflineError extends Error {
  constructor() {
    super("tunnel: machine offline");
    this.name = "OfflineError";
  }
}

/** Thrown by relay() when the daemon does not answer within the deadline. */
export class TimeoutError extends Error {
  constructor() {
    super("tunnel: daemon did not respond");
    this.name = "TimeoutError";
  }
}

export class Multiplexer {
  private readonly send: (frame: Frame) => void;
  private online = true;
  // Frames that arrived from the daemon before anyone was waiting for them,
  // parked per stream so a slightly-late relay() still collects its answer.
  private readonly queues = new Map<string, Frame[]>();
  private readonly waiters = new Map<string, Waiter[]>();
  private readonly subscribers = new Map<string, Subscriber>();

  constructor(send: (frame: Frame) => void) {
    this.send = send;
  }

  /** isOnline reports whether the daemon socket is currently attached. */
  isOnline(): boolean {
    return this.online;
  }

  /**
   * deliver routes one frame that arrived FROM the daemon. A live subscriber on
   * the stream wins; else the oldest buffered relay() waiter; else it is parked.
   */
  deliver(frame: Frame): void {
    const stream = frame.stream;
    const sub = this.subscribers.get(stream);
    if (sub) {
      sub.push(frame);
      return;
    }
    const ws = this.waiters.get(stream);
    if (ws && ws.length > 0) {
      const w = ws.shift()!;
      if (ws.length === 0) this.waiters.delete(stream);
      if (w.timer) clearTimeout(w.timer);
      w.resolve(frame);
      return;
    }
    const q = this.queues.get(stream);
    if (q) q.push(frame);
    else this.queues.set(stream, [frame]);
  }

  /**
   * relay forwards one outbound (browser-origin) frame to the daemon and, when
   * `expectReply` is true, resolves with the daemon's next frame on that stream.
   * hs1/req expect a reply (hs2/res); end/bye are fire-and-forget.
   */
  relay(frame: Frame, opts: { expectReply: boolean; timeoutMs: number }): Promise<Frame | null> {
    if (!this.online) return Promise.reject(new OfflineError());
    this.send(frame);
    if (!opts.expectReply) return Promise.resolve(null);

    const stream = frame.stream;
    const parked = this.queues.get(stream);
    if (parked && parked.length > 0) {
      const f = parked.shift()!;
      if (parked.length === 0) this.queues.delete(stream);
      return Promise.resolve(f);
    }

    return new Promise<Frame | null>((resolve, reject) => {
      const waiter: Waiter = {
        resolve,
        reject,
        timer: setTimeout(() => this.dropWaiter(stream, waiter, new TimeoutError()), opts.timeoutMs),
      };
      const arr = this.waiters.get(stream);
      if (arr) arr.push(waiter);
      else this.waiters.set(stream, [waiter]);
    });
  }

  /**
   * subscribe attaches a streaming sink for a stream (one browser SSE session).
   * Any already-parked frames are flushed first, then live frames flow until
   * unsubscribe()/fail(). Returns false if the daemon is offline.
   */
  subscribe(stream: string, sub: Subscriber): boolean {
    if (!this.online) return false;
    this.subscribers.set(stream, sub);
    const parked = this.queues.get(stream);
    if (parked) {
      this.queues.delete(stream);
      for (const f of parked) sub.push(f);
    }
    return true;
  }

  /** unsubscribe detaches a streaming sink (browser disconnected / done). */
  unsubscribe(stream: string): void {
    this.subscribers.delete(stream);
  }

  /**
   * fail marks the daemon offline and releases every browser session: in-flight
   * relays reject, subscriptions end. Called when the daemon socket drops.
   */
  fail(): void {
    this.online = false;
    for (const [stream, arr] of this.waiters) {
      for (const w of arr) {
        if (w.timer) clearTimeout(w.timer);
        w.reject(new OfflineError());
      }
      this.waiters.delete(stream);
    }
    for (const [stream, sub] of this.subscribers) {
      sub.end();
      this.subscribers.delete(stream);
    }
    this.queues.clear();
  }

  private dropWaiter(stream: string, waiter: Waiter, err: Error): void {
    const arr = this.waiters.get(stream);
    if (arr) {
      const i = arr.indexOf(waiter);
      if (i >= 0) arr.splice(i, 1);
      if (arr.length === 0) this.waiters.delete(stream);
    }
    waiter.reject(err);
  }
}

/** replyExpected is true for outbound kinds that get exactly one daemon reply. */
export function replyExpected(kind: string): boolean {
  return kind === "hs1" || kind === "req";
}
