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

async function runProbe(executable: string, outside: string): Promise<string> {
  return new Promise((resolveRun, reject) => {
    const child = spawn(executable, [outside], { detached: true, stdio: ['ignore', 'pipe', 'pipe'], env: { PATH: '/usr/bin:/bin', LANG: 'en_US.UTF-8' } });
    let stdout = '';
    let stderr = '';
    let timedOut = false;
    const stop = () => { if (child.pid) { try { process.kill(-child.pid, 'SIGKILL'); } catch {} } };
    const timer = setTimeout(() => { timedOut = true; stop(); }, 15_000);
    child.stdout.on('data', chunk => { stdout += chunk.toString(); if (stdout.length > 65_536) stop(); });
    child.stderr.on('data', chunk => { stderr += chunk.toString(); if (stderr.length > 65_536) stop(); });
    child.once('error', error => { clearTimeout(timer); reject(error); });
    child.once('close', (code, signal) => {
      clearTimeout(timer);
      let absent = false;
      if (child.pid) {
        try { process.kill(-child.pid, 0); }
        catch (error) { absent = error instanceof Error && 'code' in error && error.code === 'ESRCH'; }
      }
      if (!absent) stop();
      if (code !== 0 || timedOut || !absent) { reject(new Error(`Sandbox probe failed (exit ${code}, signal ${signal}, groupAbsent ${absent}): ${stderr}\n${stdout}`)); return; }
      resolveRun(stdout);
    });
  });
}

try {
  const { values } = parseArgs({ options: { runtime: { type: 'string' }, identity: { type: 'string' }, destination: { type: 'string' } }, strict: true });
  if (!values.runtime || !values.identity || !values.destination) throw new Error('Required: --runtime, --identity, --destination');
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
  let output: string;
  try { output = await runProbe(executable, outside); }
  finally { rmSync(container, { recursive: true, force: true }); }
  if (readFileSync(outside, 'utf8') !== 'synthetic-only') throw new Error('Outside marker changed');
  const proof: unknown = JSON.parse(output);
  if (!proof || typeof proof !== 'object' || !('sqlite' in proof) || proof.sqlite !== 'ready' || !('outsideReadDenied' in proof) || proof.outsideReadDenied !== true || !('outsideWriteDenied' in proof) || proof.outsideWriteDenied !== true || !('loopbackListenDenied' in proof) || proof.loopbackListenDenied !== true) throw new Error('Invalid sandbox evidence');
  const result = { ...proof, ownedProcessesAbsent: true, syntheticContainerAbsent: !existsSync(container), identifier, app, signedIdentityClass: 'existing development identity', providerUsed: false, startupRegistered: false };
  writeFileSync(join(root, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  process.stdout.write(JSON.stringify(result) + '\n');
} catch (error) {
  process.stderr.write((error instanceof Error ? error.message : 'Sandbox proof failed') + '\n');
  process.exitCode = 1;
}
