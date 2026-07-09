// frame: the gateway wire contract (spec 028), mirroring exec/relay/frame.go.
//
// One WebSocket envelope (Frame) carries a stream id, a kind, and an optional
// base64 body. The broker routes on stream/kind and never sees inside a sealed
// `b64`. This module is the CLIENT side of the protocol: build a sealed `req`
// and parse a sealed `res`/`chunk`, plus the handshake `hs1`/`hs2` frames.
//
// JSON shapes match Go's struct json tags exactly (field order + omitempty), and
// `[]byte` fields (ReqPayload.body, ResPayload.body, ChunkPayload.data) are
// standard base64 — the same convention Go's encoding/json uses for []byte.

import { decodeBase64, encodeBase64 } from "./codec";
import type { Handshake, Session } from "./noise";

/** Frame is the single WebSocket envelope. kind: hs1|hs2|req|res|chunk|end|bye. */
export interface Frame {
  stream: string;
  kind: string;
  b64?: string;
}

/** ReqPayload is the sealed plaintext of a "req" frame. */
export interface ReqPayload {
  id: string;
  method: string;
  path: string;
  headers?: Record<string, string>;
  body?: Uint8Array;
}

/** ResPayload is the sealed plaintext of a "res" frame. */
export interface ResPayload {
  id: string;
  status: number;
  headers?: Record<string, string>;
  body?: Uint8Array;
  stream?: boolean; // true => chunks follow
}

/** ChunkPayload is the sealed plaintext of a "chunk" frame (one SSE piece). */
export interface ChunkPayload {
  id: string;
  data?: Uint8Array;
  end?: boolean;
}

const utf8 = new TextEncoder();

// --- JSON (de)serialization matching Go's encoding/json ---------------------

/**
 * encodeReqPayload marshals a ReqPayload to bytes byte-identical to Go's
 * json.Marshal(relay.ReqPayload{...}): fields in struct order, headers/body
 * omitted when empty, body as standard base64.
 */
export function encodeReqPayload(rp: ReqPayload): Uint8Array {
  const obj: Record<string, unknown> = {
    id: rp.id,
    method: rp.method,
    path: rp.path,
  };
  if (rp.headers && Object.keys(rp.headers).length > 0) obj["headers"] = rp.headers;
  if (rp.body && rp.body.length > 0) obj["body"] = encodeBase64(rp.body);
  return utf8.encode(JSON.stringify(obj));
}

/** decodeResPayload parses the JSON bytes of a "res" frame. */
export function decodeResPayload(pt: Uint8Array): ResPayload {
  const o = JSON.parse(new TextDecoder().decode(pt)) as {
    id: string;
    status: number;
    headers?: Record<string, string>;
    body?: string;
    stream?: boolean;
  };
  const res: ResPayload = { id: o.id, status: o.status };
  if (o.headers) res.headers = o.headers;
  if (o.body) res.body = decodeBase64(o.body);
  if (o.stream) res.stream = o.stream;
  return res;
}

/** decodeChunkPayload parses the JSON bytes of a "chunk" frame. */
export function decodeChunkPayload(pt: Uint8Array): ChunkPayload {
  const o = JSON.parse(new TextDecoder().decode(pt)) as {
    id: string;
    data?: string;
    end?: boolean;
  };
  const cp: ChunkPayload = { id: o.id };
  if (o.data) cp.data = decodeBase64(o.data);
  if (o.end) cp.end = o.end;
  return cp;
}

// --- handshake frames -------------------------------------------------------

/** hs1Frame wraps the initiator's Noise msg1 as an "hs1" frame. */
export function hs1Frame(stream: string, msg1: Uint8Array): Frame {
  return { stream, kind: "hs1", b64: encodeBase64(msg1) };
}

/**
 * readHS2 consumes an "hs2" frame into the given handshake, completing it and
 * returning the transport Session. Throws if the frame is not an hs2 or the
 * handshake does not complete.
 */
export function readHS2(handshake: Handshake, frame: Frame): Session {
  if (frame.kind !== "hs2") throw new Error(`frame: expected hs2, got ${frame.kind}`);
  if (frame.b64 === undefined) throw new Error("frame: hs2 missing b64");
  handshake.readMessage(decodeBase64(frame.b64));
  const sess = handshake.session();
  if (sess === null) throw new Error("frame: handshake did not complete on hs2");
  return sess;
}

// --- sealed request / response frames --------------------------------------

/** sealReq seals a ReqPayload into a "req" frame for the given session. */
export function sealReq(session: Session, stream: string, rp: ReqPayload): Frame {
  const ct = session.seal(encodeReqPayload(rp));
  return { stream, kind: "req", b64: encodeBase64(ct) };
}

/** openRes opens a sealed "res" frame into a ResPayload. */
export function openRes(session: Session, frame: Frame): ResPayload {
  return decodeResPayload(openFrame(session, frame, "res"));
}

/** openChunk opens a sealed "chunk" frame into a ChunkPayload. */
export function openChunk(session: Session, frame: Frame): ChunkPayload {
  return decodeChunkPayload(openFrame(session, frame, "chunk"));
}

/** sealEnd seals an "end" frame (a ChunkPayload carrying just the request id). */
export function sealEnd(session: Session, stream: string, id: string): Frame {
  const ct = session.seal(utf8.encode(JSON.stringify({ id })));
  return { stream, kind: "end", b64: encodeBase64(ct) };
}

/** openFrame is the shared seal-open path: base64-decode then AEAD-open. */
export function openFrame(session: Session, frame: Frame, expectKind?: string): Uint8Array {
  if (expectKind && frame.kind !== expectKind) {
    throw new Error(`frame: expected ${expectKind}, got ${frame.kind}`);
  }
  if (frame.b64 === undefined) throw new Error("frame: missing b64");
  return session.open(decodeBase64(frame.b64));
}
