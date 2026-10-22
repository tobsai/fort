import {test} from 'node:test';
import assert from 'node:assert/strict';
import {codexLaunchPlan} from './owned-codex.ts';
import {validateOwnedProcessOptions} from '../runner/owned-process.ts';

test('owner demo has one named nine-minute work budget without changing generic process ceiling',()=>{
 const plan=codexLaunchPlan({workspace:'/tmp/fort-test-empty',temporaryDirectory:'/tmp/fort-test-tmp',remainingWorkMs:536000,packet:'owner-demo-20261017'});
 validateOwnedProcessOptions(plan);assert.equal(plan.packetBudget?.totalMs,600000);assert.equal(plan.packetBudget?.cleanupReserveMs,60000);
 assert.throws(()=>validateOwnedProcessOptions({...plan,packetBudget:undefined}),/bounded deadline/);
 assert.throws(()=>codexLaunchPlan({workspace:'/tmp/a',temporaryDirectory:'/tmp/b',remainingWorkMs:540001,packet:'owner-demo-20261017'}),/budget/);
});
