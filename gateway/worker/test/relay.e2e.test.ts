// Full-stack E2E through the real Worker + Durable Objects (miniflare):
//
//   fake DAEMON (Noise responder)  ==WS==>  /tunnel  ==DO==\
//                                                           TunnelDO
//   fake BROWSER (Noise initiator) ==POST /api/relay/req==>/
//
// A real daemon socket attaches; the browser drives sealed frames through the
// broker's proxy path; a GET /api/board round-trips to a canned response the
// daemon serves. We then assert the broker only ever handled opaque ciphertext:
// no frame that crossed either hop carries the plaintext marker.

import { describe, expect, it } from "vitest";
import { SELF } from "cloudflare:test";

import {
  type Frame,
  type KeyPair,
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
import { BASE, SECRET, mintAndJoin } from "./helpers";

const utf8 = new TextEncoder();
const MARKER = "E2E-SECRET-BOARD-c0ffee";

function sealRes(session: Session, stream: string, id: string, status: number, body: Uint8Array): Frame {
  const json = JSON.stringify({ id, status, body: encodeBase64(body) });
  return { stream, kind: "res", b64: encodeBase64(session.seal(utf8.encode(json))) };
}

function proxyReq(machineId: string, frame: Frame): Promise<Response> {
  return SELF.fetch(BASE + "/api/relay/req", {
    method: "POST",
    headers: { "content-type": "application/json", "x-gateway-secret": SECRET },
    body: JSON.stringify({ machine_id: machineId, frame }),
  });
}

describe("end-to-end sealed relay through the Worker + DO", () => {
  it("round-trips GET /api/board; the broker sees only ciphertext", async () => {
    const daemonKey = generateKeypair();
    const stream = "e2e-1";
    const { device_token, machine_id } = await mintAndJoin("e2e-box", daemonKey);

    // --- attach the fake daemon socket to /tunnel ---------------------------
    const up = await SELF.fetch(BASE + "/tunnel", {
      headers: { Upgrade: "websocket", Authorization: "Bearer " + device_token },
    });
    expect(up.status).toBe(101);
    const dsock = up.webSocket!;
    dsock.accept();
    try {
      await runRelay(dsock, daemonKey, machine_id, stream);
    } finally {
      dsock.close();
    }
  });

  it("relays to an offline machine with 503", async () => {
    const kp = generateKeypair();
    const { machine_id } = await mintAndJoin("never-dials", kp);
    const frame: Frame = { stream: "s", kind: "hs1", b64: "AAAA" };
    const res = await proxyReq(machine_id, frame);
    expect(res.status).toBe(503);
  });
});

async function runRelay(
  dsock: WebSocket,
  daemonKey: KeyPair,
  machine_id: string,
  stream: string,
): Promise<void> {
  {
    const daemonFrames: Frame[] = []; // everything the daemon socket saw or sent
    let daemonSession: Session | null = null;
    dsock.addEventListener("message", (ev: MessageEvent) => {
      const frame = JSON.parse(ev.data as string) as Frame;
      daemonFrames.push(frame);
      if (frame.kind === "hs1") {
        const resp = newResponder(daemonKey);
        resp.readMessage(decodeBase64(frame.b64!));
        const m2 = resp.writeMessage();
        daemonSession = resp.session();
        const out: Frame = { stream: frame.stream, kind: "hs2", b64: encodeBase64(m2) };
        daemonFrames.push(out);
        dsock.send(JSON.stringify(out));
      } else if (frame.kind === "req") {
        const pt = daemonSession!.open(decodeBase64(frame.b64!));
        const rp = JSON.parse(new TextDecoder().decode(pt)) as { id: string; path: string };
        const body = utf8.encode(JSON.stringify({ path: rp.path, secret: MARKER }));
        const out = sealRes(daemonSession!, frame.stream, rp.id, 200, body);
        daemonFrames.push(out);
        dsock.send(JSON.stringify(out));
      }
    });

    // --- drive the browser (Noise initiator) through the proxy path ---------
    const browserStatic = generateKeypair();
    const hs = newInitiator(browserStatic, daemonKey.publicKey);
    const proxyFrames: Frame[] = []; // everything on the browser<->worker hop

    const hs1 = hs1Frame(stream, hs.writeMessage());
    proxyFrames.push(hs1);
    const hs2Res = await proxyReq(machine_id, hs1);
    expect(hs2Res.status).toBe(200);
    const hs2 = ((await hs2Res.json()) as { frames: Frame[] }).frames[0]!;
    proxyFrames.push(hs2);
    const session = readHS2(hs, hs2);

    const reqFrame = sealReq(session, stream, { id: "r1", method: "GET", path: "/api/board" });
    proxyFrames.push(reqFrame);
    const resResp = await proxyReq(machine_id, reqFrame);
    expect(resResp.status).toBe(200);
    const resFrame = ((await resResp.json()) as { frames: Frame[] }).frames[0]!;
    proxyFrames.push(resFrame);

    // --- the browser decrypts: E2E worked -----------------------------------
    const res = openRes(session, resFrame);
    expect(res.status).toBe(200);
    const decoded = JSON.parse(new TextDecoder().decode(res.body!)) as { path: string; secret: string };
    expect(decoded.path).toBe("/api/board");
    expect(decoded.secret).toBe(MARKER);

    // --- the broker only ever handled ciphertext ----------------------------
    const allFrames = [...proxyFrames, ...daemonFrames];
    expect(allFrames.length).toBeGreaterThanOrEqual(4);
    for (const frame of allFrames) {
      expect(JSON.stringify(frame)).not.toContain(MARKER);
      if (frame.kind === "req" || frame.kind === "res") {
        const raw = new TextDecoder().decode(decodeBase64(frame.b64!));
        expect(raw).not.toContain(MARKER);
        expect(raw).not.toContain("/api/board"); // path is sealed too
      }
    }
  }
}
