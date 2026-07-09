// noise: a browser/edge mirror of Fort's Go relay crypto (exec/relay/secure.go),
// which uses github.com/flynn/noise with the cipher suite
//
//   Noise_IK_25519_ChaChaPoly_BLAKE2s
//
// X25519 DH, ChaCha20-Poly1305 AEAD (96-bit nonce = 32 zero bits ++
// little-endian u64 counter), BLAKE2s hash. This file re-implements flynn's
// symmetric/handshake state machine for the IK pattern so a browser or edge
// worker can be the Noise INITIATOR against the daemon RESPONDER — and, for
// tests, the responder too. Every operation (MixKey / MixHash / EncryptAndHash
// ordering, the Noise HKDF, Split's output order, the AEAD nonce/AD convention)
// is matched byte-for-byte; the cross-stack vector in test/ proves it.

import { BLAKE2s } from "@stablelib/blake2s";
import { ChaCha20Poly1305 } from "@stablelib/chacha20poly1305";
import { hmac } from "@stablelib/hmac";
import { hash as sha256 } from "@stablelib/sha256";
import { scalarMultBase, sharedKey } from "@stablelib/x25519";

import { encodeBase32NoPad } from "./codec";

const DHLEN = 32;
const TAGLEN = 16;
const HASHLEN = 32;

/** The exact cipher-suite protocol name the Go daemon negotiates. */
export const PROTOCOL_NAME = "Noise_IK_25519_ChaChaPoly_BLAKE2s";

/** A long-term or ephemeral X25519 keypair. `privateKey` is the raw 32 bytes. */
export interface KeyPair {
  privateKey: Uint8Array;
  publicKey: Uint8Array;
}

// --- primitives ------------------------------------------------------------

function blake2s(data: Uint8Array): Uint8Array {
  const h = new BLAKE2s(HASHLEN);
  h.update(data);
  return h.digest();
}

function hmacBlake2s(key: Uint8Array, data: Uint8Array): Uint8Array {
  return hmac(BLAKE2s, key, data);
}

function concat(a: Uint8Array, b: Uint8Array): Uint8Array {
  const out = new Uint8Array(a.length + b.length);
  out.set(a);
  out.set(b, a.length);
  return out;
}

// hkdf is Noise's HKDF (flynn/noise `hkdf`): tempKey = HMAC(ck, ikm); then each
// output_i = HMAC(tempKey, output_{i-1} ++ i) with output_0 empty.
function hkdf(ck: Uint8Array, ikm: Uint8Array, outputs: 2 | 3): Uint8Array[] {
  const tempKey = hmacBlake2s(ck, ikm);
  const out1 = hmacBlake2s(tempKey, Uint8Array.of(0x01));
  const out2 = hmacBlake2s(tempKey, concat(out1, Uint8Array.of(0x02)));
  if (outputs === 2) return [out1, out2];
  const out3 = hmacBlake2s(tempKey, concat(out2, Uint8Array.of(0x03)));
  return [out1, out2, out3];
}

// dh performs the X25519 Diffie-Hellman flynn uses (curve25519.X25519), clamping
// the scalar internally. rejectZero mirrors flynn rejecting an all-zero output.
function dh(priv: Uint8Array, pub: Uint8Array): Uint8Array {
  return sharedKey(priv, pub, true);
}

// chachaNonce is Noise's 96-bit nonce: 32 zero bits then a little-endian u64.
function chachaNonce(n: bigint): Uint8Array {
  const nonce = new Uint8Array(12);
  new DataView(nonce.buffer).setBigUint64(4, n, true);
  return nonce;
}

/** randomBytes uses the WebCrypto CSPRNG (available in browsers, edge, Node 22). */
export function randomBytes(n: number): Uint8Array {
  const b = new Uint8Array(n);
  crypto.getRandomValues(b);
  return b;
}

// --- CipherState -----------------------------------------------------------

// CipherState is one AEAD direction with its own monotonic nonce (flynn
// CipherState). ad is the associated data (the handshake hash during the
// handshake, nil/undefined for transport frames).
class CipherState {
  private readonly aead: ChaCha20Poly1305;
  private n = 0n;

  constructor(key: Uint8Array) {
    this.aead = new ChaCha20Poly1305(key);
  }

  encrypt(ad: Uint8Array | undefined, plaintext: Uint8Array): Uint8Array {
    const ct = this.aead.seal(chachaNonce(this.n), plaintext, ad);
    this.n += 1n;
    return ct;
  }

  decrypt(ad: Uint8Array | undefined, ciphertext: Uint8Array): Uint8Array {
    const pt = this.aead.open(chachaNonce(this.n), ciphertext, ad);
    if (pt === null) throw new Error("noise: decryption failed (bad tag/nonce)");
    this.n += 1n;
    return pt;
  }
}

// --- SymmetricState --------------------------------------------------------

// SymmetricState is flynn's symmetricState for the subset IK needs (no PSK).
class SymmetricState {
  ck!: Uint8Array;
  h!: Uint8Array;
  private cs: CipherState | null = null;
  hasK = false;

