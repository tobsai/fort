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

/** A fake daemon: one Noise responder session per stream, plus a route table. */
function makeFakeDaemon(daemonKey: KeyPair, routes: Record<string, unknown>) {
  const sessions = new Map<string, Session>();

  function sealRes(session: Session, obj: unknown): Frame["b64"] {
    return encodeBase64(session.seal(utf8enc.encode(JSON.stringify(obj))));
  }

  // Handle a buffered /api/req frame, returning the daemon's reply frame.
  function handleReq(frame: Frame): Frame {
    if (frame.kind === "hs1") {
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
  function handleSse(frame: Frame): Response {
    const session = sessions.get(frame.stream)!;
    const rp = JSON.parse(utf8dec.decode(openFrame(session, frame, "req"))) as ReqPayload;
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
    if (input.includes("/api/sse")) return handleSse(frame);
    const reply = handleReq(frame);
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

  it("opens a sealed SSE stream to completion", async () => {
    const daemonKey = generateKeypair();
    vi.stubGlobal("fetch", makeFakeDaemon(daemonKey, {}));

    const client = new RelayClient("machine-1", daemonKey.publicKey);
    await client.connect();

    const chunks: string[] = [];
    await client.stream("/api/events?since=0", (c) => {
      if (c.data) chunks.push(utf8dec.decode(c.data));
    });

    expect(chunks).toEqual(["event: hello\n\n", "event: world\n\n"]);
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
