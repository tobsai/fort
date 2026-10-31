// Gets an App Store provisioning profile for one bundle ID from App Store Connect, using the same API key
// as the upload, and writes it to a file. Reuses an active, unexpired profile of the right type; if none
// exists, creates one that includes every valid distribution certificate on the team. The bundle ID itself
// (and its App Group) must already be registered: the API can't assign App Groups.
//
//   node release/asc-profile.ts --bundle-id io.mtree.fort.host --type MAC_APP_STORE --output FortHost.provisionprofile
//
// Reads ASC_KEY_ID, ASC_ISSUER_ID and ASC_KEY_PATH from the environment. Never prints the key or the token.
import { createPrivateKey, sign } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

export type ProfileType = 'MAC_APP_STORE' | 'IOS_APP_STORE';
type Resource = { id: string; type: string; attributes: Record<string, unknown> };
type Fetch = (url: string, init: { method: string; headers: Record<string, string>; body?: string }) => Promise<{ ok: boolean; status: number; json(): Promise<unknown> }>;
const API = 'https://api.appstoreconnect.apple.com';
const base64url = (data: Buffer | string) => Buffer.from(data).toString('base64url');

export function token(keyId: string, issuer: string, privateKeyPem: string, now = Math.floor(Date.now() / 1000)): string {
  if (!/^[A-Z0-9]{10}$/.test(keyId)) throw new Error('ASC_KEY_ID should be the 10-character key ID');
  const head = base64url(JSON.stringify({ alg: 'ES256', kid: keyId, typ: 'JWT' }));
  const body = base64url(JSON.stringify({ iss: issuer, iat: now, exp: now + 15 * 60, aud: 'appstoreconnect-v1' }));
  const signature = sign('sha256', Buffer.from(head + '.' + body), { key: createPrivateKey(privateKeyPem), dsaEncoding: 'ieee-p1363' });
  return head + '.' + body + '.' + base64url(signature);
}

const resources = (value: unknown): Resource[] => {
  const data = (value as { data?: unknown })?.data;
  if (!Array.isArray(data)) throw new Error('Unexpected App Store Connect response');
  return data as Resource[];
};

export function exactBundle(list: Resource[], identifier: string): Resource {
  const matches = list.filter(item => item.attributes.identifier === identifier);
  if (matches.length !== 1) throw new Error(`Register the App ID ${identifier} in the Apple developer account first (found ${matches.length})`);
  return matches[0]!;
}

export function usableProfile(list: Resource[], type: ProfileType, now: Date): Resource | undefined {
  return list.filter(item => item.attributes.profileType === type && item.attributes.profileState === 'ACTIVE'
      && typeof item.attributes.expirationDate === 'string' && Date.parse(item.attributes.expirationDate) > now.getTime() + 24 * 3600_000
      && typeof item.attributes.profileContent === 'string')
    .sort((a, b) => Date.parse(String(b.attributes.expirationDate)) - Date.parse(String(a.attributes.expirationDate)))[0];
}

export function distributionCertificates(list: Resource[], now: Date): Resource[] {
  return list.filter(item => ['DISTRIBUTION', 'MAC_APP_DISTRIBUTION', 'IOS_DISTRIBUTION'].includes(String(item.attributes.certificateType))
    && typeof item.attributes.expirationDate === 'string' && Date.parse(item.attributes.expirationDate) > now.getTime());
}

export async function fetchProfile(options: { bundleId: string; type: ProfileType; jwt: string; fetch?: Fetch; now?: Date }): Promise<Buffer> {
  const call = options.fetch ?? (fetch as unknown as Fetch), now = options.now ?? new Date();
  const request = async (path: string, method = 'GET', body?: unknown) => {
    const response = await call(API + path, { method, headers: { Authorization: 'Bearer ' + options.jwt, 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
    if (!response.ok) throw new Error(`App Store Connect ${method} ${path.split('?')[0]} failed with ${response.status}`);
    return response.json();
  };
  const bundle = exactBundle(resources(await request('/v1/bundleIds?limit=200&filter[identifier]=' + encodeURIComponent(options.bundleId))), options.bundleId);
  const existing = usableProfile(resources(await request(`/v1/bundleIds/${bundle.id}/profiles?limit=200`)), options.type, now);
  if (existing) return Buffer.from(String(existing.attributes.profileContent), 'base64');
  const certificates = distributionCertificates(resources(await request('/v1/certificates?limit=200')), now);
  if (!certificates.length) throw new Error('No valid distribution certificate on the team; create an Apple Distribution certificate first');
  const created = await request('/v1/profiles', 'POST', { data: { type: 'profiles',
    attributes: { name: `Fort ${options.bundleId} ${options.type} ${now.toISOString().slice(0, 10)}`, profileType: options.type },
    relationships: { bundleId: { data: { type: 'bundleIds', id: bundle.id } }, certificates: { data: certificates.map(item => ({ type: 'certificates', id: item.id })) } } } });
  const content = (created as { data?: Resource })?.data?.attributes?.profileContent;
  if (typeof content !== 'string') throw new Error('App Store Connect returned no profile content');
  return Buffer.from(content, 'base64');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    const { values } = parseArgs({ strict: true, options: { 'bundle-id': { type: 'string' }, type: { type: 'string' }, output: { type: 'string' } } });
    const type = values.type;
    if (type !== 'MAC_APP_STORE' && type !== 'IOS_APP_STORE') throw new Error('Choose --type MAC_APP_STORE or IOS_APP_STORE');
    if (!values['bundle-id'] || !values.output) throw new Error('Give --bundle-id and --output');
    const { ASC_KEY_ID: keyId, ASC_ISSUER_ID: issuer, ASC_KEY_PATH: keyPath } = process.env;
    if (!keyId || !issuer || !keyPath) throw new Error('Set ASC_KEY_ID, ASC_ISSUER_ID and ASC_KEY_PATH');
    const profile = await fetchProfile({ bundleId: values['bundle-id'], type, jwt: token(keyId, issuer, readFileSync(keyPath, 'utf8')) });
    writeFileSync(values.output, profile, { mode: 0o644, flag: 'wx' });
    process.stdout.write(`Wrote ${type} profile for ${values['bundle-id']}\n`);
  } catch (error) { process.stderr.write((error instanceof Error ? error.message : 'Profile fetch failed') + '\n'); process.exitCode = 1; }
}
