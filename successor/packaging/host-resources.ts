import { spawnSync } from 'node:child_process';
import { mkdirSync, lstatSync, readFileSync, readdirSync, rmSync, writeFileSync, copyFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { isAbsolute, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import { buildCore } from './core-artifact.ts';
import { buildCodexExecutor } from './codex-executor-artifact.ts';

export function buildHostResources(destination: string) {
  if (process.version !== 'v26.5.0') throw new Error('Host packaging requires Node 26.5.0');
  if (!isAbsolute(destination) || destination === '/' || normalize(destination) !== destination || !destination.isWellFormed() || destination.includes('\0')) throw new Error('Use an absolute, new artifact directory');
  const source = fileURLToPath(new URL('../', import.meta.url));
  mkdirSync(destination, { mode: 0o700 });
  const created = lstatSync(destination);
  try {
    buildCore(join(destination, 'core'));
    buildCodexExecutor(join(destination, 'executor'));
    copyFileSync(join(destination, 'executor/node-openssl.cnf'), join(destination, 'node-openssl.cnf'));
    const context = join(destination, 'context'); mkdirSync(context, { mode: 0o700 });
    const compiler = spawnSync(process.execPath, [join(source, 'node_modules/typescript/bin/tsc'), '-p', join(source, 'tsconfig.context.json'), '--outDir', context], { env: {}, encoding: 'utf8', timeout: 20_000 });
    if (compiler.status !== 0) throw new Error('Context compilation failed: ' + compiler.stdout + compiler.stderr);
    copyFileSync(join(destination, 'core/LICENSE'), join(context, 'LICENSE'));
    for (const entry of ['main.mjs', 'history-maintenance.mjs']) copyFileSync(join(source, 'host', entry), join(destination, entry));
    writeFileSync(join(destination, 'package.json'), JSON.stringify({ name: 'fort-host-resources', version: '0.1.0', private: true, type: 'module', license: 'MIT' }, null, 2) + '\n', { flag: 'wx' });
    const files = readdirSync(destination, { recursive: true, encoding: 'utf8' }).flatMap(path => {
      const info = lstatSync(join(destination, path));
      if (info.isDirectory()) return [];
      if (!info.isFile() || info.nlink !== 1) throw new Error('Unexpected host resource file');
      const bytes = readFileSync(join(destination, path));
      return [{ path, bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }];
    }).sort((a, b) => a.path.localeCompare(b.path));
    const manifest = { format: 1, artifact: 'fort-host-resources', requires: { platform: 'macOS', node: '26.5.0 supplied separately', codex: '0.163.0 supplied separately', guardian: 'Fort Mac process guardian supplied separately' }, files };
    writeFileSync(join(destination, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx' });
    return manifest;
  } catch (error) {
    const current = lstatSync(destination, { throwIfNoEntry: false });
    if (current?.isDirectory() && current.dev === created.dev && current.ino === created.ino) rmSync(destination, { recursive: true, force: true });
    throw error;
  }
}

if (import.meta.main) {
  const { values } = parseArgs({ options: { destination: { type: 'string' } }, strict: true, allowPositionals: false });
  if (!values.destination) throw new Error('Specify --destination');
  process.stdout.write(JSON.stringify(buildHostResources(values.destination)) + '\n');
}
