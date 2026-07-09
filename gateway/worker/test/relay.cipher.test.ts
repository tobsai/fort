// The ciphertext-only-relay proof, run deterministically through the real broker
// multiplexer + the real @fort/gateway-shared Noise crypto — no workerd/WS, so
// it never flakes. A fake BROWSER (Noise initiator) and a fake DAEMON (Noise
// responder) exchange a sealed GET /api/board round-trip; the Multiplexer in the
// middle plays the Durable Object's relay role. We capture EVERY frame that
// crosses the broker and assert the plaintext marker is in none of them, while
// the browser still recovers it after AEAD-open. That is exactly the property
// the Worker/DO must have: it relays opaque frames and never holds plaintext.

import { describe, expect, it } from "vitest";

import {
  type Frame,
  type ReqPayload,
  type Session,
  decodeBase64,
  encodeBase64,
  generateKeypair,
  hs1Frame,
  newInitiator,
  newResponder,
  openRes,
  readHS2,
  sealReq,
} from "@fort/gateway-shared";
import { Multiplexer, replyExpected } from "../src/mux";

const utf8 = new TextEncoder();
const PLAINTEXT_MARKER = "TOP-SECRET-BOARD-PAYLOAD-9f3a";

// A daemon seals a ResPayload the same way Go's relay does: JSON with []byte
// `body` as standard base64, sealed under the session key.
function sealRes(session: Session, stream: string, id: string, status: number, body: Uint8Array): Frame {
  const json = JSON.stringify({ id, status, body: encodeBase64(body) });
  return { stream, kind: "res", b64: encodeBase64(session.seal(utf8.encode(json))) };
}

describe("ciphertext-only relay (browser initiator <-> broker mux <-> daemon responder)", () => {
  it("round-trips a sealed GET /api/board without the broker ever seeing plaintext", async () => {
    const daemonKey = generateKeypair(); // the daemon's pinned static identity
    const stream = "sess-1";

    // Every frame the broker touches, captured at both hops.
    const brokerSaw: Frame[] = [];

    // --- the DAEMON side (Noise responder). Fed by the mux's send(). ---------
    let daemonSession: Session | null = null;
    const mux = new Multiplexer((frame) => {
      brokerSaw.push(frame); // broker -> daemon
      if (frame.kind === "hs1") {
        const resp = newResponder(daemonKey);
        resp.readMessage(decodeBase64(frame.b64!));
        const m2 = resp.writeMessage();
        daemonSession = resp.session();
        deliverFromDaemon({ stream: frame.stream, kind: "hs2", b64: encodeBase64(m2) });
      } else if (frame.kind === "req") {
        const pt = daemonSession!.open(decodeBase64(frame.b64!));
        const rp = JSON.parse(new TextDecoder().decode(pt)) as { id: string; path: string };
        expect(rp.path).toBe("/api/board");
        const body = utf8.encode(JSON.stringify({ board: "ok", secret: PLAINTEXT_MARKER }));
        deliverFromDaemon(sealRes(daemonSession!, frame.stream, rp.id, 200, body));
      }
    });
    function deliverFromDaemon(frame: Frame): void {
      brokerSaw.push(frame); // daemon -> broker
      mux.deliver(frame);
    }

    // --- the BROWSER side (Noise initiator), driving frames through the mux ---
    const browserStatic = generateKeypair();
    const hs = newInitiator(browserStatic, daemonKey.publicKey);

    // 1) handshake: send hs1, receive hs2.
    const hs1 = hs1Frame(stream, hs.writeMessage());
    const hs2 = await relayThroughBroker(mux, hs1);
    const session = readHS2(hs, hs2!);

    // 2) sealed request: GET /api/board.
    const rp: ReqPayload = { id: "r1", method: "GET", path: "/api/board" };
    const reqFrame = sealReq(session, stream, rp);
    const resFrame = await relayThroughBroker(mux, reqFrame);

    // 3) the browser decrypts the response — E2E worked.
    const res = openRes(session, resFrame!);
    expect(res.status).toBe(200);
    const decoded = JSON.parse(new TextDecoder().decode(res.body!)) as { secret: string };
    expect(decoded.secret).toBe(PLAINTEXT_MARKER);

    // 4) the broker never held the plaintext: no captured frame carries the
    //    marker, and every sealed b64 decodes to ciphertext, not the plaintext.
    expect(brokerSaw.length).toBeGreaterThanOrEqual(4); // hs1, hs2, req, res
    for (const frame of brokerSaw) {
      expect(JSON.stringify(frame)).not.toContain(PLAINTEXT_MARKER);
      if (frame.kind === "req" || frame.kind === "res") {
        const raw = new TextDecoder().decode(decodeBase64(frame.b64!));
        expect(raw).not.toContain(PLAINTEXT_MARKER);
        expect(raw).not.toContain("/api/board"); // even the path is sealed
      }
    }
  });
});

// relayThroughBroker mirrors what the Worker's /api/relay/req path does with the
// TunnelDO: forward one browser frame, return the daemon's next frame on it.
async function relayThroughBroker(mux: Multiplexer, frame: Frame): Promise<Frame | null> {
  return mux.relay(frame, { expectReply: replyExpected(frame.kind), timeoutMs: 1000 });
}
