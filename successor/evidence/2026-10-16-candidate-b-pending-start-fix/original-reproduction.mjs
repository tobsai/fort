import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import vm from 'node:vm';
import { EventEmitter } from 'node:events';
import { PassThrough, Writable } from 'node:stream';
import { createInterface } from 'node:readline';
import { createHash } from 'node:crypto';
import { join, normalize, isAbsolute } from 'node:path';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
const path='/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016/successor/prototype/manual.ts';
let source=stripTypeScriptTypes(readFileSync(path,'utf8'));
source=source.split(/\nif\s*\(import\.meta\.main\)/)[0];
source=source.replace(/^import .*?;\n/gm,'').replace('export async function runManualPacket','async function runManualPacket').replaceAll('import.meta.url',JSON.stringify('file:///synthetic/successor/prototype/manual.ts'));
const fakeProcess=new EventEmitter();fakeProcess.stdin=new PassThrough();let output='';
fakeProcess.stdout=new Writable({write(chunk,_enc,done){output+=chunk.toString();done();}});
let enterStart,finishStart,finishClosed;
const entered=new Promise(resolve=>enterStart=resolve), pendingStart=new Promise(resolve=>finishStart=resolve), closed=new Promise(resolve=>finishClosed=resolve);
let aborts=0,cancelBetween=0;
const state={phase:'blocked',starts:1,receipts:[],credentialCleanup:'deleted'};
class InertPacket {
 async start(){enterStart();return pendingStart;}
 async cancelBetweenSessions(){cancelBetween++;}
}
const digest=createHash('sha256').update('inert bytes').digest('hex');
const pins=JSON.stringify({sourceCommit:'a'.repeat(40),sourceFiles:[{path:'source',sha256:digest}],appFiles:[{path:'app',sha256:digest}]});
const context=vm.createContext({
 process:fakeProcess,createInterface,createHash,join,normalize,isAbsolute,fileURLToPath,Date,JSON,URL,Buffer,
 readFileSync:file=>String(file).endsWith('pins.json')?pins:Buffer.from('inert bytes'),
 lstatSync:()=>({isFile:()=>true,isSymbolicLink:()=>false}),mkdirSync(){},existsSync:()=>false,
 unlinkSync(){throw Error('Unexpected filesystem mutation');},writeFileSync(){throw Error('Unexpected filesystem mutation');},openSync(){throw Error('Unexpected filesystem mutation');},closeSync(){},
 requireCandidateBActivation(){},createNativeSession:()=>({}),CandidatePacket:InertPacket,
 launchCandidateCodex(){throw Error('Official launcher must never be invoked');}
});
vm.runInContext(source+'\nglobalThis.entry=runManualPacket;',context);
const running=context.entry();fakeProcess.stdin.write('start\n');await entered;
fakeProcess.emit('SIGINT');await new Promise(resolve=>setImmediate(resolve));
const abortsWhilePreflightPending=aborts;
assert.equal(abortsWhilePreflightPending,0);
finishStart({closed,workDeadline:Date.now()+1000,enrollment:undefined,async abort(){aborts++;finishClosed(state);return state;}});
await running;
const result={abortsWhilePreflightPending,abortsAfterStartResolves:aborts,cancelBetween,printedReadyAfterSIGINT:output.includes('Preflight passed. Open this exact app:')};
assert.equal(result.abortsAfterStartResolves,1);assert.equal(result.printedReadyAfterSIGINT,true);
console.log(JSON.stringify(result));
