import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
let input = '';
for await (const chunk of process.stdin) input += chunk;
const probe = JSON.parse(input);
const result = { environmentCanaryAbsent: process.env.FORT_UNKNOWN_SECRET_CANARY === undefined, reads: [], networkDenied: false, childDenied: false, inheritedDescriptorDenied: false };
for (const path of probe.paths) {
  try { readFileSync(path); result.reads.push(false); }
  catch (error) { result.reads.push(error.code === 'ERR_ACCESS_DENIED'); }
}
try { await fetch(probe.serviceURL); } catch { result.networkDenied = true; }
try { spawnSync('/usr/bin/true'); } catch (error) { result.childDenied = error.code === 'ERR_ACCESS_DENIED'; }
try { readFileSync(3); } catch { result.inheritedDescriptorDenied = true; }
process.stdout.write(JSON.stringify(result));
