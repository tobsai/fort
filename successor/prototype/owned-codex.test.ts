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

test('the inert launch plan supplies a process-scoped no-files profile without a settings file or legacy sandbox override',()=>{
  const plan=codexLaunchPlan({workspace:'/synthetic/empty',temporaryDirectory:'/synthetic/tmp'});
  const overrides=plan.args.filter((_value,index)=>plan.args[index-1]==='-c');
  assert.equal(overrides.includes('default_permissions="fort_candidate_b_no_files"'),true);
  assert.equal(overrides.includes('permissions.fort_candidate_b_no_files={filesystem={"/**"="deny"},network={enabled=false}}'),true);
  assert.equal(overrides.some(value=>value.startsWith('sandbox_mode=')||value.startsWith('sandbox_workspace_write=')),false);
  assert.equal(plan.args.includes('--config-file'),false);
});
