// codes: single-use join-code generation. Format XXXX-XXXX over an alphabet
// with no easily-confused characters (no O/0/I/1) so the pasted
// `fort relay join … --code XXXX-XXXX` line is transcription-safe.

const ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"; // 32 unambiguous chars

/** newJoinCode mints a random XXXX-XXXX code from the unambiguous alphabet. */
export function newJoinCode(): string {
  const raw = new Uint8Array(8);
  crypto.getRandomValues(raw);
  let out = "";
  for (let i = 0; i < 8; i++) {
    if (i === 4) out += "-";
    out += ALPHABET[raw[i]! & 31];
  }
  return out;
}

/** normalizeCode upper-cases and trims a user-entered code for lookup. */
export function normalizeCode(code: string): string {
  return code.trim().toUpperCase();
}
