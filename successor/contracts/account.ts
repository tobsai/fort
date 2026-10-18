import type { AccountChallenge, AccountSignInResult } from './v1.ts';
function reject(): never { throw new Error('Account executor evidence was not confirmed'); }
function object(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) reject();
  const fields: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) { if (!keys.includes(key)) reject(); fields[key] = item; }
  return fields;
}
export function parseAccountChallenge(value: unknown): AccountChallenge {
  const row = object(value, ['kind', 'verificationURL', 'userCode']);
  if (row.kind !== 'deviceCode' || typeof row.verificationURL !== 'string' || row.verificationURL.length > 2048
      || !row.verificationURL.isWellFormed() || /[\x00-\x20\x7f]/.test(row.verificationURL)
      || typeof row.userCode !== 'string' || row.userCode.length < 4 || row.userCode.length > 32
      || !/^[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*$/.test(row.userCode)) reject();
  const url = new URL(row.verificationURL);
  if (url.protocol !== 'https:' || url.username || url.password || url.hash || url.search || url.port || url.href !== row.verificationURL) reject();
  return { kind: 'deviceCode', verificationURL: row.verificationURL, userCode: row.userCode };
}

export function parseAccountSignInResult(value: unknown): AccountSignInResult {
  const row = object(value, ['kind']);
  if (row.kind === 'signedIn' || row.kind === 'canceled' || row.kind === 'failed' || row.kind === 'unconfirmed') return { kind: row.kind };
  return reject();
}
