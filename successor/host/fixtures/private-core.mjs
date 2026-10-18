import { setTimeout as delay } from 'node:timers/promises';
const [directory, coreModule, channelModule, mode, recoveryBehavior] = process.argv.slice(2);
if (!directory || !coreModule || !channelModule) throw new Error('Select exact synthetic inputs');
const { createApplication } = await import(coreModule);
const { servePrivateChannel } = await import(channelModule);
const service = createApplication({ dataDirectory: directory, autoRun: true,
  ...(mode === 'owner' ? { ownerPairingPolicy: { expiresAfterDays: null } } : {}),
  provider: { provider: 'pipe-fixture', name: 'Pipe fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
  runtime: { async execute(input) { process.send?.({ kind: 'executed' }); return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() { if (recoveryBehavior === 'slow') await delay(100); } }
});
const lifetime = setTimeout(() => { process.stdin.destroy(new Error('Synthetic deadline')); }, 15_000);
try {
  await service.start({ listener: { kind: 'privateSocket', name: 'core.sock' } });
  const enrollment = mode === 'owner' && service.inspectSession().deviceIDs.length ? undefined : service.issueEnrollment();
  process.send?.({ kind: 'setup', installation: service.installation, enrollment });
  await servePrivateChannel({ endpoint: service.endpoint, input: process.stdin, output: process.stdout, ownerSetup: mode === 'owner' ? {
    begin: () => recoveryBehavior === 'setup-bad-offer' ? { kind: 'available', installationId: service.installation.installationId, setupId: 'synthetic', expiresAt: 'invalid' } : service.beginOwnerSetup(),
    complete: async request => {
      if (recoveryBehavior === 'setup-throws') throw new Error('synthetic-secret');
      const receipt = service.completeOwnerSetup(request);
      if (recoveryBehavior === 'setup-slow') await delay(100);
      if (recoveryBehavior === 'setup-wrong') return { ...receipt, deviceId: 'wrong-device' };
      return { ...receipt, unexpectedSecret: 'synthetic-secret' };
    },
  } : undefined, ownerRecovery: mode === 'owner' ? async identity => {
    const proof = await service.recoverOwnerPairing(identity);
    return recoveryBehavior === 'wrong-proof' ? { ...proof, deviceId: 'another-device' } : proof;
  } : undefined });
} catch { process.exitCode = 1; }
finally { clearTimeout(lifetime); await service.close(); process.disconnect?.(); }
if (process.exitCode === 1) process.exit(1);
