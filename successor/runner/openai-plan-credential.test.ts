import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, readFile, writeFile, chmod, symlink, link, stat, mkdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { ensureHostId, readPlanCredential, writePlanCredential, deletePlanCredential, type PlanCredential } from './openai-plan-credential.ts';

const credential: PlanCredential = { issuer: 'https://auth.openai.com', subject: 'user-synthetic', clientId: 'oaiapp_synthetic', hostId: 'urn:uuid:00000000-0000-4000-8000-000000000000', idToken: 'id.synthetic.token', accessToken: 'access-synthetic', refreshToken: 'refresh-synthetic', expiresAt: 1_900_000_000_000, scopes: ['chatgpt.tokens.use.direct', 'email', 'offline_access', 'openid', 'profile', 'resource.invoke'] };
const privateDirectory = async (t: { after(fn: () => unknown): void }) => {
  const root = await mkdtemp(join(tmpdir(), 'fort-openai-plan-')); t.after(() => rm(root, { recursive: true, force: true }));
  const state = join(root, 'state'); await mkdir(state, { mode: 0o700 }); return state;
};

test('a written credential is owner-only, round-trips and is replaced atomically', async t => {
  const state = await privateDirectory(t);
  assert.equal(await readPlanCredential(state), undefined);
  await writePlanCredential(state, credential);
  assert.equal((await stat(join(state, 'openai-plan.json'))).mode & 0o777, 0o600);
  assert.deepEqual(await readPlanCredential(state), credential);
  const refreshed = { ...credential, accessToken: 'access-two', refreshToken: 'refresh-two', expiresAt: credential.expiresAt + 3_600_000 };
  await writePlanCredential(state, refreshed);
  assert.deepEqual(await readPlanCredential(state), refreshed);
});

test('the host identifier is opaque, stable and not part of any credential file', async t => {
  const state = await privateDirectory(t);
  const first = await ensureHostId(state);
  assert.match(first, /^urn:uuid:[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  assert.equal(await ensureHostId(state), first);
  assert.equal(await readPlanCredential(state), undefined);
});

test('unsafe directories and files are refused without repair', async t => {
  const state = await privateDirectory(t);
  await writePlanCredential(state, credential);
  const file = join(state, 'openai-plan.json');
  await chmod(file, 0o644);
  await assert.rejects(readPlanCredential(state), /private/);
  await chmod(file, 0o600);
  await link(file, join(state, 'second-link'));
  await assert.rejects(readPlanCredential(state), /private/);
  await rm(join(state, 'second-link'));
  await rm(file); await symlink(join(state, 'elsewhere'), file);
  await assert.rejects(readPlanCredential(state), /private/);
  await rm(file);
  await writeFile(file, '{"accessToken":1}', { mode: 0o600 });
  await assert.rejects(readPlanCredential(state), /invalid/);
  await chmod(state, 0o755);
  await assert.rejects(writePlanCredential(state, credential), /private/);
  await assert.rejects(writePlanCredential('relative/state', credential), /private/);
});

test('deletion removes only the plan credential and is idempotent', async t => {
  const state = await privateDirectory(t);
  await writeFile(join(state, 'auth.json'), 'codex', { mode: 0o600 });
  const host = await ensureHostId(state);
  await writePlanCredential(state, credential);
  assert.equal(await deletePlanCredential(state), 'deleted');
  assert.equal(await deletePlanCredential(state), 'absent');
  assert.equal(await readPlanCredential(state), undefined);
  assert.equal(await readFile(join(state, 'auth.json'), 'utf8'), 'codex');
  assert.equal(await ensureHostId(state), host);
});
