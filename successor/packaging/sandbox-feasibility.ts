import { execFileSync, spawn, spawnSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { copyFileSync, cpSync, existsSync, mkdirSync, readFileSync, readdirSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { homedir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const source = dirname(fileURLToPath(import.meta.url));
const command = (tool: string, args: string[]) => execFileSync(tool, args, { encoding: 'utf8', timeout: 60_000, stdio: 'pipe' });
const plist = (values: string) => `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>${values}</dict></plist>`;
const xml = (value: string) => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');

async function cleanXPC(executables: string[], knownPids: number[]) {
  const owned = () => command('/bin/ps', ['-axww', '-o', 'pid=,pgid=,comm=']).split('\n').flatMap(line => {
    const match = /^\s*(\d+)\s+(\d+)\s+(.+)$/.exec(line);
    return match && executables.includes(match[3]) ? [{ pid: Number(match[1]), group: Number(match[2]) }] : [];
  });
  const observed = new Set(knownPids);
  const deadline = performance.now() + 10_000;
  let active = owned();
  while ((active.length || !knownPids.every(absent)) && performance.now() < deadline) {
    active.forEach(({ pid }) => observed.add(pid));
    await new Promise(resolve => setTimeout(resolve, 25)); active = owned();
  }
  const forced = active.length > 0;
  for (const { pid, group } of active) try { process.kill(pid === group ? -pid : pid, 'SIGKILL'); } catch {}
  const cleanupDeadline = performance.now() + 2000;
  while (active.length && performance.now() < cleanupDeadline) { await new Promise(resolve => setTimeout(resolve, 25)); active = owned(); }
  return { xpcProcessesAbsent: active.length === 0 && knownPids.every(absent), xpcForcedCleanup: forced, xpcObservedPids: [...observed] };
}

type ProbeRun = {
  outcome: 'completed' | 'timedOut' | 'failed'; stdout: string; diagnostics: string;
  parentPid: number; childPid: number | null; childGroup: number | null;
  ownedProcessesAbsent: boolean; elapsedMs: number; executionMs: number | null; hungChildObserved: boolean;
  parentStartupMs: number | null; parentWorkMs: number | null;
};
const absent = (pid: number) => {
  try { process.kill(pid, 0); return false; }
  catch (error) { return error instanceof Error && 'code' in error && error.code === 'ESRCH'; }
};
async function runProbe(executable: string, outside: string, scenario: string, timeoutMs: number): Promise<ProbeRun> {
  return new Promise((resolveRun, reject) => {
    const started = performance.now();
    const child = spawn(executable, [outside, scenario], { detached: true, stdio: ['ignore', 'pipe', 'pipe'], env: { PATH: '/usr/bin:/bin', LANG: 'en_US.UTF-8' } });
    let stdout = '', stderr = '';
    let timedOut = false, outputLimit = false, finishing = false;
    let childStartedAt: number | undefined;
    let parentStartedAt: number | undefined;
    let fallback: ReturnType<typeof setTimeout> | undefined;
    const stop = () => { if (child.pid) { try { process.kill(-child.pid, 'SIGKILL'); } catch {} } };
    const finish = async (code: number | null) => {
      if (finishing) return;
      finishing = true; clearTimeout(timer); clearTimeout(fallback);
      child.stdout.destroy(); child.stderr.destroy(); child.unref();
      const parentPid = child.pid ?? 0;
      const ownership = /^FORT_PROBE_CHILD ([0-9]+) ([0-9]+)$/m.exec(stderr);
      const childPid = ownership ? Number(ownership[1]) : null;
      const childGroup = ownership ? Number(ownership[2]) : null;
      const ownedAbsent = () => parentPid > 1 && childPid !== null && childPid > 1 && childGroup === parentPid && absent(parentPid) && absent(childPid) && absent(-parentPid);
      if (!ownedAbsent()) stop();
      const deadline = performance.now() + 1500;
      while (!ownedAbsent() && performance.now() < deadline) await new Promise(resolve => setTimeout(resolve, 20));
      const ownedProcessesAbsent = ownedAbsent();
      resolveRun({ outcome: timedOut ? 'timedOut' : code === 0 && !outputLimit && ownedProcessesAbsent && parentStartedAt !== undefined ? 'completed' : 'failed', stdout, diagnostics: stderr.replace(/^FORT_PROBE_(?:PARENT|CHILD).*\n/gm, ''), parentPid, childPid, childGroup, ownedProcessesAbsent,
        elapsedMs: performance.now() - started, parentStartupMs: parentStartedAt === undefined ? null : parentStartedAt - started,
        parentWorkMs: parentStartedAt === undefined ? null : (childStartedAt ?? performance.now()) - parentStartedAt,
        executionMs: childStartedAt === undefined ? null : performance.now() - childStartedAt, hungChildObserved: stdout.includes('synthetic child is holding its output pipe open') });
    };
    const killWithFallback = () => { stop(); fallback ??= setTimeout(() => { void finish(null); }, 2000); };
    let timer = setTimeout(() => { timedOut = true; killWithFallback(); }, 15_000);
    child.stdout.on('data', chunk => { stdout += chunk.toString(); if (stdout.length > 65_536) { stdout = stdout.slice(0, 65_536); outputLimit = true; killWithFallback(); } });
    child.stderr.on('data', chunk => {
      stderr += chunk.toString();
      const parent = /^FORT_PROBE_PARENT ([0-9]+) ([0-9]+)$/m.exec(stderr);
      if (parentStartedAt === undefined && parent && Number(parent[1]) === child.pid && Number(parent[2]) === child.pid) {
        parentStartedAt = performance.now(); clearTimeout(timer);
        timer = setTimeout(() => { timedOut = true; killWithFallback(); }, timeoutMs);
      }
      if (childStartedAt === undefined && /^FORT_PROBE_CHILD [0-9]+ [0-9]+$/m.test(stderr)) {
        childStartedAt = performance.now(); clearTimeout(timer);
        timer = setTimeout(() => { timedOut = true; killWithFallback(); }, timeoutMs);
      }
      if (stderr.length > 65_536) { stderr = stderr.slice(0, 65_536); outputLimit = true; killWithFallback(); }
    });
    child.once('error', error => { clearTimeout(timer); clearTimeout(fallback); reject(error); });
    child.once('close', code => { void finish(code); });
  });
}

try {
  const { values } = parseArgs({ options: { runtime: { type: 'string' }, identity: { type: 'string' }, destination: { type: 'string' }, scenario: { type: 'string', default: 'normal' }, 'timeout-ms': { type: 'string', default: '15000' } }, strict: true });
  if (!values.runtime || !values.identity || !values.destination) throw new Error('Required: --runtime, --identity, --destination');
  if (!['normal', 'hang', 'core', 'xpc'].includes(values.scenario)) throw new Error('Unknown synthetic scenario');
  const timeoutMs = Number(values['timeout-ms']);
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 15000) throw new Error('Timeout must be 100 to 15000 milliseconds');
  if (process.platform !== 'darwin') throw new Error('macOS proof only');
  const root = resolve(values.destination);
  mkdirSync(root);
  const app = join(root, 'FortSandboxProbe.app');
  for (const path of ['Contents/MacOS', 'Contents/Frameworks', 'Contents/Resources']) mkdirSync(join(app, path), { recursive: true });
  const identifier = `io.fort.foundation.probe.${randomUUID().toLowerCase()}`;
  writeFileSync(join(app, 'Contents/Info.plist'), plist(`<key>CFBundleIdentifier</key><string>${identifier}</string><key>CFBundleExecutable</key><string>FortSandboxProbe</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleVersion</key><string>1</string><key>LSUIElement</key><true/>`));
  const parentEntitlements = join(root, 'parent.entitlements');
  const childEntitlements = join(root, 'child.entitlements');
  writeFileSync(parentEntitlements, plist('<key>com.apple.security.app-sandbox</key><true/>'));
  writeFileSync(childEntitlements, plist('<key>com.apple.security.app-sandbox</key><true/><key>com.apple.security.inherit</key><true/>'));
  const runtime = join(root, 'runtime-staging');
  command(process.execPath, [join(source, 'node-bundle.ts'), '--runtime', values.runtime, '--identity', values.identity, '--destination', runtime]);
  const node = join(app, 'Contents/MacOS/node');
  copyFileSync(join(runtime, 'bin/node'), node);
  const changes: string[] = [];
  for (const name of readdirSync(join(runtime, 'lib'))) {
    copyFileSync(join(runtime, 'lib', name), join(app, 'Contents/Frameworks', name));
    changes.push('-change', `@loader_path/../lib/${name}`, `@loader_path/../Frameworks/${name}`);
  }
  command('/usr/bin/install_name_tool', [...changes, node]);
  renameSync(join(runtime, 'licenses'), join(app, 'Contents/Resources/NodeLicenses'));
  copyFileSync(join(runtime, 'manifest.json'), join(app, 'Contents/Resources/runtime-staging-manifest.json'));
  copyFileSync(join(source, 'runtime-fixture.mjs'), join(app, 'Contents/Resources/runtime-fixture.mjs'));
  if (values.scenario === 'core' || values.scenario === 'xpc') {
    command(process.execPath, [join(source, 'core-artifact.ts'), '--destination', join(app, 'Contents/Resources/Core')]);
    copyFileSync(join(source, 'core-fixture.mjs'), join(app, 'Contents/Resources/core-fixture.mjs'));
  }
  writeFileSync(join(app, 'Contents/Resources/node-openssl.cnf'), 'nodejs_conf = fort_node\n[fort_node]\n');
  const executable = join(app, 'Contents/MacOS/FortSandboxProbe');
  const xpcExecutables: string[] = [], xpcIdentifiers: string[] = [];
  if (values.scenario === 'xpc') {
    const nativeTransport = ['FortTransport.swift', 'MacOwnerControl.swift', 'OwnerSetup.swift', 'MacAgentEndpoint.swift', 'MacXPCTransport.swift'].map(file => join(source, '../apple/FortKit/Sources/FortKit', file));
    const nativeHost = ['MacCoreProcessTransport.swift', 'MacCoreHost.swift', 'MacHostChannel.swift'].map(file => join(source, '../apple/FortKit/Sources/FortKit', file));
    copyFileSync(join(source, '../host/fixtures/native-core.mjs'), join(app, 'Contents/Resources/native-core.mjs'));
    const details = spawnSync('/usr/bin/codesign', ['--display', '--verbose=4', join(runtime, 'bin/node')], { encoding: 'utf8', timeout: 10_000 });
    const team = /^TeamIdentifier=([A-Z0-9]+)$/m.exec(details.stderr)?.[1];
    if (details.status !== 0 || !team) throw new Error('Could not identify existing signing team');
    const parentInfo = join(app, 'Contents/Info.plist');
    writeFileSync(parentInfo, readFileSync(parentInfo, 'utf8').replace('</dict>', `<key>FortSigningTeam</key><string>${team}</string></dict>`));
    const pin = (id: string) => `anchor apple generic and identifier "${id}" and certificate leaf[subject.OU] = "${team}"`;
    for (const [suffix, client] of [['channel', identifier], ['rejected', identifier + '.wrong-client'], ['core', identifier]]) {
      const serviceId = identifier + '.' + suffix;
      const service = join(app, 'Contents/XPCServices', serviceId + '.xpc');
      const serviceExecutable = join(service, 'Contents/MacOS/FortChannel');
      mkdirSync(dirname(serviceExecutable), { recursive: true });
      if (suffix === 'core') {
        const resources = join(service, 'Contents/Resources'); mkdirSync(resources);
        for (const file of ['Core', 'NodeLicenses', 'native-core.mjs', 'node-openssl.cnf']) cpSync(join(app, 'Contents/Resources', file), join(resources, file), { recursive: true });
        cpSync(join(app, 'Contents/Frameworks'), join(service, 'Contents/Frameworks'), { recursive: true });
        const coreNode = join(service, 'Contents/MacOS/node'); copyFileSync(node, coreNode);
        const guardian = join(service, 'Contents/MacOS/FortProcessGuardian');
        command('/usr/bin/xcrun', ['swiftc', join(source, '../host/mac-process-guardian.swift'), '-o', guardian]);
        command('/usr/bin/codesign', ['--force', '--sign', values.identity, '--timestamp=none', '--options', 'runtime', '--entitlements', childEntitlements, guardian]);
        command('/usr/bin/codesign', ['--force', '--sign', values.identity, '--timestamp=none', '--options', 'runtime', '--entitlements', childEntitlements, coreNode]);
        xpcExecutables.push(coreNode, guardian);
      }
      writeFileSync(join(service, 'Contents/Info.plist'), plist(`<key>CFBundleIdentifier</key><string>${serviceId}</string><key>CFBundleExecutable</key><string>FortChannel</string><key>CFBundlePackageType</key><string>XPC!</string><key>CFBundleVersion</key><string>1</string><key>FortClientRequirement</key><string>${xml(pin(client))}</string><key>FortOutsideMarker</key><string>${xml(join(root, 'outside-marker'))}</string><key>FortClientIdentifier</key><string>${client}</string><key>FortSigningTeam</key><string>${team}</string><key>XPCService</key><dict><key>ServiceType</key><string>Application</string></dict>`));
      command('/usr/bin/xcrun', ['swiftc', ...nativeTransport, ...nativeHost, join(source, suffix === 'core' ? 'xpc-core-service.swift' : 'xpc-service.swift'), '-o', serviceExecutable]);
      command('/usr/bin/codesign', ['--force', '--sign', values.identity, '--timestamp=none', '--options', 'runtime', '--entitlements', parentEntitlements, service]);
      xpcExecutables.push(serviceExecutable); xpcIdentifiers.push(serviceId);
    }
    const mainDirectory = join(root, 'parent-source'); mkdirSync(mainDirectory);
    copyFileSync(join(source, 'sandbox-probe.swift'), join(mainDirectory, 'main.swift'));
    command('/usr/bin/xcrun', ['swiftc', '-D', 'FORT_XPC_PROBE', ...nativeTransport, join(source, 'xpc-client.swift'), join(source, 'xpc-core-client.swift'), join(mainDirectory, 'main.swift'), '-o', executable]);
    xpcExecutables.push(node);
  } else command('/usr/bin/xcrun', ['swiftc', join(source, 'sandbox-probe.swift'), '-o', executable]);
  command('/usr/bin/codesign', ['--force', '--sign', values.identity, '--timestamp=none', '--options', 'runtime', '--entitlements', childEntitlements, node]);
  command('/usr/bin/codesign', ['--force', '--sign', values.identity, '--timestamp=none', '--options', 'runtime', '--entitlements', parentEntitlements, app]);
  command('/usr/bin/codesign', ['--verify', '--deep', '--strict', app]);
  const outside = join(root, 'outside-marker');
  writeFileSync(outside, 'synthetic-only');
  const container = join(homedir(), 'Library/Containers', identifier);
  if ([identifier, ...xpcIdentifiers].some(id => existsSync(join(homedir(), 'Library/Containers', id)))) throw new Error('Synthetic container already exists; refusing reuse');
  const run = await runProbe(executable, outside, values.scenario, timeoutMs);
  const xpcMatch = /^FORT_PROBE_XPC (.+)$/m.exec(run.diagnostics);
  const xpc: unknown = xpcMatch ? JSON.parse(xpcMatch[1]) : {};
  if (!xpc || typeof xpc !== 'object') throw new Error('Malformed XPC proof');
  const servicePid = 'xpcServicePid' in xpc ? xpc.xpcServicePid : undefined;
  const knownPids = typeof servicePid === 'number' && Number.isSafeInteger(servicePid) && servicePid > 1 ? [servicePid] : [];
  const corePids: number[] = 'xpcCorePids' in xpc && Array.isArray(xpc.xpcCorePids) && xpc.xpcCorePids.every(pid => typeof pid === 'number' && Number.isSafeInteger(pid) && pid > 1) ? xpc.xpcCorePids : [];
  const runtimePids: number[] = 'xpcCoreRuntimePids' in xpc && Array.isArray(xpc.xpcCoreRuntimePids) && xpc.xpcCoreRuntimePids.every(pid => typeof pid === 'number' && Number.isSafeInteger(pid) && pid > 1) ? xpc.xpcCoreRuntimePids : [];
  const coreServicePid = 'xpcCoreServicePid' in xpc ? xpc.xpcCoreServicePid : undefined;
  if (typeof coreServicePid === 'number' && Number.isSafeInteger(coreServicePid) && coreServicePid > 1) knownPids.push(coreServicePid);
  const helpers: number[] = 'xpcCoreServicePids' in xpc && Array.isArray(xpc.xpcCoreServicePids) && xpc.xpcCoreServicePids.every(pid => typeof pid === 'number' && Number.isSafeInteger(pid) && pid > 1) ? xpc.xpcCoreServicePids : [];
  knownPids.push(...helpers);
  knownPids.push(...corePids, ...runtimePids);
  const xpcCleanup = values.scenario === 'xpc' ? await cleanXPC(xpcExecutables, knownPids) : undefined;
  const xpcCoreProcessesAbsent = new Set(corePids).size === 4 && new Set(runtimePids).size === 4 && new Set(helpers).size === 2 && corePids.every(pid => absent(pid) && absent(-pid)) && runtimePids.every(absent) && helpers.every(absent);
  if (run.ownedProcessesAbsent && (!xpcCleanup || xpcCleanup.xpcProcessesAbsent)) {
    for (const id of [identifier, ...xpcIdentifiers]) rmSync(join(homedir(), 'Library/Containers', id), { recursive: true, force: true });
  }
  if (readFileSync(outside, 'utf8') !== 'synthetic-only') throw new Error('Outside marker changed');
  let proof: object = {};
  if (run.outcome === 'completed') {
    const parsed: unknown = JSON.parse(run.stdout);
    if (!parsed || typeof parsed !== 'object' || !('sqlite' in parsed) || parsed.sqlite !== 'ready' || !('outsideReadDenied' in parsed) || parsed.outsideReadDenied !== true || !('outsideWriteDenied' in parsed) || parsed.outsideWriteDenied !== true || !('loopbackListenDenied' in parsed) || parsed.loopbackListenDenied !== true || !('privateSocketRoundTrip' in parsed) || typeof parsed.privateSocketRoundTrip !== 'boolean') throw new Error('Invalid sandbox evidence');
    if (['core', 'xpc'].includes(values.scenario) && ['coreAuthentication', 'coreExactMessages', 'coreReplay', 'coreRestart', 'coreNoDuplicate', 'coreSocketAbsent'].some(key => !(key in parsed) || Reflect.get(parsed, key) !== true)) throw new Error('Compiled core evidence was not confirmed');
    proof = parsed;
  }
  const { stdout: _stdout, ...execution } = run;
  const result = { ...proof, ...execution, ...xpc, ...xpcCleanup, ...(values.scenario === 'xpc' ? { xpcCoreProcessesAbsent } : {}), syntheticContainerAbsent: [identifier, ...xpcIdentifiers].every(id => !existsSync(join(homedir(), 'Library/Containers', id))), identifier, xpcIdentifiers, app, signedIdentityClass: 'existing development identity', providerUsed: false, startupRegistered: false };
  writeFileSync(join(root, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  process.stdout.write(JSON.stringify(result) + '\n');
  if (run.outcome !== 'completed') process.exitCode = 1;
  if (values.scenario === 'xpc' && (xpcCleanup?.xpcForcedCleanup || xpcCleanup?.xpcProcessesAbsent !== true || !xpcCoreProcessesAbsent || ['xpcSetupCancelPending', 'xpcSetupCancellation', 'xpcSetupCancelRetry', 'xpcSetupCancelForeignRejected', 'xpcSetupCancelNoHTTP', 'xpcSetupCancelRetainsReceipt', 'xpcSetupCancelClientRejected', 'xpcSetupCancelServiceRejected', 'xpcSetupCancelDisabled', 'xpcOwnerSetup', 'xpcOwnerSetupRetry', 'xpcOwnerSetupForeignRejected', 'xpcOwnerSetupNoHTTP', 'xpcSetupClientRejected', 'xpcSetupServiceRejected', 'xpcSetupDisabled', 'xpcCoreAbruptRecovery', 'xpcCoreGuarded', 'xpcOwnerRecovery', 'xpcOwnerForeignRejected', 'xpcOwnerNoHTTPRecovery', 'xpcOwnerClientRejected', 'xpcOwnerServiceRejected', 'xpcOwnerDisabled', 'xpcRoundTrip', 'xpcClientRejected', 'xpcServiceRejected', 'xpcCancellation', 'xpcTimeout', 'xpcMalformedResponseRejected', 'xpcInvalidStatusRejected', 'xpcOversizedRequestRejected', 'xpcOversizedResponseRejected', 'xpcCoreAuthentication', 'xpcCoreExactMessages', 'xpcCoreReplay', 'xpcCoreCancellation', 'xpcCoreRestart', 'xpcCoreNoDuplicate', 'xpcCoreStopped', 'xpcCoreOuterExecutableDenied', 'xpcCoreOutsideReadDenied', 'xpcCoreOutsideWriteDenied', 'xpcCoreTCPDenied'].some(key => !(key in xpc) || Reflect.get(xpc, key) !== true))) process.exitCode = 1;
} catch (error) {
  process.stderr.write((error instanceof Error ? error.message : 'Sandbox proof failed') + '\n');
  process.exitCode = 1;
}
