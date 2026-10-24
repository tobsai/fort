import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomBytes, randomUUID } from 'node:crypto';
import { DatabaseSync } from 'node:sqlite';
import { runnerEnrollment, RunnerEnrollmentError } from './runner-enrollment.ts';

const secret = () => randomBytes(32).toString('base64url');
async function fixture(t: TestContext) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-runners-')), path = join(directory, 'hub.sqlite');
  let now = Date.now(), db = new DatabaseSync(path);
  const revoked: string[] = [];
  const open = () => runnerEnrollment(db, { now: () => now, revoked: id => revoked.push(id) });
  let runners = open();
  t.after(async () => { db.close(); await rm(directory, { recursive: true, force: true }); });
  const offer = () => ({ invitationId: randomUUID(), code: secret() });
  const mac = (invitation: ReturnType<typeof offer>, runnerId = 'mac-mini') => ({ ...invitation, runnerId, runnerToken: secret(), runnerName: 'Mac mini' });
  return { offer, mac, revoked, path, get runners() { return runners; }, get db() { return db; },
    advance(ms: number) { now += ms; }, reopen() { db.close(); db = new DatabaseSync(path); runners = open(); } };
}
const refused = (code: string) => (error: unknown) => error instanceof RunnerEnrollmentError && error.code === code;

test('an invited Mac enrolls once and its token then identifies it, stored only as a digest', async t => {
  const h = await fixture(t), invitation = h.offer();
  assert.equal(h.runners.invite(invitation).invitationId, invitation.invitationId);
  const mac = h.mac(invitation);
  assert.deepEqual(h.runners.join(mac), { runnerId: 'mac-mini', runnerName: 'Mac mini' });
  assert.equal(h.runners.authenticate(mac.runnerToken), 'mac-mini');
  assert.equal(h.runners.authenticate(secret()), undefined);
  assert.deepEqual(h.runners.list().map(runner => [runner.runnerId, runner.runnerName]), [['mac-mini', 'Mac mini']]);
  const stored = JSON.stringify([...h.db.prepare('SELECT * FROM runners').all(), ...h.db.prepare('SELECT * FROM runner_invitations').all()]);
  assert.ok(!stored.includes(mac.runnerToken) && !stored.includes(mac.code));
  h.reopen();
  assert.equal(h.runners.authenticate(mac.runnerToken), 'mac-mini');
});

test('an exact retry returns the same enrollment, and an invitation enrolls only one Mac', async t => {
  const h = await fixture(t), invitation = h.offer(), mac = h.mac(invitation);
  h.runners.invite(invitation);
  h.runners.join(mac);
  assert.deepEqual(h.runners.join(mac), { runnerId: 'mac-mini', runnerName: 'Mac mini' });
  assert.throws(() => h.runners.join(h.mac(invitation, 'mac-studio')), refused('invitationDenied'));
  assert.throws(() => h.runners.join({ ...mac, runnerToken: secret() }), refused('invitationDenied'));
  assert.equal(h.runners.list().length, 1);
});

test('an expired, unknown or mismatched invitation enrolls nothing', async t => {
  const h = await fixture(t), invitation = h.offer();
  h.runners.invite(invitation);
  assert.throws(() => h.runners.join(h.mac({ ...invitation, code: secret() })), refused('invitationDenied'));
  assert.throws(() => h.runners.join(h.mac(h.offer())), refused('invitationDenied'));
  h.advance(300_001);
  assert.throws(() => h.runners.join(h.mac(invitation)), refused('invitationDenied'));
  assert.deepEqual(h.runners.list(), []);
});

test('enrollment refuses reused secrets, malformed fields and a name already enrolled', async t => {
  const h = await fixture(t), first = h.offer();
  h.runners.invite(first);
  assert.throws(() => h.runners.invite({ ...h.offer(), code: first.code }), refused('invitationReused'));
  assert.throws(() => h.runners.invite({ invitationId: 'bad id', code: secret() }), refused('invalidEnrollment'));
  assert.throws(() => h.runners.join({ ...h.mac(first), runnerToken: first.code }), refused('credentialReused'));
  assert.throws(() => h.runners.join({ ...h.mac(first), runnerName: '' }), refused('invalidEnrollment'));
  assert.throws(() => h.runners.join({ ...h.mac(first), extra: true }), refused('invalidEnrollment'));
  h.runners.join(h.mac(first));
  const second = h.offer();
  h.runners.invite(second);
  assert.throws(() => h.runners.join(h.mac(second)), refused('runnerEnrolled'));
});

test('revoking a Mac removes its token at once and tells the hub to drop it', async t => {
  const h = await fixture(t), invitation = h.offer(), mac = h.mac(invitation);
  h.runners.invite(invitation); h.runners.join(mac);
  assert.equal(h.runners.revoke('mac-mini'), true);
  assert.equal(h.runners.authenticate(mac.runnerToken), undefined);
  assert.deepEqual(h.revoked, ['mac-mini']);
  assert.equal(h.runners.revoke('mac-mini'), false);
  assert.deepEqual(h.revoked, ['mac-mini']);
  assert.throws(() => h.runners.join(mac), refused('invitationDenied'));
  const again = h.offer();
  h.runners.invite(again);
  assert.equal(h.runners.join(h.mac(again)).runnerId, 'mac-mini');
});
