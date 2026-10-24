import assert from 'node:assert/strict';
import test from 'node:test';
import { parseAccountChallenge } from './account.ts';

const authorizationURL = 'https://auth.example.invalid/authorize?client_id=c&redirect_uri=http%3A%2F%2F127.0.0.1%3A49152%2Fauth%2Fcallback&state=s';

test('a browser authorization challenge keeps its exact query', () => {
  assert.deepEqual(parseAccountChallenge({ kind: 'browserAuthorization', authorizationURL }), { kind: 'browserAuthorization', authorizationURL });
});

test('a device code challenge still refuses a query', () => {
  assert.deepEqual(parseAccountChallenge({ kind: 'deviceCode', verificationURL: 'https://device.example.invalid/verify', userCode: 'FORT-TEST' }),
    { kind: 'deviceCode', verificationURL: 'https://device.example.invalid/verify', userCode: 'FORT-TEST' });
  assert.throws(() => parseAccountChallenge({ kind: 'deviceCode', verificationURL: 'https://device.example.invalid/verify?x=1', userCode: 'FORT-TEST' }));
});

for (const [name, value] of Object.entries({
  insecure: { kind: 'browserAuthorization', authorizationURL: authorizationURL.replace('https:', 'http:') },
  credentials: { kind: 'browserAuthorization', authorizationURL: authorizationURL.replace('https://', 'https://user:secret@') },
  fragment: { kind: 'browserAuthorization', authorizationURL: authorizationURL + '#token' },
  port: { kind: 'browserAuthorization', authorizationURL: authorizationURL.replace('.invalid/', '.invalid:8443/') },
  unnormalized: { kind: 'browserAuthorization', authorizationURL: 'HTTPS://auth.example.invalid/authorize?x=1' },
  spaces: { kind: 'browserAuthorization', authorizationURL: authorizationURL + ' ' },
  oversized: { kind: 'browserAuthorization', authorizationURL: 'https://auth.example.invalid/?' + 'a'.repeat(8192) },
  extraField: { kind: 'browserAuthorization', authorizationURL, userCode: 'FORT-TEST' },
  missingURL: { kind: 'browserAuthorization' },
  mixedKind: { kind: 'browserAuthorization', verificationURL: 'https://device.example.invalid/verify', userCode: 'FORT-TEST' }
})) test('a browser authorization challenge refuses ' + name, () => { assert.throws(() => parseAccountChallenge(value)); });
