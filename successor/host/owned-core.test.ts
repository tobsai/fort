import assert from 'node:assert/strict';
import { fork, spawnSync } from 'node:child_process';
import { chmod, mkdir, mkdtemp, readFile, readdir, rm, stat, symlink, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { randomBytes } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import { test, type TestContext } from 'node:test';
import { PassThrough } from 'node:stream';
import { serveOwnedCore } from './owned-core.ts';
import {request as httpsRequest} from 'node:https';
import {networkTestIdentity} from './network-test-identity.ts';

function object(value: unknown): Record<string, unknown> {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value));
  return value as Record<string, unknown>;
}
function string(value: unknown): string { assert.equal(typeof value, 'string'); return value as string; }
function body(frame: Record<string, unknown>, status = 200) {
  assert.equal(frame.kind, 'response');
  const response = object(frame.response); assert.equal(response.status, status);
  return object(JSON.parse(Buffer.from(string(response.body), 'base64').toString()));
}
async function fixture(t: TestContext) {
  const directory = await mkdtemp('/tmp/fort-owned-core-');
  const closers: (() => Promise<void>)[] = [];
  let completedDisposals = 0;
  t.after(async () => { for (const close of closers) await close(); await rm(directory, { recursive: true, force: true }); });
  return { directory, disposals: () => completedDisposals, async start(options: { start?: 'restore' | 'resume'; behavior?: string; modulePath?: string; expectFailure?: boolean; network?: boolean } = {}) {
    const { start = 'restore', behavior = 'ordinary', modulePath = fileURLToPath(new URL('./owned-core.ts', import.meta.url)) } = options;
    const starting = performance.now();
    const child = fork(fileURLToPath(new URL('./fixtures/owned-core.mjs', import.meta.url)), [modulePath, directory, start, behavior, options.network ? 'network' : 'disabled'], {
      execPath: process.execPath, execArgv: modulePath.endsWith('.js') ? ['--jitless', '--no-experimental-strip-types'] : [],
      cwd: directory, detached: true, stdio: ['pipe', 'pipe', 'pipe', 'ipc'], env: { PATH: '/usr/bin:/bin', LANG: 'C' },
    });
    const ready = Promise.withResolvers<void>(), exited = Promise.withResolvers<{ code: number | null; signal: NodeJS.Signals | null }>();
    const responses = new Map<number, ReturnType<typeof Promise.withResolvers<Record<string, unknown>>>>();
    let pending = '', diagnostics = '', nextID = 0, executions = 0, disposals = 0, serving = 0, expectedExit = options.expectFailure ? 1 : 0;
    child.stdin?.on('error', () => {});
    child.stderr?.on('data', chunk => { diagnostics = (diagnostics + chunk.toString()).slice(-2000); });
    child.once('error', error => ready.reject(error));
    child.once('exit', (code, signal) => exited.resolve({ code, signal }));
    child.on('message', value => { const message = object(value); if (message.kind === 'executed') executions++; if (message.kind === 'serving') serving++; if (message.kind === 'disposed') { disposals++; completedDisposals++; } });
    child.stdout?.setEncoding('utf8');
    child.stdout?.on('data', chunk => {
      pending += chunk;
      for (let newline; (newline = pending.indexOf('\n')) >= 0;) {
        const frame = object(JSON.parse(pending.slice(0, newline))); pending = pending.slice(newline + 1);
        if (frame.kind === 'ready') ready.resolve();
        else if (typeof frame.id === 'number') responses.get(frame.id)?.resolve(frame);
      }
    });
    const close = async () => {
      child.stdin?.end();
      await Promise.race([exited.promise, delay(7000, undefined, { ref: false })]);
      if (child.exitCode === null && child.signalCode === null && child.pid) process.kill(-child.pid, 'SIGKILL');
      const result = await exited.promise;
      child.stdin?.destroy(); child.stdout?.destroy(); child.stderr?.destroy();
      assert.equal(result.signal, null, diagnostics); assert.equal(result.code, expectedExit, diagnostics);
      const pid = child.pid; assert.ok(pid); assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
    };
    closers.push(close);
    await Promise.race([ready.promise, exited.promise.then(() => { throw new Error('Owned fixture exited before ready: ' + diagnostics); }), delay(5000, undefined, { ref: false }).then(() => { throw new Error('Owned fixture startup timed out'); })]);
    const readyMs = performance.now() - starting;
    function send(kind: string, value: unknown) {
      const id = ++nextID, result = Promise.withResolvers<Record<string, unknown>>(); responses.set(id, result);
      const key = kind === 'request' ? 'request' : kind === 'ownerSetup' ? 'setup' : kind === 'networkControl' ? 'network' : 'recovery';
      child.stdin?.write(JSON.stringify({ protocolVersion: 1, kind, id, [key]: value }) + '\n');
      return {id,result:Promise.race([result.promise, exited.promise.then(() => { throw new Error('Owned host exited'); }), delay(8000, undefined, { ref: false }).then(() => { throw new Error('Owned command timed out'); })]).finally(() => responses.delete(id))};
    }
    const command=(kind:string,value:unknown)=>send(kind,value).result;
    return { close, command, send, cancel(id:number){child.stdin?.write(JSON.stringify({protocolVersion:1,kind:'cancel',id})+'\n');}, readyMs, executions: () => executions, disposals: () => disposals, serving: () => serving,
      async api(installationID: string, token: string | undefined, path: string, value?: unknown, status = 200) {
        return body(await command('request', { path, installationID, token, method: value === undefined ? 'GET' : 'POST', timeout: 3, body: value === undefined ? undefined : Buffer.from(JSON.stringify(value)).toString('base64') }), status);
      },
      abort() { child.send('abort'); },
      raw(bytes: string) { expectedExit = 1; child.stdin?.write(bytes); },
    };
  } };
}

