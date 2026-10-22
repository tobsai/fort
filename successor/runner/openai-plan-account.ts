import { createHash, createPublicKey, randomBytes, verify, type webcrypto } from 'node:crypto';
import { createServer } from 'node:http';
import type { PlanCredential } from './openai-plan-credential.ts';

// OpenAI "Sign in with ChatGPT" for open-source, locally hosted apps (Spec 059, October 22 revision).
// Browser authorization code flow with PKCE and a 127.0.0.1 callback; Fort never sees a password or cookie.
const ISSUER = 'https://auth.openai.com';
const AUTHORIZE_URL = `${ISSUER}/api/accounts/authorize`, TOKEN_URL = `${ISSUER}/api/accounts/oauth/token`;
const RESOURCE = 'https://api.openai.com/v1';
const SCOPE = 'openid profile email offline_access resource.invoke chatgpt.tokens.use.direct';
const PLAN_SCOPE = 'chatgpt.tokens.use.direct';
const REGISTRATION_CLIENT = 'dynamic_agent_client';
const TERMINAL_REFRESH = new Set(['invalid_grant', 'invalid_refresh_token', 'token_expired', 'refresh_token_expired', 'refresh_token_invalidated', 'refresh_token_reused']);
const HOST_PATTERN = /^urn:uuid:[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const CALLBACK_PATTERN = /^http:\/\/127\.0\.0\.1:\d{1,5}\/auth\/callback$/;

export class SignInRequired extends Error { constructor() { super('Sign in to ChatGPT again'); } }
export class PlanUsageNotGranted extends Error { constructor() { super('ChatGPT plan usage was not granted for Fort'); } }
export type TokenTransport = { postForm(url: string, form: URLSearchParams): Promise<{ status: number; body: unknown }> };
export type SigningKeys = (kid: string) => Promise<webcrypto.JsonWebKey | undefined>;
export type PendingSignIn = { authorizationURL: string; redirectURI: string; state: string; nonce: string; verifier: string; hostId: string; clientId?: string };

const random = () => randomBytes(32).toString('base64url');
const record = (value: unknown): Record<string, unknown> => value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 16 * 1024;

export function prepareSignIn(options: { hostId: string; agentName: string; redirectURI: string; clientId?: string; idTokenHint?: string }): PendingSignIn {
  if (!HOST_PATTERN.test(options.hostId)) throw new Error('Use the opaque host identifier');
  if (!CALLBACK_PATTERN.test(options.redirectURI)) throw new Error('Use the 127.0.0.1 loopback callback at /auth/callback');
  const state = random(), nonce = random(), verifier = random();
  const query = new URLSearchParams({ client_id: options.clientId ?? REGISTRATION_CLIENT });
  if (!options.clientId) query.set('agent_name_hint', options.agentName);
  query.set('ext_agent_host_id', options.hostId);
  if (options.idTokenHint) query.set('id_token_hint', options.idTokenHint);
  for (const [key, value] of Object.entries({ response_type: 'code', redirect_uri: options.redirectURI, scope: SCOPE, resource: RESOURCE, state, nonce, code_challenge_method: 'S256', code_challenge: createHash('sha256').update(verifier).digest('base64url') })) query.set(key, value);
  return { authorizationURL: `${AUTHORIZE_URL}?${query}`, redirectURI: options.redirectURI, state, nonce, verifier, hostId: options.hostId, ...(options.clientId ? { clientId: options.clientId } : {}) };
}

async function verifiedClaims(token: string, keys: SigningKeys, clientId: string, nonce: string | undefined, now: number): Promise<Record<string, unknown>> {
  const fail = () => new Error('The ChatGPT identity could not be verified');
  const parts = token.split('.');
  if (parts.length !== 3) throw fail();
  let header: Record<string, unknown>, claims: Record<string, unknown>;
  try { header = record(JSON.parse(Buffer.from(parts[0]!, 'base64url').toString())); claims = record(JSON.parse(Buffer.from(parts[1]!, 'base64url').toString())); } catch { throw fail(); }
  if (header.alg !== 'RS256' || !text(header.kid)) throw fail();
  const jwk = await keys(header.kid);
  if (!jwk) throw fail();
  let valid = false;
  try { valid = verify('RSA-SHA256', Buffer.from(`${parts[0]}.${parts[1]}`), createPublicKey({ key: jwk, format: 'jwk' }), Buffer.from(parts[2]!, 'base64url')); } catch { throw fail(); }
  const audience = Array.isArray(claims.aud) ? claims.aud : [claims.aud];
  if (!valid || claims.iss !== ISSUER || !audience.includes(clientId) || typeof claims.exp !== 'number' || claims.exp * 1000 <= now || !text(claims.sub) || (nonce !== undefined && claims.nonce !== nonce)) throw fail();
  return claims;
}

function tokenSet(body: unknown, now: number) {
  const row = record(body);
  if (!text(row.access_token) || row.token_type !== 'Bearer' || !Number.isSafeInteger(row.expires_in) || (row.expires_in as number) <= 0 || typeof row.scope !== 'string') throw new Error('The token response is invalid');
  const scopes = row.scope.split(' ').filter(Boolean);
  if (!scopes.includes(PLAN_SCOPE)) throw new PlanUsageNotGranted();
  return { accessToken: row.access_token, refreshToken: row.refresh_token, idToken: row.id_token, expiresAt: now + (row.expires_in as number) * 1000, scopes };
}

export async function completeSignIn(options: { pending: PendingSignIn; callback: URLSearchParams; transport: TokenTransport; keys: SigningKeys; now?: () => number }): Promise<PlanCredential> {
  const { pending, callback } = options, now = options.now ?? Date.now;
  if (callback.get('state') !== pending.state) throw new Error('The sign-in state did not match');
  if (callback.has('error')) throw new Error('Sign-in was declined');
  const code = callback.get('code'), clientId = callback.get('client_id') ?? pending.clientId;
  if (!text(code) || !text(clientId) || (pending.clientId !== undefined && clientId !== pending.clientId)) throw new Error('The sign-in callback is incomplete');
  const response = await options.transport.postForm(TOKEN_URL, new URLSearchParams({ grant_type: 'authorization_code', client_id: clientId, code, code_verifier: pending.verifier, redirect_uri: pending.redirectURI, resource: RESOURCE }));
  if (response.status !== 200) throw new Error('The code exchange was refused');
  const tokens = tokenSet(response.body, now());
  if (!text(tokens.refreshToken) || !text(tokens.idToken)) throw new Error('The token response is invalid');
  const claims = await verifiedClaims(tokens.idToken, options.keys, clientId, pending.nonce, now());
  return { issuer: ISSUER, subject: claims.sub as string, clientId, hostId: pending.hostId, idToken: tokens.idToken, accessToken: tokens.accessToken, refreshToken: tokens.refreshToken, expiresAt: tokens.expiresAt, scopes: tokens.scopes, ...(text(claims.email) ? { email: claims.email } : {}) };
}

// Replaces access token, expiry, scopes and the rotating refresh token together. Terminal codes require sign-in again.
export async function refreshPlanCredential(options: { credential: PlanCredential; transport: TokenTransport; keys?: SigningKeys; now?: () => number }): Promise<PlanCredential> {
  const { credential } = options, now = options.now ?? Date.now;
  const response = await options.transport.postForm(TOKEN_URL, new URLSearchParams({ grant_type: 'refresh_token', client_id: credential.clientId, refresh_token: credential.refreshToken, resource: RESOURCE }));
  if (response.status !== 200) {
    const code = record(response.body).error;
    if ((response.status === 400 || response.status === 401) && typeof code === 'string' && TERMINAL_REFRESH.has(code)) throw new SignInRequired();
    throw new Error('Token refresh failed; the saved sign-in was kept');
  }
  const tokens = tokenSet(response.body, now());
  if (!text(tokens.refreshToken)) throw new Error('The token response is invalid');
  if (tokens.idToken !== undefined) {
    if (!text(tokens.idToken) || !options.keys) throw new Error('The token response is invalid');
    const claims = await verifiedClaims(tokens.idToken, options.keys, credential.clientId, undefined, now());
    if (claims.sub !== credential.subject) throw new Error('The ChatGPT identity could not be verified');
  }
  return { ...credential, accessToken: tokens.accessToken, refreshToken: tokens.refreshToken, expiresAt: tokens.expiresAt, scopes: tokens.scopes, ...(tokens.idToken ? { idToken: tokens.idToken } : {}) };
}

// One-shot listener on 127.0.0.1. Start it before opening the system browser.
export async function listenForCallback(signal: AbortSignal): Promise<{ redirectURI: string; callback: Promise<URLSearchParams> }> {
  let settle!: (value: URLSearchParams) => void, reject!: (error: Error) => void;
  const callback = new Promise<URLSearchParams>((resolve, fail) => { settle = resolve; reject = fail; });
  callback.catch(() => {});
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1');
    if (request.method !== 'GET' || url.pathname !== '/auth/callback') { response.writeHead(404).end(); return; }
    response.writeHead(200, { 'content-type': 'text/plain; charset=utf-8', connection: 'close' }).end('You can return to Fort.');
    stop(); settle(url.searchParams);
  });
  const stop = () => { server.close(); server.closeAllConnections(); signal.removeEventListener('abort', aborted); };
  const aborted = () => { stop(); reject(new Error('Sign-in was canceled')); };
  signal.throwIfAborted();
  await new Promise<void>((resolve, fail) => { server.once('error', fail); server.listen(0, '127.0.0.1', () => resolve()); });
  signal.addEventListener('abort', aborted, { once: true });
  const address = server.address();
  if (address === null || typeof address === 'string') { stop(); throw new Error('The loopback listener did not start'); }
  return { redirectURI: `http://127.0.0.1:${address.port}/auth/callback`, callback };
}
