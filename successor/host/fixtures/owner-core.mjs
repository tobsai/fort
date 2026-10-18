import { existsSync, renameSync, writeFileSync } from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';
import { pathToFileURL } from 'node:url';
const [directory, corePath, channelPath, mode, setupBehavior, enrollmentMode] = process.argv.slice(2);
if (!directory || !corePath || !channelPath || !['initial', 'expired', 'resume'].includes(mode)) throw new Error('Select exact synthetic owner inputs');
const { createApplication } = await import(pathToFileURL(corePath).href);
const { servePrivateChannel } = await import(pathToFileURL(channelPath).href);
let disposals = 0, recoveries = 0;
const service = createApplication({ dataDirectory: directory, autoRun: true, ownerPairingPolicy: { expiresAfterDays: 1 },
  now: () => Date.parse(mode === 'initial' ? '2026-10-18T00:00:00Z' : '2026-10-20T00:00:00Z'),
  provider: { provider: 'owner-pipe-fixture', name: 'Owner pipe fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
  runtime: { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {
    if (setupBehavior === 'recovery-fail-once' && ++disposals === 1) throw new Error('Synthetic stop failure');
  } },
});
const lifetime = setTimeout(() => process.stdin.destroy(new Error('Synthetic deadline')), 15_000);
try {
  await service.start({ listener: { kind: 'privateSocket', name: 'core.sock' }, resumeOwnerHost: mode === 'resume' });
  writeFileSync('core-pid.json', JSON.stringify(process.pid), { mode: 0o600 });
  writeFileSync('setup.json', JSON.stringify({ installation: service.installation, ...(enrollmentMode !== 'no-enrollment' && mode !== 'expired' && service.inspectSession().deviceIDs.length === 0 ? { enrollment: service.issueEnrollment() } : {}) }), { mode: 0o600 });
  await servePrivateChannel({ endpoint: service.endpoint, input: process.stdin, output: process.stdout, ownerRecovery: async identity => {
    writeFileSync('recovery-started.json.tmp', JSON.stringify(++recoveries), { mode: 0o600 });
    renameSync('recovery-started.json.tmp', 'recovery-started.json');
    if (setupBehavior === 'recovery-held') while (!existsSync('release-recovery')) await delay(5);
    return service.recoverOwnerPairing(identity);
  },
    ownerSetup: { begin: service.beginOwnerSetup, complete: async request => {
      const receipt = service.completeOwnerSetup(request);
      if (setupBehavior === 'slow') { writeFileSync('setup-committed.json', 'true', { mode: 0o600 }); await delay(100); }
      if (setupBehavior === 'deadline-held') {
        writeFileSync('setup-committed.json', 'true', { mode: 0o600 });
        while (!existsSync('release-setup-response')) await delay(5);
      }
      return receipt;
    }, cancel: async identity => {
      const proof = await service.cancelOwnerSetup(identity);
      if (setupBehavior === 'cancel-slow') { writeFileSync('cancellation-committed.json', 'true', { mode: 0o600 }); await delay(100); }
      return proof;
    } }
  });
} catch { process.exitCode = 1; }
finally {
  clearTimeout(lifetime); await service.close();
  if (setupBehavior === 'recovery-residue') writeFileSync('core.sock', 'synthetic residue', { mode: 0o600 });
  if (setupBehavior === 'recovery-unclean') process.exitCode = 1;
}
if (process.exitCode === 1) process.exit(1);
