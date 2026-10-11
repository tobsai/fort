import test from 'node:test';
import assert from 'node:assert/strict';
import { foundationRouteAllowed } from './foundation-policy.mjs';
import { resolveRequestAuth, serviceAllowed } from './server/request-auth.ts';
const paths = ['/api/health','/api/connectors/catalog','/api/tts/speak','/api/bots/b1/computer','/api/bots/b1/respond','/api/bots/b1/messages/guarded','/api/auth/pairing','/api/internal/action','/api/unknown'];
test('quarantine closes all HTTP paths including vendor and approval aliases', () => {
 for (const path of paths) {
  assert.equal(foundationRouteAllowed(path), false, path);
  for (const method of ['GET','POST','DELETE']) assert.equal(serviceAllowed(method,path), false, `${method} ${path}`);
 }
});
test('owner, worker, desktop and session credentials cannot authorize quarantine requests', () => {
 for (const trust of [undefined,'owner','service']) {
  for (const path of paths) {
   for (const method of ['GET','POST','DELETE']) {
    for (const credential of ['none','desktop','cli','session']) {
     const headers = {host:'127.0.0.1'};
     const options = {url:new URL(path,'http://127.0.0.1'),loopbackTrust:trust,sessions:new Proxy({}, {get(){throw new Error('credential store accessed under quarantine');}})};
     if (credential==='desktop') { headers['x-openmausbot-desktop-owner']='fixture'; options.loopbackMutationToken='fixture'; }
     if (credential==='cli') { headers['x-openmausbot-cli-owner']='fixture'; options.cliOwnerToken='fixture'; }
     if (credential==='session') headers.authorization='Bearer omb_sess_fixture';
     assert.deepEqual(resolveRequestAuth({method,headers},options), {auth:null,status:403,error:'Fort foundation is quarantined; request authorization disabled'});
    }
   }
  }
 }
});

import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
const root = new URL('./', import.meta.url);
const inventory = JSON.parse(readFileSync(new URL('QUARANTINE-ENTRYPOINTS.json',root),'utf8'));
const refusal = 'Fort foundation is source-only: runtime extraction and independence review required';
test('every inventoried entry refuses before loading archived dependencies', async () => {
 const required = ['server/index.ts','server/openmausbot.ts','server/pair-cli.ts','server/cli.ts','server/server-launcher.ts','src/main.tsx','vite.config.ts'];
 for (const entry of required) assert.ok(inventory.entries.some(item=>item.entry===entry),entry);
 for (const {entry,archive,archiveSha256} of inventory.entries) {
  const text = readFileSync(new URL(entry,root),'utf8');
  // This proves there is no static dependency graph to resolve before refusal.
  assert.equal(text, `throw new Error(${JSON.stringify(refusal)});\n`,entry);
  assert.equal(createHash('sha256').update(readFileSync(new URL(archive,root))).digest('hex'),archiveSha256,archive);
  if (entry.endsWith('.tsx')) {
   await assert.rejects(import(`data:text/javascript,${encodeURIComponent(text)}#${entry}`), new RegExp('source-only'),entry);
  } else {
   // Only verified refusal-only contents are executed; no provider/config code loads.
   const result=spawnSync(process.execPath,['--experimental-strip-types',new URL(entry,root).pathname],{env:{},encoding:'utf8',timeout:3000});
   assert.equal(result.status,1,entry);
   assert.match(result.stderr,/source-only/,entry);
   assert.equal(result.stdout,'',entry);
  }
 }
});
