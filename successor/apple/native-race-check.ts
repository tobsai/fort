import assert from 'node:assert/strict';
import { createServer, request as forwardRequest, type IncomingMessage, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { randomBytes } from 'node:crypto';
import { mkdtemp, writeFile, rm, chmod } from 'node:fs/promises';
import { appendFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { execFile, execFileSync } from 'node:child_process';
import { promisify } from 'node:util';
import { createFixtureService } from '../service/service.ts';
import type { Snapshot, Approval, DurableEvent } from '../contracts/v1.ts';

const [source, destinationA, destinationB, resultPrefix] = process.argv.slice(2);
assert(source && destinationA && destinationB && resultPrefix, 'Pass built xctestrun, two distinct Simulator destinations and a fresh result prefix');
assert(destinationA !== destinationB, 'Two distinct installed devices are required');
const directory = await mkdtemp(join(tmpdir(), 'fort-native-race-'));
const makeService = () => createFixtureService({dataDirectory:join(directory,'ledger'),autoRun:true,fixtureBehavior:'approval'});
let service = makeService();
const controlToken = randomBytes(32).toString('base64url');
const ready = new Map<string,ServerResponse>();
const observed = new Map<string,string>();
const enrolledDevices = new Set<string>();
const resolved: Approval[] = [];
const held: { request:IncomingMessage; response:ServerResponse; body:Buffer }[] = [];
const problems: string[] = [];
let readyTimer: ReturnType<typeof setTimeout> | undefined;
let decisionTimer: ReturnType<typeof setTimeout> | undefined;
let releasedDecisions = false;
const finish = (response:ServerResponse,status:number) => { if (!response.writableEnded) { response.writeHead(status); response.end(); } };
const forward = (incoming:IncomingMessage,outgoing:ServerResponse,body?:Buffer) => {
  const request = forwardRequest(new URL(incoming.url!,service.baseURL),{method:incoming.method,headers:incoming.headers},response => {
    const collect = incoming.url === '/v1/devices/enroll' || incoming.url?.endsWith('/resolve');
    const chunks:Buffer[] = [];
    if (collect) response.on('data',chunk=>chunks.push(chunk));
    response.on('end',()=>{
      if (!collect) return;
      try {
        const result = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        if (incoming.url === '/v1/devices/enroll') {
          assert.equal(response.statusCode,201); assert.equal(typeof result.deviceId,'string'); enrolledDevices.add(result.deviceId);
        } else { assert.equal(response.statusCode,200); resolved.push(result.approval); }
      } catch { problems.push('A native enrollment or decision response failed verification'); }
    });
    outgoing.writeHead(response.statusCode!,response.headers); response.pipe(outgoing);
  });
  request.on('error',()=>{ problems.push('Fixture forwarding failed'); finish(outgoing,503); });
  if (body) request.end(body); else incoming.pipe(request);
};
const proxy = createServer(async (incoming,outgoing) => {
  if (incoming.method !== 'POST' || !incoming.url?.endsWith('/resolve')) { forward(incoming,outgoing); return; }
  const chunks:Buffer[] = []; let bytes = 0;
  for await (const chunk of incoming) { bytes += chunk.length; if (bytes > 1_000_000) { finish(outgoing,413); return; } chunks.push(chunk); }
  if (releasedDecisions) { problems.push('Unexpected extra native decision'); finish(outgoing,409); return; }
  held.push({request:incoming,response:outgoing,body:Buffer.concat(chunks)});
  if (held.length === 1) decisionTimer = setTimeout(()=>{ problems.push('Both native decisions did not arrive'); held.forEach(x=>finish(x.response,503)); },20_000);
  if (held.length === 2) {
    clearTimeout(decisionTimer); releasedDecisions = true;
    held.forEach(x=>forward(x.request,x.response,x.body));
  }
});
const controls = createServer((request,response) => {
  request.resume();
  if (request.method !== 'POST' || request.headers.authorization !== 'Bearer '+controlToken) { finish(response,401); return; }
  const arrived = /^\/ready\/([AB])$/.exec(request.url!);
  const observation = /^\/observed\/([AB])\/(approved|rejected)$/.exec(request.url!);
  if (arrived) {
    if (ready.has(arrived[1])) { finish(response,409); return; }
    ready.set(arrived[1],response);
    if (ready.size === 1) readyTimer = setTimeout(()=>{ problems.push('Both installed clients did not reach the approval'); ready.forEach(r=>finish(r,503)); },120_000);
    if (ready.size === 2) { clearTimeout(readyTimer); ready.forEach(r=>finish(r,200)); }
  } else if (observation) { observed.set(observation[1],observation[2]); finish(response,200); }
  else finish(response,404);
});
const startLoopback = async (server:ReturnType<typeof createServer>) => {
  server.listen(0,'127.0.0.1'); await once(server,'listening');
  const address = server.address(); assert(address && typeof address !== 'string'); return 'http://127.0.0.1:'+address.port;
};
const runPaths:string[] = [];
try {
  await service.start();
  const proxyURL = await startLoopback(proxy); const controlsURL = await startLoopback(controls);
  const run = async (role:'A'|'B',destination:string) => {
    const testRun = JSON.parse(execFileSync('/usr/bin/plutil',['-convert','json','-o','-',source],{encoding:'utf8'}));
    let targetName = '';
    for (const [name,target] of Object.entries(testRun) as [string,Record<string,unknown>][]) if (target.IsUITestBundle) {
      targetName = name;
      target.EnvironmentVariables = {...target.EnvironmentVariables as Record<string,string>,FORT_FIXTURE_ROLE:role,FORT_FIXTURE_URL:proxyURL,FORT_FIXTURE_INSTALLATION:service.installation.installationId,FORT_FIXTURE_CODE:service.issueEnrollment().code,FORT_FIXTURE_CONTROL_URL:controlsURL,FORT_FIXTURE_CONTROL_TOKEN:controlToken};
    }
    assert(targetName, 'Expected a UI test target');
    const jsonPath = join(directory,role+'.json'); const runPath = join(dirname(source),'Fort058-race-'+role+'.xctestrun'); runPaths.push(runPath);
    await writeFile(jsonPath,JSON.stringify(testRun),{mode:0o600}); execFileSync('/usr/bin/plutil',['-convert','xml1','-o',runPath,jsonPath]); await chmod(runPath,0o600);
    const resultPath = resultPrefix+'-'+role+'.xcresult'; const logPath = resultPath+'.log'; await writeFile(logPath,'',{mode:0o600});
    const operation = promisify(execFile)('/usr/bin/xcodebuild',['test-without-building','-xctestrun',runPath,'-destination',destination,'-parallel-testing-enabled','NO','-only-testing:'+targetName+'/NativeJourneyTests/testRaceApprovalAcrossDevices','-collect-test-diagnostics','never','-resultBundlePath',resultPath],{timeout:240_000,maxBuffer:8_000_000});
    operation.child.stdout?.on('data',chunk=>appendFileSync(logPath,chunk)); operation.child.stderr?.on('data',chunk=>appendFileSync(logPath,chunk));
    await operation;
  };
  const tests = await Promise.allSettled([run('A',destinationA),run('B',destinationB)]);
  assert(tests.every(r=>r.status === 'fulfilled'), 'A native race test failed; inspect the two result logs');
  assert.deepEqual(problems,[]);
  assert.equal(ready.size,2); assert.equal(held.length,2); assert.equal(enrolledDevices.size,2); assert.equal(resolved.length,2);
  assert(held[0].request.headers.authorization !== held[1].request.headers.authorization,'Decisions must use distinct enrolled device credentials');
  assert.equal(held[0].request.url,held[1].request.url);
  const commands = held.map(x=>JSON.parse(x.body.toString('utf8')));
  assert.deepEqual(commands.map(x=>x.decision).sort(),['approve','reject']);
  const action = commands.map(({decision,...rest})=>rest); assert.deepEqual(action[0],action[1]);
  const winner = resolved[0]; assert.deepEqual(resolved[1],winner); assert(enrolledDevices.has(winner.decidedByDeviceId!));
  assert.equal(observed.size,2); assert([...observed.values()].every(state=>state===winner.state));
  const enrollment = await fetch(service.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({installationId:service.installation.installationId,code:service.issueEnrollment().code,deviceName:'Race evidence verifier'})}).then(r=>r.json()) as {deviceToken:string};
  const headers = {'Authorization':'Bearer '+enrollment.deviceToken,'X-Fort-Installation-Id':service.installation.installationId};
  const inspect = async () => {
    const agents = await fetch(service.baseURL+'/v1/agents',{headers}).then(r=>r.json()) as {agents:{homeConversationId:string}[]};
    const snapshot = await fetch(service.baseURL+'/v1/conversations/'+agents.agents[0].homeConversationId+'/snapshot',{headers}).then(r=>r.json()) as Snapshot;
    const page = await fetch(service.baseURL+'/v1/events?after=0&recoveryEpoch='+service.installation.recoveryEpoch,{headers}).then(r=>r.json()) as {events:DurableEvent[]};
    assert.equal(snapshot.approvals.length,1); assert.deepEqual(snapshot.approvals[0],winner);
    assert.equal(page.events.filter(e=>e.kind==='approval.updated').length,1);
    assert.deepEqual(snapshot.messages.filter(m=>m.role==='user').map(m=>m.text),['Native two-device race']);
    assert.equal(snapshot.messages.filter(m=>m.role==='assistant').length,winner.state==='approved'?1:0);
    assert.deepEqual(snapshot.runs.map(r=>r.state),[winner.state==='approved'?'completed':'canceled']);
    return snapshot;
  };
  const before = await inspect(); await service.close(); service = makeService(); await service.start(); const after = await inspect(); assert.deepEqual(after,before);
  await writeFile(resultPrefix+'-evidence.json',JSON.stringify({nativeDevices:2,opposingDecisionRequests:2,distinctDeviceCredentials:true,oneDurableDecision:true,winner:winner.state,bothNativeClientsObservedAfterRestart:[...observed.values()],serviceRestartPreservedSnapshot:true,userMessages:1,assistantMessages:after.messages.filter(m=>m.role==='assistant').length},null,2)+'\n',{mode:0o600});
  console.log('PASS two installed native clients raced opposing decisions, converged after app restart, and retained one winner after service restart');
} finally {
  clearTimeout(readyTimer); clearTimeout(decisionTimer); ready.forEach(r=>finish(r,503)); held.forEach(x=>finish(x.response,503));
  await Promise.all([proxy,controls].map(server=>server.listening?new Promise<void>(r=>server.close(()=>r())):Promise.resolve()));
  await service.close(); for (const path of runPaths) await rm(path,{force:true}); await rm(directory,{recursive:true,force:true});
}
