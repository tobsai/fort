import {test} from 'node:test';
import assert from 'node:assert/strict';
import {runManualPacket} from './manual.ts';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import vm from 'node:vm';
import {EventEmitter} from 'node:events';
import {PassThrough,Writable} from 'node:stream';
import {createInterface} from 'node:readline';
import {createHash} from 'node:crypto';
import {join,normalize,isAbsolute} from 'node:path';
import {fileURLToPath} from 'node:url';

test('manual entry refuses indirect use before pin reads, native inspection, enrollment or process launch',async()=>{
  await assert.rejects(()=>runManualPacket(),/approved only through the pinned manual packet/);
});

for(const stop of ['SIGINT','SIGTERM','EOF'] as const)test(`${stop} cancels a pending start before it can expose enrollment or readiness`,async()=>{
  let source=stripTypeScriptTypes(readFileSync(new URL('./manual.ts',import.meta.url),'utf8'));
  source=source.split(/\nif\s*\(import\.meta\.main\)/)[0]!;
  source=source.replace(/^import .*?;\n/gm,'').replace('export async function runManualPacket','async function runManualPacket').replaceAll('import.meta.url',JSON.stringify('file:///synthetic/successor/prototype/manual.ts'));
  const input=new PassThrough();let output='';
  const fakeProcess=Object.assign(new EventEmitter(),{stdin:input,stdout:new Writable({write(chunk,_encoding,done){output+=chunk.toString();done();}})});
  const entered=Promise.withResolvers<void>(),pendingStart=Promise.withResolvers<unknown>(),closed=Promise.withResolvers<unknown>();
  let starts=0,cancellations=0,aborts=0,cancelBetween=0,enrollmentWrites=0;
  let packetOptions:{root:string;attempt?:string}|undefined;const pinReads:string[]=[];
  const state={phase:'blocked',starts:1,receipts:[],credentialCleanup:'deleted'};
  class InertPacket {
    constructor(options:{root:string;attempt?:string}){packetOptions=options;}
    async start(signal?:AbortSignal){starts++;signal?.addEventListener('abort',()=>{cancellations++;},{once:true});entered.resolve();return pendingStart.promise;}
    async cancelBetweenSessions(){cancelBetween++;}
  }
  const digest=createHash('sha256').update('inert bytes').digest('hex');
  const pins=JSON.stringify({sourceCommit:'a'.repeat(40),sourceFiles:[{path:'source',sha256:digest}],appFiles:[{path:'app',sha256:digest}]});
  const context=vm.createContext({
    process:fakeProcess,createInterface,createHash,join,normalize,isAbsolute,fileURLToPath,Date,JSON,URL,Buffer,AbortController,
    readFileSync:(file:unknown)=>{if(String(file).endsWith('pins.json')){pinReads.push(String(file));return pins;}return Buffer.from('inert bytes');},
    lstatSync:()=>({isFile:()=>true,isSymbolicLink:()=>false}),mkdirSync(){},existsSync:()=>false,
    unlinkSync(){throw Error('Unexpected filesystem mutation');},writeFileSync(){enrollmentWrites++;},openSync(){return 1;},closeSync(){},
    requireCandidateBActivation(){},createNativeSession:()=>({}),CandidatePacket:InertPacket,
    launchCandidateCodex(){throw Error('Official launcher must never be invoked');}
  });
  const entry:()=>Promise<void>=vm.runInContext(source+'\nrunManualPacket;',context);
  const running=entry();await new Promise(resolve=>setImmediate(resolve));
  assert.equal(starts,0);assert.equal(enrollmentWrites,0);
  assert.equal(packetOptions?.root,'/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-replacement-20261016');
  assert.equal(packetOptions?.attempt,'replacement-20261016');
  assert.deepEqual(pinReads,['/synthetic/successor/evidence/2026-10-16-candidate-b-replacement/pins.json']);
  assert.equal(output.includes('Preflight passed. Open this exact app:'),false);
  input.write('start\n');await entered.promise;
  if(stop==='EOF')input.end();else fakeProcess.emit(stop);
  await new Promise(resolve=>setImmediate(resolve));
  const cancellationsWhilePending=cancellations;
  pendingStart.resolve({closed:closed.promise,baseURL:'http://synthetic.invalid',workDeadline:Date.now()+1000,enrollment:{code:'synthetic-only'},async abort(){aborts++;closed.resolve(state);return state;}});
  await running;input.destroy();
  assert.equal(cancellationsWhilePending,1);
  assert.ok(aborts>=1);
  assert.equal(cancelBetween,1);
  assert.equal(enrollmentWrites,0);
  assert.equal(output.includes('Preflight passed. Open this exact app:'),false);
  assert.equal(fakeProcess.listenerCount('SIGINT'),0);
  assert.equal(fakeProcess.listenerCount('SIGTERM'),0);
});
