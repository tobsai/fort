// relay-client round-trip: the strongest verification available without a live
// daemon. We stand up a FAKE worker+daemon in-process using the shared library's
// Noise RESPONDER (the exact code the Go daemon mirrors), wire it to a stubbed
// global fetch, and drive the browser-side RelayClient through it:
//
//   • connect()  completes the IK handshake (hs1 -> hs2) and derives a session.
//   • fetch()    seals a GET, the fake daemon opens it and seals a response,
//                and the client opens it back to the original bytes.
//   • two sequential fetches keep the per-session AEAD nonce aligned.
//   • stream()   parses the sealed SSE (res header + chunk frames) end to end.
//
// If any of the seal/open/nonce/JSON conventions were wrong, these fail.

import {
  decodeBase64,
  encodeBase64,
  generateKeypair,
  newResponder,
  openFrame,
  type Frame,
  type KeyPair,
  type Session,
} from "@fort/gateway-shared";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RelayClient } from "@/lib/relay-client";

const utf8enc = new TextEncoder();
const utf8dec = new TextDecoder();

interface ReqPayload {
  id: string;
  method: string;
  path: string;
  headers?: Record<string, string>;
}

interface ObservedRelay {
  kind: string;
  stream: string;
  outerRequestID: string | null;
  inner?: ReqPayload;
}

