// @fort/gateway-shared: the browser/edge crypto mirror of Fort's Go relay
// (spec 028). Noise IK initiator + responder, the ChaCha20-Poly1305 transport
// Session, and the gateway frame wire types.

export {
  PROTOCOL_NAME,
  Handshake,
  Session,
  newInitiator,
  newResponder,
  generateKeypair,
  keypairFromPrivateKey,
  fingerprint,
  randomBytes,
} from "./noise";
export type { KeyPair, HandshakeOptions } from "./noise";

export {
  encodeReqPayload,
  decodeResPayload,
  decodeChunkPayload,
  hs1Frame,
  readHS2,
  sealReq,
  openRes,
  openChunk,
  sealEnd,
  openFrame,
} from "./frame";
export type { Frame, ReqPayload, ResPayload, ChunkPayload } from "./frame";

export { encodeBase64, decodeBase64, encodeBase32NoPad } from "./codec";