async function journey(t: TestContext, modulePath?: string) {
  const local = await fixture(t), first = await local.start({ modulePath });
  const offer = body(await first.command('ownerSetup', { operation: 'begin', timeout: 3 }));
  assert.equal(offer.kind, 'available');
  const installationId = string(offer.installationId), deviceToken = randomBytes(32).toString('base64url');
  const setup = { installationId, setupId: string(offer.setupId), deviceId: 'synthetic-owner', deviceToken };
  const receipt = body(await first.command('ownerSetup', { operation: 'complete', ...setup, timeout: 3 }));
  assert.deepEqual(receipt, { installationId, deviceId: setup.deviceId, expiresAt: null });
  const installation = await first.api(installationId, deviceToken, '/v1/installation');
  assert.equal(installation.installationId, installationId);
  await first.api(installationId, deviceToken, '/v1/fixture-bootstrap', undefined, 404);
  await first.api(installationId, undefined, '/v1/installation', undefined, 401);
  const agent = await first.api(installationId, deviceToken, '/v1/agents', { name: 'Owned host Home', model: 'exact-synthetic', instructions: 'Echo the supplied text.' }, 201);
  const path = '/v1/conversations/' + string(agent.homeConversationId);
  const request = { requestId: 'owned-exact-turn', text: '  Cedar 🌲 Café e\u0301\r\n', recipientAgentId: agent.agentId,
    bindingRevisionId: object(agent.binding).bindingRevisionId, recoveryEpoch: installation.recoveryEpoch, skills: [] };
  const accepted = await first.api(installationId, deviceToken, path + '/messages', request, 201);
  let snapshot = await first.api(installationId, deviceToken, path + '/snapshot');
  for (let count = 0; count < 50 && (!Array.isArray(snapshot.messages) || snapshot.messages.length !== 2); count++) {
    await delay(10); snapshot = await first.api(installationId, deviceToken, path + '/snapshot');
  }
  assert.ok(Array.isArray(snapshot.messages)); assert.deepEqual(snapshot.messages.map(message => object(message).text), [request.text, request.text]);
  assert.equal(first.executions(), 1);
  await first.close(); assert.equal(first.disposals(), 1);
  await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
  const second = await local.start({ modulePath });
  assert.equal((await second.api(installationId, deviceToken, '/v1/installation')).installationId, installationId);
  assert.deepEqual(body(await second.command('ownerSetup', { operation: 'begin', timeout: 3 })), { kind: 'paired', ...receipt });
  assert.deepEqual((await second.api(installationId, deviceToken, path + '/messages', request, 201)).receipt, accepted.receipt);
  assert.equal(second.executions(), 0);
  assert.deepEqual(await second.api(installationId, deviceToken, '/v1/host/stop', {}), { runtimeStopped: true, revoked: false });
  await second.close(); assert.equal(second.disposals(), 1);
  const paused = await local.start({ modulePath });
  assert.equal((await paused.api(installationId, deviceToken, '/v1/runtime')).state, 'stopped');
  await paused.api(installationId, deviceToken, '/v1/agents', { name: 'Blocked', model: 'exact-synthetic', instructions: 'Do not execute.' }, 503);
  assert.equal(paused.executions(), 0);
  await paused.close();
  const resumed = await local.start({ modulePath, start: 'resume' });
  const retained = await resumed.api(installationId, deviceToken, path + '/snapshot');
  assert.deepEqual(retained.messages, snapshot.messages);
  await resumed.api(installationId, deviceToken, '/v1/agents', { name: 'Resumed', model: 'exact-synthetic', instructions: 'Explicit start.' }, 201);
  await resumed.close();
  if (process.env.FORT_OWNED_CORE_BENCHMARK === '1') {
    const samples = { freshMs: first.readyMs, reopenedMs: [second.readyMs, paused.readyMs, resumed.readyMs] };
    assert.ok(samples.freshMs <= 2000); assert.ok(samples.reopenedMs.every(value => value <= 500));
    process.stdout.write('OWNED_CORE_STARTUPS=' + JSON.stringify(samples) + '\n');
  }
}

