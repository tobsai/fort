import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';
const [mode] = process.argv.slice(2);
writeFileSync('fixture-pid.json', JSON.stringify(process.pid), { mode: 0o600 });
const lifetime = setTimeout(() => process.exit(3), 20_000);
const emit = value => process.stdout.write(JSON.stringify(value) + '\n');
if (mode !== 'no-ready') emit({ protocolVersion: 1, kind: 'ready' });
if (mode === 'hang') {
  process.on('SIGTERM', () => {});
  const child = spawn(process.execPath, ['--input-type=module', '-e', 'process.on("SIGTERM",()=>{});setInterval(()=>{},1000);'], { stdio: ['ignore', 'inherit', 'inherit'], env: {} });
  writeFileSync('descendant-pid.json', JSON.stringify(child.pid), { mode: 0o600 });
}
if (mode !== 'blocked-input') {
  process.stdin.setEncoding('utf8');
  let pending = '';
  let held = 0;
  process.stdin.on('data', chunk => {
    pending += chunk;
    for (let newline; (newline = pending.indexOf('\n')) >= 0;) {
      const frame = JSON.parse(pending.slice(0, newline)); pending = pending.slice(newline + 1);
      if (mode === 'gate' && frame.kind === 'cancel') { emit({ protocolVersion: 1, kind: 'error', id: frame.id, error: 'canceled' }); continue; }
      if (frame.kind === 'cancel' || mode === 'ignore-cancel' || mode === 'hang') continue;
      const response = { protocolVersion: 1, kind: 'response', id: frame.id, response: { status: 200, body: Buffer.from('exact').toString('base64') } };
      if (mode.startsWith('setup-proof-')) {
        const proof = { installationId: mode === 'setup-proof-installation' ? 'foreign' : frame.setup.installationId,
          deviceId: mode === 'setup-proof-device' ? 'foreign' : frame.setup.deviceId,
          expiresAt: mode === 'setup-proof-expiry' ? 'invalid' : null };
        if (mode === 'setup-proof-missing-expiry') delete proof.expiresAt;
        emit({ ...response, response: { status: mode === 'setup-proof-status' ? 503 : 200, body: Buffer.from(JSON.stringify(proof)).toString('base64') } });
      }
      else if (mode.startsWith('setup-offer-')) {
        const offer = { kind: mode === 'setup-offer-kind' ? 'unknown' : 'available',
          installationId: mode === 'setup-offer-installation' ? 'wrong/path' : 'installation',
          setupId: mode === 'setup-offer-id' ? '' : 'setup', expiresAt: mode === 'setup-offer-expiry' ? '2026-10-18T00:00:00Z' : '2026-10-18T00:00:00.000Z' };
        emit({ ...response, response: { status: 200, body: Buffer.from(JSON.stringify(offer)).toString('base64') } });
      }
      else if (mode.startsWith('recovery-proof-')) {
        const proof = { installationId: mode === 'recovery-proof-installation' ? 'foreign' : frame.recovery.installationID,
          deviceId: mode === 'recovery-proof-device' ? 'foreign' : frame.recovery.deviceID,
          runtimeStopped: mode !== 'recovery-proof-runtime', revoked: mode !== 'recovery-proof-revocation' };
        emit({ ...response, response: { status: mode === 'recovery-proof-status' ? 503 : 200, body: Buffer.from(JSON.stringify(proof)).toString('base64') } });
      }
      else if (mode === 'gate' && frame.request.path === '/v1/hold') { writeFileSync('held.json', JSON.stringify(++held), { mode: 0o600 }); }
      else if (mode === 'out-of-order') setTimeout(() => emit({ ...response, response: { status: 200, body: frame.request.body } }), (9 - frame.id) * 10);
      else if (mode === 'malformed') process.stdout.write('not-json\n');
      else if (mode === 'oversized-frame') process.stdout.write(Buffer.alloc(12 * 1024 * 1024 + 1, 65));
      else if (mode === 'oversized-body') emit({ ...response, response: { status: 200, body: Buffer.alloc(8 * 1024 * 1024 + 1, 65).toString('base64') } });
      else if (mode === 'wrong-id') emit({ ...response, id: frame.id + 1 });
      else if (mode === 'wrong-version') emit({ ...response, protocolVersion: 2 });
      else if (mode === 'invalid-status') emit({ ...response, response: { status: 999, body: '' } });
      else if (mode === 'duplicate') { emit(response); emit(response); }
      else if (mode === 'exit') process.exit(2);
      else emit(response);
    }
  });
  process.stdin.on('end', () => { if (mode !== 'hang') { clearTimeout(lifetime); process.exit(0); } });
}
process.stdout.on('error', () => { if (mode !== 'hang') process.exit(2); });
