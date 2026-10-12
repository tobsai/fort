import { test } from 'node:test';
import assert from 'node:assert/strict';
import { runConfinementCanary } from './confinement-canary.ts';

test('native diagnostic controls test named denials without claiming production confinement',async t=>{
  const report=await runConfinementCanary();t.diagnostic(JSON.stringify(report));
  assert.equal(report.platform,'darwin');assert.equal(report.mechanism,'sandbox-exec-deprecated');assert.equal(report.productionIsolationProven,false);
  for(const name of ['workspace','owner','ledger','sibling','provider','home','symlink','environment','descriptor','process','ipc','network','exec'])assert.equal(report.control[name],true,`positive control ${name}`);
  assert.equal(report.tool.workspace,true);assert.equal(report.host.workspace,true);assert.equal(report.host.provider,true);
  for(const name of ['owner','ledger','sibling','provider','home','symlink','environment','descriptor','process','ipc','network','exec'])assert.equal(report.tool[name],false,`tool boundary ${name}`);
  for(const name of ['owner','ledger','sibling','home','symlink','environment','descriptor','process','ipc','network','exec'])assert.equal(report.host[name],false,`host boundary ${name}`);
  assert.equal(report.policy,'diagnostic-targeted-denials');assert.equal(report.closedDescriptor,false);assert.equal(report.inheritedDescriptor,true,'an inherited file descriptor bypasses the path denial');assert.equal(report.cleaned,true);
});
