// codec: byte helpers that match the Go daemon exactly.
//
// - base64 std (Go `encoding/base64.StdEncoding`) — the WebSocket frame `b64`
//   field and JSON `[]byte` values.
// - base32 std, no padding (Go `base32.StdEncoding.WithPadding(NoPadding)`) —
//   used only inside the identity fingerprint.
//
// Both are implemented here rather than pulled from a dependency so their output
// is provably identical to Go's standard-library encoders.

const B64_ALPHABET =
  "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
const B64_LOOKUP: Record<string, number> = (() => {
  const m: Record<string, number> = {};
  for (let i = 0; i < B64_ALPHABET.length; i++) m[B64_ALPHABET[i]!] = i;
  return m;
})();

/** encodeBase64 mirrors Go's base64.StdEncoding.EncodeToString. */
export function encodeBase64(data: Uint8Array): string {
  let out = "";
  let i = 0;
  for (; i + 3 <= data.length; i += 3) {
    const n = (data[i]! << 16) | (data[i + 1]! << 8) | data[i + 2]!;
    out +=
      B64_ALPHABET[(n >>> 18) & 63]! +
      B64_ALPHABET[(n >>> 12) & 63]! +
      B64_ALPHABET[(n >>> 6) & 63]! +
      B64_ALPHABET[n & 63]!;
  }
  const rem = data.length - i;
  if (rem === 1) {
    const n = data[i]! << 16;
    out += B64_ALPHABET[(n >>> 18) & 63]! + B64_ALPHABET[(n >>> 12) & 63]! + "==";
  } else if (rem === 2) {
    const n = (data[i]! << 16) | (data[i + 1]! << 8);
    out +=
      B64_ALPHABET[(n >>> 18) & 63]! +
      B64_ALPHABET[(n >>> 12) & 63]! +
      B64_ALPHABET[(n >>> 6) & 63]! +
      "=";
  }
  return out;
}

/** decodeBase64 mirrors Go's base64.StdEncoding.DecodeString (padded input). */
export function decodeBase64(s: string): Uint8Array {
  const clean = s.replace(/=+$/, "");
  const out = new Uint8Array((clean.length * 6) >> 3);
  let bits = 0;
  let value = 0;
  let o = 0;
  for (let i = 0; i < clean.length; i++) {
    const c = B64_LOOKUP[clean[i]!];
    if (c === undefined) throw new Error("codec: invalid base64 character");
    value = (value << 6) | c;
    bits += 6;
    if (bits >= 8) {
      bits -= 8;
      out[o++] = (value >>> bits) & 0xff;
    }
  }
  return out.subarray(0, o);
}

const B32_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";

/**
 * encodeBase32NoPad mirrors Go's
 * base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString.
 */
export function encodeBase32NoPad(data: Uint8Array): string {
  let bits = 0;
  let value = 0;
  let out = "";
  for (let i = 0; i < data.length; i++) {
    value = (value << 8) | data[i]!;
    bits += 8;
    while (bits >= 5) {
      bits -= 5;
      out += B32_ALPHABET[(value >>> bits) & 31]!;
      value &= (1 << bits) - 1; // keep `value` small (< 2^bits) to stay in int range
    }
  }
  if (bits > 0) {
    out += B32_ALPHABET[(value << (5 - bits)) & 31]!;
  }
  return out;
}
