# @fort/gateway-shared

The browser/edge **crypto mirror** of Fort's Go relay (spec 028, part 2).

`fort serve` dials out to the gateway and terminates each client session with a
Noise handshake in Go (`exec/relay/secure`, using
[`github.com/flynn/noise`](https://github.com/flynn/noise)). This package is the
**other end of that exact protocol** in TypeScript, so a browser or a Cloudflare
Worker can be the Noise **initiator** against the daemon **responder** — the
gateway broker in between only ever sees ciphertext.

It lives **outside** the Go module (it is a separate deploy artifact); the Go
build is unaffected.

## The suite

```
Noise_IK_25519_ChaChaPoly_BLAKE2s
```

- **Pattern:** IK — the initiator knows (pins) the responder's static key up
  front. Message 1 (`e, es, s, ss`) is written by the client; message 2
  (`e, ee, se`) by the daemon; `Split()` then yields the two transport keys.
- **DH:** X25519 (`@stablelib/x25519`), scalar clamped exactly as flynn's
  `curve25519.X25519`.
- **AEAD:** ChaCha20-Poly1305 (`@stablelib/chacha20poly1305`) with the Noise
  96-bit nonce = 32 zero bits followed by a **little-endian** u64 counter.
- **Hash:** BLAKE2s-256 (`@stablelib/blake2s`); the Noise HKDF is HMAC-BLAKE2s
  (`@stablelib/hmac`), re-implemented to match flynn's 2-/3-output `hkdf`.
- **Fingerprint:** base32(no-pad, `sha256(pub)[:16]`) grouped `xxxx-xxxx-…` —
  byte-identical to Go's `secure.FingerprintOf` (`@stablelib/sha256`).

Fort's session convention is preserved: the **initiator** encrypts with `cs1`
and decrypts with `cs2`; the **responder** mirrors that.

### Pinned dependency versions

| Package | Version | Role |
| --- | --- | --- |
| `@stablelib/x25519` | 2.0.1 | X25519 DH |
| `@stablelib/chacha20poly1305` | 2.0.1 | AEAD transport |
| `@stablelib/blake2s` | 2.0.1 | Noise hash |
| `@stablelib/hmac` | 2.0.1 | HKDF (HMAC-BLAKE2s) |
| `@stablelib/sha256` | 2.0.1 | fingerprint |
| `typescript` | 5.9.3 | typecheck |
| `vitest` | 4.1.10 | test runner |

base32/base64 are implemented locally (`src/codec.ts`) so their output is
provably identical to Go's standard-library encoders.

## API

```ts
import { newInitiator, generateKeypair, keypairFromPrivateKey, fingerprint }
  from "@fort/gateway-shared";

const client = generateKeypair();               // client static identity
const hs = newInitiator(client, daemonStaticPub); // pins the daemon's key
const msg1 = hs.writeMessage();                  // -> send as an "hs1" frame
hs.readMessage(msg2);                            // <- from the "hs2" frame
const session = hs.session()!;                   // ChaCha20-Poly1305 transport
const sealed = session.seal(plaintext);          // -> "req" / "end" frame body
const opened = session.open(ciphertext);         // <- "res" / "chunk" frame body
```

`src/frame.ts` mirrors `exec/relay/frame.go` (`Frame`, `ReqPayload`,
`ResPayload`, `ChunkPayload`) and provides the client-side helpers to build a
sealed `req`/`end` and parse a sealed `res`/`chunk`, plus the `hs1`/`hs2`
handshake frames.

## How the cross-stack vector proves interop

`exec/relay/secure/vector_test.go` drives the **real flynn/noise engine** with a
**deterministic** random source: fixed initiator/responder static keys and fixed
ephemerals (`noise.Config.Random = bytes.NewReader(seed)`), so every handshake
byte is reproducible. It emits `testdata/ik-vector.json` — the private keys, both
handshake messages, and sample sealed transport frames after `Split()`.

`test/noise.vector.test.ts` loads that JSON and runs the TypeScript mirror with
the **same** fixed keys, asserting **byte-for-byte** against the Go output:

1. **msg1 is byte-identical** — proves the whole IK write path
   (MixKey / MixHash / EncryptAndHash ordering, the Noise HKDF, the ChaCha nonce
   and the handshake-hash AD).
2. **TS opens Go's msg2** and completes — proves the read path (DHEE/DHSE +
   DecryptAndHash).
3. **TS `cs2` opens Go's responder-sealed frame** — proves `Split()`'s output
   order and the transport nonce/AD convention (decrypt direction).
4. **TS `cs1` seal equals Go's initiator-sealed frame** (byte-identical) —
   proves the encrypt direction.
5. **A TS responder reproduces Go's msg2 byte-identically** and interoperates on
   the transport keys — proves the responder write path too.
6. **`fingerprint(pub)` equals the Go value** in the vector.
7. **`encodeReqPayload` equals Go's `encoding/json`** output — the frame layer's
   wire bytes match.

Because the ephemeral keys are pinned on **both** sides, msg1 and msg2 are true
byte-matches (not just a re-derived-key round trip): there was **nothing left
unpinned**. Regenerate the vector any time with
`go test ./exec/relay/secure/ -run TestGenerateIKVector`; the seeds are fixed, so
the committed JSON never drifts.

## Develop

```sh
cd gateway/shared
npm install
npx tsc --noEmit   # typecheck
npx vitest run     # cross-stack vector + round-trip tests
```
