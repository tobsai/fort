import { test } from 'node:test';
import assert from 'node:assert/strict';
import { codexLaunchPlan, launchCandidateCodex, CODEX_EXECUTABLE } from './owned-codex.ts';

test('candidate provider entry refuses use outside the approved manual packet before path inspection or launch', () => {
  assert.throws(() => launchCandidateCodex({ workspace:'/does-not-exist',temporaryDirectory:'/does-not-exist' }),/approved only through the pinned manual packet/);
  const plan = codexLaunchPlan({ workspace:'/synthetic/empty',temporaryDirectory:'/synthetic/tmp' });
  assert.equal(plan.executable,CODEX_EXECUTABLE);
  assert.deepEqual(Object.keys(plan.env).sort(),['HOME','LANG','PATH','TMPDIR','TZ']);
  assert.equal(plan.deadlineMs + (plan.terminationGraceMs ?? 0) + (plan.killGraceMs ?? 0),300_000);
  assert.equal(plan.args.includes('analytics.enabled=false'),true);
  assert.equal(plan.args.includes('skip_host_skill_discovery'),true);
});
