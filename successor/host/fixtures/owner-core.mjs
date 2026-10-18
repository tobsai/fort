import { writeFileSync } from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';
import { pathToFileURL } from 'node:url';
const [directory, corePath, channelPath, mode, setupBehavior, enrollmentMode] = process.argv.slice(2);
if (!directory || !corePath || !channelPath || !['initial', 'expired', 'resume'].includes(mode)) throw new Error('Select exact synthetic owner inputs');
const { createApplication } = await import(pathToFileURL(corePath).href);
const { servePrivateChannel } = await import(pathToFileURL(channelPath).href);
const service = createApplication({ dataDirectory: directory, autoRun: true, ownerPairingPolicy: { expiresAfterDays: 1 },
  now: () => Date.parse(mode === 'initial' ? '2026-10-18T00:00:00Z' : '2026-10-20T00:00:00Z'),
  provider: { provider: 'owner-pipe-fixture', name: 'Owner pipe fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
  runtime: { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} },
});
const lifetime = setTimeout(() => process.stdin.destroy(new Error('Synthetic deadline')), 15_000);
try {
  await service.start({ listener: { kind: 'privateSocket', name: 'core.sock' }, resumeOwnerHost: mode === 'resume' });
  writeFileSync('setup.json', JSON.stringify({ installation: service.installation, ...(enrollmentMode !== 'no-enrollment' && mode !== 'expired' && service.inspectSession().deviceIDs.length === 0 ? { enrollment: service.issueEnrollment() } : {}) }), { mode: 0o600 });
  await servePrivateChannel({ endpoint: service.endpoint, input: process.stdin, output: process.stdout, ownerRecovery: service.recoverOwnerPairing,
    ownerSetup: { begin: service.beginOwnerSetup, complete: async request => {
      const receipt = service.completeOwnerSetup(request);
      if (setupBehavior === 'slow') { writeFileSync('setup-committed.json', 'true', { mode: 0o600 }); await delay(100); }
      return receipt;
    }, cancel: async identity => {
      const proof = await service.cancelOwnerSetup(identity);
      if (setupBehavior === 'cancel-slow') { writeFileSync('cancellation-committed.json', 'true', { mode: 0o600 }); await delay(100); }
      return proof;
    } }
  });
} catch { process.exitCode = 1; }
finally { clearTimeout(lifetime); await service.close(); }
if (process.exitCode === 1) process.exit(1);
