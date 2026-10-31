import assert from 'node:assert/strict';
import { generateKeyPairSync, verify } from 'node:crypto';
import test from 'node:test';
import { distributionCertificates, exactBundle, fetchProfile, token, usableProfile } from './asc-profile.ts';

const now = new Date('2026-10-31T00:00:00Z');
const resource = (id: string, attributes: Record<string, unknown>, type = 'x') => ({ id, type, attributes });

test('token is an ES256 JWT App Store Connect accepts', () => {
  const { privateKey, publicKey } = generateKeyPairSync('ec', { namedCurve: 'P-256' });
  const jwt = token('ABCDE12345', 'issuer-1', privateKey.export({ type: 'pkcs8', format: 'pem' }).toString(), 1_800_000_000);
  const [head, body, signature] = jwt.split('.');
  assert.deepEqual(JSON.parse(Buffer.from(head!, 'base64url').toString()), { alg: 'ES256', kid: 'ABCDE12345', typ: 'JWT' });
  assert.deepEqual(JSON.parse(Buffer.from(body!, 'base64url').toString()), { iss: 'issuer-1', iat: 1_800_000_000, exp: 1_800_000_900, aud: 'appstoreconnect-v1' });
  assert.ok(verify('sha256', Buffer.from(head + '.' + body), { key: publicKey, dsaEncoding: 'ieee-p1363' }, Buffer.from(signature!, 'base64url')));
  assert.throws(() => token('bad', 'issuer', privateKey.export({ type: 'pkcs8', format: 'pem' }).toString()), /10-character/);
});

test('bundle lookup is exact, since the API filter also returns longer IDs', () => {
  const list = [resource('1', { identifier: 'io.mtree.fort' }), resource('2', { identifier: 'io.mtree.fort.host' })];
  assert.equal(exactBundle(list, 'io.mtree.fort').id, '1');
  assert.equal(exactBundle(list, 'io.mtree.fort.host').id, '2');
  assert.throws(() => exactBundle(list, 'io.mtree.other'), /Register the App ID/);
});

test('reuses the latest active profile of the right type that lasts past today', () => {
  const list = [
    resource('dev', { profileType: 'MAC_APP_DEVELOPMENT', profileState: 'ACTIVE', expirationDate: '2027-06-01T00:00:00Z', profileContent: 'AA==' }),
    resource('old', { profileType: 'MAC_APP_STORE', profileState: 'ACTIVE', expirationDate: '2027-01-01T00:00:00Z', profileContent: 'AA==' }),
    resource('new', { profileType: 'MAC_APP_STORE', profileState: 'ACTIVE', expirationDate: '2027-09-01T00:00:00Z', profileContent: 'AA==' }),
    resource('bad', { profileType: 'MAC_APP_STORE', profileState: 'INVALID', expirationDate: '2028-01-01T00:00:00Z', profileContent: 'AA==' }),
    resource('soon', { profileType: 'MAC_APP_STORE', profileState: 'ACTIVE', expirationDate: '2026-10-31T12:00:00Z', profileContent: 'AA==' })];
  assert.equal(usableProfile(list, 'MAC_APP_STORE', now)?.id, 'new');
  assert.equal(usableProfile(list, 'IOS_APP_STORE', now), undefined);
});

test('only valid distribution certificates go into a new profile', () => {
  const list = [resource('a', { certificateType: 'DISTRIBUTION', expirationDate: '2027-01-01T00:00:00Z' }),
    resource('b', { certificateType: 'DEVELOPMENT', expirationDate: '2027-01-01T00:00:00Z' }),
    resource('c', { certificateType: 'MAC_APP_DISTRIBUTION', expirationDate: '2026-01-01T00:00:00Z' })];
  assert.deepEqual(distributionCertificates(list, now).map(item => item.id), ['a']);
});

test('creates a profile when none fits, and reuses one when it does', async () => {
  const calls: string[] = [];
  const reply = (body: unknown) => ({ ok: true, status: 200, json: async () => body });
  const make = (profiles: unknown[]) => async (url: string, init: { method: string; body?: string }) => {
    calls.push(init.method + ' ' + url.replace('https://api.appstoreconnect.apple.com', ''));
    if (url.includes('/v1/bundleIds?')) return reply({ data: [resource('B1', { identifier: 'io.mtree.fort.host' })] });
    if (url.includes('/profiles?')) return reply({ data: profiles });
    if (url.includes('/v1/certificates')) return reply({ data: [resource('C1', { certificateType: 'DISTRIBUTION', expirationDate: '2027-01-01T00:00:00Z' })] });
    const body = JSON.parse(init.body!);
    assert.equal(body.data.attributes.profileType, 'MAC_APP_STORE');
    assert.deepEqual(body.data.relationships.certificates.data, [{ type: 'certificates', id: 'C1' }]);
    assert.deepEqual(body.data.relationships.bundleId.data, { type: 'bundleIds', id: 'B1' });
    return reply({ data: resource('P1', { profileContent: Buffer.from('created').toString('base64') }) });
  };
  assert.equal((await fetchProfile({ bundleId: 'io.mtree.fort.host', type: 'MAC_APP_STORE', jwt: 't', fetch: make([]), now })).toString(), 'created');
  assert.ok(calls.at(-1)?.startsWith('POST /v1/profiles'));
  calls.length = 0;
  const kept = [resource('P0', { profileType: 'MAC_APP_STORE', profileState: 'ACTIVE', expirationDate: '2027-06-01T00:00:00Z', profileContent: Buffer.from('kept').toString('base64') })];
  assert.equal((await fetchProfile({ bundleId: 'io.mtree.fort.host', type: 'MAC_APP_STORE', jwt: 't', fetch: make(kept), now })).toString(), 'kept');
  assert.equal(calls.some(call => call.startsWith('POST')), false);
});
