import { spawnSync, execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { isAbsolute, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

export function buildCore(destination: string) {
  if (!isAbsolute(destination)) throw new Error('Use an absolute, new artifact directory');
  const source = fileURLToPath(new URL('../', import.meta.url));
  const repository = fileURLToPath(new URL('../../', import.meta.url));
  mkdirSync(destination, { mode: 0o700 });
  try {
    const compiler = spawnSync(process.execPath, [join(source, 'node_modules/typescript/bin/tsc'), '-p', join(source, 'tsconfig.core.json'), '--outDir', destination], { encoding: 'utf8', timeout: 20_000 });
    if (compiler.status !== 0) throw new Error('Core compilation failed: ' + compiler.stdout + compiler.stderr);
    let license: Buffer;
    try { license = readFileSync(join(repository, 'LICENSE')); }
    catch (error) {
      if (!(error instanceof Error && 'code' in error && error.code === 'ENOENT')) throw error;
      license = execFileSync('git', ['show', 'HEAD:LICENSE'], { cwd: repository });
    }
    writeFileSync(join(destination, 'LICENSE'), license, { flag: 'wx' });
    writeFileSync(join(destination, 'package.json'), JSON.stringify({ name: 'fort-core', version: '0.1.0', private: true, type: 'module', license: 'MIT', engines: { node: '26.5.0' }, exports: { '.': { types: './service/openai.d.ts', import: './service/openai.js' } } }, null, 2) + '\n', { flag: 'wx' });
    const files = readdirSync(destination, { recursive: true, encoding: 'utf8' }).filter(path => statSync(join(destination, path)).isFile()).sort().map(path => {
      const bytes = readFileSync(join(destination, path));
      return { path, bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') };
    });
    const manifest = { format: 1, artifact: 'fort-core', runtime: 'Node 26.5.0 supplied separately', files };
    writeFileSync(join(destination, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx' });
    return manifest;
  } catch (error) { rmSync(destination, { recursive: true, force: true }); throw error; }
}

if (import.meta.main) {
  const { values } = parseArgs({ options: { destination: { type: 'string' } }, strict: true });
  if (!values.destination) throw new Error('Specify --destination');
  process.stdout.write(JSON.stringify(buildCore(values.destination)) + '\n');
}
