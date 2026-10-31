import { join } from 'node:path';
import type { HandoffReport, HandoffRuntime, HubLedger, Staying } from './handoff-contract.ts';
import { readHubCredential } from './hub-credential.ts';
import { deletePrivate, readPrivate, writePrivate } from './private-file.ts';

// The Mac side of hub-owned Agents (Spec 063, revision 2026-10-31). Once this Mac is enrolled and connected, it hands its
// Agents to the hub (definitions only, never history), records which ones moved so its own copies go read-only, and keeps a
// client credential so the Mac app can show and talk to the hub's Agents. Every step is safe to repeat.
type Fetch = (url: string, init?: RequestInit) => Promise<Response>;
export type HubClientCredential = { hubURL: string; installationId: string; deviceId: string; token: string };
export type HandoffStatus =
  | { state: 'notEnrolled' }
  | { state: 'unsupported'; detail: string }
  | { state: 'failed'; detail: string }
  | { state: 'synced'; at: string; agentIds: string[]; staying: Staying[] };

const CLIENT = 'hub-client.json';
const IDENTIFIER = /^[A-Za-z0-9_-]{1,128}$/, TOKEN = /^[A-Za-z0-9_-]{43}$/;

function parseClient(value: unknown, hubURL: string): HubClientCredential | undefined {
  const row = value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
  if (row.hubURL !== hubURL || typeof row.installationId !== 'string' || !IDENTIFIER.test(row.installationId) || typeof row.deviceId !== 'string' || !IDENTIFIER.test(row.deviceId)
    || typeof row.token !== 'string' || !TOKEN.test(row.token)) return undefined;
  return { hubURL, installationId: row.installationId, deviceId: row.deviceId, token: row.token };
}

export async function readHubClient(stateDirectory: string): Promise<HubClientCredential | undefined> {
  const credential = await readHubCredential(stateDirectory);
  const contents = credential ? await readPrivate(join(stateDirectory, CLIENT)) : undefined;
  if (!credential || contents === undefined) return undefined;
  try { return parseClient(JSON.parse(contents), credential.hubURL); } catch { return undefined; }
}
export function deleteHubClient(stateDirectory: string) { return deletePrivate(stateDirectory, CLIENT); }

async function reason(response: Response): Promise<string> {
  const reply = await response.json().catch(() => undefined) as { error?: { message?: unknown } } | undefined;
  return typeof reply?.error?.message === 'string' ? reply.error.message.replace(/\s+/g, ' ').slice(0, 200) : `The hub answered ${response.status}.`;
}

export async function syncAgentsToHub(options: { stateDirectory: string; ledger: HubLedger; fetch?: Fetch }): Promise<HandoffStatus> {
  const credential = await readHubCredential(options.stateDirectory);
  if (!credential) return { state: 'notEnrolled' };
  const send = options.fetch ?? fetch, authorization = 'Bearer ' + credential.token, url = credential.hubURL + '/v1/runners/agents';
  // 1. Which runtimes the hub serves. An older hub has no handoff, so this Mac keeps running its Agents.
  const offer = await send(url, { headers: { authorization }, redirect: 'error' });
  if (offer.status === 404) return { state: 'unsupported', detail: 'This hub doesn’t take Agents yet, so they keep running on this Mac.' };
  if (!offer.ok) return { state: 'failed', detail: await reason(offer) };
  const served = await offer.json() as { runtimes?: unknown };
  const runtimes = (Array.isArray(served.runtimes) ? served.runtimes : []).filter((row): row is HandoffRuntime =>
    !!row && typeof row === 'object' && typeof (row as HandoffRuntime).provider === 'string' && typeof (row as HandoffRuntime).runtimeVersion === 'string');
  // 2. Hand over what the hub can run. The hub adds what it lacks and keeps what it has, so a repeat is harmless.
  const { payload, staying } = options.ledger.exportHandoff(runtimes);
  let moved = options.ledger.handoff()?.agentIds ?? [];
  if (payload.agentIds.length) {
    const handed = await send(url, { method: 'POST', redirect: 'error', headers: { authorization, 'content-type': 'application/json' }, body: JSON.stringify(payload) });
    if (!handed.ok) return { state: 'failed', detail: await reason(handed) };
    const report = await handed.json() as HandoffReport;
    if (!Array.isArray(report.agentIds) || report.agentIds.some(agentId => !payload.agentIds.includes(agentId))) return { state: 'failed', detail: 'The hub’s reply didn’t match this handoff.' };
    // 3. Only Agents the hub confirmed go read-only here.
    moved = options.ledger.recordHandoff({ hubURL: credential.hubURL, agentIds: report.agentIds }).agentIds;
  }
  // 4. A client credential, so the Mac app can open the hub.
  if (!(await readHubClient(options.stateDirectory))) {
    const issued = await send(credential.hubURL + '/v1/runners/client', { method: 'POST', redirect: 'error', headers: { authorization } });
    if (!issued.ok) return { state: 'failed', detail: await reason(issued) };
    const client = parseClient({ ...(await issued.json() as object), hubURL: credential.hubURL }, credential.hubURL);
    if (!client) return { state: 'failed', detail: 'The hub’s client credential couldn’t be read.' };
    await writePrivate(options.stateDirectory, CLIENT, JSON.stringify(client));
  }
  return { state: 'synced', at: new Date().toISOString(), agentIds: moved, staying };
}
