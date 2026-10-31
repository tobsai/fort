import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { HELPER, RUNTIMES, clientEntitlements, entitlementsPlist, helperEntitlements, helperExceptions, hostConfiguration, profileProblems, signatureProblems, signingPlan } from './mac-signing-plan.ts';

const config = hostConfiguration({ format: '1', clientIdentifier: 'io.mtree.fort', serviceIdentifier: 'io.mtree.fort.host', signingTeam: 'T3JB5MYZ93', appGroup: 'group.io.mtree.fort' });
const claude = { 'com.apple.security.temporary-exception.files.home-relative-path.read-only': ['/.local/share/claude/'] };

test('signs inside out: libraries, runtimes, helper, then the app', () => {
  const plan = signingPlan(config, ['libz.dylib', 'libb.dylib'], 'distribution', claude);
  assert.deepEqual(plan.map(step => step.kind), ['library', 'library', 'runtime', 'runtime', 'runtime', 'helper', 'client']);
  assert.deepEqual(plan.slice(0, 2).map(step => step.path), [HELPER + '/Contents/Frameworks/libb.dylib', HELPER + '/Contents/Frameworks/libz.dylib']);
  assert.equal(plan.at(-2)?.path, 'Contents/Helpers/FortHost.app');
  assert.equal(plan.at(-1)?.path, '');
  assert.throws(() => signingPlan(config, ['../escape.dylib'], 'distribution'), /Unexpected library/);
});

test('runtimes match what MacHostProgram.swift accepts: <service>.<name>, sandbox + inherit only', () => {
  const runtimes = signingPlan(config, [], 'distribution').filter(step => step.kind === 'runtime');
  assert.deepEqual(runtimes.map(step => [step.identifier, step.path]), [
    ['io.mtree.fort.host.node', HELPER + '/Contents/MacOS/node'],
    ['io.mtree.fort.host.guardian', HELPER + '/Contents/MacOS/FortProcessGuardian'],
    ['io.mtree.fort.host.codex', HELPER + '/Contents/MacOS/codex']]);
  for (const step of runtimes) {
    assert.deepEqual(step.entitlements, { 'com.apple.security.app-sandbox': true, 'com.apple.security.inherit': true });
    assert.equal(step.profile, null);
  }
  // Keep the plan's paths and suffixes in step with the Swift side.
  const swift = readFileSync(fileURLToPath(new URL('../apple/FortKit/Sources/FortKit/MacHostProgram.swift', import.meta.url)), 'utf8');
  for (const [suffix, path] of RUNTIMES) {
    assert.match(swift, new RegExp(`\\(\\w+,"${suffix}",`));
    assert.ok(swift.includes(`"${path.slice(HELPER.length + 1)}"`), path);
  }
  assert.ok(swift.includes('["com.apple.security.app-sandbox","com.apple.security.inherit"]'));
});

test('app and helper each get their own sandbox, one shared group, no inherit, and a profile', () => {
  const helper = helperEntitlements(config, 'distribution', claude), client = clientEntitlements(config, 'distribution');
  for (const entitlements of [helper, client]) {
    assert.equal(entitlements['com.apple.security.app-sandbox'], true);
    assert.deepEqual(entitlements['com.apple.security.application-groups'], ['group.io.mtree.fort']);
    assert.equal('com.apple.security.inherit' in entitlements, false);
    assert.equal(entitlements['com.apple.developer.team-identifier'], 'T3JB5MYZ93');
  }
  assert.equal(helper['com.apple.application-identifier'], 'T3JB5MYZ93.io.mtree.fort.host');
  assert.equal(client['com.apple.application-identifier'], 'T3JB5MYZ93.io.mtree.fort');
  assert.equal(helper['com.apple.security.network.server'], true);
  assert.equal('com.apple.security.network.server' in client, false);
  assert.deepEqual(helper['com.apple.security.temporary-exception.files.home-relative-path.read-only'], ['/.local/share/claude/']);
  const plan = signingPlan(config, [], 'distribution');
  assert.deepEqual(plan.filter(step => step.profile).map(step => [step.kind, step.profile]), [['helper', 'helper'], ['client', 'client']]);
  assert.equal(signingPlan(config, [], 'adhoc').some(step => step.profile), false);
  assert.equal('com.apple.application-identifier' in clientEntitlements(config, 'adhoc'), false);
});

