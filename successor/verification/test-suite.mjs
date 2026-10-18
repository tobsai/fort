import { readdirSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import { spawnSync } from 'node:child_process';

const root = fileURLToPath(new URL('../', import.meta.url));
const special = new Map([
  ['host/guardian.test.ts', 'macos'],
  ['prototype/mvp-app.test.ts', 'macos'],
  ['runner/confinement-canary.test.ts', 'macos'],
  ['runner/guarded-codex-process.test.ts', 'macos'],
  ['packaging/node-bundle.test.ts', 'signed'],
  ['packaging/sandbox-feasibility.test.ts', 'signed']
]);
function discover(directory = '') {
  return readdirSync(join(root, directory), { withFileTypes: true }).flatMap(entry => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      if (entry.name.startsWith('.') || ['node_modules', 'DerivedData', 'evidence'].includes(entry.name) || /\.(xcodeproj|xcworkspace)$/.test(entry.name)) return [];
      return discover(path);
    }
    return entry.isFile() && /\.test\.(ts|mjs)$/.test(entry.name) ? [path] : [];
  }).sort();
}

const { values } = parseArgs({ options: { suite: { type: 'string', default: 'portable' }, list: { type: 'boolean', default: false } }, strict: true });
if (!['portable', 'macos', 'signed'].includes(values.suite)) throw new Error('Choose portable, macos or signed');
const discovered = discover();
for (const path of special.keys()) if (!discovered.includes(path)) throw new Error('Classified test is missing: ' + path);
const files = discovered.filter(path => (special.get(path) ?? 'portable') === values.suite);
if (!files.length) throw new Error('No tests discovered');
if (values.list) process.stdout.write(JSON.stringify({ suite: values.suite, discovered: discovered.length, files }) + '\n');
else {
  if (process.version !== 'v26.5.0') throw new Error('Tests require Node 26.5.0');
  if (values.suite !== 'portable' && process.platform !== 'darwin') throw new Error('This suite requires macOS');
  if (values.suite === 'signed' && !process.env.FORT_SIGNING_IDENTITY) throw new Error('Select an existing signing identity for the signed suite');
  process.stdout.write(`Running ${files.length}/${discovered.length} test files in the ${values.suite} suite.\n`);
  const result = spawnSync(process.execPath, ['--test', '--test-concurrency=4', ...files], { cwd: root, stdio: 'inherit', timeout: 600_000 });
  if (result.error) throw result.error;
  process.exitCode = result.status ?? 1;
}
