import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,realpathSync,mkdirSync,writeFileSync,readdirSync,rmSync,lstatSync,chmodSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {inspectOwnerDemoAttempt,describeOwnerDemoFailure,checkOwnerDemoWrites} from './owner-demo-startup.ts';
const fixture=(t:{after:(fn:()=>void)=>void})=>{const path=realpathSync(mkdtempSync(join(tmpdir(),'fort-startup-')));t.after(()=>rmSync(path,{recursive:true,force:true}));return join(path,'demo');};
test('a filesystem refusal before the session directory does not consume the approved action or expose raw errors',t=>{
 const root=fixture(t),error=Object.assign(new Error('SECRET_PROVIDER_TOKEN'),{code:'EPERM',path:'/secret/path'});
 const report=describeOwnerDemoFailure(error,'session-start',root);
 assert.equal(report.attempt,'not-created');assert.equal(report.clockStarted,false);assert.equal(report.retrySameApprovedAction,true);assert.equal(report.code,'EPERM');assert.match(report.nextAction,/require_escalated/);
 assert.equal(JSON.stringify(report).includes('SECRET'),false);assert.equal(JSON.stringify(report).includes('/secret/path'),false);
});
test('session directory or retained lock blocks retry even without a state file',t=>{
 const root=fixture(t);mkdirSync(root);mkdirSync(join(root,'session-1'));
 assert.equal(describeOwnerDemoFailure(new Error('private'),'session-start',root).retrySameApprovedAction,false);
 rmSync(join(root,'session-1'),{recursive:true});writeFileSync(join(root,'active.lock'),'synthetic');
 assert.equal(inspectOwnerDemoAttempt(root),'created-or-unconfirmed');
});
test('write preflight leaves no session, timer, runtime, credential or directory behind',t=>{
 const root=fixture(t);checkOwnerDemoWrites(root);assert.throws(()=>lstatSync(root),{code:'ENOENT'});assert.equal(inspectOwnerDemoAttempt(root),'not-created');
 mkdirSync(root);checkOwnerDemoWrites(root);assert.deepEqual(readdirSync(root),[]);
 mkdirSync(join(root,'session-1'));assert.throws(()=>checkOwnerDemoWrites(root),/consumed|unconfirmed/);assert.deepEqual(readdirSync(root),['session-1']);
});

test('unreadable session evidence cannot authorize retry',t=>{
 const root=fixture(t);mkdirSync(root);chmodSync(root,0);
 try{assert.equal(inspectOwnerDemoAttempt(root),'created-or-unconfirmed');assert.equal(describeOwnerDemoFailure(Object.assign(new Error('private'),{code:'EACCES'}),'session-start',root).retrySameApprovedAction,false);}
 finally{chmodSync(root,0o700);}
});
test('unclassified failures identify the stage without leaking the exception or claiming a filesystem cause',t=>{
 const root=fixture(t),report=describeOwnerDemoFailure(Object.assign(new Error('SECRET'),{code:'SECRET_CUSTOM_CODE'}),'preparation',root);
 assert.equal(report.stage,'preparation');assert.equal(report.code,'unclassified');assert.equal(report.retrySameApprovedAction,false);assert.equal(JSON.stringify(report).includes('SECRET'),false);
});
