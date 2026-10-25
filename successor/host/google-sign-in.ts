import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { createHmac, createPublicKey, randomBytes, timingSafeEqual, verify } from 'node:crypto';

// The hub's Google gate (Spec 066). The web client signs in with Google in the browser and posts the
// ID token here; the hub checks Google's signature, its own one-time nonce, the client ID and the
// owner's verified email, pins the Google account's stable ID on first use, and answers with a
// signed session cookie. Nothing secret comes from Google: the client ID is public, and the cookie
// key lives only on the hub's volume.
export type GoogleSignInOptions = {
  clientId: string;
  // The iPhone app's iOS OAuth client ID. When set, paired iPhones sign in with Google too.
  iosClientId?: string;
  // Verified Google emails allowed in. The first one to sign in is pinned by its Google account ID.
  emails: string[];
  // Google's current signing keys. The default fetches them from Google and caches them.
  keys?: () => Promise<JsonWebKey[]>;
  sessionHours?: number;
};
export type GoogleSession = { sub: string; email: string; expiresAt: number };

export const SESSION_COOKIE = '__Host-fort_session';
const ISSUERS = ['accounts.google.com', 'https://accounts.google.com'];
const CERTS = 'https://www.googleapis.com/oauth2/v3/certs';
const NONCE_MS = 10 * 60_000, SKEW_S = 60;

export class SignInError extends Error {
  readonly status: number; readonly code: string;
  constructor(status: number, code: string, message: string) { super(message); this.status = status; this.code = code; }
}
const deny = (message: string): never => { throw new SignInError(401, 'signInDenied', message); };
const b64 = (value: Buffer | string) => Buffer.from(value).toString('base64url');
const json = (part: string): Record<string, unknown> => {
  try { const value = JSON.parse(Buffer.from(part, 'base64url').toString('utf8')) as unknown; if (value && typeof value === 'object') return value as Record<string, unknown>; } catch { /* below */ }
  return deny('That Google sign-in could not be read.');
};

function googleKeys(): () => Promise<JsonWebKey[]> {
  let cached: { keys: JsonWebKey[]; until: number } | undefined;
  return async () => {
    if (cached && cached.until > Date.now()) return cached.keys;
    const response = await fetch(CERTS, { signal: AbortSignal.timeout(5000) });
    if (!response.ok) throw new SignInError(503, 'googleUnavailable', 'Google’s sign-in keys could not be fetched. Try again.');
    const keys = ((await response.json()) as { keys?: JsonWebKey[] }).keys ?? [];
    const age = Number(/max-age=(\d+)/.exec(response.headers.get('cache-control') ?? '')?.[1] ?? 3600);
    cached = { keys, until: Date.now() + Math.min(age, 6 * 3600) * 1000 };
    return keys;
  };
}

