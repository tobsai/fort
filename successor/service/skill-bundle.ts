import { createHash } from 'node:crypto';
import type { SkillBundle, SkillBundleFile } from '../contracts/v1.ts';

export class InvalidSkillBundle extends Error {}
function reject(message: string): never { throw new InvalidSkillBundle(message); }
function object(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) reject('Use a bundle manifest object.');
  const result = value as Record<string, unknown>;
  if (Object.keys(result).some(key => !keys.includes(key))) reject('The bundle contains an unsupported manifest field.');
  return result;
}
function text(value: unknown, limit: number): string {
  if (typeof value !== 'string' || !value.isWellFormed() || !value.trim() || Buffer.byteLength(value) > limit) reject('A bundle manifest field is missing or exceeds its limit.');
  return value;
}
function path(value: unknown): string {
  const result = text(value, 512);
  if (/[\\:\x00-\x1f\x7f]/.test(result) || result.split('/').some(part => !part || part === '.' || part === '..' || part.endsWith('.') || part.endsWith(' '))) reject('Bundle paths must be safe relative file paths.');
  return result;
}
function strings(value: unknown, limit: number): string[] {
  if (!Array.isArray(value) || value.length > limit) reject('A bundle manifest list exceeds its limit.');
  const result = value.map(item => text(item, 128));
  if (new Set(result).size !== result.length) reject('Bundle manifest lists cannot contain duplicates.');
  return result;
}
const hash = (value: string | Buffer) => createHash('sha256').update(value).digest('hex');

export function parseSkillBundle(value: unknown): SkillBundle {
  const v = object(value, ['schemaVersion','name','version','source','license','entrypoint','supportedRuntimes','requestedTools','scripts','files','digest']);
  if (v.schemaVersion !== 1 || !Array.isArray(v.files) || v.files.length < 1 || v.files.length > 64 || !Array.isArray(v.scripts) || v.scripts.length > 16) reject('Unsupported bundle schema or file count.');
  let totalBytes = 0;
  const files: SkillBundleFile[] = v.files.map((item): SkillBundleFile => {
    const file = object(item, ['path','kind','executable','contentBase64','bytes','sha256']);
    if (file.kind !== 'file' || typeof file.executable !== 'boolean' || typeof file.contentBase64 !== 'string' || file.contentBase64.length > 350_000) reject('Only bounded regular files with explicit executable metadata can be imported.');
    const bytes = Buffer.from(file.contentBase64, 'base64');
    if (bytes.toString('base64') !== file.contentBase64 || bytes.length > 262_144) reject('File content must be canonical base64 within the file limit.');
    totalBytes += bytes.length;
    if (totalBytes > 524_288) reject('Bundle content exceeds 512 KiB.');
    const sha256 = hash(bytes);
    if ((file.sha256 !== undefined && file.sha256 !== sha256) || (file.bytes !== undefined && file.bytes !== bytes.length)) reject('File content does not match its recorded digest or size.');
    return { path:path(file.path),kind:'file',executable:file.executable,contentBase64:file.contentBase64,bytes:bytes.length,sha256 };
  }).sort((a,b)=>a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
  const paths = new Set<string>();
  for (const file of files) {
    const key = file.path.normalize('NFC').toLowerCase();
    if (paths.has(key)) reject('Bundle paths collide on a supported filesystem.');
    paths.add(key);
  }
  for (const key of paths) if (key.split('/').slice(0,-1).some((_part,index,parts)=>paths.has(parts.slice(0,index+1).join('/')))) reject('A bundle file cannot also be a parent directory.');
  const scripts = v.scripts.map(item => {
    const script = object(item, ['path','interpreter','resourceScope']);
    return { path:path(script.path),interpreter:text(script.interpreter,128),resourceScope:strings(script.resourceScope,32) };
  }).sort((a,b)=>a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
  if (new Set(scripts.map(script=>script.path)).size !== scripts.length || scripts.some(script=>!files.some(file=>file.path===script.path)) || files.some(file=>file.executable&&!scripts.some(script=>script.path===file.path))) reject('Every script and executable file needs an exact file and declared interpreter/resource scope.');
  const entrypoint = path(v.entrypoint), entry = files.find(file=>file.path===entrypoint);
  if (!entry || entry.executable || scripts.some(script=>script.path===entrypoint) || entry.bytes > 16_384) reject('Select a non-executable instruction entry file within 16 KiB.');
  let instructions: string;
  try { instructions = new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(Buffer.from(entry.contentBase64,'base64')); }
  catch { reject('The instruction entry file must contain valid UTF-8 text.'); }
  text(instructions,16_384);
  const supportedRuntimes = strings(v.supportedRuntimes,8);
  if (!supportedRuntimes.length) reject('Declare at least one supported runtime.');
  const canonical = { schemaVersion:1 as const,name:text(v.name,128),version:text(v.version,128),source:text(v.source,2048),license:text(v.license,512),entrypoint,supportedRuntimes,requestedTools:strings(v.requestedTools,32),scripts,files };
  const digest = hash(JSON.stringify(canonical));
  if (v.digest !== undefined && v.digest !== digest) reject('The bundle does not match its recorded content digest.');
  return { ...canonical,digest };
}

export function skillInstructions(bundle: SkillBundle): string {
  const entry = bundle.files.find(file=>file.path===bundle.entrypoint);
  if (!entry) throw new InvalidSkillBundle('The instruction entry file is missing.');
  return new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(Buffer.from(entry.contentBase64,'base64'));
}
