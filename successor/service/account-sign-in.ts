import type { DatabaseSync } from 'node:sqlite';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import type { AccountChallenge, AccountIdentity, AccountSignInResult, AccountSignInStatus } from '../contracts/v1.ts';
import { parseAccountChallenge, parseAccountSignInResult, parseAccountIdentity, sameAccount } from '../contracts/account.ts';

type Attempt = { revision: number; operationId: string; deviceId: string; recoveryEpoch: string; expiresAt: number }
  & ({ state: 'active' | 'canceling' } | { state: 'finished'; result: AccountSignInResult; selected?: boolean });
type AccountWork = { controller: AbortController; task: Promise<void>; challenge?: AccountChallenge };
type Scope = { deviceId: string; recoveryEpoch: string; running: boolean };
type Choice = { installationId: string; recoveryEpoch: string; provider: string; hostId: string; account: AccountIdentity };
type Options = { identity: Pick<Choice, 'installationId' | 'provider' | 'hostId'>; runtime: ChatRuntime | undefined; now(): number; scope(): Scope | undefined;
  commit<T>(action: () => T): T; changed(): void; settled(): void; keys?: { attempt: string; choice: string } };
export class AccountSignInError extends Error {
  readonly status: number; readonly code: string;
  constructor(status: number, code: string, message: string) { super(message); this.status = status; this.code = code; }
}
function reject(status = 409): never { throw new AccountSignInError(status, 'accountOperationUnavailable', 'Refresh the exact account operation before trying again.'); }
function fields(value: unknown, allowed: string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => !allowed.includes(key))) reject(400);
  return Object.fromEntries(Object.entries(value));
}
function id(value: unknown): string { if (typeof value !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(value)) reject(400); return value; }
function integer(value: unknown, minimum: number): number { if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < minimum) reject(400); return value; }