test('owned core preserves exact send/replay, owner pairing and stopped intent until explicit resume', t => journey(t));

test('the compiled artifact runs the owned core journey with vendor implementations absent', async t => {
  const directory = await mkdtemp('/tmp/fort-owned-artifact-');
  t.after(() => rm(directory, { recursive: true, force: true }));
  const artifact = join(directory, 'Core');
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('../packaging/core-artifact.ts', import.meta.url)), '--destination', artifact], { encoding: 'utf8', timeout: 30_000 });
  assert.equal(result.status, 0, result.stderr);
  await journey(t, join(artifact, 'host/owned-core.js'));
});

test('owned core refuses a missing runtime before creating a ledger or becoming ready', async t => {
  const local = await fixture(t);
  await assert.rejects(local.start({ behavior: 'missing-runtime', expectFailure: true }), /before ready/);
  assert.deepEqual(await readdir(local.directory), ['core-pid.json']);
});

test('failed private socket startup disposes its runtime and preserves the unknown path', async t => {
  const local = await fixture(t), socket = join(local.directory, 'core.sock');
  await writeFile(socket, 'synthetic unrelated marker', { mode: 0o600 });
  await assert.rejects(local.start({ expectFailure: true }), /before ready/);
  assert.equal(local.disposals(), 1);
  assert.equal(await readFile(socket, 'utf8'), 'synthetic unrelated marker');
  await rm(socket);
  const reopened = await local.start();
  assert.equal(body(await reopened.command('ownerSetup', { operation: 'begin', timeout: 3 })).kind, 'available');
  await reopened.close(); assert.equal(local.disposals(), 2);
});

test('a ledger set up for another runtime refuses to start and says why', async t => {
  const local = await fixture(t), first = await local.start();
  const offer = body(await first.command('ownerSetup', { operation: 'begin', timeout: 3 }));
  const installationId = string(offer.installationId), deviceToken = randomBytes(32).toString('base64url');
  await first.command('ownerSetup', { operation: 'complete', installationId, setupId: string(offer.setupId), deviceId: 'synthetic-owner', deviceToken, timeout: 3 });
  await first.api(installationId, deviceToken, '/v1/agents', { name: 'Bound agent', model: 'exact-synthetic', instructions: 'Echo.' }, 201);
  assert.equal(first.serving(), 1);
  await first.close();
  await assert.rejects(local.start({ behavior: 'other-provider', expectFailure: true }), /Installation runtime identity changed/);
});

