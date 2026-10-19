import { spawnSync, execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, readdirSync, rmSync, lstatSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { isAbsolute, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

export function buildCore(destination: string) {
  if (process.version !== 'v26.5.0') throw new Error('Core packaging requires Node 26.5.0');
  if (!isAbsolute(destination) || destination === '/' || normalize(destination) !== destination || !destination.isWellFormed() || destination.includes('\0')) throw new Error('Use an absolute, new artifact directory');
  const source = fileURLToPath(new URL('../', import.meta.url));
  const repository = fileURLToPath(new URL('../../', import.meta.url));
  mkdirSync(destination, { mode: 0o700 });
  const created = lstatSync(destination);
  try {
    const compiler = spawnSync(process.execPath, [join(source, 'node_modules/typescript/bin/tsc'), '-p', join(source, 'tsconfig.core.json'), '--outDir', destination], { env: {}, encoding: 'utf8', timeout: 20_000 });
    if (compiler.status !== 0) throw new Error('Core compilation failed: ' + compiler.stdout + compiler.stderr);
    let license: Buffer;
    try { license = readFileSync(join(repository, 'LICENSE')); }
    catch (error) {
      if (!(error instanceof Error && 'code' in error && error.code === 'ENOENT')) throw error;
      license = execFileSync('git', ['show', 'HEAD:LICENSE'], { cwd: repository, env: {}, timeout: 5000 });
    }
    writeFileSync(join(destination, 'LICENSE'), license, { flag: 'wx' });
    writeFileSync(join(destination, 'package.json'), JSON.stringify({ name: 'fort-core', version: '0.1.0', private: true, type: 'module', license: 'MIT', engines: { node: '26.5.0' }, exports: { '.': { types: './service/openai.d.ts', import: './service/openai.js' }, './host-channel': { types: './host/private-channel.d.ts', import: './host/private-channel.js' }, './secure-network': { types: './host/secure-network.d.ts', import: './host/secure-network.js' }, './network-control': { types: './host/network-control.d.ts', import: './host/network-control.js' }, './history-maintenance': { types: './host/history-maintenance.d.ts', import: './host/history-maintenance.js' }, './owned-host': { types: './host/owned-core.d.ts', import: './host/owned-core.js' } } }, null, 2) + '\n', { flag: 'wx' });
    const files = readdirSync(destination, { recursive: true, encoding: 'utf8' }).flatMap(path => {
      const info = lstatSync(join(destination, path));
      if (info.isDirectory()) return [];
      if (!info.isFile() || info.nlink !== 1) throw new Error('Unexpected core artifact file');
      const bytes = readFileSync(join(destination, path));
      return [{ path, bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }];
    }).sort((a, b) => a.path.localeCompare(b.path));
    const manifest = { format: 1, artifact: 'fort-core', runtime: 'Node 26.5.0 supplied separately', files };
    writeFileSync(join(destination, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx' });
    return manifest;
  } catch (error) {
    const current = lstatSync(destination, { throwIfNoEntry: false });
    if (current?.isDirectory() && current.dev === created.dev && current.ino === created.ino) rmSync(destination, { recursive: true, force: true });
    throw error;
  }
}

if (import.meta.main) {
  const { values } = parseArgs({ options: { destination: { type: 'string' } }, strict: true });
  if (!values.destination) throw new Error('Specify --destination');
  process.stdout.write(JSON.stringify(buildCore(values.destination)) + '\n');
}
