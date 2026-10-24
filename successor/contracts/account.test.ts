import assert from 'node:assert/strict';
import test from 'node:test';
import { parseAccountChallenge } from './account.ts';

const login = 'https://claude.com/cai/oauth/authorize?code=true&client_id=fixture&response_type=code&redirect_uri=http%3A%2F%2Flocalhost%3A54545%2Fcallback&state=abc';

test('a browser sign-in challenge carries only an exact https page, query allowed', () => {
  assert.deepEqual(parseAccountChallenge({ kind: 'browser', verificationURL: login }), { kind: 'browser', verificationURL: login });
  for (const verificationURL of ['http://claude.com/cai/oauth/authorize?a=1', 'https://user@claude.com/x?a=1', 'https://claude.com:8443/x',
    'https://claude.com/x#frag', 'https://claude.com/x?a= 1', 'https://claude.com/' + 'a'.repeat(2048)])
    assert.throws(() => parseAccountChallenge({ kind: 'browser', verificationURL }), /not confirmed/, verificationURL);
  assert.throws(() => parseAccountChallenge({ kind: 'browser', verificationURL: login, userCode: 'FORT-TEST' }), /not confirmed/);
});

test('a device-code challenge still refuses query strings', () => {
  assert.deepEqual(parseAccountChallenge({ kind: 'deviceCode', verificationURL: 'https://auth.openai.com/codex/device', userCode: 'FORT-TEST' }),
    { kind: 'deviceCode', verificationURL: 'https://auth.openai.com/codex/device', userCode: 'FORT-TEST' });
  assert.throws(() => parseAccountChallenge({ kind: 'deviceCode', verificationURL: 'https://auth.openai.com/codex/device?x=1', userCode: 'FORT-TEST' }), /not confirmed/);
  assert.throws(() => parseAccountChallenge({ kind: 'deviceCode', verificationURL: 'https://auth.openai.com/codex/device' }), /not confirmed/);
});
