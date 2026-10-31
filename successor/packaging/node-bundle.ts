import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, realpathSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, resolve } from 'node:path';
import { parseArgs } from 'node:util';

type Reference = { original: string; source: string };
type Image = { source: string; name: string; kind: 'executable' | 'library'; references: Reference[]; rpaths: string[] };
const command = (tool: string, args: string[]) => execFileSync(tool, args, { encoding: 'utf8', timeout: 20_000, stdio: 'pipe' });
const hash = (path: string) => createHash('sha256').update(readFileSync(path)).digest('hex');
const systemImage = (path: string) => path.startsWith('/usr/lib/') || path.startsWith('/System/Library/');

function rpaths(source: string): string[] {
  return [...command('/usr/bin/otool', ['-l', source]).matchAll(/cmd LC_RPATH\n\s+cmdsize \d+\n\s+path (.+) \(offset \d+\)/g)].map(match => match[1]);
}

// Homebrew's Node links Homebrew libraries, so its closure is copied from the Cellar. The official
// nodejs.org build links only system libraries; --official accepts one extracted release whose root
// holds bin/node and LICENSE, so the exact pinned version can ship (Homebrew can't pin a patch release).
function closure(runtime: string, official?: string): Image[] {
  const images = new Map<string, Image>();
  const names = new Map<string, string>();
  const rootRpaths = rpaths(runtime).map(path => path.replace('@loader_path', dirname(runtime)).replace('@executable_path', dirname(runtime)));
  function visit(source: string, kind: Image['kind']): void {
    source = realpathSync(source);
    if (images.has(source)) return;
    if (official ? !source.startsWith(official + '/') : !source.startsWith('/opt/homebrew/Cellar/')) throw new Error(`Unreviewed runtime dependency origin: ${source}`);
    const name = kind === 'executable' ? 'node' : basename(source);
    if (names.has(name) && names.get(name) !== source) throw new Error(`Conflicting library filename: ${name}`);
    names.set(name, source);
    const paths = rpaths(source);
    const image: Image = { source, name, kind, references: [], rpaths: paths };
    images.set(source, image);
    const identity = kind === 'library' ? command('/usr/bin/otool', ['-D', source]).split('\n')[1]?.trim() : undefined;
    const dependencies = command('/usr/bin/otool', ['-L', source]).split('\n').slice(1).map(line => line.trim().split(' (compatibility version')[0]).filter(Boolean);
    for (const original of dependencies) {
      if (systemImage(original) || original === identity) continue;
      const expand = (path: string) => path.replace('@loader_path', dirname(source)).replace('@executable_path', dirname(runtime));
      const candidates = original.startsWith('@rpath/')
        ? [...paths.map(expand), ...rootRpaths].map(path => join(path, original.slice('@rpath/'.length)))
        : [expand(original)];
      const found = candidates.find(path => !path.startsWith('@') && existsSync(path));
      if (!found) throw new Error(`Unresolved dependency ${original} of ${source}`);
      const resolved = realpathSync(found);
      image.references.push({ original, source: resolved });
      visit(resolved, 'library');
    }
  }
  visit(runtime, 'executable');
  return [...images.values()];
}

function packageRoot(source: string, official?: string): string {
  if (official) {
    if (!source.startsWith(official + '/')) throw new Error(`Missing package provenance: ${source}`);
    return official;
  }
  const match = source.match(/^(\/opt\/homebrew\/Cellar\/[^/]+\/[^/]+)\//);
  if (!match) throw new Error(`Missing package provenance: ${source}`);
  return match[1];
}

function officialRoot(runtime: string): string {
  const root = dirname(dirname(runtime));
  if (basename(dirname(runtime)) !== 'bin' || basename(runtime) !== 'node' || !/^node-v\d+\.\d+\.\d+-darwin-arm64$/.test(basename(root)) || !existsSync(join(root, 'LICENSE'))) throw new Error('Use bin/node inside an extracted official node-vX.Y.Z-darwin-arm64 release');
  return root;
}

function bundle(runtime: string, destination: string, identity: string, official: boolean): void {
  const officialDirectory = official ? officialRoot(runtime) : undefined;
  const images = closure(runtime, officialDirectory);
  mkdirSync(destination);
  for (const directory of ['bin', 'lib', 'licenses']) mkdirSync(join(destination, directory));
  const notices: { source: string; file: string; sha256: string }[] = [];
  for (const root of new Set(images.map(image => packageRoot(image.source, officialDirectory)))) {
    const licenses = readdirSync(root).filter(name => /^(license|copying|notice|authors)([._-]|$)/i.test(name));
    const sqliteHeader = join(root, 'include/sqlite3.h');
    const sqliteDedication = basename(dirname(root)) === 'sqlite' && existsSync(sqliteHeader) && readFileSync(sqliteHeader, 'utf8').slice(0, 1000).includes('The author disclaims copyright');
    if (sqliteDedication) licenses.push('include/sqlite3.h');
    if (!sqliteDedication && !licenses.some(name => /^(license|copying|notice)/i.test(name))) throw new Error(`Missing legal attribution: ${root}`);
    const directory = join(destination, 'licenses', `${basename(dirname(root))}-${basename(root)}`);
    mkdirSync(directory);
    for (const name of licenses) {
      const file = join(directory, basename(name));
      copyFileSync(join(root, name), file);
      notices.push({ source: join(root, name), file: file.slice(destination.length + 1), sha256: hash(file) });
    }
  }
  const manifest: { source: string; file: string; sourceSHA256: string; bundledSHA256: string; bytes: number }[] = [];
  for (const image of images) {
    const file = `${image.kind === 'executable' ? 'bin' : 'lib'}/${image.name}`;
    const target = join(destination, file);
    copyFileSync(image.source, target);
    const changes: string[] = [];
    if (image.kind === 'library') changes.push('-id', `@loader_path/${image.name}`);
    for (const reference of image.references) {
      changes.push('-change', reference.original, `@loader_path/${image.kind === 'executable' ? '../lib/' : ''}${basename(reference.source)}`);
    }
    for (const path of image.rpaths) changes.push('-delete_rpath', path);
    if (changes.length) command('/usr/bin/install_name_tool', [...changes, target]);
    command('/usr/bin/codesign', ['--force', '--sign', identity, '--timestamp=none', '--options', 'runtime', target]);
    command('/usr/bin/codesign', ['--verify', '--strict', target]);
    manifest.push({ source: image.source, file, sourceSHA256: hash(image.source), bundledSHA256: hash(target), bytes: readFileSync(target).byteLength });
  }
  writeFileSync(join(destination, 'manifest.json'), JSON.stringify({ schemaVersion: 1, purpose: 'Local packaging feasibility; not distribution approval', images: manifest, notices }, null, 2) + '\n');
  process.stdout.write(JSON.stringify({ destination, images: manifest.length, notices: notices.length, bytes: manifest.reduce((sum, image) => sum + image.bytes, 0) }) + '\n');
}

try {
  if (process.platform !== 'darwin') throw new Error('macOS packaging tool');
  const { values } = parseArgs({ options: { runtime: { type: 'string' }, destination: { type: 'string' }, identity: { type: 'string', default: '-' }, official: { type: 'boolean', default: false } }, strict: true });
  if (!values.runtime || !values.destination) throw new Error('Required: --runtime <installed Node> --destination <new directory>');
  bundle(realpathSync(values.runtime), resolve(values.destination), values.identity, values.official);
} catch (error) {
  process.stderr.write((error instanceof Error ? error.message : 'Node bundle failed') + '\n');
  process.exitCode = 1;
}