  initializeSymmetric(protocolName: Uint8Array): void {
    if (protocolName.length <= HASHLEN) {
      this.h = new Uint8Array(HASHLEN);
      this.h.set(protocolName);
    } else {
      this.h = blake2s(protocolName);
    }
    this.ck = this.h.slice();
  }

  mixKey(dhOutput: Uint8Array): void {
    const [ck, k] = hkdf(this.ck, dhOutput, 2);
    this.ck = ck!;
    this.cs = new CipherState(k!); // fresh state => nonce reset to 0, as flynn does
    this.hasK = true;
  }

  mixHash(data: Uint8Array): void {
    const h = new BLAKE2s(HASHLEN);
    h.update(this.h);
    h.update(data);
    this.h = h.digest();
  }

  encryptAndHash(plaintext: Uint8Array): Uint8Array {
    if (!this.hasK) {
      this.mixHash(plaintext);
      return plaintext;
    }
    const ct = this.cs!.encrypt(this.h, plaintext);
    this.mixHash(ct);
    return ct;
  }

  decryptAndHash(data: Uint8Array): Uint8Array {
    if (!this.hasK) {
      this.mixHash(data);
      return data;
    }
    const pt = this.cs!.decrypt(this.h, data);
    this.mixHash(data);
    return pt;
  }

  split(): [CipherState, CipherState] {
    const [k1, k2] = hkdf(this.ck, new Uint8Array(0), 2);
    return [new CipherState(k1!), new CipherState(k2!)];
  }
}

// --- Session ---------------------------------------------------------------

/**
 * Session seals/opens transport frames after a completed handshake, matching
 * Go's secure.Session. Fort's convention: the initiator encrypts with cs1 and
 * decrypts with cs2; the responder mirrors that.
 */
export class Session {
  constructor(
    private readonly enc: CipherState,
    private readonly dec: CipherState,
  ) {}

  /** seal encrypts one frame (no associated data), matching Session.Seal. */
  seal(plaintext: Uint8Array): Uint8Array {
    return this.enc.encrypt(undefined, plaintext);
  }

  /** open decrypts one frame, throwing on any tampering, matching Session.Open. */
  open(ciphertext: Uint8Array): Uint8Array {
    return this.dec.decrypt(undefined, ciphertext);
  }
}

// --- Handshake -------------------------------------------------------------

// IK message patterns (flynn HandshakeIK). ResponderPreMessages = [s].
type Token = "e" | "s" | "ee" | "es" | "se" | "ss";
const IK_MESSAGES: Token[][] = [
  ["e", "es", "s", "ss"], // msg1: initiator -> responder
  ["e", "ee", "se"], // msg2: responder -> initiator
];

/** Options for a handshake; `ephemeral` pins the ephemeral key (tests/vectors). */
export interface HandshakeOptions {
  ephemeral?: KeyPair;
  prologue?: Uint8Array;
}

/** Handshake is one side of a Noise IK handshake (flynn HandshakeState). */
export class Handshake {
  private readonly ss = new SymmetricState();
  private readonly s: KeyPair; // local static
  private e: KeyPair | null = null; // local ephemeral
  private rs: Uint8Array | null; // remote static public
  private re: Uint8Array | null = null; // remote ephemeral public
  private readonly initiator: boolean;
  private msgIdx = 0;
  private shouldWrite: boolean;
  private readonly genEphemeral: () => KeyPair;
  private sess: Session | null = null;

  constructor(
    initiator: boolean,
    staticKeypair: KeyPair,
    peerStatic: Uint8Array | null,
    opts: HandshakeOptions = {},
  ) {
    this.initiator = initiator;
    this.s = staticKeypair;
    this.rs = peerStatic;
    this.shouldWrite = initiator;
    if (opts.ephemeral) {
      const fixed = opts.ephemeral;
      let used = false;
      this.genEphemeral = () => {
        if (used) throw new Error("noise: fixed ephemeral already consumed");
        used = true;
        return fixed;
      };
    } else {
      this.genEphemeral = generateKeypair;
    }

    const prologue = opts.prologue ?? new Uint8Array(0);
    this.ss.initializeSymmetric(new TextEncoder().encode(PROTOCOL_NAME));
    this.ss.mixHash(prologue);
    // IK pre-message: the responder's static is known to both sides up front.
    // Both initiator and responder MixHash the responder's static public key.
    if (initiator) {
      if (!this.rs) throw new Error("noise: initiator requires a pinned peer static key");
      this.ss.mixHash(this.rs);
    } else {
      this.ss.mixHash(this.s.publicKey);
    }
  }

  /** session returns the transport Session once the handshake completes. */
  session(): Session | null {
    return this.sess;
  }

  private finish(cs1: CipherState, cs2: CipherState): void {
    // Split(): cs1 encrypts initiator->responder, cs2 the reverse.
    this.sess = this.initiator ? new Session(cs1, cs2) : new Session(cs2, cs1);
  }

