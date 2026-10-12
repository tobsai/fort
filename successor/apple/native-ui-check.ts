import { createServer, request as forwardRequest } from 'node:http';
import { once } from 'node:events';
import { randomBytes } from 'node:crypto';
import { createFixtureService } from '../service/service.ts';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { appendFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { execFile, execFileSync } from 'node:child_process';
import { promisify } from 'node:util';

const [source, destination, resultPath, scenario = "recovery"] = process.argv.slice(2);
if (!['recovery','approval','unknown'].includes(scenario)) throw new Error('Use recovery, approval or unknown scenario');
if (!source || !destination || !resultPath) throw new Error('Pass built .xctestrun, Xcode destination, and fresh result bundle path');
const directory = await mkdtemp(join(tmpdir(),'fort-native-ui-'));
let lostContact = false;
const makeService = () => createFixtureService({dataDirectory:join(directory,'ledger'),autoRun:true,fixtureBehavior:scenario === 'approval' ? 'approval' : 'echo',checkpoint(point) {
  if (scenario === 'unknown' && point === 'afterStarted' && !lostContact) { lostContact = true; throw new Error('Injected fixture contact loss after start'); }
}});
let service = makeService();
let unavailable = false;
let dropAcceptance = false;
let dropped = 0;
const closureResponses:number[] = [];
let closureRequests = 0;
let holdRefresh = false;
let heldRefresh: (() => void)[] = [];
let refreshObservers: (() => void)[] = [];
const controlToken = randomBytes(32).toString('base64url');
const proxy = createServer((incoming,outgoing) => {
  if (unavailable) { outgoing.writeHead(503,{'Content-Type':'application/json'}); outgoing.end(JSON.stringify({error:{code:'fixtureOffline',message:'Fixture temporarily offline'}})); return; }
  if (incoming.method === 'POST' && incoming.url?.endsWith('/close-unknown')) closureRequests += 1;
  const forward = () => {
  const forwarded = forwardRequest(new URL(incoming.url!,service.baseURL),{method:incoming.method,headers:incoming.headers},response => {
    if (incoming.url?.endsWith('/close-unknown')) closureResponses.push(response.statusCode ?? 0);
    if (dropAcceptance && incoming.method === 'POST' && incoming.url?.endsWith('/messages') && [200,201].includes(response.statusCode!)) {
      dropAcceptance = false; unavailable = true; dropped += 1;
      response.resume(); response.on('end',()=>outgoing.destroy()); return;
    }
    outgoing.writeHead(response.statusCode!,response.headers); response.pipe(outgoing);
  });
  forwarded.on('error',()=>{ if (!outgoing.headersSent) outgoing.writeHead(503); outgoing.end(); });
  incoming.pipe(forwarded);
  };
  if (holdRefresh && incoming.method === 'GET' && incoming.url === '/v1/installation') {
    heldRefresh.push(forward); refreshObservers.splice(0).forEach(notify=>notify());
  } else forward();
});
const controls = createServer(async (request,response) => {
  if (request.method !== 'POST' || request.headers.authorization !== 'Bearer '+controlToken) { response.writeHead(401); response.end(); return; }
  request.resume();
  if (request.url === '/hold-refresh') holdRefresh = true;
  else if (request.url === '/await-held-refresh') { if (!heldRefresh.length) await new Promise<void>(resolve=>refreshObservers.push(resolve)); }
  else if (request.url === '/release-refresh') { holdRefresh = false; heldRefresh.splice(0).forEach(forward=>forward()); }
  else if (request.url === '/offline') unavailable = true;
  else if (request.url === '/online') unavailable = false;
  else if (request.url === '/drop-acceptance') dropAcceptance = true;
  else if (request.url === '/restart') { await service.dispatchNextFixture(); await service.close(); service = makeService(); await service.start(); }
  else { response.writeHead(404); response.end(); return; }
  response.writeHead(200); response.end();
});
const startLoopback = async (server: ReturnType<typeof createServer>) => {
  server.listen(0,'127.0.0.1'); await once(server,'listening');
  const address = server.address(); if (!address || typeof address === 'string') throw new Error('Expected fixture port');
  return 'http://127.0.0.1:'+address.port;
};
const runPath = join(dirname(source),'Fort058-fixture-test.xctestrun');
const jsonPath = join(directory,'test-run.json');
try {
  await service.start();
  const proxyURL = await startLoopback(proxy);
  const controlsURL = await startLoopback(controls);
  const testRun = JSON.parse(execFileSync('/usr/bin/plutil',['-convert','json','-o','-',source],{encoding:'utf8'}));
  let testTarget = '';
  for (const [name,target] of Object.entries(testRun) as [string,Record<string,unknown>][]) {
    if (target.IsUITestBundle) testTarget = name;
    if (target.IsUITestBundle) target.EnvironmentVariables = {...target.EnvironmentVariables as Record<string,string>,
      FORT_FIXTURE_URL:proxyURL,FORT_FIXTURE_CONTROL_URL:controlsURL,FORT_FIXTURE_CONTROL_TOKEN:controlToken,FORT_FIXTURE_INSTALLATION:service.installation.installationId,FORT_FIXTURE_CODE:service.issueEnrollment().code};
  }
  await writeFile(jsonPath,JSON.stringify(testRun),{mode:0o600});
  execFileSync('/usr/bin/plutil',['-convert','xml1','-o',runPath,jsonPath]);
  try {
    writeFileSync(resultPath+'.log','',{mode:0o600});
    const operation = promisify(execFile)('/usr/bin/xcodebuild',['test-without-building','-xctestrun',runPath,'-destination',destination,'-parallel-testing-enabled','NO','-only-testing:'+testTarget+'/NativeJourneyTests/'+(scenario === 'approval' ? 'testApproveAndCancelFixtureRuns' : scenario === 'unknown' ? 'testCloseUnknownWithoutRetry' : 'testEnrollSendAndReplayAfterAppRestart'),'-collect-test-diagnostics','never','-resultBundlePath',resultPath],{timeout:240_000,maxBuffer:8_000_000});
    operation.child.stdout?.on('data',chunk=>appendFileSync(resultPath+'.log',chunk));
    operation.child.stderr?.on('data',chunk=>appendFileSync(resultPath+'.log',chunk));
    const result = await operation;
    await writeFile(resultPath+'.log',result.stdout+result.stderr,{mode:0o600});
    if (scenario === 'recovery' && dropped !== 1) throw new Error('Lost acknowledgement UI phase did not execute');
    const enrollment = await fetch(service.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({installationId:service.installation.installationId,code:service.issueEnrollment().code,deviceName:'UI evidence verifier'})}).then(r=>r.json()) as {deviceToken:string};
    const headers = {'Authorization':'Bearer '+enrollment.deviceToken,'X-Fort-Installation-Id':service.installation.installationId};
    const agents = await fetch(service.baseURL+'/v1/agents',{headers}).then(r=>r.json()) as {agents:{homeConversationId:string}[]};
    const snapshot = await fetch(service.baseURL+'/v1/conversations/'+agents.agents[0].homeConversationId+'/snapshot',{headers}).then(r=>r.json()) as {messages:{role:string;text:string;requestId:string}[];runs:{state:string;requestId:string}[]};
    if (scenario === 'approval') {
      if (snapshot.messages.filter(m=>m.role==='user').length !== 2 || snapshot.messages.filter(m=>m.role==='assistant').length !== 1 || snapshot.runs.map(r=>r.state).join(',') !== 'completed,canceled') throw new Error('UI approval/cancel evidence did not match the ledger');
    } else if (scenario === 'unknown') {
      if (closureRequests !== 1 || closureResponses.join(',') !== '200') throw new Error('Expected exactly one successful explicit close command');
      if (!lostContact || snapshot.runs.map(r=>r.state).join(',') !== 'closedUnknown,completed') throw new Error('UI unknown-outcome journey did not preserve uncertainty and release later work');
      if (snapshot.messages.filter(m=>m.role==='user').map(m=>m.text).join('|') !== 'Native uncertain fixture request|Queued behind uncertain work') throw new Error('Unknown-outcome UI changed typed text or duplicated requests');
      const replies = snapshot.messages.filter(m=>m.role==='assistant');
      if (replies.length !== 1 || replies[0].requestId !== snapshot.runs[1].requestId) throw new Error('Unknown original request was retried or later reply was lost');
    } else if (snapshot.messages.filter(m=>m.role==='user').length !== 3 || snapshot.messages.filter(m=>m.role==='assistant').length !== 3 || snapshot.runs.some(r=>r.state!=='completed')) throw new Error('UI journey did not end with exactly three accepted requests and replies');
    if (scenario === 'recovery' && snapshot.messages.filter(m=>m.role==='user').map(m=>m.text).join('|') !== ['Native UI fixture message','Saved while offline','Lost reply'].join('|')) throw new Error('Native UI changed typed message text');
    console.log('PASS installed native UI '+scenario+' journey and ledger verification; XCTest results at '+resultPath);
  } catch (error) {
    const failed = error as Error & {stdout?:string;stderr?:string};
    if (failed.stdout !== undefined || failed.stderr !== undefined) await writeFile(resultPath+'.log',(failed.stdout??'')+(failed.stderr??''),{mode:0o600});
    else appendFileSync(resultPath+'.log','\nHarness verification failure: '+failed.message+'\n');
    appendFileSync(resultPath+'.log','\nFixture closure diagnostics: '+JSON.stringify({closureRequests,closureResponses})+'\n');
    throw new Error('Native UI check did not pass; inspect '+resultPath+'.log');
  }
} finally {
  holdRefresh = false; heldRefresh.splice(0).forEach(forward=>forward()); refreshObservers.splice(0).forEach(notify=>notify());
  await Promise.all([new Promise<void>(r=>proxy.close(()=>r())),new Promise<void>(r=>controls.close(()=>r()))]);
  await service.close();
  await rm(runPath,{force:true});
  await rm(directory,{recursive:true,force:true});
}