export function googleSignIn(dataDirectory: string, options: GoogleSignInOptions) {
  const clientId = options.clientId.trim(), emails = options.emails.map(email => email.trim().toLowerCase()).filter(Boolean);
  const iosClientId = options.iosClientId?.trim() || undefined;
  if (![clientId, iosClientId ?? clientId].every(id => /^[\w.-]+\.apps\.googleusercontent\.com$/.test(id))) throw new Error('Use a Google OAuth web client ID');
  if (emails.length === 0) throw new Error('Name at least one Google email to let in');
  const sessionMs = (options.sessionHours ?? 24 * 7) * 3_600_000;
  const keys = options.keys ?? googleKeys();
  const keyFile = join(dataDirectory, 'web-session.key'), ownerFile = join(dataDirectory, 'google-owner.json');
  if (!existsSync(keyFile)) writeFileSync(keyFile, randomBytes(32), { mode: 0o600, flag: 'wx' });
  const secret = readFileSync(keyFile);
  if (secret.length !== 32) throw new Error('The web session key is damaged');
  const sign = (value: string) => createHmac('sha256', secret).update(value).digest();
  const signed = (value: string) => value + '.' + b64(sign(value));
  const opened = (value: string): string | undefined => {
    const at = value.lastIndexOf('.');
    if (at < 1) return undefined;
    const body = value.slice(0, at), mac = Buffer.from(value.slice(at + 1), 'base64url'), expected = sign(body);
    return mac.length === expected.length && timingSafeEqual(mac, expected) ? body : undefined;
  };
  const pinned = () => existsSync(ownerFile) ? (JSON.parse(readFileSync(ownerFile, 'utf8')) as { sub: string }).sub : undefined;
  const used = new Map<string, number>();

  // A fresh nonce for the browser to hand Google; it comes back inside the ID token and works once.
  function nonce(): string {
    for (const [value, until] of used) if (until <= Date.now()) used.delete(value);
    return signed(Date.now().toString(36) + '~' + b64(randomBytes(16)));
  }

  async function verifyIdToken(token: unknown, audience: string): Promise<{ sub: string; email: string }> {
    if (typeof token !== 'string' || token.length > 8192) return deny('Send the Google sign-in credential.');
    const parts = token.split('.');
    if (parts.length !== 3) return deny('That Google sign-in could not be read.');
    const header = json(parts[0]!), claims = json(parts[1]!);
    if (header.alg !== 'RS256' || typeof header.kid !== 'string') return deny('That Google sign-in is not signed the way Google signs.');
    const jwk = (await keys()).find(key => (key as { kid?: string }).kid === header.kid);
    if (!jwk) return deny('That Google sign-in is not signed by a current Google key.');
    if (!verify('RSA-SHA256', Buffer.from(parts[0] + '.' + parts[1]), createPublicKey({ key: jwk, format: 'jwk' }), Buffer.from(parts[2]!, 'base64url'))) return deny('That Google sign-in’s signature does not match.');
    const now = Date.now() / 1000;
    if (!ISSUERS.includes(claims.iss as string) || claims.aud !== audience) return deny('That Google sign-in was not made for this hub.');
    if (typeof claims.exp !== 'number' || claims.exp + SKEW_S < now || typeof claims.iat !== 'number' || claims.iat - SKEW_S > now) return deny('That Google sign-in has expired. Sign in again.');
    const issued = typeof claims.nonce === 'string' ? opened(claims.nonce) : undefined;
    const at = issued ? parseInt(issued.split('~')[0]!, 36) : NaN;
    if (!issued || !(Date.now() - at < NONCE_MS) || used.has(claims.nonce as string)) return deny('That Google sign-in was already used or has expired. Sign in again.');
    if (claims.email_verified !== true || typeof claims.email !== 'string' || typeof claims.sub !== 'string' || !emails.includes(claims.email.toLowerCase())) return deny('This Google account is not allowed on this hub.');
    const owner = pinned();
    if (owner !== undefined && owner !== claims.sub) return deny('This Google account is not allowed on this hub.');
    used.set(claims.nonce as string, at + NONCE_MS);
    if (owner === undefined) writeFileSync(ownerFile, JSON.stringify({ sub: claims.sub }), { mode: 0o600 });
    return { sub: claims.sub, email: claims.email.toLowerCase() };
  }

  const mint = (who: { sub: string; email: string }) => {
    const session = { ...who, expiresAt: Date.now() + sessionMs };
    return { session, value: signed(b64(JSON.stringify(session))) };
  };
  const read = (raw: string | undefined): GoogleSession | undefined => {
    const body = raw ? opened(raw) : undefined;
    if (!body) return undefined;
    let value: GoogleSession;
    try { value = JSON.parse(Buffer.from(body, 'base64url').toString('utf8')) as GoogleSession; } catch { return undefined; }
    if (typeof value?.expiresAt !== 'number' || value.expiresAt <= Date.now() || !emails.includes(value.email) || value.sub !== pinned()) return undefined;
    return value;
  };

  return {
    clientId,
    iosClientId,
    nonce,
    // Checks a posted Google ID token and returns the Set-Cookie value for a new session.
    async signIn(token: unknown): Promise<{ session: GoogleSession; cookie: string }> {
      const { session, value } = mint(await verifyIdToken(token, clientId));
      return { session, cookie: `${SESSION_COOKIE}=${value}; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=${Math.floor(sessionMs / 1000)}` };
    },
    // The iPhone's sign-in: the same checks against the iOS client, answered with the session value
    // itself, which the app keeps in its Keychain and sends as X-Fort-Session.
    async signInNative(token: unknown): Promise<{ session: GoogleSession; value: string }> {
      if (!iosClientId) throw new SignInError(404, 'notFound', 'iPhone Google sign-in is not enabled.');
      return mint(await verifyIdToken(token, iosClientId));
    },
    signOutCookie: `${SESSION_COOKIE}=; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=0`,
    // The session a request carries, as the browser's cookie or the iPhone's header, if it is ours,
    // current, and still the owner's.
    session(cookieHeader: string | undefined, nativeHeader?: string | string[]): GoogleSession | undefined {
      const cookie = cookieHeader?.split(/;\s*/).find(part => part.startsWith(SESSION_COOKIE + '='))?.slice(SESSION_COOKIE.length + 1);
      return read(cookie) ?? (typeof nativeHeader === 'string' ? read(nativeHeader) : undefined);
    },
  };
}
export type GoogleGate = ReturnType<typeof googleSignIn>;
