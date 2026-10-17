import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,symlink,rm,realpath} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createMvpNative,type MvpApplication} from './mvp-native.ts';

async function fixture(t:{after:(fn:()=>Promise<void>)=>void}){
  const root=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-app-')));t.after(()=>rm(root,{recursive:true,force:true}));
  const app=join(root,'real/FortMac.app'),executable=join(app,'Contents/MacOS/FortMac');
  await mkdir(join(app,'Contents/MacOS'),{recursive:true});await writeFile(executable,'inert fixture');await symlink(join(root,'real'),join(root,'alias'));
  const directory=join(root,'Library/Containers/io.fort.candidateb.mac/Data/Library/Application Support/FortCandidateB');
  await mkdir(directory,{recursive:true});
  return {root,app,executable,directory,alias:join(root,'alias/FortMac.app'),readPlist:(_path:string,key:string)=>key==='CFBundleExecutable'?'FortMac':'io.fort.candidateb.mac'};
}

test('native session discovers the registered container and canonical app identity through a path alias',async t=>{
  const f=await fixture(t),native=createMvpNative(f.alias,{homeDirectory:f.root,readPlist:f.readPlist,control:()=>[]});
  assert.equal(native.directory,f.directory);assert.equal(native.appPath,f.app);assert.equal(native.executable,f.executable);
  native.session.assertFresh();
  const installation='synthetic-installation';await writeFile(join(f.directory,'connection.json'),JSON.stringify({installationID:installation,deviceID:'synthetic-device'}));
  const {createHash}=await import('node:crypto'),scope=join(f.directory,createHash('sha256').update(installation).digest('hex'));await mkdir(scope);await writeFile(join(scope,'pending.json'),'[]');
  assert.deepEqual(native.session.inspect(installation),{appExited:true,pendingCount:0});
});

test('the exact running app is detected across aliases and stopped before native inspection succeeds',async t=>{
  const f=await fixture(t);let running=true,stops=0;
  const app:MvpApplication={pid:123,bundleIdentifier:'io.fort.candidateb.mac',bundlePath:f.alias,executablePath:join(f.alias,'Contents/MacOS/FortMac')};
  const native=createMvpNative(f.app,{homeDirectory:f.root,readPlist:f.readPlist,control:action=>{if(action==='stop'){stops++;running=false;}return running?[app]:[];}});
  assert.throws(()=>native.session.assertFresh(),/not fresh/);assert.throws(()=>native.assertExited(),/running/);
  assert.equal(await native.session.stop!(),true);assert.equal(stops,1);native.session.assertFresh();
});

test('a different app with the same bundle identifier blocks cleanup without terminating it',async t=>{
  const f=await fixture(t);let stops=0;
  const native=createMvpNative(f.app,{homeDirectory:f.root,readPlist:f.readPlist,control:action=>{if(action==='stop')stops++;return [{pid:999,bundleIdentifier:'io.fort.candidateb.mac',bundlePath:f.root,executablePath:f.executable}];}});
  assert.throws(()=>native.assertExited(),/different app/);assert.equal(await native.session.stop!(),false);assert.equal(stops,0);
});

test('unregistered containers and unexpected executable identities are refused without a storage fallback',async t=>{
  const f=await fixture(t);
  for(const key of ['MCMMetadataIdentifier','CFBundleIdentifier','CFBundleExecutable'])assert.throws(()=>createMvpNative(f.app,{homeDirectory:f.root,readPlist:(path,requested)=>requested===key?'unexpected':f.readPlist(path,requested),control:()=>[]}),/identity/);
});
