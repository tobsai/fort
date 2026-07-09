// types: shapes shared between the web server routes and the client. These
// mirror the worker's internal API contract (gateway/worker/src/types.ts).

/** A registered machine as the worker's /api/relay/machines returns it. */
export interface MachineSummary {
  machine_id: string;
  name: string;
  /** base32 fingerprint of the daemon static key (the string to verify). */
  fingerprint: string;
  online: boolean;
  /**
   * Daemon static X25519 public key, standard base64. The browser needs this to
   * run the Noise IK initiator and to pin (TOFU) against `fingerprint`. Public,
   * not secret — but only ever served to an authenticated session.
   */
  public_key: string;
}
