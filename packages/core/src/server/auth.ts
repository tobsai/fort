/**
 * Fort Google OAuth Authentication
 *
 * Handles Google OAuth2 flow, signed session cookies, and email allowlist.
 * Uses raw node:https — no external OAuth libraries required.
 */

import { createHmac, timingSafeEqual } from 'node:crypto';
import { request as httpsRequest } from 'node:https';

// Session cookie name and version prefix
const COOKIE_NAME = 'fort_session';
const SESSION_VERSION = 'v1';
const COOKIE_MAX_AGE_SECONDS = 7 * 24 * 60 * 60; // 7 days

export interface GoogleAuthConfig {
  clientId: string;
  clientSecret: string;
  sessionSecret: string;
  allowedEmails: string[];
  callbackUrl: string;
  authEnabled: boolean;
}

export function loadAuthConfig(): GoogleAuthConfig {
  const clientId = process.env.GOOGLE_CLIENT_ID ?? '';
  const clientSecret = process.env.GOOGLE_CLIENT_SECRET ?? '';
  const sessionSecret = process.env.SESSION_SECRET ?? '';
  const raw = process.env.FORT_ALLOWED_EMAILS ?? 'tobiasgunn@gmail.com';
  const allowedEmails = raw.split(',').map((e) => e.trim()).filter(Boolean);
  const callbackUrl =
    process.env.GOOGLE_CALLBACK_URL ?? 'http://localhost:4077/auth/google/callback';
  const authEnabled = process.env.FORT_AUTH_ENABLED !== 'false';
  return { clientId, clientSecret, sessionSecret, allowedEmails, callbackUrl, authEnabled };
}

// ---------------------------------------------------------------------------
// Cookie signing / verification
// ---------------------------------------------------------------------------

/**
 * Build a signed session cookie value.
 * Format: `v1:<email>:<timestampMs>:<hmac-sha256-hex>`
 *
 * Email addresses never contain `:` so splitting on `:` is unambiguous.
 */
export function signCookie(email: string, secret: string): string {
  const ts = Date.now().toString();
  const data = `${SESSION_VERSION}:${email}:${ts}`;
  const sig = createHmac('sha256', secret).update(data).digest('hex');
  return `${data}:${sig}`;
}

/**
 * Verify a signed cookie value.
 * Returns the email if valid and not expired, otherwise null.
 */
export function verifyCookie(value: string, secret: string): string | null {
  // Split into exactly 4 parts: version, email, timestamp, signature
  const firstColon = value.indexOf(':');
  if (firstColon < 0) return null;
  const version = value.slice(0, firstColon);
  if (version !== SESSION_VERSION) return null;

  const rest = value.slice(firstColon + 1);
  const lastColon = rest.lastIndexOf(':');
  if (lastColon < 0) return null;
  const sig = rest.slice(lastColon + 1);
  const emailAndTs = rest.slice(0, lastColon);

  const tsColon = emailAndTs.lastIndexOf(':');
  if (tsColon < 0) return null;
  const email = emailAndTs.slice(0, tsColon);
  const tsStr = emailAndTs.slice(tsColon + 1);

  if (!email || !tsStr || !sig) return null;

  const ts = parseInt(tsStr, 10);
  if (isNaN(ts) || Date.now() - ts > COOKIE_MAX_AGE_SECONDS * 1000) return null;

  const data = `${SESSION_VERSION}:${email}:${tsStr}`;
  const expected = createHmac('sha256', secret).update(data).digest('hex');

  // Constant-time comparison
  try {
    const sigBuf = Buffer.from(sig, 'hex');
    const expBuf = Buffer.from(expected, 'hex');
    if (sigBuf.length !== expBuf.length) return null;
    if (!timingSafeEqual(sigBuf, expBuf)) return null;
  } catch {
    return null;
  }

  return email;
}

// ---------------------------------------------------------------------------
// Allowlist
// ---------------------------------------------------------------------------

export function isEmailAllowed(email: string, allowedEmails: string[]): boolean {
  return allowedEmails.some((e) => e.toLowerCase() === email.toLowerCase());
}

// ---------------------------------------------------------------------------
// Cookie header helpers
// ---------------------------------------------------------------------------

export function parseCookies(cookieHeader: string | undefined): Record<string, string> {
  if (!cookieHeader) return {};
  const result: Record<string, string> = {};
  for (const part of cookieHeader.split(';')) {
    const idx = part.indexOf('=');
    if (idx < 0) continue;
    const key = part.slice(0, idx).trim();
    const val = part.slice(idx + 1).trim();
    if (key) {
      try {
        result[key] = decodeURIComponent(val);
      } catch {
        result[key] = val;
      }
    }
  }
  return result;
}