const canonicalRequestID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** A fake daemon: one Noise responder session per stream, plus a route table. */
function makeFakeDaemon(
  daemonKey: KeyPair,
  routes: Record<string, unknown>,
  observe?: (request: ObservedRelay) => void,
) {
  const sessions = new Map<string, Session>();

  function sealRes(session: Session, obj: unknown): Frame["b64"] {
    return encodeBase64(session.seal(utf8enc.encode(JSON.stringify(obj))));
  }

  // Handle a buffered /api/req frame, returning the daemon's reply frame.
  function handleReq(frame: Frame, outerRequestID: string | null): Frame {
    if (frame.kind === "hs1") {
      observe?.({ kind: frame.kind, stream: frame.stream, outerRequestID });
      const hs = newResponder(daemonKey);
      hs.readMessage(decodeBase64(frame.b64!));
      const m2 = hs.writeMessage();
      const session = hs.session();
      if (!session) throw new Error("fake daemon: handshake did not complete");
      sessions.set(frame.stream, session);
      return { stream: frame.stream, kind: "hs2", b64: encodeBase64(m2) };
    }
    if (frame.kind === "req") {
      const session = sessions.get(frame.stream)!;
      const rp = JSON.parse(utf8dec.decode(openFrame(session, frame, "req"))) as ReqPayload;
      observe?.({ kind: frame.kind, stream: frame.stream, outerRequestID, inner: rp });
      const body = routes[rp.path];
      const bodyBytes = utf8enc.encode(JSON.stringify(body ?? { path: rp.path }));
      const res = {
        id: rp.id,
        status: body === undefined ? 404 : 200,
        body: encodeBase64(bodyBytes),
      };
      return { stream: frame.stream, kind: "res", b64: sealRes(session, res) };
    }
    throw new Error(`fake daemon: unexpected kind ${frame.kind}`);
  }

  // Handle an /api/sse frame: return an NDJSON stream of res + chunk frames.
  function handleSse(frame: Frame, outerRequestID: string | null): Response {
    const session = sessions.get(frame.stream)!;
    const rp = JSON.parse(utf8dec.decode(openFrame(session, frame, "req"))) as ReqPayload;
    observe?.({ kind: frame.kind, stream: frame.stream, outerRequestID, inner: rp });
    const lines: Frame[] = [
      { stream: frame.stream, kind: "res", b64: sealRes(session, { id: rp.id, status: 200, stream: true }) },
      { stream: frame.stream, kind: "chunk", b64: sealRes(session, { id: rp.id, data: encodeBase64(utf8enc.encode("event: hello\n\n")) }) },
      { stream: frame.stream, kind: "chunk", b64: sealRes(session, { id: rp.id, data: encodeBase64(utf8enc.encode("event: world\n\n")) }) },
      { stream: frame.stream, kind: "chunk", b64: sealRes(session, { id: rp.id, end: true }) },
    ];
    const ndjson = lines.map((f) => JSON.stringify(f)).join("\n") + "\n";
    return new Response(ndjson, {
      status: 200,
      headers: { "content-type": "application/x-ndjson" },
    });
  }

  // The stub global fetch: dispatch by the app route the RelayClient calls.
  return async function fakeFetch(input: string, init?: RequestInit): Promise<Response> {
    const { frame } = JSON.parse(String(init?.body)) as { machine_id: string; frame: Frame };
    const outerRequestID = new Headers(init?.headers).get("X-Fort-Request-ID");
    if (input.includes("/api/sse")) return handleSse(frame, outerRequestID);
    const reply = handleReq(frame, outerRequestID);
    return new Response(JSON.stringify({ frames: [reply] }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };
}

describe("RelayClient over a fake Noise daemon", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("handshakes and opens a sealed GET round-trip", async () => {
    const daemonKey = generateKeypair();
    vi.stubGlobal(
      "fetch",
      makeFakeDaemon(daemonKey, { "/api/summary": { total: 3, running: 1, queued: 2 } }),
    );

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();
    const res = await client.fetch("/api/summary");

    expect(res.status).toBe(200);
    expect(JSON.parse(utf8dec.decode(res.body!))).toEqual({ total: 3, running: 1, queued: 2 });
  });

  it("keeps two sequential sealed fetches on one session in nonce order", async () => {
    const daemonKey = generateKeypair();
    vi.stubGlobal("fetch", makeFakeDaemon(daemonKey, { "/a": { n: 1 }, "/b": { n: 2 } }));

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();
    const a = await client.fetch("/a");
    const b = await client.fetch("/b");

    expect(JSON.parse(utf8dec.decode(a.body!))).toEqual({ n: 1 });
    expect(JSON.parse(utf8dec.decode(b.body!))).toEqual({ n: 2 });
  });

  it("uses one canonical request ID in the outer POST and sealed inner request", async () => {
    const daemonKey = generateKeypair();
    const observed: ObservedRelay[] = [];
    vi.stubGlobal("fetch", makeFakeDaemon(daemonKey, { "/trace": { ok: true } }, (item) => observed.push(item)));

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();
    await client.fetch("/trace", {
      headers: { "x-fort-request-id": "caller-controlled", "x-test": "preserved" },
    });

    const application = observed.find((item) => item.inner?.path === "/trace")!;
    expect(application.outerRequestID).toMatch(canonicalRequestID);
    expect(application.inner?.id).toBe(application.outerRequestID);
    expect(application.inner?.headers?.["X-Fort-Request-ID"]).toBe(application.outerRequestID);
    expect(application.inner?.headers?.["x-fort-request-id"]).toBeUndefined();
    expect(application.inner?.headers?.["x-test"]).toBe("preserved");
  });

  it("preserves one request ID across the handshake-only retry on a fresh stream", async () => {
    const daemonKey = generateKeypair();
    const daemon = makeFakeDaemon(daemonKey, {});
    const ids: Array<string | null> = [];
    const streams: string[] = [];
    let attempts = 0;
    vi.stubGlobal("fetch", async (input: string, init?: RequestInit) => {
      const { frame } = JSON.parse(String(init?.body)) as { frame: Frame };
      if (frame.kind === "hs1") {
        attempts++;
        ids.push(new Headers(init?.headers).get("X-Fort-Request-ID"));
        streams.push(frame.stream);
        if (attempts === 1) {
          return Response.json({ error: "temporarily unavailable" }, { status: 503 });
        }
      }
      return daemon(input, init);
    });

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();

    expect(attempts).toBe(2);
    expect(ids[0]).toMatch(canonicalRequestID);
    expect(ids[1]).toBe(ids[0]);
    expect(streams[1]).not.toBe(streams[0]);
  });

  it("never replays a sealed application request and reports only its bounded request ID", async () => {
    const daemonKey = generateKeypair();
    const daemon = makeFakeDaemon(daemonKey, {});
    let applicationAttempts = 0;
    let requestID: string | null = null;
    vi.stubGlobal("fetch", async (input: string, init?: RequestInit) => {
      const { frame } = JSON.parse(String(init?.body)) as { frame: Frame };
      if (frame.kind === "req") {
        applicationAttempts++;
        requestID = new Headers(init?.headers).get("X-Fort-Request-ID");
        return Response.json(
          { error: "secret upstream body that must not be surfaced", ciphertext: "AAAA" },
          { status: 503 },
        );
      }
      return daemon(input, init);
    });

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();
    const error = await client.fetch("/mutate", { method: "POST" }).catch((cause: unknown) => cause);

    expect(applicationAttempts).toBe(1);
    expect(requestID).toMatch(canonicalRequestID);
    expect(error).toBeInstanceOf(Error);
    expect((error as Error).message).toContain(requestID);
    expect((error as Error).message).not.toContain("secret upstream body");
    expect((error as Error).message).not.toContain("AAAA");
    expect((error as Error).message.length).toBeLessThan(160);
  });

  it("closes a Noise session with one fire-and-forget bye frame", async () => {
    const daemonKey = generateKeypair();
    const fake = makeFakeDaemon(daemonKey, { "/a": { n: 1 } });
    const kinds: string[] = [];
    vi.stubGlobal("fetch", async (input: string, init?: RequestInit) => {
      const { frame } = JSON.parse(String(init?.body)) as { frame: Frame };
      kinds.push(frame.kind);
      if (frame.kind === "bye") {
        return new Response(JSON.stringify({ frames: [] }), {
          status: 200,
          headers: { "content-type": "application/json" },
        });
      }
      return fake(input, init);
    });

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();
    await client.fetch("/a");
    await client.close();
    await client.close();

    expect(kinds).toEqual(["hs1", "req", "bye"]);
  });

  it("opens a sealed SSE stream to completion", async () => {
    const daemonKey = generateKeypair();
    const observed: ObservedRelay[] = [];
    vi.stubGlobal("fetch", makeFakeDaemon(daemonKey, {}, (item) => observed.push(item)));

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();

    const chunks: string[] = [];
    await client.stream("/api/events?since=0", (c) => {
      if (c.data) chunks.push(utf8dec.decode(c.data));
    });

    expect(chunks).toEqual(["event: hello\n\n", "event: world\n\n"]);
    const request = observed.find((item) => item.inner?.path === "/api/events?since=0")!;
    expect(request.outerRequestID).toMatch(canonicalRequestID);
    expect(request.inner?.id).toBe(request.outerRequestID);
    expect(request.inner?.headers?.["X-Fort-Request-ID"]).toBe(request.outerRequestID);
  });

  it("rejects a handshake against the wrong pinned key (MITM defense)", async () => {
    const daemonKey = generateKeypair();
    const wrongKey = generateKeypair();
    vi.stubGlobal("fetch", makeFakeDaemon(daemonKey, {}));

    // Pin the wrong static key: the IK handshake must not complete cleanly.
    const client = new RelayClient("machine-1", wrongKey.publicKey);
    await expect(client.connect()).rejects.toThrow();
  });
});
