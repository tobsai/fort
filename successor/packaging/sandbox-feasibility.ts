import { execFileSync, spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { homedir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const source = dirname(fileURLToPath(import.meta.url));
const command = (tool: string, args: string[]) => execFileSync(tool, args, { encoding: 'utf8', timeout: 60_000, stdio: 'pipe' });
const plist = (values: string) => `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>${values}</dict></plist>`;

type ProbeRun = {
  outcome: 'completed' | 'timedOut' | 'failed'; stdout: string; diagnostics: string;
  parentPid: number; childPid: number | null; childGroup: number | null;
  ownedProcessesAbsent: boolean; elapsedMs: number; executionMs: number | null; hungChildObserved: boolean;
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
      resolveRun({ outcome: timedOut ? 'timedOut' : code === 0 && !outputLimit && ownedProcessesAbsent ? 'completed' : 'failed', stdout, diagnostics: stderr.replace(/^FORT_PROBE_CHILD.*\n/m, ''), parentPid, childPid, childGroup, ownedProcessesAbsent, elapsedMs: performance.now() - started, executionMs: childStartedAt === undefined ? null : performance.now() - childStartedAt, hungChildObserved: stdout.includes('synthetic child is holding its output pipe open') });
    };
    const killWithFallback = () => { stop(); fallback ??= setTimeout(() => { void finish(null); }, 2000); };
    let timer = setTimeout(() => { timedOut = true; killWithFallback(); }, 15_000);
    child.stdout.on('data', chunk => { stdout += chunk.toString(); if (stdout.length > 65_536) { stdout = stdout.slice(0, 65_536); outputLimit = true; killWithFallback(); } });
    child.stderr.on('data', chunk => {
      stderr += chunk.toString();
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
  if (values.scenario !== 'normal' && values.scenario !== 'hang') throw new Error('Unknown synthetic scenario');
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
  writeFileSync(join(app, 'Contents/Resources/node-openssl.cnf'), 'nodejs_conf = fort_node\n[fort_node]\n');
  const executable = join(app, 'Contents/MacOS/FortSandboxProbe');
  command('/usr/bin/xcrun', ['swiftc', join(source, 'sandbox-probe.swift'), '-o', executable]);
  command('/usr/bin/codesign', ['--force', '--sign', values.identity, '--timestamp=none', '--options', 'runtime', '--entitlements', childEntitlements, node]);
  command('/usr/bin/codesign', ['--force', '--sign', values.identity, '--timestamp=none', '--options', 'runtime', '--entitlements', parentEntitlements, app]);
  command('/usr/bin/codesign', ['--verify', '--deep', '--strict', app]);
  const outside = join(root, 'outside-marker');
  writeFileSync(outside, 'synthetic-only');
  const container = join(homedir(), 'Library/Containers', identifier);
  if (existsSync(container)) throw new Error('Synthetic container already exists; refusing reuse');
  const run = await runProbe(executable, outside, values.scenario, timeoutMs);
  if (run.ownedProcessesAbsent) rmSync(container, { recursive: true, force: true });
  if (readFileSync(outside, 'utf8') !== 'synthetic-only') throw new Error('Outside marker changed');
  let proof: object = {};
  if (run.outcome === 'completed') {
    const parsed: unknown = JSON.parse(run.stdout);
    if (!parsed || typeof parsed !== 'object' || !('sqlite' in parsed) || parsed.sqlite !== 'ready' || !('outsideReadDenied' in parsed) || parsed.outsideReadDenied !== true || !('outsideWriteDenied' in parsed) || parsed.outsideWriteDenied !== true || !('loopbackListenDenied' in parsed) || parsed.loopbackListenDenied !== true || !('privateSocketRoundTrip' in parsed) || typeof parsed.privateSocketRoundTrip !== 'boolean') throw new Error('Invalid sandbox evidence');
    proof = parsed;
  }
  const { stdout: _stdout, ...execution } = run;
  const result = { ...proof, ...execution, syntheticContainerAbsent: !existsSync(container), identifier, app, signedIdentityClass: 'existing development identity', providerUsed: false, startupRegistered: false };
  writeFileSync(join(root, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  process.stdout.write(JSON.stringify(result) + '\n');
  if (run.outcome !== 'completed') process.exitCode = 1;
} catch (error) {
  process.stderr.write((error instanceof Error ? error.message : 'Sandbox proof failed') + '\n');
  process.exitCode = 1;
}