export function getSessionEmail(
  cookieHeader: string | undefined,
  secret: string,
): string | null {
  const cookies = parseCookies(cookieHeader);
  const val = cookies[COOKIE_NAME];
  if (!val) return null;
  return verifyCookie(val, secret);
}

export function buildSessionCookieHeader(
  email: string,
  secret: string,
  isSecure: boolean,
): string {
  const value = signCookie(email, secret);
  const parts = [
    `${COOKIE_NAME}=${encodeURIComponent(value)}`,
    'HttpOnly',
    'SameSite=Lax',
    `Max-Age=${COOKIE_MAX_AGE_SECONDS}`,
    'Path=/',
  ];
  if (isSecure) parts.push('Secure');
  return parts.join('; ');
}

export function buildClearCookieHeader(): string {
  return `${COOKIE_NAME}=; HttpOnly; SameSite=Lax; Max-Age=0; Path=/`;
}

// ---------------------------------------------------------------------------
// Google OAuth2 URL builder
// ---------------------------------------------------------------------------

export function buildGoogleAuthUrl(clientId: string, callbackUrl: string): string {
  const params = new URLSearchParams({
    client_id: clientId,
    redirect_uri: callbackUrl,
    response_type: 'code',
    scope: 'openid email profile',
    access_type: 'online',
  });
  return `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;
}

// ---------------------------------------------------------------------------
// Raw HTTPS helpers (no external deps)
// ---------------------------------------------------------------------------

function httpsPost(
  url: string,
  body: string,
  headers: Record<string, string | number>,
): Promise<{ statusCode: number; data: string }> {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url);
    const req = httpsRequest(
      {
        hostname: parsed.hostname,
        path: parsed.pathname + parsed.search,
        method: 'POST',
        headers: { 'Content-Length': Buffer.byteLength(body), ...headers },
      },
      (res) => {
        let data = '';
        res.on('data', (chunk: Buffer) => { data += chunk.toString(); });
        res.on('end', () => resolve({ statusCode: res.statusCode ?? 0, data }));
      },
    );
    req.on('error', reject);
    req.write(body);
    req.end();
  });
}

function httpsGet(
  url: string,
  headers: Record<string, string>,
): Promise<{ statusCode: number; data: string }> {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url);
    const req = httpsRequest(
      {
        hostname: parsed.hostname,
        path: parsed.pathname + parsed.search,
        method: 'GET',
        headers,
      },
      (res) => {
        let data = '';
        res.on('data', (chunk: Buffer) => { data += chunk.toString(); });
        res.on('end', () => resolve({ statusCode: res.statusCode ?? 0, data }));
      },
    );
    req.on('error', reject);
    req.end();
  });
}

// ---------------------------------------------------------------------------
// OAuth2 code → email exchange
// ---------------------------------------------------------------------------

/**
 * Exchange an authorization code for the authenticated user's email address.
 * Throws if token exchange fails or the email is not present in the response.
 */
export async function exchangeCodeForEmail(
  code: string,
  clientId: string,
  clientSecret: string,
  callbackUrl: string,
): Promise<string> {
  const body = new URLSearchParams({
    code,
    client_id: clientId,
    client_secret: clientSecret,
    redirect_uri: callbackUrl,
    grant_type: 'authorization_code',
  }).toString();

  const tokenRes = await httpsPost('https://oauth2.googleapis.com/token', body, {
    'Content-Type': 'application/x-www-form-urlencoded',
  });

  if (tokenRes.statusCode !== 200) {
    throw new Error(`Google token exchange failed (${tokenRes.statusCode}): ${tokenRes.data}`);
  }

  let tokens: Record<string, unknown>;
  try {
    tokens = JSON.parse(tokenRes.data) as Record<string, unknown>;
  } catch {
    throw new Error('Invalid token response from Google');
  }

  const accessToken = tokens.access_token as string | undefined;
  if (!accessToken) throw new Error('No access_token in Google response');

  const userRes = await httpsGet('https://www.googleapis.com/oauth2/v2/userinfo', {
    Authorization: `Bearer ${accessToken}`,
  });

  if (userRes.statusCode !== 200) {
    throw new Error(`Google userinfo failed (${userRes.statusCode})`);
  }

  let userInfo: Record<string, unknown>;
  try {
    userInfo = JSON.parse(userRes.data) as Record<string, unknown>;
  } catch {
    throw new Error('Invalid userinfo response from Google');
  }

  const email = userInfo.email as string | undefined;
  if (!email) throw new Error('No email in Google userinfo response');

  return email;
}