test('the helper exception comes from the reviewed FortHost.entitlements and nothing else is allowed', () => {
  const source = readFileSync(fileURLToPath(new URL('../apple/Host/FortHost.entitlements', import.meta.url)), 'utf8');
  assert.ok(source.includes('com.apple.security.temporary-exception.files.home-relative-path.read-only'));
  assert.ok(source.includes('<string>/.local/share/claude/</string>'));
  assert.throws(() => helperExceptions({ 'com.apple.security.files.all': true }), /Unreviewed helper entitlement/);
  assert.throws(() => helperExceptions({ 'com.apple.security.temporary-exception.files.home-relative-path.read-only': ['/../'] }), /Invalid helper exception/);
});

test('host configuration follows mac-application.ts rules', () => {
  assert.throws(() => hostConfiguration({ ...config, appGroup: 'T3JB5MYZ93.io.mtree.fort' }), /Invalid host configuration/);
  assert.throws(() => hostConfiguration({ ...config, serviceIdentifier: config.clientIdentifier }), /Invalid host configuration/);
  assert.throws(() => hostConfiguration({ ...config, signingTeam: 'short' }), /Invalid host configuration/);
  assert.throws(() => hostConfiguration({ ...config, extra: 'x' }), /Invalid host configuration/);
});

test('entitlements render as a sorted XML property list', () => {
  const xml = entitlementsPlist({ 'b.key': ['x&y'], 'a.key': true, 'c.key': 'T<1>' });
  assert.ok(xml.indexOf('a.key') < xml.indexOf('b.key') && xml.indexOf('b.key') < xml.indexOf('c.key'));
  assert.ok(xml.includes('<true/>') && xml.includes('<string>x&amp;y</string>') && xml.includes('<string>T&lt;1&gt;</string>'));
});

test('signature check catches wrong identifier, team and entitlements', () => {
  const plan = signingPlan(config, [], 'distribution'), runtime = plan.find(step => step.identifier === 'io.mtree.fort.host.codex')!;
  const good = { identifier: 'io.mtree.fort.host.codex', team: 'T3JB5MYZ93', entitlements: { 'com.apple.security.app-sandbox': true, 'com.apple.security.inherit': true } };
  assert.deepEqual(signatureProblems(runtime, good, config, 'distribution'), []);
  assert.equal(signatureProblems(runtime, { ...good, identifier: 'codex' }, config, 'distribution').length, 1);
  assert.equal(signatureProblems(runtime, { ...good, team: null }, config, 'distribution').length, 1);
  assert.deepEqual(signatureProblems(runtime, { ...good, team: null }, config, 'adhoc'), []);
  assert.equal(signatureProblems(runtime, { ...good, entitlements: { ...good.entitlements, 'com.apple.security.cs.allow-jit': true } }, config, 'distribution').length, 1);
  assert.equal(signatureProblems(runtime, { ...good, entitlements: { 'com.apple.security.app-sandbox': true, 'com.apple.security.inherit': false } }, config, 'distribution').length, 1);
  const library = signingPlan(config, ['libx.dylib'], 'distribution')[0]!;
  assert.deepEqual(signatureProblems(library, { identifier: 'libx', team: 'T3JB5MYZ93', entitlements: {} }, config, 'distribution'), []);
});

test('profile check wants a current Mac App Store profile for the right bundle and group', () => {
  const now = new Date('2026-10-31T00:00:00Z');
  const profile = { TeamIdentifier: ['T3JB5MYZ93'], Platform: ['OSX'], ExpirationDate: '2027-10-31T00:00:00Z',
    Entitlements: { 'com.apple.application-identifier': 'T3JB5MYZ93.io.mtree.fort.host', 'com.apple.security.application-groups': ['group.io.mtree.fort'] } };
  assert.deepEqual(profileProblems(profile, 'io.mtree.fort.host', config, now), []);
  assert.equal(profileProblems(profile, 'io.mtree.fort', config, now).length, 1);
  assert.equal(profileProblems({ ...profile, ProvisionedDevices: true }, 'io.mtree.fort.host', config, now).length, 1);
  assert.equal(profileProblems({ ...profile, ProvisionsAllDevices: true }, 'io.mtree.fort.host', config, now).length, 1);
  assert.equal(profileProblems({ ...profile, Platform: ['iOS'] }, 'io.mtree.fort.host', config, now).length, 1);
  assert.equal(profileProblems({ ...profile, ExpirationDate: '2026-01-01T00:00:00Z' }, 'io.mtree.fort.host', config, now).length, 1);
  assert.equal(profileProblems({ ...profile, Entitlements: { ...profile.Entitlements, 'com.apple.security.application-groups': [] } }, 'io.mtree.fort.host', config, now).length, 1);
});
