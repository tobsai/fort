import { closeSync, readFileSync, writeSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { INSPECTION_ENABLED } from '../utm-execution-v1/inspection-verifier.mjs';

if (!INSPECTION_ENABLED || process.argv.length !== 3 || process.argv[2] !== 'registry') process.exit(78);
const environment = { PATH: '/usr/bin:/bin', LANG: 'C', LC_ALL: 'C', TZ: 'UTC' };
if (Object.keys(process.env).length !== 4 || Object.entries(environment).some(([key, value]) => process.env[key] !== value)) {
  throw new Error('Unexpected environment; values omitted');
}
const own = fileURLToPath(import.meta.url);
const denied = fileURLToPath(new URL('./denied-probe.txt', import.meta.url));
const permissions = {
  read_allowed: readFileSync(own).length > 0,
  read_denied: false,
  global_read: process.permission.has('fs.read'),
  write: process.permission.has('fs.write'),
  child: process.permission.has('child'),
  worker: process.permission.has('worker'),
};
try { readFileSync(denied); } catch (error) {
  if (error.code !== 'ERR_ACCESS_DENIED') throw new Error('Unexpected permission result');
  permissions.read_denied = true;
}
writeSync(1, JSON.stringify({ case: 'registry', runtime: 'node', environment, launcher_keys: [],
  runtime_identity: process.version + '@' + process.execPath, permissions }) + '\n');
closeSync(1); closeSync(2);
setInterval(() => {}, 1000);
