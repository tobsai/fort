import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,mkdir,writeFile,readFile,realpath,symlink,stat} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {validateApproval,MVP_OPERATOR,MVP_SCOPE,requireMvpApproval} from './mvp-activation.ts';
import {archiveMvpNativeCache} from './mvp-native.ts';

test('only explicit owner scope approval can activate the batch; missing approval is not permission',async t=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-mvp-approval-'));t.after(()=>rm(directory,{recursive:true,force:true}));
  const missing=join(directory,'missing-owner-approval.json');
  const value={schemaVersion:1,decision:'approved',actor:'owner',scopeId:MVP_SCOPE,scopeSHA256:'exact-scope',operatorThreadId:MVP_OPERATOR,sourceThreadId:'01a0f307-3940-7333-96b0-a3b0b854e0e8',approvedAtUTC:'2026-10-17T00:00:00Z'};
  validateApproval(value,'exact-scope',Date.parse('2026-10-17T01:00:00Z'));
  for(const patch of [{decision:'pending'},{actor:'agent'},{scopeSHA256:'other'},{sourceThreadId:'another-task'},{operatorThreadId:'another-operator'},{approvedAtUTC:'2099-01-01T00:00:00Z'}])assert.throws(()=>validateApproval({...value,...patch},'exact-scope',Date.parse('2026-10-17T01:00:00Z')));
  assert.throws(()=>requireMvpApproval({sourceCommit:'synthetic',appPath:'/synthetic/app',scopeSHA256:'exact-scope',approvalPresent:false},missing),/approval is pending/);
});

test('native cache preparation archives exact bytes and refuses overwrite, live app or symlinks',async t=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-native-')));t.after(()=>rm(base,{recursive:true,force:true}));
  const root=join(base,'session'),cache=join(base,'cache');await mkdir(root);await mkdir(cache);await mkdir(join(cache,'history'));
  await writeFile(join(cache,'history','pending.json'),'[]\n');await writeFile(join(cache,'connection.json'),'synthetic connection metadata');
  assert.throws(()=>archiveMvpNativeCache(root,cache,()=>{throw new Error('App still running');}),/running/);
  await symlink(join(cache,'connection.json'),join(cache,'link'));assert.throws(()=>archiveMvpNativeCache(root,cache,()=>{}),/symlink/);await rm(join(cache,'link'));
  archiveMvpNativeCache(root,cache,()=>{});await assert.rejects(()=>stat(cache),/ENOENT/);
  assert.equal(await readFile(join(root,'native-cache-before/history/pending.json'),'utf8'),'[]\n');
  assert.equal(await readFile(join(root,'native-cache-before/connection.json'),'utf8'),'synthetic connection metadata');
  const manifest=JSON.parse(await readFile(join(root,'native-cache-before.json'),'utf8'));assert.equal(manifest.files.length,2);
  assert.throws(()=>archiveMvpNativeCache(root,cache,()=>{}),/already exists/);
});
