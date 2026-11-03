import { createHash } from 'node:crypto';
import { chmodSync, mkdirSync, renameSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { gunzipSync } from 'node:zlib';

// Image build step (Spec 063, revision 2026-11-03): installs the official Claude Code CLI for Linux from Anthropic's npm
// package, at the exact version the hub's Claude Agents are bound to. That version is the Claude entry in FORT_RUNTIMES
// (Railway passes it to the build); with none, the pinned default. The default's tarball digest is pinned here. Any other
// version is checked against the npm registry's own integrity value, the check `npm install` makes.
export const DEFAULT_CLAUDE_CODE = { version: '2.1.311', sha256: '4353c611f9a33063c5525348a15f1212bdc0085e4a7a81b8370b790003df1b56' };
const PACKAGE = '@anthropic-ai/claude-code-linux-x64', REGISTRY = 'https://registry.npmjs.org/';
const VERSION = /^\d+\.\d+\.\d+$/;

// The Claude version FORT_RUNTIMES names, or undefined. Anything that is not an exact version is refused, not guessed at.
export function claudeVersionFrom(runtimes: string | undefined): string | undefined {
  if (!runtimes) return undefined;
  const listed: unknown = JSON.parse(runtimes);
  if (!Array.isArray(listed)) throw new Error('FORT_RUNTIMES is not a list');
  const row = listed.find(entry => entry && typeof entry === 'object' && (entry as { provider?: unknown }).provider === 'claude-code') as { runtimeVersion?: unknown } | undefined;
  if (!row) return undefined;
  if (typeof row.runtimeVersion !== 'string' || !VERSION.test(row.runtimeVersion)) throw new Error('The Claude entry in FORT_RUNTIMES has no exact version');
  return row.runtimeVersion;
}

// Reads one file out of a plain (ustar) tar archive.
export function untarFile(archive: Buffer, name: string): Buffer {
  for (let offset = 0; offset + 512 <= archive.length;) {
    const header = archive.subarray(offset, offset + 512);
    if (header.every(byte => byte === 0)) break;
    const field = (start: number, length: number) => header.subarray(start, start + length).toString('utf8').replace(/\0.*$/s, '');
    const path = (field(345, 155) ? field(345, 155) + '/' : '') + field(0, 100), size = parseInt(field(124, 12).trim() || '0', 8);
    if (!Number.isSafeInteger(size) || size < 0) throw new Error('Unreadable Claude Code package');
    if (path === name && (field(156, 1) === '0' || field(156, 1) === '')) return Buffer.from(archive.subarray(offset + 512, offset + 512 + size));
    offset += 512 + Math.ceil(size / 512) * 512;
  }
  throw new Error('The Claude Code package has no ' + name);
}

async function fetchBytes(url: string): Promise<Buffer> {
  const response = await fetch(url, { redirect: 'error' });
  if (!response.ok) throw new Error('Download failed with ' + response.status);
  return Buffer.from(await response.arrayBuffer());
}

export async function installClaudeCode(root: string, version: string): Promise<void> {
  if (!VERSION.test(version)) throw new Error('Select an exact Claude Code version');
  const tarball = `${REGISTRY}${PACKAGE}/-/claude-code-linux-x64-${version}.tgz`;
  const bytes = await fetchBytes(tarball);
  if (version === DEFAULT_CLAUDE_CODE.version) {
    const digest = createHash('sha256').update(bytes).digest('hex');
    if (digest !== DEFAULT_CLAUDE_CODE.sha256) throw new Error('Claude Code digest mismatch: ' + digest);
  } else {
    const metadata = JSON.parse((await fetchBytes(`${REGISTRY}${PACKAGE}/${version}`)).toString('utf8')) as { version?: unknown; dist?: { tarball?: unknown; integrity?: unknown } };
    const integrity = metadata.dist?.integrity;
    if (metadata.version !== version || metadata.dist?.tarball !== tarball || typeof integrity !== 'string' || !integrity.startsWith('sha512-')) throw new Error('The registry did not describe this Claude Code version');
    if (createHash('sha512').update(bytes).digest('base64') !== integrity.slice('sha512-'.length)) throw new Error('Claude Code integrity mismatch');
  }
  const binary = untarFile(gunzipSync(bytes), 'package/claude');
  mkdirSync(root, { recursive: true, mode: 0o755 });
  const target = join(root, version);
  writeFileSync(target + '.partial', binary, { mode: 0o755 });
  chmodSync(target + '.partial', 0o755);
  renameSync(target + '.partial', target);
  writeFileSync(join(root, 'version'), version + '\n', { mode: 0o644 });
}

if (import.meta.main) {
  try {
    const root = process.argv[2] ?? '/opt/fort/claude';
    let version = DEFAULT_CLAUDE_CODE.version;
    try { version = claudeVersionFrom(process.env.FORT_RUNTIMES) ?? version; }
    catch (error) { process.stdout.write('Using Claude Code ' + version + ': ' + (error instanceof Error ? error.message : 'FORT_RUNTIMES unreadable') + '\n'); }
    // A version the registry can't give stops nothing: the hub gets the default, and Claude stays on the Macs until they match.
    try { await installClaudeCode(root, version); }
    catch (error) {
      if (version === DEFAULT_CLAUDE_CODE.version) throw error;
      process.stdout.write('Claude Code ' + version + ' could not be installed (' + (error instanceof Error ? error.message : 'unknown') + '); using ' + DEFAULT_CLAUDE_CODE.version + '\n');
      version = DEFAULT_CLAUDE_CODE.version; await installClaudeCode(root, version);
    }
    process.stdout.write('Installed Claude Code ' + version + '\n');
  } catch (error) { process.stderr.write('Claude Code install failed: ' + (error instanceof Error ? error.message : 'unknown error') + '\n'); process.exitCode = 1; }
}
