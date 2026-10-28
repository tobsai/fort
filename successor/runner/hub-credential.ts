import { randomBytes } from 'node:crypto';
import { join } from 'node:path';
import type { ChatRuntime } from './chat-runtime.ts';
import type { ExecutorBinding } from './executor-wire.ts';
import { keepConnected, type RunnerLane, type RunnerStatus } from './hub-runner.ts';
import { deletePrivate, privateDirectory, readPrivate, writePrivate } from './private-file.ts';

// The Mac's hub runner token (Spec 063, step 4). Toby chose on 2026-10-24 to keep it like the ChatGPT
// sign-in: an owner-only file in the helper's private state directory, never in the ledger or logs.
// It lets the hub send this Mac turns pinned to it, and the hub can revoke it at any time.
// node is the ID this Mac's lanes serve when the hub pools several Macs under one node (Spec 063,
// revision 2026-10-28). Without it the Mac serves its own runner ID, as before.
export type HubCredential = { hubURL: string; runnerId: string; token: string; node?: string };
type Fetch = (url: string, init?: RequestInit) => Promise<Response>;

const CREDENTIAL = 'hub-runner.json';
const IDENTIFIER = /^[A-Za-z0-9_-]{1,128}$/, TOKEN = /^[A-Za-z0-9_-]{43}$/;

function hubURL(value: string): string {
  const url = new URL(value);
  const local = url.protocol === 'http:' && ['127.0.0.1', '[::1]', 'localhost'].includes(url.hostname);
  if ((url.protocol !== 'https:' && !local) || url.username || url.password || url.search || url.hash || (url.pathname !== '/' && url.pathname !== '')) throw new Error('Use the hub’s HTTPS address');
  return url.origin;
}
function parse(value: unknown): HubCredential {
  const row = value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
  if (Object.keys(row).some(key => !['hubURL', 'runnerId', 'token', 'node'].includes(key)) || typeof row.hubURL !== 'string' || typeof row.runnerId !== 'string' || !IDENTIFIER.test(row.runnerId)
    || typeof row.token !== 'string' || !TOKEN.test(row.token) || (row.node !== undefined && (typeof row.node !== 'string' || !IDENTIFIER.test(row.node)))) throw new Error('The saved hub credential is invalid');
  return { hubURL: hubURL(row.hubURL), runnerId: row.runnerId, token: row.token, ...(typeof row.node === 'string' && row.node !== row.runnerId ? { node: row.node } : {}) };
}

export async function readHubCredential(directory: string): Promise<HubCredential | undefined> {
  await privateDirectory(directory);
  const contents = await readPrivate(join(directory, CREDENTIAL));
  if (contents === undefined) return undefined;
  try { return parse(JSON.parse(contents)); } catch { throw new Error('The saved hub credential is invalid'); }
}

export function deleteHubCredential(directory: string): Promise<'deleted' | 'absent'> { return deletePrivate(directory, CREDENTIAL); }

// Node reports every network failure as "fetch failed"; the cause's code says which one it was.
function unreachable(origin: string, error: unknown): Error {
  const cause = error instanceof Error ? (error.cause as { code?: unknown } | undefined) : undefined;
  const code = typeof cause?.code === 'string' && /^[A-Z0-9_]{1,64}$/.test(cause.code) ? cause.code : undefined;
  const certificate = code !== undefined && /CERT|SIGNATURE|ISSUER|SELF_SIGNED/.test(code);
  return new Error((certificate ? 'This Mac couldn’t check the hub’s certificate' : `This Mac couldn’t reach ${new URL(origin).host}`) + (code ? ` (${code})` : '') + '.');
}

// Joins the hub with an owner invitation and keeps the new token only after the hub confirms it.
export async function enrollWithHub(options: { stateDirectory: string; hubURL: string; invitationId: string; code: string; runnerId: string; runnerName: string; fetch?: Fetch }): Promise<{ runnerId: string; node: string }> {
  const origin = hubURL(options.hubURL);
  if (await readHubCredential(options.stateDirectory)) throw new Error('This Mac is already enrolled with a hub. Remove that enrollment first.');
  const token = randomBytes(32).toString('base64url');
  const response = await (options.fetch ?? fetch)(origin + '/v1/runners/join', { method: 'POST', redirect: 'error', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ invitationId: options.invitationId, code: options.code, runnerId: options.runnerId, runnerToken: token, runnerName: options.runnerName }) })
    .catch((error: unknown) => { throw unreachable(origin, error); });
  const reply = await response.json().catch(() => undefined) as { runnerId?: unknown; node?: unknown; error?: { message?: unknown } } | undefined;
  if (response.status !== 201 || reply?.runnerId !== options.runnerId) {
    // The hub's own reason (expired, already used, doesn't match), kept short and to one line.
    const reason = typeof reply?.error?.message === 'string' ? reply.error.message.replace(/\s+/g, ' ').trim().slice(0, 200) : '';
    throw new Error('The hub refused this enrollment' + (reason ? ': ' + reason : '') + (/expired/i.test(reason) ? ' Invitations last 5 minutes; make a new one.' : ''));
  }
  const node = typeof reply.node === 'string' && IDENTIFIER.test(reply.node) ? reply.node : options.runnerId;
  await writePrivate(options.stateDirectory, CREDENTIAL, JSON.stringify(parse({ hubURL: origin, runnerId: options.runnerId, token, ...(node === options.runnerId ? {} : { node }) })));
  return { runnerId: options.runnerId, node };
}

// Runs this Mac as the enrolled runner until the signal ends or the hub refuses the token.
export async function runAsRunner(options: { stateDirectory: string; binding: ExecutorBinding; createRuntime(): ChatRuntime; lanes?: RunnerLane[]; signal: AbortSignal; onStatus?(status: RunnerStatus): void; fetch?: Fetch }): Promise<RunnerStatus> {
  const credential = await readHubCredential(options.stateDirectory);
  if (!credential) throw new Error('This Mac is not enrolled with a hub');
  if ([options.binding, ...(options.lanes ?? []).map(lane => lane.binding)].some(binding => binding.hostId !== (credential.node ?? credential.runnerId))) throw new Error('This binding is pinned to a different Mac than the one enrolled');
  return keepConnected({ url: credential.hubURL, token: credential.token, binding: options.binding, createRuntime: options.createRuntime, ...(options.lanes ? { lanes: options.lanes } : {}),
    signal: options.signal, ...(options.onStatus ? { onStatus: options.onStatus } : {}), ...(options.fetch ? { fetch: options.fetch } : {}) });
}
