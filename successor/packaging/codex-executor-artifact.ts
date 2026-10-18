import { spawnSync, execFileSync } from 'node:child_process';
import { mkdirSync, lstatSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { isAbsolute, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

export function buildCodexExecutor(destination: string) {
  if (process.version !== 'v26.5.0') throw new Error('Executor packaging requires Node 26.5.0');
  if (!isAbsolute(destination) || destination === '/' || normalize(destination) !== destination || !destination.isWellFormed() || destination.includes('\0')) throw new Error('Use an absolute, new artifact directory');
  const source = fileURLToPath(new URL('../', import.meta.url));
  const repository = fileURLToPath(new URL('../../', import.meta.url));
  mkdirSync(destination, { mode: 0o700 });
  const created = lstatSync(destination);
  try {
    const compiler = spawnSync(process.execPath, [join(source, 'node_modules/typescript/bin/tsc'), '-p', join(source, 'tsconfig.codex-executor.json'), '--outDir', destination], { env: {}, encoding: 'utf8', timeout: 20_000 });
    if (compiler.status !== 0) throw new Error('Executor compilation failed: ' + compiler.stdout + compiler.stderr);
    let license: Buffer;
    try { license = readFileSync(join(repository, 'LICENSE')); }
    catch (error) {
      if (!(error instanceof Error && 'code' in error && error.code === 'ENOENT')) throw error;
      license = execFileSync('git', ['show', 'HEAD:LICENSE'], { cwd: repository, env: {}, timeout: 5000 });
    }
    writeFileSync(join(destination, 'LICENSE'), license, { flag: 'wx' });
    writeFileSync(join(destination, 'package.json'), JSON.stringify({ name: 'fort-codex-executor', version: '0.1.0', private: true,
      type: 'module', license: 'MIT', os: ['darwin'], engines: { node: '26.5.0' },
      exports: { '.': { types: './runner/codex-executor.d.ts', import: './runner/codex-executor.js' } } }, null, 2) + '\n', { flag: 'wx' });
    const files = readdirSync(destination, { recursive: true, encoding: 'utf8' }).flatMap(path => {
      const info = lstatSync(join(destination, path));
      if (info.isDirectory()) return [];
      if (!info.isFile() || info.nlink !== 1) throw new Error('Unexpected executor artifact file');
      const bytes = readFileSync(join(destination, path));
      return [{ path, bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }];
    }).sort((a, b) => a.path.localeCompare(b.path));
    const manifest = { format: 1, artifact: 'fort-codex-executor',
      requires: { platform: 'macOS', node: '26.5.0 supplied separately', codex: '0.157.1 supplied separately', guardian: 'Fort Mac process guardian supplied separately' }, files };
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
  process.stdout.write(JSON.stringify(buildCodexExecutor(values.destination)) + '\n');
}