export function accountSignIn(db: DatabaseSync, options: Options) {
  const runtime = options.runtime, signIn = runtime?.signIn?.bind(runtime);
  // Spec 064: each runtime on a host keeps its own attempt and choice. The primary keeps the original keys.
  const keys = options.keys ?? { attempt: 'accountSignIn', choice: 'accountChoice' };
  let current: Attempt | undefined;
  let active: AccountWork | undefined;
  let choice: Choice | undefined;
  const save = (record: Attempt, confirmed?: Choice) => { options.commit(() => {
    db.prepare('INSERT OR REPLACE INTO metadata VALUES (?,?)').run(keys.attempt, JSON.stringify(record));
    if (confirmed) db.prepare('INSERT OR REPLACE INTO metadata VALUES (?,?)').run(keys.choice, JSON.stringify(confirmed));
    options.changed();
  }); current = record; if (confirmed) choice = confirmed; };
  const stored = db.prepare('SELECT data FROM metadata WHERE key=?').get(keys.attempt);
  if (stored) {
    const row = fields(JSON.parse(String(stored.data)), ['revision', 'operationId', 'deviceId', 'recoveryEpoch', 'expiresAt', 'state', 'result', 'selected']);
    const identity = { revision: integer(row.revision, 1), operationId: id(row.operationId), deviceId: id(row.deviceId), recoveryEpoch: id(row.recoveryEpoch), expiresAt: integer(row.expiresAt, 1) };
    if (row.state === 'finished') {
      const result = parseAccountSignInResult(row.result);
      if (row.selected !== undefined && (typeof row.selected !== 'boolean' || result.kind !== 'signedIn' || !result.account)) reject();
      current = { ...identity, state: 'finished', result, ...(row.selected === true ? { selected: true } : {}) };
    }
    else if ((row.state === 'active' || row.state === 'canceling') && !Object.hasOwn(row, 'result') && !Object.hasOwn(row, 'selected')) save({ ...identity, state: 'finished', result: { kind: 'unconfirmed' } });
    else reject();
  }
  const storedChoice = db.prepare('SELECT data FROM metadata WHERE key=?').get(keys.choice);
  if (storedChoice) {
    try {
      const row = fields(JSON.parse(String(storedChoice.data)), ['installationId', 'recoveryEpoch', 'provider', 'hostId', 'account']);
      choice = { installationId: id(row.installationId), recoveryEpoch: id(row.recoveryEpoch), provider: id(row.provider), hostId: id(row.hostId), account: parseAccountIdentity(row.account) };
    } catch { choice = undefined; }
  }
  function retainLegacyChoice() {
    if (!storedChoice && !choice && current
        && current.state === 'finished' && current.selected && current.result.kind === 'signedIn' && current.result.account)
      save(current, { ...options.identity, recoveryEpoch: current.recoveryEpoch, account: current.result.account });
  }
  function matchesChoice(account: AccountIdentity | undefined, recoveryEpoch: string): boolean {
    return choice !== undefined && choice.installationId === options.identity.installationId && choice.recoveryEpoch === recoveryEpoch
      && choice.provider === options.identity.provider && choice.hostId === options.identity.hostId && sameAccount(choice.account, account);
  }
  function authorize(deviceId: string): Scope {
    const scope = options.scope();
    if (!scope || scope.deviceId !== deviceId) throw new AccountSignInError(403, 'hostOnly', 'Use the owning Mac for provider sign-in.');
    return scope;
  }
  function status(deviceId: string): AccountSignInStatus {
    const scope = authorize(deviceId), revision = current?.revision ?? 0;
    if (!signIn) return { state: 'unavailable', revision };
    if (!current || current.deviceId !== deviceId || current.recoveryEpoch !== scope.recoveryEpoch) return { state: 'idle', revision };
    const identity = { revision, operationId: current.operationId, expiresAt: current.expiresAt };
    if (current.state === 'finished') return { ...identity, state: 'finished', result: parseAccountSignInResult(current.result),
      ...(current.result.kind === 'signedIn' && current.result.account ? { selected: current.selected === true } : {}) };
    if (current.state === 'canceling' || active?.controller.signal.aborted || options.now() >= current.expiresAt) return { ...identity, state: 'canceling' };
    return active?.challenge ? { ...identity, state: 'challenge', challenge: { ...active.challenge } } : { ...identity, state: 'starting' };
  }
  function start(deviceId: string, value: unknown): AccountSignInStatus {
    const scope = authorize(deviceId), row = fields(value, ['operationId', 'expectedRevision', 'recoveryEpoch', 'expiresAt']);
    const operationId = id(row.operationId), expectedRevision = integer(row.expectedRevision, 0), recoveryEpoch = id(row.recoveryEpoch), expiresAt = integer(row.expiresAt, 1);
    if (recoveryEpoch !== scope.recoveryEpoch) reject();
    if (current?.operationId === operationId) {
      if (current.deviceId !== deviceId || current.recoveryEpoch !== recoveryEpoch || current.revision !== expectedRevision + 1 || current.expiresAt !== expiresAt) reject();
      return status(deviceId);
    }
    if (!signIn || !scope.running || active || expectedRevision !== (current?.revision ?? 0) || expectedRevision >= Number.MAX_SAFE_INTEGER
        || expiresAt <= options.now() || expiresAt > options.now() + 115_000) reject();
    retainLegacyChoice();
    const record: Attempt = { revision: expectedRevision + 1, operationId, deviceId, recoveryEpoch, expiresAt, state: 'active' };
    save(record);
    const operation: AccountWork = { controller: new AbortController(), task: Promise.resolve() };
    active = operation;
    const timer = setTimeout(() => operation.controller.abort(), Math.max(1, expiresAt - options.now()));
    operation.task = Promise.resolve().then(async () => {
      let result: AccountSignInResult = { kind: 'unconfirmed' };
      try {
        if (operation.controller.signal.aborted) return;
        result = parseAccountSignInResult(await signIn({ durationMs: Math.max(1, expiresAt - options.now()), onChallenge: value => {
          const selected = options.scope();
          if (active !== operation || operation.controller.signal.aborted || operation.challenge || selected?.deviceId !== deviceId || selected.recoveryEpoch !== recoveryEpoch || options.now() >= expiresAt) reject();
          operation.challenge = parseAccountChallenge(value); options.commit(options.changed);
        } }, operation.controller.signal));
      } catch { result = { kind: 'unconfirmed' }; }
      finally {
        clearTimeout(timer); operation.challenge = undefined;
        if (active === operation) {
          const scope = options.scope();
          const account = result.kind === 'signedIn' ? result.account : undefined;
          const first = !storedChoice && !choice && !db.prepare("SELECT 1 FROM entities WHERE kind='runs' LIMIT 1").get();
          const selected = account && !operation.controller.signal.aborted && options.now() < expiresAt && scope?.running
            && scope.deviceId === deviceId && scope.recoveryEpoch === recoveryEpoch
            && (first || matchesChoice(account, recoveryEpoch));
          save({ ...record, state: 'finished', result, ...(selected ? { selected: true } : {}) },
            selected && first ? { ...options.identity, recoveryEpoch, account } : undefined);
          active = undefined; options.settled();
        }
      }
    });
    void operation.task.catch(() => {});
    return status(deviceId);
  }
  async function cancel(deviceId: string, value: unknown): Promise<AccountSignInStatus> {
    const scope = authorize(deviceId), row = fields(value, ['operationId', 'revision', 'recoveryEpoch']);
    if (!current || id(row.operationId) !== current.operationId || integer(row.revision, 1) !== current.revision
        || id(row.recoveryEpoch) !== scope.recoveryEpoch || current.recoveryEpoch !== scope.recoveryEpoch || current.deviceId !== deviceId) reject();
    if (current.state !== 'finished' && active) {
      if (current.state !== 'canceling') save({ ...current, state: 'canceling' });
      active.controller.abort(); await active.task;
    }
    if (current.operationId !== row.operationId || current.revision !== row.revision) reject();
    return status(deviceId);
  }
  function selected() {
    const scope = options.scope();
    return scope && current?.deviceId === scope.deviceId && current.recoveryEpoch === scope.recoveryEpoch
      && current.state === 'finished' && current.selected && current.result.kind === 'signedIn' && current.result.account
      ? { ...current.result.account } : undefined;
  }
  function select(deviceId: string, value: unknown): AccountSignInStatus {
    const scope = authorize(deviceId), row = fields(value, ['operationId', 'revision', 'recoveryEpoch', 'account']);
    let account;
    try { account = parseAccountIdentity(row.account); } catch { reject(400); }
    if (!scope.running || active || !current || current.state !== 'finished' || current.result.kind !== 'signedIn'
        || current.deviceId !== deviceId || current.recoveryEpoch !== scope.recoveryEpoch || id(row.recoveryEpoch) !== scope.recoveryEpoch
        || id(row.operationId) !== current.operationId || integer(row.revision, 1) !== current.revision || !sameAccount(account, current.result.account)) reject();
    if (!current.selected || !matchesChoice(account, scope.recoveryEpoch))
      save({ ...current, selected: true }, { ...options.identity, recoveryEpoch: scope.recoveryEpoch, account });
    return status(deviceId);
  }
  return { status, start, cancel, select, selected, get busy() { return active !== undefined; },
    async stop() { const operation = active; if (operation) { operation.controller.abort(); await operation.task; } } };
}
