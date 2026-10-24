import type { AccountChallenge, AccountSignInResult, AccountIdentity } from './v1.ts';
function reject(): never { throw new Error('Account executor evidence was not confirmed'); }
function object(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) reject();
  const fields: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) { if (!keys.includes(key)) reject(); fields[key] = item; }
  return fields;
}
// A device code to type on any page, or a browser page that completes the provider's own sign-in on this Mac.
export function parseAccountChallenge(value: unknown): AccountChallenge {
  const browser = value && typeof value === 'object' && (value as { kind?: unknown }).kind === 'browser';
  const row = object(value, browser ? ['kind', 'verificationURL'] : ['kind', 'verificationURL', 'userCode']);
  if (typeof row.verificationURL !== 'string' || row.verificationURL.length > 2048
      || !row.verificationURL.isWellFormed() || /[\x00-\x20\x7f]/.test(row.verificationURL)) reject();
  if (!browser && (row.kind !== 'deviceCode' || typeof row.userCode !== 'string' || row.userCode.length < 4 || row.userCode.length > 32
      || !/^[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*$/.test(row.userCode))) reject();
  const url = new URL(row.verificationURL);
  if (url.protocol !== 'https:' || url.username || url.password || url.hash || (url.search && !browser) || url.port || url.href !== row.verificationURL) reject();
  return browser ? { kind: 'browser', verificationURL: row.verificationURL } : { kind: 'deviceCode', verificationURL: row.verificationURL, userCode: row.userCode as string };
}

export function parseAccountIdentity(value: unknown): AccountIdentity {
  const row = object(value, ['accountId', 'userLabel']);
  if (typeof row.accountId !== 'string' || !/^[A-Za-z0-9_.:-]{1,256}$/.test(row.accountId)
      || typeof row.userLabel !== 'string' || !row.userLabel.isWellFormed() || !row.userLabel.trim()
      || row.userLabel !== row.userLabel.trim() || Buffer.byteLength(row.userLabel) > 320
      || /[\p{Cc}\p{Cf}]/u.test(row.userLabel)) reject();
  return { accountId: row.accountId, userLabel: row.userLabel };
}
export function sameAccount(left: AccountIdentity | undefined, right: AccountIdentity | undefined): boolean {
  return left !== undefined && right !== undefined && left.accountId === right.accountId && left.userLabel === right.userLabel;
}
export function parseAccountSignInResult(value: unknown): AccountSignInResult {
  const row = object(value, ['kind', 'account']);
  if (row.kind === 'signedIn') return { kind: row.kind, ...(row.account === undefined ? {} : { account: parseAccountIdentity(row.account) }) };
  if (row.account === undefined && (row.kind === 'canceled' || row.kind === 'failed' || row.kind === 'unconfirmed')) return { kind: row.kind };
  return reject();
}
