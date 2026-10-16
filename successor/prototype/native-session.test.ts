import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {createNativeSession} from './native-session.ts';

test('automatic shutdown confirms the exact native process exit without touching credentials',async()=>{
  let exited=false,stops=0,erases=0;
  const native=createNativeSession({directory:'/synthetic/cache',appExecutable:'/synthetic/FortMac',appExited:()=>exited,stopApp:async()=>{stops++;exited=true;},eraseCredential:async()=>{erases++;return true;}});
  assert.equal(await native.stop!(),true);assert.equal(await native.stop!(),true);
  assert.equal(stops,1);assert.equal(erases,0);
  const refused=createNativeSession({directory:'/synthetic/cache',appExecutable:'/synthetic/FortMac',appExited:()=>false,stopApp:async()=>{throw Error('Synthetic stop failure');}});
  assert.equal(await refused.stop!(),false);
});

test('a paired owner who sends nothing has an uncreated queue; historical inspections still fail closed',async t=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-native-zero-'));t.after(()=>rm(directory,{recursive:true,force:true}));
  const native=createNativeSession({directory,appExecutable:'/synthetic/FortMac',appExited:()=>true,eraseCredential:async()=>true});
  await writeFile(join(directory,'connection.json'),JSON.stringify({installationID:'installation',deviceID:'device'}));
  const scope=join(directory,createHash('sha256').update('installation').digest('hex'));await mkdir(scope);
  assert.throws(()=>native.inspect('installation'),/native state/);
  assert.deepEqual(native.inspect('installation',{allowUncreatedQueue:true}),{appExited:true,pendingCount:0});
  await writeFile(join(scope,'pending.json'),'invalid');
  assert.throws(()=>native.inspect('installation',{allowUncreatedQueue:true}),/native state/);
});

test('native restart inspection reads the actual installation queue and cleanup targets only its own credential',async t=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-native-inspection-'));t.after(()=>rm(directory,{recursive:true,force:true}));
  const erased:string[]=[];
  const native=createNativeSession({directory,appExecutable:'/synthetic/FortMac',appExited:()=>true,eraseCredential:async account=>{erased.push(account);return true;}});
  native.assertFresh();
  await writeFile(join(directory,'connection.json'),JSON.stringify({installationID:'installation',deviceID:'device',baseURL:'http://127.0.0.1:1234',fixtureLoopback:true}));
  const scope=join(directory,createHash('sha256').update('installation').digest('hex'));await mkdir(scope);
  assert.throws(()=>native.inspect('installation'),/native state/);
  await writeFile(join(scope,'pending.json'),'[{"text":"synthetic pending content"}]');
  assert.deepEqual(native.inspect('installation'),{appExited:true,pendingCount:1});
  await writeFile(join(scope,'pending.json'),'[]');
  assert.deepEqual(native.inspect('installation'),{appExited:true,pendingCount:0});
  await assert.rejects(()=>native.cleanup('other-installation'),/native state/);
  assert.deepEqual(erased,[]);
  assert.equal(await native.cleanup('installation'),true);
  assert.deepEqual(erased,['installation/device']);
  await assert.rejects(()=>readFile(join(directory,'connection.json')),/ENOENT/);
  assert.equal(await readFile(join(scope,'pending.json'),'utf8'),'[]');
});

test('cleanup can remove an issued orphan item and reports a refused deletion without erasing recovery metadata',async t=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-native-cleanup-'));t.after(()=>rm(directory,{recursive:true,force:true}));
  const erased:string[]=[];
  const orphan=createNativeSession({directory,appExecutable:'/synthetic/FortMac',appExited:()=>true,eraseCredential:async account=>{erased.push(account);return true;}});
  assert.equal(await orphan.cleanup('installation',['issued-device']),true);
  assert.deepEqual(erased,['installation/issued-device']);
  await writeFile(join(directory,'connection.json'),JSON.stringify({installationID:'installation',deviceID:'issued-device'}));
  const refused=createNativeSession({directory,appExecutable:'/synthetic/FortMac',appExited:()=>true,eraseCredential:async()=>false});
  assert.equal(await refused.cleanup('installation',['issued-device']),false);
  assert.match(await readFile(join(directory,'connection.json'),'utf8'),/issued-device/);
});
