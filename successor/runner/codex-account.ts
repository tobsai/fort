import type { AccountIdentity, AccountSignInResult } from '../contracts/v1.ts';
import { parseAccountIdentity } from '../contracts/account.ts';
import { isAbsolute, normalize } from 'node:path';
import { CodexConnection, object } from './codex-rpc.ts';
import { verifyRuntimeControls } from './codex-controls.ts';

export type CodexDeviceChallenge = { loginId: string; verificationURL: string; userCode: string };
export type CodexSignInResult = AccountSignInResult;
type SignInOptions = {
  connection: CodexConnection; workspace: string; runtimeVersion: '0.163.0';
  signal: AbortSignal; timeoutMs: number; onChallenge: (challenge: CodexDeviceChallenge) => void;
};

type Completion = { loginId: string; success: boolean };
const active = new WeakSet<CodexConnection>();
const uuid = (value: unknown): value is string => typeof value === 'string'
  && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);

function parseChallenge(value: unknown): CodexDeviceChallenge {
  const row = object(value);
  if (row.type !== 'chatgptDeviceCode' || !uuid(row.loginId)
      || row.verificationUrl !== 'https://auth.openai.com/codex/device'
      || typeof row.userCode !== 'string' || row.userCode.length < 4 || row.userCode.length > 32
      || !/^[A-Z0-9]+(?:-[A-Z0-9]+)*$/.test(row.userCode)) throw new Error('Invalid device challenge');
  return { loginId: row.loginId, verificationURL: row.verificationUrl, userCode: row.userCode };
}

export async function readCodexAccount(connection: CodexConnection, signal: AbortSignal): Promise<AccountIdentity | null> {
  const row = object(await connection.request('account/read', { refreshToken: false }, signal));
  if (connection.isClosed || row.requiresOpenaiAuth !== true) throw new Error('Unsupported account');
  if (row.account === null) return null;
  const account = object(row.account), routing = object(row.workspaceRouting);
  if (account.type !== 'chatgpt' || typeof account.email !== 'string' || !/^[^\s@]+@[^\s@]+$/.test(account.email)) throw new Error('Unsupported account');
  return parseAccountIdentity({ accountId: routing.chatgptAccountId, userLabel: account.email });
}

function waitForCompletion(ready: Promise<Completion | 'closed'>, signal: AbortSignal): Promise<Completion | 'closed' | 'aborted'> {
  if (signal.aborted) return Promise.resolve('aborted');
  return new Promise(resolve => {
    const aborted = () => resolve('aborted');
    signal.addEventListener('abort', aborted, { once: true });
    void ready.then(value => { signal.removeEventListener('abort', aborted); resolve(value); });
  });
}

export async function signInCodexAccount(options: SignInOptions): Promise<CodexSignInResult> {
  const { connection, workspace, onChallenge } = options;
  if (active.has(connection)) throw new Error('This account connection is already in use');
  active.add(connection);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let unsubscribe: (() => void) | undefined;
  try {
    if (options.runtimeVersion !== '0.163.0' || !isAbsolute(workspace) || workspace === '/'
        || normalize(workspace) !== workspace || !workspace.isWellFormed() || workspace.includes('\0')
        || !Number.isSafeInteger(options.timeoutMs) || options.timeoutMs < 1 || options.timeoutMs > 600_000) return { kind: 'unconfirmed' };
    if (options.signal.aborted) return { kind: 'canceled' };
    const timeout = new AbortController();
    timer = setTimeout(() => timeout.abort(), options.timeoutMs);
    const signal = AbortSignal.any([options.signal, timeout.signal]);
    const ready = Promise.withResolvers<Completion | 'closed'>();
    let completion: Completion | undefined;
    let challenge: CodexDeviceChallenge | undefined;
    let loginStarted = false;
    unsubscribe = connection.subscribe({ closed: () => ready.resolve('closed'), frame: bytes => {
      try {
        const frame = object(JSON.parse(Buffer.from(bytes).toString('utf8')));
        if (frame.id !== undefined) {
          if (typeof frame.method === 'string' && (typeof frame.id === 'string' || typeof frame.id === 'number'))
            connection.write(JSON.stringify({ id: frame.id, error: { code: -32601, message: 'Unsupported account server request' } }) + '\n');
          throw new Error('Unexpected server request');
        }
        if (frame.method !== 'account/login/completed') return;
        const row = object(frame.params);
        if (!loginStarted || !uuid(row.loginId) || typeof row.success !== 'boolean'
            || !(row.error === null || (typeof row.error === 'string' && row.error.length <= 16_384))
            || (row.success && row.error !== null)
            || (challenge && row.loginId !== challenge.loginId)
            || (completion && (row.loginId !== completion.loginId || row.success !== completion.success))) throw new Error('Invalid completion');
        completion = { loginId: row.loginId, success: row.success };
        ready.resolve(completion);
      } catch { connection.close(); }
    } });
    await connection.request('initialize', { clientInfo: { name: 'fort', title: 'Fort', version: '0.1.0' },
      capabilities: { experimentalApi: true, requestAttestation: false } }, signal);
    connection.notify('initialized', {});
    await verifyRuntimeControls(connection, workspace, signal);
    const existing = await readCodexAccount(connection, signal);
    if (existing) return { kind: 'signedIn', account: existing };
    loginStarted = true;
    challenge = parseChallenge(await connection.request('account/login/start', { type: 'chatgptDeviceCode' }, signal));
    if (connection.isClosed || (completion && completion.loginId !== challenge.loginId)) return { kind: 'unconfirmed' };
    onChallenge({ ...challenge });
    const observed = await waitForCompletion(ready.promise, signal);
    if (observed === 'closed') return { kind: 'unconfirmed' };
    if (observed === 'aborted') {
      const cleanup = AbortSignal.timeout(3000);
      const receipt = object(await connection.request('account/login/cancel', { loginId: challenge.loginId }, cleanup));
      if (receipt.status !== 'canceled' && receipt.status !== 'notFound') return { kind: 'unconfirmed' };
      const existing = await readCodexAccount(connection, cleanup);
      if (existing) return { kind: 'signedIn', account: existing };
      return { kind: receipt.status === 'canceled' ? 'canceled' : 'unconfirmed' };
    }
    if (observed.loginId !== challenge.loginId || connection.isClosed) return { kind: 'unconfirmed' };
    const authenticated = await readCodexAccount(connection, signal);
    if (authenticated) return { kind: 'signedIn', account: authenticated };
    return { kind: observed.success ? 'unconfirmed' : 'failed' };
  } catch { return { kind: 'unconfirmed' }; }
  finally {
    if (timer !== undefined) clearTimeout(timer);
    unsubscribe?.();
    try { await connection.dispose(); }
    catch { throw new Error('Account connection cleanup was not confirmed'); }
    finally { active.delete(connection); }
  }
}