test('owned core disposes transferred runtime without mutating rejected storage or pre-aborted startup', async t => {
  const root = await mkdtemp('/tmp/fort-owned-preflight-');
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = join(root, 'private'), shared = join(root, 'shared'), linked = join(root, 'linked'), file = join(root, 'file');
  await mkdir(directory, { mode: 0o700 }); await mkdir(shared, { mode: 0o700 }); await chmod(shared, 0o755);
  await symlink(directory, linked); await writeFile(file, 'owned test marker');
  const cases = [directory, shared, linked, file, join(root, 'absent'), 'relative'];
  for (const dataDirectory of cases) {
    let disposed = 0;
    await assert.rejects(serveOwnedCore({ dataDirectory, input: new PassThrough(), output: new PassThrough(),
      provider: { provider: 'preflight-fixture', name: 'Preflight', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
      runtime: { async execute() { throw new Error('No execution permitted'); }, async dispose() { await delay(1); disposed++; } },
    }));
    assert.equal(disposed, 1);
  }
  let disposed = 0;
  await assert.rejects(serveOwnedCore({ dataDirectory: join(root, 'aborted'), input: new PassThrough(), output: new PassThrough(), signal: AbortSignal.abort(),
    provider: { provider: 'preflight-fixture', name: 'Preflight', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
    runtime: { async execute() { throw new Error('No execution permitted'); }, async dispose() { disposed++; } },
  }));
  assert.equal(disposed, 1);
  assert.deepEqual(await readdir(directory), []); assert.deepEqual(await readdir(shared), []);
  assert.equal((await stat(shared)).mode & 0o777, 0o755);
  assert.deepEqual((await readdir(root)).sort(), ['file', 'linked', 'private', 'shared']);
});

test('owned core confirms pending setup cancellation and exact recovery without a network control route', async t => {
  const local = await fixture(t), host = await local.start();
  const offer = body(await host.command('ownerSetup', { operation: 'begin', timeout: 3 }));
  const identity = { installationId: string(offer.installationId), setupId: string(offer.setupId), deviceId: 'synthetic-owner' };
  const canceled = body(await host.command('ownerSetup', { operation: 'cancel', ...identity, timeout: 3 }));
  assert.deepEqual(canceled, { ...identity, outcome: 'notPaired' });
  const replacement = body(await host.command('ownerSetup', { operation: 'begin', timeout: 3 }));
  assert.notEqual(replacement.setupId, offer.setupId);
  const token = randomBytes(32).toString('base64url');
  body(await host.command('ownerSetup', { operation: 'complete', ...identity, setupId: replacement.setupId, deviceToken: token, timeout: 3 }));
  const rejected = body(await host.command('ownerRecovery', { installationID: identity.installationId, deviceID: 'foreign', timeout: 3 }), 409);
  assert.deepEqual(rejected, { installationId: identity.installationId, deviceId: 'foreign', outcome: 'identityRejected' });
  await host.api(identity.installationId, token, '/v1/pairing/recover', {}, 404);
  const proof = body(await host.command('ownerRecovery', { installationID: identity.installationId, deviceID: identity.deviceId, timeout: 3 }));
  assert.deepEqual(proof, { installationId: identity.installationId, deviceId: identity.deviceId, runtimeStopped: true, revoked: true });
  await host.api(identity.installationId, token, '/v1/installation', undefined, 401);
  await host.close(); assert.equal(host.disposals(), 1);
});

test('abort, malformed input and failed cleanup cannot leave a ready owned host', async t => {
  for (const behavior of ['abort', 'malformed', 'cleanup-failure']) {
    const local = await fixture(t), host = await local.start({ behavior, expectFailure: behavior === 'cleanup-failure' });
    if (behavior === 'abort') host.abort();
    if (behavior === 'malformed') host.raw('not-json\n');
    await host.close(); assert.equal(host.disposals(), 1);
    await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
  }
});


async function networkHost(t:TestContext,options:Parameters<Awaited<ReturnType<typeof fixture>>['start']>[0]={network:true}) {
  const local=await fixture(t),host=await local.start(options);
  const offer=body(await host.command('ownerSetup',{operation:'begin',timeout:3}));
  const owner={installationId:string(offer.installationId),deviceId:'network-primary'},token=randomBytes(32).toString('base64url');
  body(await host.command('ownerSetup',{operation:'complete',...owner,setupId:offer.setupId,deviceToken:token,timeout:3}));
  const control=(command:unknown,timeout=3)=>host.command('networkControl',{command,timeout});
  const identity=networkTestIdentity(),configuration={bindAddress:'127.0.0.1',prefixLength:32,port:0,serverName:'127.0.0.1',certificate:identity.certificate.toString('base64'),privateKey:identity.privateKey.toString('base64')};
  const call=(url:string,credential?:string)=>new Promise<number>((resolve,reject)=>{
    const req=httpsRequest(new URL('/v1/installation',url),{agent:false,ca:identity.ca,headers:{'X-Fort-Installation-Id':owner.installationId,...(credential?{Authorization:'Bearer '+credential}:{})}},res=>{res.resume();res.once('end',()=>resolve(res.statusCode!));});
    req.once('error',reject);req.end();
  });
  return {local,host,owner,token,control,configuration,call,async start() {
    const initial=body(await control({operation:'inspect',owner}));
    const command={operation:'start',owner,sessionId:initial.sessionId,generation:initial.generation,configuration};
    const active=body(await control(command)),listener=object(active.listener);assert.equal(listener.state,'active');
    return {initial,command,active,url:string(listener.url)};
  }};
}

test('explicit private owner control opens an authenticated TLS endpoint and confirms cleanup',async t=>{
  const f=await networkHost(t),{initial,command,active,url}=await f.start();
  assert.deepEqual(initial.listener,{state:'stopped',cause:'initial'});assert.equal(active.generation,1);
  assert.equal(await f.call(url),401);assert.equal(await f.call(url,f.token),200);
  assert.deepEqual(body(await f.control(command)),active);
  assert.equal((await f.control({operation:'inspect',owner:{...f.owner,deviceId:'foreign'}})).error,'unavailable');
  await f.host.api(f.owner.installationId,f.token,'/v1/network/control',{},404);
  const stopped=body(await f.control({operation:'stop',owner:f.owner,sessionId:initial.sessionId,generation:1}));
  assert.deepEqual(stopped.listener,{state:'stopped',cause:'requested'});await assert.rejects(f.call(url,f.token));
  await f.host.close();assert.equal(f.host.disposals(),1);
});

test('private listener control remains absent without explicit host composition',async t=>{
  const f=await networkHost(t,{expectFailure:true});
  await assert.rejects(f.control({operation:'inspect',owner:f.owner}),/Owned host exited/);
  await f.host.close();assert.equal(f.host.disposals(),1);
});

test('owner stop and disconnect close the listener while preserving exact cleanup inspection',async t=>{
  for(const action of ['/v1/host/stop','/v1/pairing/disconnect']) {
    const f=await networkHost(t),{url,active}=await f.start();
    await f.host.api(f.owner.installationId,f.token,action,{});
    let status=body(await f.control({operation:'inspect',owner:f.owner}));
    for(let count=0;count<100 && object(status.listener).state!=='stopped';count++){await delay(1);status=body(await f.control({operation:'inspect',owner:f.owner}));}
    assert.deepEqual(status.listener,{state:'stopped',cause:'ownerUnavailable'});assert.equal(status.generation,active.generation);
    await assert.rejects(f.call(url,f.token));
    assert.equal((await f.control({operation:'start',owner:f.owner,sessionId:status.sessionId,generation:status.generation,configuration:f.configuration})).error,'unavailable');
    assert.equal(object(body(await f.control({operation:'stop',owner:f.owner,sessionId:status.sessionId,generation:status.generation})).listener).state,'stopped');
    await f.host.close();
  }
});

test('a paired phone cannot acquire private listener control or revoke the primary scope',async t=>{
  const f=await networkHost(t),{active}=await f.start();
  const invitation={invitationId:'network-invitation',code:randomBytes(32).toString('base64url')};
  await f.host.api(f.owner.installationId,f.token,'/v1/pairing/invitation',invitation,201);
  const phone={...invitation,installationId:f.owner.installationId,deviceId:'synthetic-phone',deviceToken:randomBytes(32).toString('base64url'),deviceName:'Synthetic phone'};
  await f.host.api(f.owner.installationId,undefined,'/v1/devices/join',phone,201);
  for(const operation of ['inspect','start','stop']) {
    const command={operation,owner:{...f.owner,deviceId:phone.deviceId},...(operation==='inspect'?{}:{sessionId:active.sessionId,generation:active.generation}),...(operation==='start'?{configuration:f.configuration}:{})};
    assert.equal((await f.control(command)).error,'unavailable');
  }
  assert.deepEqual(body(await f.control({operation:'inspect',owner:f.owner})),active);
  await f.host.close();
});

test('private listener EOF, host abort and runtime cleanup failure release the actual endpoint',async t=>{
  for(const behavior of ['ordinary','abort','cleanup-failure']) {
    const f=await networkHost(t,{network:true,behavior,expectFailure:behavior==='cleanup-failure'}),{url}=await f.start();
    if(behavior==='abort')f.host.abort();await f.host.close();
    await assert.rejects(f.call(url,f.token));assert.equal(f.host.disposals(),1);
    await assert.rejects(stat(join(f.local.directory,'core.sock')),{code:'ENOENT'});
  }
});

test('canceled and timed-out private starts recover by exact retry without a second listener',async t=>{
  for(const cancel of [true,false]) {
    const f=await networkHost(t,{network:true,behavior:'network-slow-open'}),initial=body(await f.control({operation:'inspect',owner:f.owner}));
    const command={operation:'start',owner:f.owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration};
    const attempt=f.host.send('networkControl',{command,timeout:cancel?3:0.000001});
    if(cancel)f.host.cancel(attempt.id);
    const response=await attempt.result;
    assert.equal(response.kind,'error');assert.equal(response.error,cancel?'canceled':'timeout');
    const active=body(await f.control(command));assert.equal(active.generation,1);
    assert.equal(object(active.listener).state,'active');assert.equal(await f.call(string(object(active.listener).url),f.token),200);
    await f.host.close();
  }
});

test('malformed listener commands stay redacted and cannot disturb an active generation',async t=>{
  const f=await networkHost(t),{active}=await f.start();
  for(const command of [null,{}, {operation:'inspect',owner:f.owner,secret:'synthetic-marker'}, {operation:'start',owner:f.owner,sessionId:active.sessionId,generation:active.generation,configuration:{...f.configuration,privateKey:'synthetic-marker'}}]) {
    const response=await f.control(command);assert.equal(response.error,'unavailable');assert.ok(!JSON.stringify(response).includes('synthetic-marker'));
  }
  assert.deepEqual(body(await f.control({operation:'inspect',owner:f.owner})),active);await f.host.close();
});


test('unconfirmed listener cleanup still disposes the private service and fails host shutdown',{timeout:10000},async t=>{
  const f=await networkHost(t,{network:true,behavior:'network-close-failure',expectFailure:true}),{url}=await f.start();
  await f.host.close();assert.equal(f.host.disposals(),1);await assert.rejects(f.call(url,f.token));
  await assert.rejects(stat(join(f.local.directory,'core.sock')),{code:'ENOENT'});
});

test('malformed private listener envelopes fail closed and dispose an already active endpoint',async t=>{
  for(const envelope of [null,{command:{},timeout:0},{command:{},timeout:31},{command:{},timeout:3,extra:true},{command:'x'.repeat(2*1024*1024),timeout:3}]) {
    const f=await networkHost(t,{network:true,expectFailure:true}),{url}=await f.start();
    await assert.rejects(f.host.command('networkControl',envelope),/Owned host exited/);
    await f.host.close();assert.equal(f.host.disposals(),1);await assert.rejects(f.call(url,f.token));
  }
});

test('a restarted private host stays stopped and rejects its prior listener session',async t=>{
  const f=await networkHost(t),{command,url}=await f.start();await f.host.close();
  const next=await f.local.start({network:true});
  const initial=body(await next.command('networkControl',{command:{operation:'inspect',owner:f.owner},timeout:3}));
  assert.deepEqual(initial.listener,{state:'stopped',cause:'initial'});assert.equal(initial.generation,0);assert.notEqual(initial.sessionId,command.sessionId);
  assert.equal((await next.command('networkControl',{command,timeout:3})).error,'unavailable');
  await assert.rejects(f.call(url,f.token));await next.close();
  const ledger=await readFile(join(f.local.directory,'openai-candidate-b.sqlite'));
  assert.ok(!ledger.includes(f.configuration.privateKey));assert.ok(!ledger.includes(f.configuration.certificate));
});

test('compiled-only private owner networking preserves exact authentication and shutdown',async t=>{
  const directory=await mkdtemp('/tmp/fort-owner-network-artifact-');t.after(()=>rm(directory,{recursive:true,force:true}));
  const artifact=join(directory,'core');
  const built=spawnSync(process.execPath,[fileURLToPath(new URL('../packaging/core-artifact.ts',import.meta.url)),'--destination',artifact],{encoding:'utf8',timeout:30000});
  assert.equal(built.status,0,built.stderr);
  const f=await networkHost(t,{network:true,modulePath:join(artifact,'host/owned-core.js')}),{url}=await f.start();
  assert.equal(await f.call(url,f.token),200);await f.host.close();await assert.rejects(f.call(url,f.token));
  await rm(join(artifact,'host/network-control.js'));await rm(join(artifact,'host/secure-network.js'));
  const without=await f.local.start({modulePath:join(artifact,'host/owned-core.js')});
  assert.equal((await without.api(f.owner.installationId,f.token,'/v1/installation')).installationId,f.owner.installationId);
  await without.close();
});

test('twenty isolated private owner listener cycles meet their budget',{skip:!process.env.FORT_OWNER_NETWORK_BENCHMARK},async t=>{
  const destination=process.env.FORT_OWNER_NETWORK_BENCHMARK;assert.ok(destination);
  const f=await networkHost(t),samples:number[]=[];
  for(let index=0;index<25;index++) {
    const started=performance.now(),{command,active,url}=await f.start();
    assert.deepEqual(body(await f.control(command)),active);assert.equal(await f.call(url,f.token),200);
    const stopped=body(await f.control({operation:'stop',owner:f.owner,sessionId:active.sessionId,generation:active.generation}));
    assert.equal(object(stopped.listener).state,'stopped');if(index>=5)samples.push(performance.now()-started);
  }
  await f.host.close();const sorted=[...samples].sort((a,b)=>a-b);
  await writeFile(destination,JSON.stringify({node:process.version,platform:process.platform,architecture:process.arch,warmups:5,samplesMs:samples,p50Ms:sorted[9],p95Ms:sorted[18],maximumMs:sorted[19]},null,2)+'\n',{flag:'wx'});
  assert.ok(sorted[18]<100);
});


test('private EOF cancels a stalled opening before waiting for command workers',{timeout:10000},async t=>{
  const f=await networkHost(t,{network:true,behavior:'network-stalled-open'}),initial=body(await f.control({operation:'inspect',owner:f.owner}));
  const pending=f.host.send('networkControl',{timeout:3,command:{operation:'start',owner:f.owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration}});
  const lost=assert.rejects(pending.result,/Owned host exited/);
  assert.equal(object(body(await f.control({operation:'inspect',owner:f.owner})).listener).state,'starting');
  await f.host.close();await lost;assert.equal(f.host.disposals(),1);
});


test('mutating the caller option after host start cannot enable private networking',async t=>{
  const f=await networkHost(t,{behavior:'network-option-mutation',expectFailure:true});
  await assert.rejects(f.control({operation:'inspect',owner:f.owner}),/Owned host exited/);
  await f.host.close();
});