  private mixDH(token: "ee" | "es" | "se" | "ss"): void {
    switch (token) {
      case "ee":
        this.ss.mixKey(dh(this.e!.privateKey, this.re!));
        break;
      case "es":
        this.ss.mixKey(
          this.initiator
            ? dh(this.e!.privateKey, this.rs!)
            : dh(this.s.privateKey, this.re!),
        );
        break;
      case "se":
        this.ss.mixKey(
          this.initiator
            ? dh(this.s.privateKey, this.re!)
            : dh(this.e!.privateKey, this.rs!),
        );
        break;
      case "ss":
        this.ss.mixKey(dh(this.s.privateKey, this.rs!));
        break;
    }
  }

  /** writeMessage produces the next handshake message (flynn WriteMessage). */
  writeMessage(payload: Uint8Array = new Uint8Array(0)): Uint8Array {
    if (!this.shouldWrite) throw new Error("noise: unexpected writeMessage");
    if (this.msgIdx > IK_MESSAGES.length - 1) throw new Error("noise: no messages left");

    const parts: Uint8Array[] = [];
    for (const token of IK_MESSAGES[this.msgIdx]!) {
      if (token === "e") {
        this.e = this.genEphemeral();
        parts.push(this.e.publicKey);
        this.ss.mixHash(this.e.publicKey);
      } else if (token === "s") {
        parts.push(this.ss.encryptAndHash(this.s.publicKey));
      } else {
        this.mixDH(token);
      }
    }
    this.shouldWrite = false;
    this.msgIdx++;
    parts.push(this.ss.encryptAndHash(payload));

    let out: Uint8Array = new Uint8Array(0);
    for (const p of parts) out = concat(out, p);

    if (this.msgIdx >= IK_MESSAGES.length) {
      const [cs1, cs2] = this.ss.split();
      this.finish(cs1, cs2);
    }
    return out;
  }

  /** readMessage consumes the peer's handshake message (flynn ReadMessage). */
  readMessage(message: Uint8Array): Uint8Array {
    if (this.shouldWrite) throw new Error("noise: unexpected readMessage");
    if (this.msgIdx > IK_MESSAGES.length - 1) throw new Error("noise: no messages left");

    let msg = message;
    for (const token of IK_MESSAGES[this.msgIdx]!) {
      if (token === "e") {
        if (msg.length < DHLEN) throw new Error("noise: message too short (e)");
        this.re = msg.slice(0, DHLEN);
        msg = msg.subarray(DHLEN);
        this.ss.mixHash(this.re);
      } else if (token === "s") {
        const expected = DHLEN + (this.ss.hasK ? TAGLEN : 0);
        if (msg.length < expected) throw new Error("noise: message too short (s)");
        this.rs = this.ss.decryptAndHash(msg.slice(0, expected));
        msg = msg.subarray(expected);
      } else {
        this.mixDH(token);
      }
    }
    const payload = this.ss.decryptAndHash(msg);
    this.shouldWrite = true;
    this.msgIdx++;

    if (this.msgIdx >= IK_MESSAGES.length) {
      const [cs1, cs2] = this.ss.split();
      this.finish(cs1, cs2);
    }
    return payload;
  }
}

// --- public API ------------------------------------------------------------

/** generateKeypair mints a fresh X25519 static identity (matches secure.GenerateKeypair). */
export function generateKeypair(): KeyPair {
  const privateKey = randomBytes(32);
  return { privateKey, publicKey: scalarMultBase(privateKey) };
}

/** keypairFromPrivateKey derives the public key for a raw 32-byte private key. */
export function keypairFromPrivateKey(privateKey: Uint8Array): KeyPair {
  if (privateKey.length !== 32) throw new Error("noise: private key must be 32 bytes");
  return { privateKey, publicKey: scalarMultBase(privateKey) };
}

/**
 * newInitiator starts the client (initiator) side, pinning the daemon's static
 * public key. Matches secure.NewInitiator.
 */
export function newInitiator(
  staticKeypair: KeyPair,
  pinnedResponderStatic: Uint8Array,
  opts?: HandshakeOptions,
): Handshake {
  return new Handshake(true, staticKeypair, pinnedResponderStatic, opts);
}

/** newResponder starts the daemon (responder) side. Matches secure.NewResponder. */
export function newResponder(staticKeypair: KeyPair, opts?: HandshakeOptions): Handshake {
  return new Handshake(false, staticKeypair, null, opts);
}

/**
 * fingerprint is the human-comparable identity of a public key, byte-identical
 * to Go's secure.FingerprintOf: base32 (std, no padding) of the first 16 bytes
 * of sha256(pub), grouped xxxx-xxxx-... in 4s.
 */
export function fingerprint(pub: Uint8Array): string {
  const sum = sha256(pub);
  const s = encodeBase32NoPad(sum.subarray(0, 16));
  let out = "";
  for (let i = 0; i < s.length; i++) {
    if (i > 0 && i % 4 === 0) out += "-";
    out += s[i];
  }
  return out;
}
