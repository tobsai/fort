import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { launchOwnedProcess } from './owned-process.ts';

test('owned runtime disposal confirms exit even when the child ignores graceful termination', async () => {
  const owned = launchOwnedProcess({ executable:process.execPath, args:[fileURLToPath(new URL('./owned-process-fixture.mjs',import.meta.url))], cwd:fileURLToPath(new URL('.',import.meta.url)), env:{ PATH:'/usr/bin:/bin', FORT_SYNTHETIC_MODE:'ignore-term' }, deadlineMs:5_000, terminationGraceMs:50, killGraceMs:500 });
  await owned.connection.request('ping',{},new AbortController().signal);
  let lateFrames = 0;
  owned.connection.subscribe({ frame:() => { lateFrames++; }, closed:() => {} });
  await owned.dispose(); await owned.dispose();
  assert.equal(owned.cleanupConfirmed,true);
  assert.equal(owned.exit?.signal,'SIGKILL');
  assert.equal(lateFrames,0);
});

test('owned session deadline stops an idle child and disposal confirms its exit', async () => {
  const owned = launchOwnedProcess({ executable:process.execPath,args:[fileURLToPath(new URL('./owned-process-fixture.mjs',import.meta.url))],cwd:fileURLToPath(new URL('.',import.meta.url)),env:{ PATH:'/usr/bin:/bin' },deadlineMs:250,terminationGraceMs:100,killGraceMs:500 });
  await owned.connection.request('ping',{},new AbortController().signal);
  await new Promise(resolve => setTimeout(resolve,400));
  assert.equal(owned.cleanupConfirmed,true);
  await owned.dispose();
  assert.equal(owned.exit?.signal,'SIGTERM');
  await assert.rejects(owned.connection.request('ping',{},new AbortController().signal));
});
